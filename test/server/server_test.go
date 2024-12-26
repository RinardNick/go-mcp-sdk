package server

import (
	"context"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestServer(t *testing.T) {
	// Create server with test options
	options := &server.InitializationOptions{
		Version: "1.0",
		Capabilities: map[string]interface{}{
			"tools":     true,
			"resources": true,
		},
		Config: map[string]interface{}{
			"maxSessions":      10,
			"enableValidation": true,
			"logLevel":         "debug",
		},
	}
	s := server.NewServer(options)

	// Test tool registration
	t.Run("Tool Registration", func(t *testing.T) {
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

		// Test successful registration
		if err := s.RegisterTool(tool); err != nil {
			t.Errorf("Failed to register tool: %v", err)
		}

		// Test duplicate registration
		if err := s.RegisterTool(tool); err == nil {
			t.Error("Expected error registering duplicate tool")
		}

		// Test empty name
		invalidTool := tool
		invalidTool.Name = ""
		if err := s.RegisterTool(invalidTool); err == nil {
			t.Error("Expected error registering tool with empty name")
		}

		// Verify registered tool
		tools := s.GetTools()
		if len(tools) != 1 {
			t.Errorf("Expected 1 tool, got %d", len(tools))
		}
		if tools[0].Name != tool.Name {
			t.Errorf("Expected tool name %s, got %s", tool.Name, tools[0].Name)
		}
	})

	// Test tool handler registration
	t.Run("Tool Handler Registration", func(t *testing.T) {
		handler := func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
			return &types.ToolResult{
				Result: map[string]interface{}{
					"output": params["param1"],
				},
			}, nil
		}

		// Test successful registration
		if err := s.RegisterToolHandler("test_tool", handler); err != nil {
			t.Errorf("Failed to register handler: %v", err)
		}

		// Test duplicate registration
		if err := s.RegisterToolHandler("test_tool", handler); err == nil {
			t.Error("Expected error registering duplicate handler")
		}

		// Test unregistered tool
		if err := s.RegisterToolHandler("unknown_tool", handler); err == nil {
			t.Error("Expected error registering handler for unknown tool")
		}

		// Test nil handler
		if err := s.RegisterToolHandler("test_tool", nil); err == nil {
			t.Error("Expected error registering nil handler")
		}
	})

	// Test resource registration
	t.Run("Resource Registration", func(t *testing.T) {
		resource := types.Resource{
			URI:  "test://resource",
			Name: "test_resource",
		}

		// Test successful registration
		if err := s.RegisterResource(resource); err != nil {
			t.Errorf("Failed to register resource: %v", err)
		}

		// Test duplicate registration
		if err := s.RegisterResource(resource); err == nil {
			t.Error("Expected error registering duplicate resource")
		}

		// Test empty URI
		invalidResource := resource
		invalidResource.URI = ""
		if err := s.RegisterResource(invalidResource); err == nil {
			t.Error("Expected error registering resource with empty URI")
		}

		// Verify registered resource
		resources := s.GetResources()
		if len(resources) != 1 {
			t.Errorf("Expected 1 resource, got %d", len(resources))
		}
		if resources[0].URI != resource.URI {
			t.Errorf("Expected resource URI %s, got %s", resource.URI, resources[0].URI)
		}
	})

	// Test tool execution
	t.Run("Tool Execution", func(t *testing.T) {
		ctx := context.Background()

		// Test successful execution
		result, err := s.HandleToolCall(ctx, types.ToolCall{
			Name: "test_tool",
			Parameters: map[string]any{
				"param1": "test input",
			},
		})
		if err != nil {
			t.Errorf("Failed to execute tool: %v", err)
		}
		if output, ok := result.Result.(map[string]interface{})["output"]; !ok || output != "test input" {
			t.Errorf("Expected output 'test input', got %v", output)
		}

		// Test unregistered tool
		_, err = s.HandleToolCall(ctx, types.ToolCall{
			Name: "unknown_tool",
			Parameters: map[string]any{
				"param1": "test",
			},
		})
		if err == nil {
			t.Error("Expected error executing unknown tool")
		}

		// Test invalid parameters
		_, err = s.HandleToolCall(ctx, types.ToolCall{
			Name: "test_tool",
			Parameters: map[string]any{
				"invalid_param": "test",
			},
		})
		if err == nil {
			t.Error("Expected error with invalid parameters")
		}
	})

	// Test server lifecycle
	t.Run("Server Lifecycle", func(t *testing.T) {
		ctx := context.Background()

		// Test start
		if err := s.Start(ctx); err != nil {
			t.Errorf("Failed to start server: %v", err)
		}

		// Test stop
		if err := s.Stop(ctx); err != nil {
			t.Errorf("Failed to stop server: %v", err)
		}
	})
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

	// Test tool execution without validation
	t.Run("Tool Execution Without Validation", func(t *testing.T) {
		ctx := context.Background()

		// Test with invalid parameters (should still work)
		result, err := s.HandleToolCall(ctx, types.ToolCall{
			Name: "test_tool",
			Parameters: map[string]any{
				"invalid_param": "test",
			},
		})
		if err != nil {
			t.Errorf("Expected success with validation disabled, got error: %v", err)
		}
		if result.Result.(map[string]interface{})["output"] != nil {
			t.Error("Expected nil output with invalid parameter")
		}
	})
}
