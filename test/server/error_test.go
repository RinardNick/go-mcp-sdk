package server

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestErrorHandling(t *testing.T) {
	t.Run("Malformed JSON in tool parameters", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
		})

		// Register a test tool
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
			return types.NewToolResult(map[string]interface{}{
				"output": params["param1"],
			})
		})
		if err != nil {
			t.Fatalf("Failed to register tool handler: %v", err)
		}

		// Test with invalid JSON in parameters
		invalidJSON := json.RawMessage(`{"param1": invalid}`)
		toolCall := types.ToolCall{
			Parameters: make(map[string]interface{}),
		}
		if err := json.Unmarshal(invalidJSON, &toolCall.Parameters); err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}

		// Test with malformed schema
		malformedSchema := json.RawMessage(`{invalid schema}`)
		err = s.RegisterTool(types.Tool{
			Name:        "invalid_tool",
			Description: "A tool with invalid schema",
			InputSchema: malformedSchema,
		})
		if err != nil {
			// We expect an error here
			t.Logf("Got expected error for malformed schema: %v", err)
		} else {
			t.Error("Expected error for malformed schema, got nil")
		}
	})

	t.Run("Malformed JSON in initialization", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
		})

		// Test with invalid capabilities JSON
		invalidJSON := json.RawMessage(`{"capabilities": invalid}`)
		var params types.InitializeParams
		if err := json.Unmarshal(invalidJSON, &params); err == nil {
			t.Error("Expected error for invalid JSON in initialization params, got nil")
		}

		// Test with malformed client info
		invalidJSON = json.RawMessage(`{"clientInfo": {"name": 123}}`) // name should be string
		if err := json.Unmarshal(invalidJSON, &params); err == nil {
			t.Error("Expected error for invalid client info type, got nil")
		}

		// Try to initialize with the malformed params
		result, err := s.HandleInitialize(context.Background(), params)
		if err == nil {
			t.Error("Expected error for malformed initialization params, got nil")
		}
		if result != nil {
			t.Errorf("Expected nil result for malformed initialization params, got %v", result)
		}
	})

	t.Run("Partial message handling", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
		})

		// Test with partial tool call JSON
		partialJSON := json.RawMessage(`{"name": "test_tool", "parameters":`)
		var toolCall types.ToolCall
		if err := json.Unmarshal(partialJSON, &toolCall); err == nil {
			t.Error("Expected error for partial JSON in tool call, got nil")
		}

		// Test with incomplete initialization params
		partialJSON = json.RawMessage(`{"protocolVersion": "1.0", "clientInfo":`)
		var params types.InitializeParams
		if err := json.Unmarshal(partialJSON, &params); err == nil {
			t.Error("Expected error for partial JSON in initialization params, got nil")
		}

		// Test with truncated tool schema
		partialSchema := json.RawMessage(`{"type": "object", "properties":`)
		err := s.RegisterTool(types.Tool{
			Name:        "partial_tool",
			Description: "A tool with partial schema",
			InputSchema: partialSchema,
		})
		if err == nil {
			t.Error("Expected error for partial schema, got nil")
		}

		// Test with incomplete tool result
		partialResult := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					// Missing required "text" field
				},
			},
		}
		_, err = types.NewToolResult(partialResult)
		if err == nil {
			t.Error("Expected error for incomplete tool result, got nil")
		}
	})

	t.Run("Message size limits", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
		})

		// Create a large string that exceeds reasonable limits
		largeString := make([]byte, 11*1024*1024) // 11MB, larger than MaxMessageSize
		for i := range largeString {
			largeString[i] = 'a'
		}

		// Test with oversized tool parameters
		toolParams := map[string]interface{}{
			"param1": string(largeString),
		}
		toolCall := types.ToolCall{
			Parameters: toolParams,
		}

		// Try to handle the tool call with large parameters
		toolResult, err := s.HandleToolCall(context.Background(), toolCall)
		if err == nil {
			t.Error("Expected error for oversized tool parameters, got nil")
		}
		if toolResult != nil {
			t.Errorf("Expected nil result for oversized tool parameters, got %v", toolResult)
		}

		// Test with oversized tool result
		largeResult := map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": string(largeString),
				},
			},
		}
		_, err = types.NewToolResult(largeResult)
		if err == nil {
			t.Error("Expected error for oversized tool result, got nil")
		}

		// Test with oversized tool schema
		largeSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": string(largeString),
				},
			},
		}
		schemaBytes, err := types.NewToolInputSchema(largeSchema)
		if err != nil {
			t.Logf("Got expected error for oversized schema: %v", err)
		} else {
			// If we somehow got schema bytes, try to register the tool
			err = s.RegisterTool(types.Tool{
				Name:        "large_tool",
				Description: "A tool with large schema",
				InputSchema: schemaBytes,
			})
			if err == nil {
				t.Error("Expected error for oversized schema, got nil")
			}
		}

		// Test with oversized initialization params
		initParams := types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.Implementation{
				Name:    string(largeString),
				Version: "1.0",
			},
		}

		// Try to initialize with the large params
		initResult, err := s.HandleInitialize(context.Background(), initParams)
		if err == nil {
			t.Error("Expected error for oversized initialization params, got nil")
		}
		if initResult != nil {
			t.Errorf("Expected nil result for oversized initialization params, got %v", initResult)
		}
	})

	t.Run("Concurrent error conditions", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
		})

		// Register a test tool
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

		// Register tool handler that simulates errors
		err = s.RegisterToolHandler("test_tool", func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
			// Simulate random errors
			if params["param1"] == "error" {
				return nil, types.InternalError("simulated error")
			}
			return types.NewToolResult(map[string]interface{}{
				"output": params["param1"],
			})
		})
		if err != nil {
			t.Fatalf("Failed to register tool handler: %v", err)
		}

		// Test concurrent tool calls with errors
		const numCalls = 100
		errChan := make(chan error, numCalls)
		doneChan := make(chan struct{})

		go func() {
			for i := 0; i < numCalls; i++ {
				go func(i int) {
					param := "test"
					if i%2 == 0 {
						param = "error"
					}

					result, err := s.HandleToolCall(context.Background(), types.ToolCall{
						Name: "test_tool",
						Parameters: map[string]interface{}{
							"param1": param,
						},
					})

					if param == "error" {
						if err == nil {
							errChan <- fmt.Errorf("Expected error for error param, got nil")
						}
						if result != nil {
							errChan <- fmt.Errorf("Expected nil result for error param, got %v", result)
						}
					} else {
						if err != nil {
							errChan <- fmt.Errorf("Expected no error for valid param, got %v", err)
						}
						if result == nil {
							errChan <- fmt.Errorf("Expected non-nil result for valid param")
						}
					}

					errChan <- nil
				}(i)
			}

			// Wait for all goroutines to finish
			time.Sleep(100 * time.Millisecond)
			close(doneChan)
		}()

		// Collect errors
		var errors []error
	loop:
		for {
			select {
			case err := <-errChan:
				if err != nil {
					errors = append(errors, err)
				}
			case <-doneChan:
				break loop
			}
		}

		// Check for errors
		for _, err := range errors {
			t.Error(err)
		}
	})
}
