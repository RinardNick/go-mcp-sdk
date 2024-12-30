package server

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestServerWithoutValidation(t *testing.T) {
	t.Run("Tool Execution Without Validation", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Config: map[string]interface{}{
				"enableValidation": false,
			},
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
			output := "test value"
			if val, ok := params["param1"].(string); ok {
				output = val
			}
			return types.NewToolResult(map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": output,
					},
				},
				"isError": false,
			})
		})
		if err != nil {
			t.Fatalf("Failed to register tool handler: %v", err)
		}

		// Test tool execution without validation
		result, err := s.HandleToolCall(context.Background(), types.ToolCall{
			Name: "test_tool",
			Parameters: map[string]interface{}{
				"param1": "test value",
				"param2": 123, // This would normally fail validation
			},
		})

		if err != nil {
			t.Errorf("Expected no error with validation disabled, got %v", err)
			return
		}
		if result == nil {
			t.Error("Expected non-nil result with validation disabled")
			return
		}
		if result.Result == nil {
			t.Error("Expected non-nil result.Result with validation disabled")
			return
		}

		// Verify that the handler still works with the valid parameter
		var resultMap struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(result.Result, &resultMap); err != nil {
			t.Errorf("Failed to unmarshal result: %v", err)
			return
		}
		if len(resultMap.Content) == 0 {
			t.Error("Expected non-empty content")
		} else if resultMap.Content[0].Text != "test value" {
			t.Errorf("Expected output 'test value', got %v", resultMap.Content[0].Text)
		}
	})
}

func TestConnectionTimeouts(t *testing.T) {
	t.Run("Initialization Timeout", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Config: map[string]interface{}{
				"connectionTimeout": float64(100), // milliseconds
			},
			Capabilities: map[string]interface{}{
				"tools":     true,
				"resources": true,
			},
		})

		// Create a context with a timeout longer than server timeout
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Simulate slow client by sleeping
		time.Sleep(150 * time.Millisecond)

		_, err := s.HandleInitialize(ctx, types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.Implementation{
				Name:    "test-client",
				Version: "1.0",
			},
			Capabilities: types.ClientCapabilities{
				Tools: &types.ToolCapabilities{
					SupportsProgress:     true,
					SupportsCancellation: true,
				},
			},
		})

		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Errorf("Expected deadline exceeded error, got: %v", err)
		}
	})

	t.Run("Tool Execution Timeout", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Config: map[string]interface{}{
				"connectionTimeout": float64(100), // milliseconds
			},
			Capabilities: map[string]interface{}{
				"tools":     true,
				"resources": true,
			},
		})

		// Register a slow tool
		err := s.RegisterTool(types.Tool{
			Name:        "slow_tool",
			Description: "A tool that takes time to execute",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		})
		if err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}

		// Register tool handler that sleeps
		err = s.RegisterToolHandler("slow_tool", func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
			time.Sleep(150 * time.Millisecond)
			return types.NewToolResult(map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "done"},
				},
				"isError": false,
			})
		})
		if err != nil {
			t.Fatalf("Failed to register tool handler: %v", err)
		}

		// Execute tool with timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err = s.HandleToolCall(ctx, types.ToolCall{
			Name:       "slow_tool",
			Parameters: map[string]interface{}{},
		})

		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Errorf("Expected deadline exceeded error, got: %v", err)
		}
	})

	t.Run("Concurrent Timeout Handling", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Config: map[string]interface{}{
				"connectionTimeout": float64(100), // milliseconds
			},
			Capabilities: map[string]interface{}{
				"tools":     true,
				"resources": true,
			},
		})

		// Register a tool that returns quickly
		err := s.RegisterTool(types.Tool{
			Name:        "quick_tool",
			Description: "A tool that executes quickly",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		})
		if err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}

		err = s.RegisterToolHandler("quick_tool", func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
			return types.NewToolResult(map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "done quickly"},
				},
				"isError": false,
			})
		})
		if err != nil {
			t.Fatalf("Failed to register tool handler: %v", err)
		}

		// Test that a quick operation succeeds even when server has a longer timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		result, err := s.HandleToolCall(ctx, types.ToolCall{
			Name:       "quick_tool",
			Parameters: map[string]interface{}{},
		})

		if err != nil {
			t.Errorf("Expected success for quick operation, got error: %v", err)
		}
		if result == nil {
			t.Error("Expected result from quick operation, got nil")
		}
	})
}

