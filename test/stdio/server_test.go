package stdio

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	serverStdio "github.com/RinardNick/go-mcp-sdk/pkg/server/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

type mockReader struct {
	data []byte
	pos  int
}

func (r *mockReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, nil
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

type mockWriter struct {
	bytes.Buffer
}

func TestStdioTransport(t *testing.T) {
	// Create a server with some test tools and resources
	s := server.NewServer(nil)

	// Register test tool
	tool := types.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]any{
			"param1": map[string]any{
				"type":        "string",
				"description": "A test parameter",
			},
		},
	}
	if err := s.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Register test handler
	handler := func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
		return &types.ToolResult{
			Result: map[string]interface{}{
				"output": params["param1"],
			},
		}, nil
	}
	if err := s.RegisterToolHandler("test_tool", handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Register test resource
	resource := types.Resource{
		URI:  "test://resource",
		Name: "test_resource",
	}
	if err := s.RegisterResource(resource); err != nil {
		t.Fatalf("Failed to register resource: %v", err)
	}

	// Create transport with mock reader/writer
	transport := serverStdio.NewTransport(s)

	t.Run("List Tools", func(t *testing.T) {
		// Create request
		req := serverStdio.Request{
			Jsonrpc: "2.0",
			Method:  "mcp/list_tools",
			ID:      1,
		}
		reqBytes, _ := json.Marshal(req)

		// Create mock reader/writer
		reader := &mockReader{data: append(reqBytes, '\n')}
		writer := &mockWriter{}
		transport.Reader = reader
		transport.Writer = writer

		// Start transport
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := transport.Start(ctx); err != nil {
			t.Fatalf("Failed to start transport: %v", err)
		}

		// Wait for response
		time.Sleep(100 * time.Millisecond)

		// Parse response
		var resp types.Response
		if err := json.NewDecoder(bytes.NewReader(writer.Bytes())).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response
		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result struct {
			Tools []types.Tool `json:"tools"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if len(result.Tools) != 1 {
			t.Fatalf("Expected 1 tool, got %d", len(result.Tools))
		}
		if result.Tools[0].Name != tool.Name {
			t.Errorf("Expected tool name %s, got %s", tool.Name, result.Tools[0].Name)
		}
	})

	t.Run("List Resources", func(t *testing.T) {
		// Create request
		req := serverStdio.Request{
			Jsonrpc: "2.0",
			Method:  "mcp/list_resources",
			ID:      2,
		}
		reqBytes, _ := json.Marshal(req)

		// Create mock reader/writer
		reader := &mockReader{data: append(reqBytes, '\n')}
		writer := &mockWriter{}
		transport.Reader = reader
		transport.Writer = writer

		// Start transport
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := transport.Start(ctx); err != nil {
			t.Fatalf("Failed to start transport: %v", err)
		}

		// Wait for response
		time.Sleep(100 * time.Millisecond)

		// Parse response
		var resp types.Response
		if err := json.NewDecoder(bytes.NewReader(writer.Bytes())).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response
		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result struct {
			Resources []types.Resource `json:"resources"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if len(result.Resources) != 1 {
			t.Fatalf("Expected 1 resource, got %d", len(result.Resources))
		}
		if result.Resources[0].URI != resource.URI {
			t.Errorf("Expected resource URI %s, got %s", resource.URI, result.Resources[0].URI)
		}
	})

	t.Run("Call Tool", func(t *testing.T) {
		// Create request
		toolCall := types.ToolCall{
			Name: "test_tool",
			Parameters: map[string]any{
				"param1": "test",
			},
		}
		req := serverStdio.Request{
			Jsonrpc: "2.0",
			Method:  "mcp/call_tool",
			ID:      3,
		}
		req.Params, _ = json.Marshal(toolCall)
		reqBytes, _ := json.Marshal(req)

		// Create mock reader/writer
		reader := &mockReader{data: append(reqBytes, '\n')}
		writer := &mockWriter{}
		transport.Reader = reader
		transport.Writer = writer

		// Start transport
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := transport.Start(ctx); err != nil {
			t.Fatalf("Failed to start transport: %v", err)
		}

		// Wait for response
		time.Sleep(100 * time.Millisecond)

		// Parse response
		var resp types.Response
		if err := json.NewDecoder(bytes.NewReader(writer.Bytes())).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response
		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result types.ToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if result.Error != nil {
			t.Fatalf("Unexpected tool error: %v", result.Error)
		}
	})

	t.Run("Batch Request", func(t *testing.T) {
		// Create batch request
		batch := serverStdio.BatchRequest{
			Requests: []serverStdio.Request{
				{
					Jsonrpc: "2.0",
					Method:  "mcp/list_tools",
					ID:      4,
				},
				{
					Jsonrpc: "2.0",
					Method:  "mcp/list_resources",
					ID:      5,
				},
			},
		}
		reqBytes, _ := json.Marshal(batch)

		// Create mock reader/writer
		reader := &mockReader{data: append(reqBytes, '\n')}
		writer := &mockWriter{}
		transport.Reader = reader
		transport.Writer = writer

		// Start transport
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := transport.Start(ctx); err != nil {
			t.Fatalf("Failed to start transport: %v", err)
		}

		// Wait for response
		time.Sleep(100 * time.Millisecond)

		// Parse response
		var batchResp serverStdio.BatchResponse
		if err := json.NewDecoder(bytes.NewReader(writer.Bytes())).Decode(&batchResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify responses
		if len(batchResp.Responses) != 2 {
			t.Fatalf("Expected 2 responses, got %d", len(batchResp.Responses))
		}

		for _, resp := range batchResp.Responses {
			if resp.Error != nil {
				t.Errorf("Unexpected error in batch response: %v", resp.Error)
			}
		}
	})
}
