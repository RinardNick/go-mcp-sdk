package client

import (
	"context"
	"fmt"
	"io"
	"os"

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

	// GetResourceTemplates returns a list of available resource templates
	GetResourceTemplates(ctx context.Context) ([]types.Resource, error)

	// ApplyResourceTemplate applies a resource template with the given parameters
	ApplyResourceTemplate(ctx context.Context, template types.ResourceTemplate) (*types.ResourceTemplateResult, error)

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
	session := &Session{
		reader: reader,
		writer: writer,
		client: client,
	}

	// Initialize the client
	ctx := context.Background()
	if err := client.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	return session, nil
}

// Run starts the session
func (s *Session) Run(ctx context.Context) error {
	// TODO: Implement session handling
	return nil
}

// GetTools returns a list of available tools
func (s *Session) GetTools() []types.Tool {
	ctx := context.Background()
	tools, err := s.client.ListTools(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to list tools: %v\n", err)
		return nil
	}
	return tools
}

// ExecuteTool executes a tool with the given parameters
func (s *Session) ExecuteTool(ctx context.Context, call types.ToolCall) (*types.ToolResult, error) {
	return s.client.ExecuteTool(ctx, call)
}

// Close shuts down the session and cleans up resources
func (s *Session) Close() error {
	return s.client.Close()
}
