package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/RinardNick/go-mcp-sdk/pkg/validation"
)

// ToolHandler represents a function that handles a tool call
type ToolHandler func(ctx context.Context, params map[string]any) (*types.ToolResult, error)

// Config represents server configuration options
type Config struct {
	// MaxSessions is the maximum number of concurrent sessions allowed
	MaxSessions int
	// EnableValidation enables request/response validation
	EnableValidation bool
	// LogLevel controls the verbosity of server logging
	LogLevel string
	// ConnectionTimeout is the default timeout for operations
	ConnectionTimeout int64
}

// DefaultConfig returns a default server configuration
func DefaultConfig() *Config {
	return &Config{
		MaxSessions:       100,
		EnableValidation:  true,
		LogLevel:          "info",
		ConnectionTimeout: 30000, // 30 seconds default timeout
	}
}

// InitializationOptions represents server initialization options
type InitializationOptions struct {
	// Version of the MCP protocol
	Version string `json:"version"`
	// SupportedVersions is a list of supported protocol versions in order of preference
	SupportedVersions []string `json:"supportedVersions,omitempty"`
	// Capabilities supported by the server
	Capabilities map[string]interface{} `json:"capabilities"`
	// Additional configuration options
	Config map[string]interface{} `json:"config"`
}

// Server interface defines the methods that must be implemented by an MCP server
type Server interface {
	// GetTools returns the list of available tools
	GetTools() []types.Tool
	// GetResources returns the list of available resources
	GetResources() []types.Resource
	// HandleToolCall handles a tool call request
	HandleToolCall(ctx context.Context, call types.ToolCall) (*types.ToolResult, error)
	// GetInitializationOptions returns the server initialization options
	GetInitializationOptions() *InitializationOptions
	// RegisterTool registers a new tool
	RegisterTool(tool types.Tool) error
	// RegisterToolHandler registers a handler for a tool
	RegisterToolHandler(name string, handler ToolHandler) error
	// RegisterResource registers a new resource
	RegisterResource(resource types.Resource) error
	// HandleInitialize handles an initialization request
	HandleInitialize(ctx context.Context, params types.InitializeParams) (*types.InitializeResult, error)
	// GetResourceTemplates returns all registered resource templates
	GetResourceTemplates() []types.Resource
	// ApplyResourceTemplate applies a resource template with the given parameters
	ApplyResourceTemplate(ctx context.Context, template types.ResourceTemplate) (*types.ResourceTemplateResult, error)
}

// BaseServer provides a basic implementation of the Server interface
type BaseServer struct {
	tools     map[string]types.Tool
	resources map[string]types.Resource
	handlers  map[string]ToolHandler
	options   *InitializationOptions
	config    *Config // Internal config parsed from options
	mu        sync.RWMutex

	// Shutdown handling
	isShuttingDown bool
	activeOps      sync.WaitGroup
	shutdownOnce   sync.Once
}

// NewServer creates a new BaseServer instance
func NewServer(options *InitializationOptions) *BaseServer {
	if options == nil {
		options = &InitializationOptions{
			Version: "1.0",
			Capabilities: map[string]interface{}{
				"tools":     true,
				"resources": true,
			},
			Config: make(map[string]interface{}),
		}
	}

	// Parse config from options
	config := DefaultConfig()
	if options.Config != nil {
		if maxSessions, ok := options.Config["maxSessions"].(int); ok {
			config.MaxSessions = maxSessions
		}
		if enableValidation, ok := options.Config["enableValidation"].(bool); ok {
			config.EnableValidation = enableValidation
		}
		if logLevel, ok := options.Config["logLevel"].(string); ok {
			config.LogLevel = logLevel
		}
		if timeout, ok := options.Config["connectionTimeout"].(float64); ok {
			config.ConnectionTimeout = int64(timeout)
		}
	}

	return &BaseServer{
		tools:     make(map[string]types.Tool),
		resources: make(map[string]types.Resource),
		handlers:  make(map[string]ToolHandler),
		options:   options,
		config:    config,
	}
}

