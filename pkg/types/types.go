package types

import (
	"encoding/json"
	"fmt"
)

// ClientCapabilities represents client capabilities
type ClientCapabilities struct {
	Tools *ToolCapabilities `json:"tools"`
}

// ToolCapabilities represents tool-related capabilities
type ToolCapabilities struct {
	SupportsProgress     bool `json:"supportsProgress"`
	SupportsCancellation bool `json:"supportsCancellation"`
}

// RootsCapability represents the roots capability
type RootsCapability struct {
	ListChanged bool `json:"list_changed,omitempty"`
}

// Implementation represents a client or server implementation
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeParams represents client initialization parameters
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ClientInfo      Implementation     `json:"clientInfo"`
	Capabilities    ClientCapabilities `json:"capabilities"`
}

// ClientInfo represents information about the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool represents a tool that can be executed
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	InputSchema json.RawMessage        `json:"input_schema"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewToolInputSchema creates a new json.RawMessage from a map
func NewToolInputSchema(schema map[string]interface{}) (json.RawMessage, error) {
	bytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool schema: %w", err)
	}
	if err := validateMessageSize(bytes); err != nil {
		return nil, fmt.Errorf("tool schema validation failed: %w", err)
	}
	if err := validateJSON(bytes); err != nil {
		return nil, fmt.Errorf("tool schema validation failed: %w", err)
	}
	return json.RawMessage(bytes), nil
}

// ToolCall represents a tool execution request
type ToolCall struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Result json.RawMessage `json:"result"`
	Error  *MCPError       `json:"error,omitempty"`
}

// NewToolResult creates a new ToolResult from a map
func NewToolResult(result map[string]interface{}) (*ToolResult, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool result: %w", err)
	}
	if err := validateMessageSize(bytes); err != nil {
		return nil, fmt.Errorf("tool result validation failed: %w", err)
	}
	if err := validateJSON(bytes); err != nil {
		return nil, fmt.Errorf("tool result validation failed: %w", err)
	}
	if content, ok := result["content"].([]map[string]interface{}); ok {
		for _, item := range content {
			if err := validateRequiredFields(item, []string{"type", "text"}); err != nil {
				return nil, fmt.Errorf("tool result validation failed: %w", err)
			}
		}
	}
	return &ToolResult{
		Result: json.RawMessage(bytes),
	}, nil
}

// Resource represents a resource available to the server
type Resource struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	URI         string                 `json:"uri"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	IsTemplate  bool                   `json:"is_template,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Schema      json.RawMessage        `json:"schema,omitempty"`
}

// ResourceTemplate represents a resource template with parameter values
type ResourceTemplate struct {
	ResourceID string                 `json:"resource_id"`
	Parameters map[string]interface{} `json:"parameters"`
}

// ResourceTemplateResult represents the result of applying a resource template
type ResourceTemplateResult struct {
	Resource Resource  `json:"resource"`
	Error    *MCPError `json:"error,omitempty"`
}

// MCPError represents an error in the MCP protocol
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error codes
const (
	ErrParse          = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603
)

// Error implements the error interface
func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// NewMCPError creates a new MCPError
func NewMCPError(code int, message string, data interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// MethodNotFoundError creates a new method not found error
func MethodNotFoundError(data interface{}) *MCPError {
	return NewMCPError(ErrMethodNotFound, "Method not found", data)
}

// InvalidParamsError creates a new invalid params error
func InvalidParamsError(data interface{}) *MCPError {
	return NewMCPError(ErrInvalidParams, "Invalid params", data)
}

// InternalError creates a new internal error
func InternalError(data interface{}) *MCPError {
	return NewMCPError(ErrInternal, "Internal error", data)
}

// Response represents a JSON-RPC response
type Response struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

// ParseError represents a JSON parsing error
type ParseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewParseError(message string, data interface{}) *MCPError {
	return NewMCPError(ErrParse, message, data)
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("Parse error: %s", e.Message)
}

// Progress represents a progress notification from a tool
type Progress struct {
	ToolID  string `json:"toolID,tool_id"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

// ProgressHandler is a function that handles progress notifications
type ProgressHandler func(Progress)

// Notification represents a notification from the server
type Notification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// ProgressNotification represents a progress notification
type ProgressNotification struct {
	Progress Progress `json:"progress"`
}

// ServerInfo represents information about the server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult represents the result of initialization
type InitializeResult struct {
	ProtocolVersion string             `json:"protocol_version"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"server_info"`
}

