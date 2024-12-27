package client

import (
	"context"
	"io"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

// Client represents an MCP client
type Client interface {
	// Initialize sends the initialization request to the server
	Initialize(ctx context.Context) error

	// ListTools returns a list of available tools
	ListTools(ctx context.Context) ([]types.Tool, error)

	// ExecuteTool executes a tool with the given parameters
	ExecuteTool(ctx context.Context, call types.ToolCall) (*types.ToolResult, error)

	// ListResources returns a list of available resources
	ListResources(ctx context.Context) ([]types.Resource, error)

	// Close shuts down the client and cleans up resources
	Close() error
}

// Session represents an MCP session
type Session struct {
	reader io.Reader
	writer io.Writer
	client Client
}

// NewSession creates a new session with the given reader, writer, and client
func NewSession(reader io.Reader, writer io.Writer, client Client) (*Session, error) {
	return &Session{
		reader: reader,
		writer: writer,
		client: client,
	}, nil
}

// Run starts the session
func (s *Session) Run(ctx context.Context) error {
	// TODO: Implement session handling
	return nil
}