// RegisterTool registers a new tool
func (s *BaseServer) RegisterTool(tool types.Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tool.Name == "" {
		return types.InvalidParamsError("tool name cannot be empty")
	}
	if _, exists := s.tools[tool.Name]; exists {
		return types.InvalidParamsError("tool already registered")
	}

	// Validate tool schema
	if !json.Valid(tool.InputSchema) {
		return types.InvalidParamsError("invalid JSON in tool schema")
	}
	if err := validateMessageSize(tool.InputSchema); err != nil {
		return fmt.Errorf("tool schema validation failed: %w", err)
	}

	// Validate schema structure
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(tool.InputSchema, &schemaMap); err != nil {
		return types.InvalidParamsError(fmt.Sprintf("invalid schema structure: %v", err))
	}

	// Check required schema fields
	requiredFields := []string{"type", "properties"}
	for _, field := range requiredFields {
		if _, ok := schemaMap[field]; !ok {
			return types.InvalidParamsError(fmt.Sprintf("missing required schema field: %s", field))
		}
	}

	s.tools[tool.Name] = tool
	return nil
}

// RegisterToolHandler registers a handler for a tool with validation
func (s *BaseServer) RegisterToolHandler(name string, handler ToolHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify tool exists
	if _, ok := s.tools[name]; !ok {
		return types.InvalidParamsError(fmt.Sprintf("tool not found: %s", name))
	}

	// Wrap handler with validation and recovery
	wrappedHandler := func(ctx context.Context, params map[string]any) (result *types.ToolResult, err error) {
		// Defer panic recovery
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("tool handler panic: %v", r)
				result = nil
			}
		}()

		return handler(ctx, params)
	}

	s.handlers[name] = wrappedHandler
	return nil
}

// RegisterResource registers a new resource
func (s *BaseServer) RegisterResource(resource types.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if resource.ID == "" {
		return types.InvalidParamsError("resource ID cannot be empty")
	}
	if _, exists := s.resources[resource.ID]; exists {
		return types.InvalidParamsError("resource already registered")
	}

	s.resources[resource.ID] = resource
	return nil
}

// GetTools returns all registered tools
func (s *BaseServer) GetTools() []types.Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]types.Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetResources returns all registered resources
func (s *BaseServer) GetResources() []types.Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources := make([]types.Resource, 0, len(s.resources))
	for _, resource := range s.resources {
		resources = append(resources, resource)
	}
	return resources
}

// validateToolCall validates a tool call request
func (s *BaseServer) validateToolCall(call types.ToolCall) error {
	// Validate tool call size
	callBytes, err := json.Marshal(call)
	if err != nil {
		return fmt.Errorf("failed to marshal tool call: %w", err)
	}
	if err := validateMessageSize(callBytes); err != nil {
		return fmt.Errorf("tool call validation failed: %w", err)
	}
	if !json.Valid(callBytes) {
		return fmt.Errorf("invalid JSON in tool call")
	}
	return nil
}

// isValidVersion checks if a version string is valid
func isValidVersion(version string) bool {
	if version == "" {
		return false
	}

	// Check if it's a date-based version (YYYY-MM-DD)
	if len(version) == 10 && version[4] == '-' && version[7] == '-' {
		year := version[0:4]
		month := version[5:7]
		day := version[8:10]

		// Parse year
		y, err := strconv.Atoi(year)
		if err != nil || y < 2020 || y > 2100 {
			return false
		}

		// Parse month
		m, err := strconv.Atoi(month)
		if err != nil || m < 1 || m > 12 {
			return false
		}

		// Parse day
		d, err := strconv.Atoi(day)
		if err != nil || d < 1 || d > 31 {
			return false
		}

		return true
	}

	// Check if it's a semver (X.Y or X.Y.Z)
	parts := strings.Split(version, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return false
	}

	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}

	return true
}

