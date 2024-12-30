package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

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
}

// DefaultConfig returns a default server configuration
func DefaultConfig() *Config {
	return &Config{
		MaxSessions:      100,
		EnableValidation: true,
		LogLevel:         "info",
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
}

// BaseServer provides a basic implementation of the Server interface
type BaseServer struct {
	tools     map[string]types.Tool
	resources map[string]types.Resource
	handlers  map[string]ToolHandler
	options   *InitializationOptions
	config    *Config // Internal config parsed from options
	mu        sync.RWMutex
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
	// Validate initialization params size
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal initialization params: %w", err)
	}
	if err := validateMessageSize(paramsBytes); err != nil {
		return fmt.Errorf("initialization params validation failed: %w", err)
	}
	if !json.Valid(paramsBytes) {
		return fmt.Errorf("invalid JSON in initialization params")
	}

	// Validate protocol version format
	if !isValidVersion(params.ProtocolVersion) {
		return fmt.Errorf("invalid protocol version format: %s", params.ProtocolVersion)
	}

	// Validate client info
	if params.ClientInfo.Name == "" {
		return fmt.Errorf("client name cannot be empty")
	}
	if params.ClientInfo.Version == "" {
		return fmt.Errorf("client version cannot be empty")
	}

	return nil
}

// HandleInitialize handles an initialization request
func (s *BaseServer) HandleInitialize(ctx context.Context, params types.InitializeParams) (*types.InitializeResult, error) {
	// Validate the initialization params
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initialization params: %w", err)
	}
	if err := s.validateMessage(paramsBytes); err != nil {
		return nil, fmt.Errorf("invalid initialization params: %w", err)
	}

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
		return nil, types.NewMCPError(types.ErrInvalidParams, "Invalid params", errMsg)
	}

	// Validate other initialization params
	if err := s.validateInitializeParams(params); err != nil {
		return nil, types.InvalidParamsError(err.Error())
	}

	return s.createInitializeResult(params.ProtocolVersion), nil
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
	// Defer panic recovery
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tool execution panic: %v", r)
			result = nil
		}
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Validate the tool call message
	callBytes, err := json.Marshal(call)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool call: %w", err)
	}
	if err := s.validateMessage(callBytes); err != nil {
		return nil, fmt.Errorf("invalid tool call message: %w", err)
	}

	// Validate tool call
	if err := s.validateToolCall(call); err != nil {
		return nil, fmt.Errorf("tool call validation failed: %w", err)
	}

	// Find tool
	tool, ok := s.tools[call.Name]
	if !ok {
		return nil, types.InvalidParamsError(fmt.Sprintf("tool not found: %s", call.Name))
	}

	// Find handler
	handler, ok := s.handlers[call.Name]
	if !ok {
		return nil, types.InvalidParamsError(fmt.Sprintf("no handler registered for tool: %s", call.Name))
	}

	// Validate parameters against schema if validation is enabled
	if s.config.EnableValidation && tool.InputSchema != nil {
		var schema map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool schema: %w", err)
		}

		if err := validation.ValidateParameters(call.Parameters, schema); err != nil {
			return nil, fmt.Errorf("invalid tool parameters: %w", err)
		}
	}

	// Execute tool with panic recovery
	result, err = handler(ctx, call.Parameters)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return result, nil
}

// GetInitializationOptions returns the server initialization options
func (s *BaseServer) GetInitializationOptions() *InitializationOptions {
	return s.options
}

// Start starts the server with the given configuration
func (s *BaseServer) Start(ctx context.Context) error {
	return nil // Base implementation does nothing
}

// Stop gracefully stops the server
func (s *BaseServer) Stop(ctx context.Context) error {
	return nil // Base implementation does nothing
}
