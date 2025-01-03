package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

// MockServer represents a mock MCP server for testing
type MockServer struct {
	t           *testing.T
	methodCalls map[string]int
}

// NewMockServer creates a new mock server for testing
func NewMockServer(t *testing.T) *MockServer {
	return &MockServer{
		t:           t,
		methodCalls: make(map[string]int),
	}
}

// HandleInitialize handles the initialize request
func (s *MockServer) HandleInitialize(req json.RawMessage) (json.RawMessage, error) {
	initResult := types.InitializeResult{
		ProtocolVersion: "1.0",
		Capabilities: types.ServerCapabilities{
			Tools: &types.ToolsCapability{
				ListChanged: false,
			},
		},
		ServerInfo: types.Implementation{
			Name:    "test-server",
			Version: "1.0",
		},
	}
	resultBytes, err := json.Marshal(initResult)
	require.NoError(s.t, err)
	return resultBytes, nil
}

// HandleRequest handles a request and returns a response
func (s *MockServer) HandleRequest(method string, params json.RawMessage) (json.RawMessage, error) {
	s.methodCalls[method]++

	switch method {
	case "tools/list":
		return nil, types.MethodNotFoundError("Method not found: tools/list")

	case "mcp/list_tools":
		tools := []types.Tool{
			{
				Name:        "test-tool",
				Description: "A test tool",
				InputSchema: []byte("{}"),
			},
		}
		return json.Marshal(tools)

	case "tools/call":
		var callParams struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		err := json.Unmarshal(params, &callParams)
		require.NoError(s.t, err)
		require.Equal(s.t, "test-tool", callParams.Name)

		result := map[string]interface{}{
			"result": "success",
		}
		return json.Marshal(result)

	case "mcp/call_tool":
		s.t.Error("mcp/call_tool was called when it shouldn't have been")
		return nil, types.MethodNotFoundError("Method not found: mcp/call_tool")

	default:
		return nil, types.MethodNotFoundError("Method not found: " + method)
	}
}

// GetMethodCalls returns the number of times a method was called
func (s *MockServer) GetMethodCalls(method string) int {
	return s.methodCalls[method]
}

// TestMethodNameCompatibilityWithClient runs the method name compatibility tests with the given client
func TestMethodNameCompatibilityWithClient(t *testing.T, client interface {
	Initialize(context.Context) error
	ListTools(context.Context) ([]types.Tool, error)
	ExecuteTool(context.Context, types.ToolCall) (*types.ToolResult, error)
}) {
	// Initialize the client
	err := client.Initialize(context.Background())
	require.NoError(t, err)

	// First call should try both methods
	tools, err := client.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "test-tool", tools[0].Name)

	// Second call should only use the cached successful method
	tools, err = client.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "test-tool", tools[0].Name)

	// Execute a tool - should only try tools/call
	toolCall := types.ToolCall{
		Name:       "test-tool",
		Parameters: map[string]interface{}{},
	}
	result, err := client.ExecuteTool(context.Background(), toolCall)
	require.NoError(t, err)
	require.NotNil(t, result)
}