// InitializationOptions represents server initialization options
type InitializationOptions struct {
	ServerName    string             `json:"server_name"`
	ServerVersion string             `json:"server_version"`
	Capabilities  ServerCapabilities `json:"capabilities"`
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

// validateJSON checks if a JSON message is well-formed
func validateJSON(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON: %s", string(data))
	}
	return nil
}

// validateRequiredFields checks if a map has all required fields
func validateRequiredFields(data map[string]interface{}, required []string) error {
	for _, field := range required {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	return nil
}

// Message represents a chat message
type Message struct {
	Role     string    `json:"role"`
	Content  string    `json:"content"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

// ServerCapabilities represents the capabilities of the server
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools"`
}

// ToolsCapability represents the tool-related capabilities of the server
type ToolsCapability struct {
	ListChanged bool `json:"list_changed"`
}

// LoggingCapability represents logging capabilities
type LoggingCapability struct{}

// PromptsCapability represents prompt capabilities
type PromptsCapability struct {
	ListChanged bool `json:"list_changed,omitempty"`
}

// ResourcesCapability represents resource capabilities
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"list_changed,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler for Implementation
func (i *Implementation) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Try name
	if name, ok := raw["name"]; ok {
		if err := json.Unmarshal(name, &i.Name); err != nil {
			return err
		}
	}

	// Try version
	if version, ok := raw["version"]; ok {
		if err := json.Unmarshal(version, &i.Version); err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalJSON implements json.Unmarshaler for InitializeParams
func (p *InitializeParams) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Try protocol_version (snake_case first)
	if pv, ok := raw["protocol_version"]; ok {
		if err := json.Unmarshal(pv, &p.ProtocolVersion); err != nil {
			return err
		}
	} else if pv, ok := raw["protocolVersion"]; ok { // Then camelCase
		if err := json.Unmarshal(pv, &p.ProtocolVersion); err != nil {
			return err
		}
	}

	// Try client_info (snake_case first)
	if ci, ok := raw["client_info"]; ok {
		if err := json.Unmarshal(ci, &p.ClientInfo); err != nil {
			return err
		}
	} else if ci, ok := raw["clientInfo"]; ok { // Then camelCase
		if err := json.Unmarshal(ci, &p.ClientInfo); err != nil {
			return err
		}
	}

	// Try capabilities
	if cap, ok := raw["capabilities"]; ok {
		if err := json.Unmarshal(cap, &p.Capabilities); err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalJSON implements json.Unmarshaler for ToolCapabilities
func (tc *ToolCapabilities) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Try supports_progress (snake_case first)
	if sp, ok := raw["supports_progress"]; ok {
		if err := json.Unmarshal(sp, &tc.SupportsProgress); err != nil {
			return err
		}
	} else if sp, ok := raw["supportsProgress"]; ok { // Then camelCase
		if err := json.Unmarshal(sp, &tc.SupportsProgress); err != nil {
			return err
		}
	}

	// Try supports_cancellation (snake_case first)
	if sc, ok := raw["supports_cancellation"]; ok {
		if err := json.Unmarshal(sc, &tc.SupportsCancellation); err != nil {
			return err
		}
	} else if sc, ok := raw["supportsCancellation"]; ok { // Then camelCase
		if err := json.Unmarshal(sc, &tc.SupportsCancellation); err != nil {
			return err
		}
	}

	return nil
}

// UnmarshalJSON implements json.Unmarshaler for Tool
func (t *Tool) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Try name
	if name, ok := raw["name"]; ok {
		if err := json.Unmarshal(name, &t.Name); err != nil {
			return err
		}
	}

	// Try description
	if desc, ok := raw["description"]; ok {
		if err := json.Unmarshal(desc, &t.Description); err != nil {
			return err
		}
	}

	// Try parameters
	if params, ok := raw["parameters"]; ok {
		if err := json.Unmarshal(params, &t.Parameters); err != nil {
			return err
		}
	}

	// Try input_schema (snake_case first)
	if schema, ok := raw["input_schema"]; ok {
		t.InputSchema = schema
	} else if schema, ok := raw["inputSchema"]; ok { // Then camelCase
		t.InputSchema = schema
	}

	// Try metadata
	if meta, ok := raw["metadata"]; ok {
		if err := json.Unmarshal(meta, &t.Metadata); err != nil {
			return err
		}
	}

	return nil
}
