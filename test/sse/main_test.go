package sse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/sse"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestSSEClient(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc":
			// Handle RPC requests
			var req struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			switch req.Method {
			case "mcp/list_tools":
				// Create tool input schema
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
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      1,
					"result": map[string]interface{}{
						"tools": []types.Tool{
							{
								Name:        "test_tool",
								Description: "A test tool",
								InputSchema: schemaBytes,
							},
						},
					},
				})

			case "mcp/list_resources":
				json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      1,
					"result": map[string]interface{}{
						"resources": []types.Resource{
							{
								ID:          "test_resource",
								Name:        "test_resource",
								Description: "A test resource",
								Type:        "test",
								URI:         "test://resource",
								Metadata:    map[string]interface{}{},
							},
						},
					},
				})

			case "mcp/call_tool":
				result, err := types.NewToolResult(map[string]interface{}{
					"output": "test output",
				})
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      1,
					"result":  result,
				})
			}

		case "/events":
			// Set headers for SSE
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			// Send test event
			w.Write([]byte("event: message\ndata: {\"type\":\"test\"}\n\n"))
			w.(http.Flusher).Flush()

			// Keep connection open
			<-r.Context().Done()
		}
	}))
	defer server.Close()

	// Create client
	client, err := sse.NewSSEClient(server.URL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test ListTools
	t.Run("ListTools", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tools, err := client.ListTools(ctx)
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) != 1 {
			t.Fatalf("Expected 1 tool, got %d", len(tools))
		}

		if tools[0].Name != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got '%s'", tools[0].Name)
		}
	})

	// Test ListResources
	t.Run("ListResources", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resources, err := client.ListResources(ctx)
		if err != nil {
			t.Fatalf("ListResources failed: %v", err)
		}

		if len(resources) != 1 {
			t.Fatalf("Expected 1 resource, got %d", len(resources))
		}

		if resources[0].URI != "test://resource" {
			t.Errorf("Expected resource URI 'test://resource', got '%s'", resources[0].URI)
		}
	})

	// Test ExecuteTool
	t.Run("ExecuteTool", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		toolCall := types.ToolCall{
			Name: "test_tool",
			Parameters: map[string]interface{}{
				"param1": "test",
			},
		}

		result, err := client.ExecuteTool(ctx, toolCall)
		if err != nil {
			t.Fatalf("ExecuteTool failed: %v", err)
		}

		var resultMap map[string]interface{}
		if err := json.Unmarshal(result.Result, &resultMap); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		output, ok := resultMap["output"].(string)
		if !ok {
			t.Fatal("Expected string output in result")
		}

		if output != "test output" {
			t.Errorf("Expected output 'test output', got '%s'", output)
		}
	})
}