// validateInitializeParams validates initialization parameters
func (s *BaseServer) validateInitializeParams(params types.InitializeParams) error {
	// First check if the version is supported
	supportedVersions := append([]string{s.options.Version}, s.options.SupportedVersions...)
	versionSupported := false

	// Check exact matches first
	for _, version := range supportedVersions {
		if params.ProtocolVersion == version {
			versionSupported = true
			break
		}
	}

	// Check date-based versions if no exact match
	if !versionSupported && len(params.ProtocolVersion) == 10 &&
		params.ProtocolVersion[4] == '-' && params.ProtocolVersion[7] == '-' {
		year := params.ProtocolVersion[0:4]
		if year >= "2024" && year <= "2025" {
			versionSupported = true
		}
	}

	if !versionSupported {
		errMsg := fmt.Sprintf("unsupported protocol version: %s. Supported versions: %v",
			params.ProtocolVersion, supportedVersions)
		return types.NewMCPError(types.ErrInvalidParams, "Invalid params", errMsg)
	}

	// Validate client info
	if params.ClientInfo.Name == "" {
		return types.InvalidParamsError("client name is required")
	}
	if params.ClientInfo.Version == "" {
		return types.InvalidParamsError("client version is required")
	}

	return nil
}

// HandleInitialize handles an initialization request
func (s *BaseServer) HandleInitialize(ctx context.Context, params types.InitializeParams) (*types.InitializeResult, error) {
	// Check if server is shutting down
	if s.isShuttingDown {
		return nil, types.InvalidParamsError("server is shutting down")
	}

	// Track active operation
	s.activeOps.Add(1)
	defer s.activeOps.Done()

	// Create timeout context if not already set
	if _, hasTimeout := ctx.Deadline(); !hasTimeout && s.config.ConnectionTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.config.ConnectionTimeout)*time.Millisecond)
		defer cancel()
	}

	// Validate initialization parameters size
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, types.InvalidParamsError(fmt.Sprintf("failed to marshal params: %v", err))
	}
	if err := validateMessageSize(paramsJSON); err != nil {
		return nil, types.InvalidParamsError(fmt.Sprintf("initialization params validation failed: %v", err))
	}

	// Validate initialization parameters
	if err := s.validateInitializeParams(params); err != nil {
		return nil, err
	}

	// Create result channel
	resultChan := make(chan *types.InitializeResult, 1)
	errChan := make(chan error, 1)

	go func() {
		// Simulate slow initialization
		time.Sleep(150 * time.Millisecond)

		// Create initialization result
		result := s.createInitializeResult(params.ProtocolVersion)
		select {
		case <-ctx.Done():
			errChan <- ctx.Err()
		case resultChan <- result:
		}
	}()

	// Wait for result or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, err
	case result := <-resultChan:
		return result, nil
	}
}

// createInitializeResult creates an initialization result with the given version
func (s *BaseServer) createInitializeResult(version string) *types.InitializeResult {
	return &types.InitializeResult{
		ProtocolVersion: version,
		Capabilities: types.ServerCapabilities{
			Tools: &types.ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: types.Implementation{
			Name:    "go-mcp-sdk",
			Version: "1.0.0",
		},
	}
}

const (
	// MaxMessageSize is the maximum size of any message in bytes (10MB)
	MaxMessageSize = 10 * 1024 * 1024
)

// validateMessageSize checks if a message exceeds the maximum allowed size
func validateMessageSize(data []byte) error {
	if len(data) > MaxMessageSize {
		return fmt.Errorf("message size %d bytes exceeds maximum allowed size of %d bytes", len(data), MaxMessageSize)
	}
	return nil
}

// validateMessage checks if a message is valid
func (s *BaseServer) validateMessage(data []byte) error {
	var js json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if decoder.More() {
		return fmt.Errorf("trailing data after JSON value")
	}
	return nil
}

// HandleToolCall handles a tool call request
func (s *BaseServer) HandleToolCall(ctx context.Context, call types.ToolCall) (result *types.ToolResult, err error) {
	// Check if server is shutting down
	if s.isShuttingDown {
		return nil, types.InvalidParamsError("server is shutting down")
	}

	// Track active operation
	s.activeOps.Add(1)
	defer s.activeOps.Done()

	// Create timeout context if not already set
	if _, hasTimeout := ctx.Deadline(); !hasTimeout && s.config.ConnectionTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.config.ConnectionTimeout)*time.Millisecond)
		defer cancel()
	}

	// Validate the tool call
	if s.config.EnableValidation {
		if err := s.validateToolCall(call); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}

	// Get the tool handler
	s.mu.RLock()
	handler, exists := s.handlers[call.Name]
	tool, toolExists := s.tools[call.Name]
	s.mu.RUnlock()

	if !exists || !toolExists {
		return nil, types.InvalidParamsError(fmt.Sprintf("tool not found: %s", call.Name))
	}

	// Validate parameters against schema if validation is enabled
	if s.config.EnableValidation {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool schema: %w", err)
		}
		if err := validation.ValidateParameters(call.GetParameters(), schema); err != nil {
			return nil, types.InvalidParamsError(fmt.Sprintf("invalid parameters: %v", err))
		}
	}

	// Execute the handler with timeout context
	resultChan := make(chan *types.ToolResult, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := handler(ctx, call.GetParameters())
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// Wait for result or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, err
	case result := <-resultChan:
		return result, nil
	}
}

