package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/sse"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestSSEClient(t *testing.T) {
	// Helper function to marshal JSON
	mustMarshal := func(v interface{}) []byte {
		data, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		return data
	}

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

			// Set content type for JSON response
			w.Header().Set("Content-Type", "application/json")

			switch req.Method {
			case "tools/list":
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

				tools := []types.Tool{
					{
						Name:        "test_tool",
						Description: "A test tool",
						InputSchema: schemaBytes,
					},
				}

				result := map[string]interface{}{
					"tools": tools,
				}
				response := types.Response{
					Jsonrpc: "2.0",
					Result:  json.RawMessage(mustMarshal(result)),
					ID:      1,
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

			case "tools/call":
				var params struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments"`
				}
				if err := json.Unmarshal(req.Params, &params); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				result := map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "test output",
						},
					},
					"isError": false,
				}
				response := types.Response{
					Jsonrpc: "2.0",
					Result:  json.RawMessage(mustMarshal(result)),
					ID:      1,
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

			case "mcp/list_resources":
				resources := []types.Resource{
					{
						ID:          "test_resource",
						Name:        "test_resource",
						Description: "A test resource",
						Type:        "test",
						URI:         "test://resource",
						Metadata:    map[string]interface{}{},
					},
				}
				result := map[string]interface{}{
					"resources": resources,
				}
				response := types.Response{
					Jsonrpc: "2.0",
					Result:  json.RawMessage(mustMarshal(result)),
					ID:      1,
				}
				if err := json.NewEncoder(w).Encode(response); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

		case "/events":
			// Set headers for SSE
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			// Send test event
			fmt.Fprintf(w, "event: message\ndata: {\"type\":\"test\"}\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}

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

		content, ok := resultMap["content"].([]interface{})
		if !ok || len(content) == 0 {
			t.Fatal("Expected non-empty content array")
		}

		textContent, ok := content[0].(map[string]interface{})
		if !ok {
			t.Fatal("Expected content to be a map")
		}

		if text, ok := textContent["text"].(string); !ok || text != "test output" {
			t.Errorf("Expected text 'test output', got '%v'", textContent["text"])
		}

		if isError, ok := resultMap["isError"].(bool); !ok || isError {
			t.Error("Expected isError to be false")
		}
	})
}
