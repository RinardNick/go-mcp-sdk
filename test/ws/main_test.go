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
			"method":  "tools/list",
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
		// Create a new result for each request to avoid sharing state
		result, err := types.NewToolResult(map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": params["param1"].(string),
				},
			},
			"isError": false,
		})
		if err != nil {
			return nil, err
		}
		return result, nil
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

		// Create a channel to collect errors
		errChan := make(chan error, numClients)

		// Add a delay between client connections
		for i := 0; i < numClients; i++ {
			go func(clientID int) {
				defer wg.Done()

				// Add a small delay between client connections
				time.Sleep(time.Duration(clientID) * 10 * time.Millisecond)

				// Connect
				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				if err != nil {
					errChan <- fmt.Errorf("Client %d failed to connect: %v", clientID, err)
					return
				}
				defer conn.Close()

				// Send tool call request
				req := map[string]interface{}{
					"jsonrpc": "2.0",
					"method":  "tools/call",
					"params": map[string]interface{}{
						"name": "test_tool",
						"Parameters": map[string]interface{}{
							"param1": fmt.Sprintf("test input %d", clientID),
						},
					},
					"id": clientID,
				}
				if err := conn.WriteJSON(req); err != nil {
					errChan <- fmt.Errorf("Client %d failed to write request: %v", clientID, err)
					return
				}

				// Read response with timeout
				done := make(chan struct{})
				var resp types.Response
				go func() {
					if err := conn.ReadJSON(&resp); err != nil {
						errChan <- fmt.Errorf("Client %d failed to read response: %v", clientID, err)
						return
					}
					close(done)
				}()

				select {
				case <-done:
					// Response received successfully
				case <-time.After(5 * time.Second):
					errChan <- fmt.Errorf("Client %d timed out waiting for response", clientID)
					return
				}

				if resp.Error != nil {
					errChan <- fmt.Errorf("Client %d got unexpected error: %v", clientID, resp.Error)
					return
				}

				// Verify response
				var resultMap struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
					IsError bool `json:"isError"`
				}
				if err := json.Unmarshal(resp.Result, &resultMap); err != nil {
					errChan <- fmt.Errorf("Client %d failed to unmarshal result: %v", clientID, err)
					return
				}

				if len(resultMap.Content) == 0 {
					errChan <- fmt.Errorf("Client %d: Expected non-empty content", clientID)
					return
				}

				expected := fmt.Sprintf("test input %d", clientID)
				if resultMap.Content[0].Text != expected {
					errChan <- fmt.Errorf("Client %d: Expected text %q, got %v", clientID, expected, resultMap.Content[0].Text)
					return
				}

				if resultMap.IsError {
					errChan <- fmt.Errorf("Client %d: Expected isError to be false", clientID)
					return
				}
			}(i)
		}

		// Wait for all goroutines to finish
		wg.Wait()
		close(errChan)

		// Check for any errors
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			for _, err := range errors {
				t.Error(err)
			}
		}
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
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": params["param1"],
				},
			},
			"isError": false,
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
			"param1": "test",
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
		t.Error("Expected non-empty content")
	} else if resultMap.Content[0].Text != "test" {
		t.Errorf("Expected text 'test', got %v", resultMap.Content[0].Text)
	}

	if resultMap.IsError {
		t.Error("Expected isError to be false")
	}
}