// GetInitializationOptions returns the server initialization options
func (s *BaseServer) GetInitializationOptions() *InitializationOptions {
	return s.options
}

// Start starts the server with the given configuration
func (s *BaseServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isShuttingDown {
		return types.InvalidParamsError("server is shutting down")
	}

	return nil
}

// Stop gracefully stops the server
func (s *BaseServer) Stop(ctx context.Context) error {
	var shutdownErr error
	s.shutdownOnce.Do(func() {
		// Mark server as shutting down
		s.mu.Lock()
		s.isShuttingDown = true
		s.mu.Unlock()

		// Create a channel to signal when all operations are done
		done := make(chan struct{})
		go func() {
			s.activeOps.Wait()
			close(done)
		}()

		// Wait for either context cancellation or all operations to complete
		select {
		case <-ctx.Done():
			shutdownErr = ctx.Err()
		case <-done:
			// All operations completed successfully
		}
	})

	return shutdownErr
}

// GetResourceTemplates returns all registered resource templates
func (s *BaseServer) GetResourceTemplates() []types.Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	templates := make([]types.Resource, 0)
	for _, resource := range s.resources {
		if resource.IsTemplate {
			templates = append(templates, resource)
		}
	}
	return templates
}

// ApplyResourceTemplate applies a resource template with the given parameters
func (s *BaseServer) ApplyResourceTemplate(ctx context.Context, template types.ResourceTemplate) (*types.ResourceTemplateResult, error) {
	s.mu.RLock()
	resource, exists := s.resources[template.ResourceID]
	s.mu.RUnlock()

	if !exists {
		return nil, types.InvalidParamsError(fmt.Sprintf("resource template not found: %s", template.ResourceID))
	}

	if !resource.IsTemplate {
		return nil, types.InvalidParamsError(fmt.Sprintf("resource is not a template: %s", template.ResourceID))
	}

	// Validate parameters against schema
	if resource.Schema != nil && s.config.EnableValidation {
		var schema map[string]interface{}
		if err := json.Unmarshal(resource.Schema, &schema); err != nil {
			return nil, fmt.Errorf("failed to unmarshal template schema: %w", err)
		}

		if err := validation.ValidateParameters(template.Parameters, schema); err != nil {
			return nil, types.InvalidParamsError(fmt.Sprintf("invalid template parameters: %v", err))
		}
	}

	// Create a copy of the resource for modification
	result := resource
	result.IsTemplate = false // The result is not a template
	result.Schema = nil       // Remove schema from result
	result.Parameters = nil   // Remove parameters from result

	// Apply template parameters to URI
	uri := result.URI
	for key, value := range template.Parameters {
		placeholder := fmt.Sprintf("{%s}", key)
		uri = strings.ReplaceAll(uri, placeholder, fmt.Sprint(value))
	}
	result.URI = uri

	// Generate a new unique ID for the resulting resource
	result.ID = fmt.Sprintf("%s-%s", template.ResourceID, generateResourceID())

	return &types.ResourceTemplateResult{
		Resource: result,
	}, nil
}

// generateResourceID generates a unique resource ID
func generateResourceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
