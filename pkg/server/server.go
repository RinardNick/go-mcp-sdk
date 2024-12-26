package server

import (
	"context"
	"fmt"
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

	s.tools[tool.Name] = tool
	return nil
}

// RegisterToolHandler registers a handler for a tool
func (s *BaseServer) RegisterToolHandler(name string, handler ToolHandler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return types.InvalidParamsError("tool name cannot be empty")
	}
	if handler == nil {
		return types.InvalidParamsError("handler cannot be nil")
	}
	if _, exists := s.tools[name]; !exists {
		return types.InvalidParamsError("tool not registered")
	}
	if _, exists := s.handlers[name]; exists {
		return types.InvalidParamsError("handler already registered for tool")
	}

	s.handlers[name] = handler
	return nil
}

// RegisterResource registers a new resource
func (s *BaseServer) RegisterResource(resource types.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if resource.URI == "" {
		return types.InvalidParamsError("resource URI cannot be empty")
	}
	if _, exists := s.resources[resource.URI]; exists {
		return types.InvalidParamsError("resource already registered")
	}

	s.resources[resource.URI] = resource
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

// HandleToolCall handles a tool call request
func (s *BaseServer) HandleToolCall(ctx context.Context, call types.ToolCall) (*types.ToolResult, error) {
	s.mu.RLock()
	tool, exists := s.tools[call.Name]
	handler := s.handlers[call.Name]
	s.mu.RUnlock()

	if !exists {
		return nil, types.MethodNotFoundError("tool not found")
	}

	if handler == nil {
		return nil, types.MethodNotFoundError(fmt.Sprintf("no handler registered for tool: %s", call.Name))
	}

	if s.config.EnableValidation {
		if err := validation.ValidateParameters(call.Parameters, tool.Parameters); err != nil {
			return nil, types.InvalidParamsError(err.Error())
		}
	}

	return handler(ctx, call.Parameters)
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
