package stdio

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
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

func TestStdioServer(t *testing.T) {
	// Create test server
	s := server.NewServer(&server.InitializationOptions{
		Version: "1.0",
		Capabilities: map[string]interface{}{
			"tools":     true,
			"resources": true,
		},
	})

	// Add test tool
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"param1": map[string]interface{}{
				"type":        "string",
				"description": "A test parameter",
			},
		},
		"required": []string{"param1"},
	}
	schemaBytes, err := types.NewToolInputSchema(inputSchema)
	if err != nil {
		t.Fatalf("Failed to create input schema: %v", err)
	}

	err = s.RegisterTool(types.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: schemaBytes,
	})
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Register tool handler
	err = s.RegisterToolHandler("test_tool", func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
		result := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "test output",
				},
			},
			"isError": false,
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		return &types.ToolResult{
			Result: resultBytes,
		}, nil
	})
	if err != nil {
		t.Fatalf("Failed to register tool handler: %v", err)
	}

	// Test tool call
	toolCall := types.ToolCall{
		Name: "test_tool",
		Parameters: map[string]interface{}{
			"param1": "test value",
		},
	}

	result, err := s.HandleToolCall(context.Background(), toolCall)
	if err != nil {
		t.Fatalf("Failed to handle tool call: %v", err)
	}

	// Check result
	var resultMap struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result.Result, &resultMap); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(resultMap.Content) == 0 {
		t.Fatal("Expected non-empty content")
	}
	if resultMap.Content[0].Text != "test output" {
		t.Errorf("Expected output 'test output', got %v", resultMap.Content[0].Text)
	}
	if resultMap.IsError {
		t.Error("Expected isError to be false")
	}
}
