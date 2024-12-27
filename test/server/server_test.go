package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestServer(t *testing.T) {
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
		// Convert params to map[string]interface{} for NewToolResult
		paramsInterface := make(map[string]interface{}, len(params))
		for k, v := range params {
			paramsInterface[k] = v
		}

		result, err := types.NewToolResult(map[string]interface{}{
			"output": "test output",
		})
		if err != nil {
			return nil, err
		}
		return result, nil
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
	var resultMap map[string]interface{}
	if err := json.Unmarshal(result.Result, &resultMap); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if resultMap["output"] != "test output" {
		t.Errorf("Expected output 'test output', got %v", resultMap["output"])
	}
}

func TestServerWithoutValidation(t *testing.T) {
	options := &server.InitializationOptions{
		Version: "1.0",
		Capabilities: map[string]interface{}{
			"tools":     true,
			"resources": true,
		},
		Config: map[string]interface{}{
			"maxSessions":      10,
			"enableValidation": false,
			"logLevel":         "debug",
		},
	}
	s := server.NewServer(options)

	// Register test tool and handler
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

	tool := types.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: schemaBytes,
	}
	if err := s.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	handler := func(ctx context.Context, params map[string]interface{}) (*types.ToolResult, error) {
		result, err := types.NewToolResult(map[string]interface{}{
			"output": params["param1"],
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	if err := s.RegisterToolHandler("test_tool", handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Test tool execution without validation
	t.Run("Tool Execution Without Validation", func(t *testing.T) {
		ctx := context.Background()

		// Test with invalid parameters (should still work)
		result, err := s.HandleToolCall(ctx, types.ToolCall{
			Name: "test_tool",
			Parameters: map[string]interface{}{
				"invalid_param": "test",
			},
		})
		if err != nil {
			t.Errorf("Expected success with validation disabled, got error: %v", err)
		}

		// Check result
		var resultMap map[string]interface{}
		if err := json.Unmarshal(result.Result, &resultMap); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}
		if resultMap["output"] != nil {
			t.Error("Expected nil output with invalid parameter")
		}
	})
}
