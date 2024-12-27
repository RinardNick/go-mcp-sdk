package ws_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	serverws "github.com/RinardNick/go-mcp-sdk/pkg/server/ws"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/gorilla/websocket"
)

func TestWebSocketTransportReconnection(t *testing.T) {
	s := server.NewServer(nil)
	config := &serverws.Config{
		Address:      ":0",
		WSPath:       "/ws",
		PingInterval: 100 * time.Millisecond,
		PongWait:     200 * time.Millisecond,
	}
	transport := serverws.NewTransport(s, config)

	// Create test server
	server := httptest.NewServer(transport.GetHandler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Test connection drop and reconnect
	t.Run("Connection Drop and Reconnect", func(t *testing.T) {
		// First connection
		conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect to WebSocket: %v", err)
		}

		// Wait for connection setup
		time.Sleep(50 * time.Millisecond)

		// Force close the connection
		conn1.Close()

		// Wait a bit
		time.Sleep(50 * time.Millisecond)

		// New connection should work
		conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to reconnect to WebSocket: %v", err)
		}
		defer conn2.Close()

		// Test that new connection works
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "mcp/list_tools",
			"id":      1,
		}
		if err := conn2.WriteJSON(req); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		var resp types.Response
		if err := conn2.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}
	})
}

func TestWebSocketTransportConcurrentClients(t *testing.T) {
	s := server.NewServer(nil)

	// Register test tool
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

	handler := func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
		return types.NewToolResult(map[string]interface{}{
			"output": params["param1"],
		})
	}
	if err := s.RegisterToolHandler("test_tool", handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	config := &serverws.Config{
		Address: ":0",
		WSPath:  "/ws",
	}
	transport := serverws.NewTransport(s, config)

	// Create test server
	server := httptest.NewServer(transport.GetHandler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Test concurrent clients
	t.Run("Concurrent Clients", func(t *testing.T) {
		numClients := 10
		var wg sync.WaitGroup
		wg.Add(numClients)

		for i := 0; i < numClients; i++ {
			go func(clientID int) {
				defer wg.Done()

				// Connect
				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				if err != nil {
					t.Errorf("Client %d failed to connect: %v", clientID, err)
					return
				}
				defer conn.Close()

				// Send tool call request
				req := map[string]interface{}{
					"jsonrpc": "2.0",
					"method":  "mcp/call_tool",
					"params": map[string]interface{}{
						"name": "test_tool",
						"parameters": map[string]interface{}{
							"param1": fmt.Sprintf("test input %d", clientID),
						},
					},
					"id": clientID,
				}
				if err := conn.WriteJSON(req); err != nil {
					t.Errorf("Client %d failed to write request: %v", clientID, err)
					return
				}

				// Read response
				var resp types.Response
				if err := conn.ReadJSON(&resp); err != nil {
					t.Errorf("Client %d failed to read response: %v", clientID, err)
					return
				}

				if resp.Error != nil {
					t.Errorf("Client %d got unexpected error: %v", clientID, resp.Error)
					return
				}

				// Verify response
				var result map[string]interface{}
				if err := json.Unmarshal(resp.Result, &result); err != nil {
					t.Errorf("Client %d failed to unmarshal result: %v", clientID, err)
					return
				}

				output, ok := result["output"]
				if !ok {
					t.Errorf("Client %d: Expected output in result", clientID)
					return
				}
				expected := fmt.Sprintf("test input %d", clientID)
				if output != expected {
					t.Errorf("Client %d: Expected output %q, got %v", clientID, expected, output)
				}
			}(i)
		}

		wg.Wait()
	})
}

func TestWSServer(t *testing.T) {
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
