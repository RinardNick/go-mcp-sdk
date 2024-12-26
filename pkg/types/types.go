package types

import (
	"encoding/json"
)

// Resource represents an MCP resource
type Resource struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// Tool represents a callable tool provided by the server
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall represents a request to execute a tool
type ToolCall struct {
	Name       string         `json:"name"`
	Parameters map[string]any `json:"parameters"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Result any       `json:"result"`
	Error  *MCPError `json:"error,omitempty"`
}

// Message represents a message in the conversation
type Message struct {
	Role     string    `json:"role"`
	Content  string    `json:"content"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
}

// InitializationOptions represents options for initializing an MCP client or server
type InitializationOptions struct {
	// Add any initialization options here
}

// MCPError represents an error in the MCP protocol
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Response represents a JSON-RPC response
type Response struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	ID      int64           `json:"id"`
}

func (e *MCPError) Error() string {
	return e.Message
}

// Standard error codes
const (
	ErrParseError     = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603
)

// NewMCPError creates a new MCP error with the given code and message
func NewMCPError(code int, message string, data interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// ParseError creates a new parse error
func ParseError(data interface{}) *MCPError {
	return NewMCPError(ErrParseError, "Parse error", data)
}

// InvalidRequestError creates a new invalid request error
func InvalidRequestError(data interface{}) *MCPError {
	return NewMCPError(ErrInvalidRequest, "Invalid request", data)
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