func TestGracefulShutdown(t *testing.T) {
	t.Run("Shutdown With Active Operations", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Config: map[string]interface{}{
				"connectionTimeout": float64(100), // milliseconds
			},
			Capabilities: map[string]interface{}{
				"tools":     true,
				"resources": true,
			},
		})

		// Register a slow tool
		err := s.RegisterTool(types.Tool{
			Name:        "slow_tool",
			Description: "A tool that takes time to execute",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		})
		if err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}

		// Register tool handler that sleeps
		err = s.RegisterToolHandler("slow_tool", func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return types.NewToolResult(map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": "completed"},
					},
					"isError": false,
				})
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
		if err != nil {
			t.Fatalf("Failed to register tool handler: %v", err)
		}

		// Start server
		if err := s.Start(context.Background()); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}

		// Start a long-running operation
		operationCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		var operationErr error
		var operationResult *types.ToolResult

		go func() {
			defer wg.Done()
			operationResult, operationErr = s.HandleToolCall(operationCtx, types.ToolCall{
				Name:       "slow_tool",
				Parameters: map[string]interface{}{},
			})
		}()

		// Wait a bit for the operation to start
		time.Sleep(50 * time.Millisecond)

		// Initiate graceful shutdown with a timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer shutdownCancel()

		// Shutdown should wait for active operations
		if err := s.Stop(shutdownCtx); err != nil {
			t.Errorf("Failed to stop server: %v", err)
		}

		// Wait for operation to complete
		wg.Wait()

		// Operation should complete successfully
		if operationErr != nil {
			t.Errorf("Operation failed: %v", operationErr)
		}
		if operationResult == nil {
			t.Error("Expected operation result, got nil")
		} else {
			var resultMap struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}
			if err := json.Unmarshal(operationResult.Result, &resultMap); err != nil {
				t.Errorf("Failed to unmarshal result: %v", err)
			} else if len(resultMap.Content) == 0 || resultMap.Content[0].Text != "completed" {
				t.Errorf("Expected 'completed' result, got: %v", resultMap.Content)
			}
		}

		// New operations should be rejected
		_, err = s.HandleToolCall(context.Background(), types.ToolCall{
			Name:       "slow_tool",
			Parameters: map[string]interface{}{},
		})
		if err == nil {
			t.Error("Expected error when calling tool after shutdown")
		}
		if !strings.Contains(err.Error(), "Invalid params") {
			t.Errorf("Expected invalid params error, got: %v", err)
		}
	})

	t.Run("Shutdown With No Active Operations", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Config: map[string]interface{}{
				"connectionTimeout": float64(100), // milliseconds
			},
			Capabilities: map[string]interface{}{
				"tools":     true,
				"resources": true,
			},
		})

		// Start server
		if err := s.Start(context.Background()); err != nil {
			t.Fatalf("Failed to start server: %v", err)
		}

		// Initiate graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Shutdown should be quick with no active operations
		start := time.Now()
		if err := s.Stop(shutdownCtx); err != nil {
			t.Errorf("Failed to stop server: %v", err)
		}
		duration := time.Since(start)

		if duration > 50*time.Millisecond {
			t.Errorf("Shutdown took too long with no active operations: %v", duration)
		}

		// New operations should be rejected
		_, err := s.HandleToolCall(context.Background(), types.ToolCall{
			Name:       "slow_tool",
			Parameters: map[string]interface{}{},
		})
		if err == nil {
			t.Error("Expected error when calling tool after shutdown")
		}
		if !strings.Contains(err.Error(), "Invalid params") {
			t.Errorf("Expected invalid params error, got: %v", err)
		}
	})
}
