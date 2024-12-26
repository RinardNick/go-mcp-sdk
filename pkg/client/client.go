package client

import (
	"context"
	"io"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

// Client represents an MCP client interface
type Client interface {
	// ListResources lists available resources from the server
	ListResources(ctx context.Context) ([]types.Resource, error)

	// ListTools lists available tools from the server
	ListTools(ctx context.Context) ([]types.Tool, error)

	// ExecuteTool executes a tool on the server
	ExecuteTool(ctx context.Context, call types.ToolCall) (*types.ToolResult, error)

	// SendRequest sends a raw JSON-RPC request
	SendRequest(ctx context.Context, method string, params interface{}) (*types.Response, error)

	// SendNotification sends a JSON-RPC notification (no response expected)
	SendNotification(ctx context.Context, method string, params interface{}) error

	// SendBatchRequest sends multiple JSON-RPC requests as a batch
	SendBatchRequest(ctx context.Context, methods []string, params []interface{}) ([]types.Response, error)

	// Close closes the client connection
	Close() error
}

// Session represents an MCP client session
type Session struct {
	reader io.Reader
	writer io.Writer
	client Client
	tools  []types.Tool
}

// NewSession creates a new MCP client session
func NewSession(reader io.Reader, writer io.Writer, client Client) (*Session, error) {
	session := &Session{
		reader: reader,
		writer: writer,
		client: client,
	}

	// Initialize tools
	tools, err := client.ListTools(context.Background())
	if err != nil {
		return nil, err
	}
	session.tools = tools

	return session, nil
}

// ListResources lists available resources from the server
func (s *Session) ListResources(ctx context.Context) ([]types.Resource, error) {
	return s.client.ListResources(ctx)
}

// ExecuteTool executes a tool on the server
func (s *Session) ExecuteTool(ctx context.Context, call types.ToolCall) (*types.ToolResult, error) {
	return s.client.ExecuteTool(ctx, call)
}

// GetTools returns the available tools for this session
func (s *Session) GetTools() []types.Tool {
	return s.tools
}

// Close closes the session
func (s *Session) Close() error {
	return s.client.Close()
}
