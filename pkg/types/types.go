package types

import (
	"encoding/json"
	"fmt"
)

// ClientCapabilities represents the capabilities of the client
type ClientCapabilities struct {
	Experimental map[string]interface{} `json:"experimental,omitempty"`
	Sampling     map[string]interface{} `json:"sampling,omitempty"`
	Roots        *RootsCapability       `json:"roots,omitempty"`
	Tools        *ToolCapabilities      `json:"tools,omitempty"`
}

// ToolCapabilities represents the tool-related capabilities
type ToolCapabilities struct {
	SupportsProgress     bool `json:"supportsProgress,omitempty"`
	SupportsCancellation bool `json:"supportsCancellation,omitempty"`
}

// RootsCapability represents the roots capability
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeParams represents the parameters for initialization
type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      ClientInfo         `json:"clientInfo"`
}

// ClientInfo represents information about the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tool represents a tool that can be executed
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// NewToolInputSchema creates a new json.RawMessage from a map
func NewToolInputSchema(schema map[string]interface{}) (json.RawMessage, error) {
	bytes, err := json.Marshal(schema)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	return &ToolResult{
		Result: json.RawMessage(bytes),
	}, nil
}

// Resource represents an MCP resource
type Resource struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
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
