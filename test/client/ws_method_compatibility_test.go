package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/ws"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func handleWebSocketRequest(t *testing.T, mockServer *MockServer, msg []byte) ([]byte, error) {
	// Try to unmarshal as batch request first
	var batch []struct {
		Jsonrpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
		ID      int64           `json:"id"`
	}
	if err := json.Unmarshal(msg, &batch); err == nil {
		// Handle batch request
		responses := make([]interface{}, len(batch))
		for i, req := range batch {
			var result json.RawMessage
			var respErr error

			if req.Method == "initialize" {
				result, respErr = mockServer.HandleInitialize(req.Params)
			} else {
				result, respErr = mockServer.HandleRequest(req.Method, req.Params)
			}

			if respErr != nil {
				if mcpErr, ok := respErr.(*types.MCPError); ok {
					responses[i] = struct {
						Jsonrpc string          `json:"jsonrpc"`
						Error   *types.MCPError `json:"error"`
						ID      int64           `json:"id"`
					}{
						Jsonrpc: "2.0",
						Error:   mcpErr,
						ID:      req.ID,
					}
				} else {
					responses[i] = struct {
						Jsonrpc string          `json:"jsonrpc"`
						Error   *types.MCPError `json:"error"`
						ID      int64           `json:"id"`
					}{
						Jsonrpc: "2.0",
						Error:   types.InternalError(respErr.Error()),
						ID:      req.ID,
					}
				}
			} else {
				responses[i] = struct {
					Jsonrpc string          `json:"jsonrpc"`
					Result  json.RawMessage `json:"result"`
					ID      int64           `json:"id"`
				}{
					Jsonrpc: "2.0",
					Result:  result,
					ID:      req.ID,
				}
			}
		}
		return json.Marshal(responses)
	}

	// Try to unmarshal as single request
	var req struct {
		Jsonrpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
		ID      int64           `json:"id"`
	}
	if err := json.Unmarshal(msg, &req); err != nil {
		return nil, fmt.Errorf("invalid request format")
	}

	// Handle single request
	var result json.RawMessage
	var respErr error

	if req.Method == "initialize" {
		result, respErr = mockServer.HandleInitialize(req.Params)
	} else {
		result, respErr = mockServer.HandleRequest(req.Method, req.Params)
	}

	var response interface{}
	if respErr != nil {
		if mcpErr, ok := respErr.(*types.MCPError); ok {
			response = struct {
				Jsonrpc string          `json:"jsonrpc"`
				Error   *types.MCPError `json:"error"`
				ID      int64           `json:"id"`
			}{
				Jsonrpc: "2.0",
				Error:   mcpErr,
				ID:      req.ID,
			}
		} else {
			response = struct {
				Jsonrpc string          `json:"jsonrpc"`
				Error   *types.MCPError `json:"error"`
				ID      int64           `json:"id"`
			}{
				Jsonrpc: "2.0",
				Error:   types.InternalError(respErr.Error()),
				ID:      req.ID,
			}
		}
	} else {
		response = struct {
			Jsonrpc string          `json:"jsonrpc"`
			Result  json.RawMessage `json:"result"`
			ID      int64           `json:"id"`
		}{
			Jsonrpc: "2.0",
			Result:  result,
			ID:      req.ID,
		}
	}
	return json.Marshal(response)
}

func TestWebSocketMethodNameCompatibility(t *testing.T) {
	t.Run("Method Name Compatibility", func(t *testing.T) {
		// Create a mock server
		mockServer := NewMockServer(t)

		// Create a WebSocket server
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}

		var wg sync.WaitGroup
		wg.Add(1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			defer conn.Close()

			defer wg.Done()

			for {
				// Read message
				_, msg, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						return
					}
					t.Logf("Failed to read message: %v", err)
					return
				}

				// Handle request
				responseBytes, err := handleWebSocketRequest(t, mockServer, msg)
				require.NoError(t, err)

				// Send response
				err = conn.WriteMessage(websocket.TextMessage, responseBytes)
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						return
					}
					t.Logf("Failed to write message: %v", err)
					return
				}
			}
		}))
		defer server.Close()

		// Create WebSocket client
		wsURL := "ws" + server.URL[4:] // Convert http:// to ws://
		client, err := ws.NewWebSocketClient(wsURL)
		require.NoError(t, err)

		client.SetInitializeParams(&types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.ClientInfo{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		// Run the standard compatibility tests
		TestMethodNameCompatibilityWithClient(t, client)

		// Close the client and wait for the server to finish
		client.Close()
		wg.Wait()

		// Verify method call counts
		require.Equal(t, 1, mockServer.GetMethodCalls("tools/list"), "tools/list should be called once")
		require.Equal(t, 2, mockServer.GetMethodCalls("mcp/list_tools"), "mcp/list_tools should be called twice")
		require.Equal(t, 1, mockServer.GetMethodCalls("tools/call"), "tools/call should be called once")
		require.Equal(t, 0, mockServer.GetMethodCalls("mcp/call_tool"), "mcp/call_tool should not be called")
	})
}

func TestWebSocketBatchRequests(t *testing.T) {
	t.Run("Batch Request Success", func(t *testing.T) {
		// Create a mock server
		mockServer := NewMockServer(t)

		// Create a WebSocket server
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}

		var wg sync.WaitGroup
		wg.Add(1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			defer conn.Close()

			defer wg.Done()

			for {
				// Read message
				_, msg, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						return
					}
					t.Logf("Failed to read message: %v", err)
					return
				}

				// Handle request
				responseBytes, err := handleWebSocketRequest(t, mockServer, msg)
				require.NoError(t, err)

				// Send response
				err = conn.WriteMessage(websocket.TextMessage, responseBytes)
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						return
					}
					t.Logf("Failed to write message: %v", err)
					return
				}
			}
		}))
		defer server.Close()

		// Create WebSocket client
		wsURL := "ws" + server.URL[4:] // Convert http:// to ws://
		client, err := ws.NewWebSocketClient(wsURL)
		require.NoError(t, err)

		client.SetInitializeParams(&types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.ClientInfo{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		// Initialize the client
		err = client.Initialize(context.Background())
		require.NoError(t, err)

		// Send batch request
		requests := []ws.BatchRequest{
			{
				Method: "tools/list",
				Params: nil,
			},
			{
				Method: "mcp/list_tools",
				Params: nil,
			},
			{
				Method: "tools/call",
				Params: struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments"`
				}{
					Name:      "test-tool",
					Arguments: map[string]interface{}{},
				},
			},
		}

		responses, err := client.SendBatchRequest(context.Background(), requests)
		require.NoError(t, err)
		require.Len(t, responses, len(requests))

		// Verify responses
		require.NotNil(t, responses[0].Error) // tools/list should fail
		require.Equal(t, types.ErrMethodNotFound, responses[0].Error.Code)

		require.Nil(t, responses[1].Error) // mcp/list_tools should succeed
		var tools []types.Tool
		err = json.Unmarshal(responses[1].Result, &tools)
		require.NoError(t, err)
		require.Len(t, tools, 1)
		require.Equal(t, "test-tool", tools[0].Name)

		require.Nil(t, responses[2].Error) // tools/call should succeed
		var result struct {
			Result string `json:"result"`
		}
		err = json.Unmarshal(responses[2].Result, &result)
		require.NoError(t, err)
		require.Equal(t, "success", result.Result)

		// Close the client and wait for the server to finish
		client.Close()
		wg.Wait()

		// Verify method call counts
		require.Equal(t, 1, mockServer.GetMethodCalls("tools/list"), "tools/list should be called once")
		require.Equal(t, 1, mockServer.GetMethodCalls("mcp/list_tools"), "mcp/list_tools should be called once")
		require.Equal(t, 1, mockServer.GetMethodCalls("tools/call"), "tools/call should be called once")
		require.Equal(t, 0, mockServer.GetMethodCalls("mcp/call_tool"), "mcp/call_tool should not be called")
	})

	t.Run("Empty Batch Request", func(t *testing.T) {
		// Create a mock server
		mockServer := NewMockServer(t)

		// Create a WebSocket server
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}

		var wg sync.WaitGroup
		wg.Add(1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			defer conn.Close()

			defer wg.Done()

			for {
				// Read message
				_, msg, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						return
					}
					t.Logf("Failed to read message: %v", err)
					return
				}

				// Handle request
				responseBytes, err := handleWebSocketRequest(t, mockServer, msg)
				require.NoError(t, err)

				// Send response
				err = conn.WriteMessage(websocket.TextMessage, responseBytes)
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						return
					}
					t.Logf("Failed to write message: %v", err)
					return
				}
			}
		}))
		defer server.Close()

		// Create WebSocket client
		wsURL := "ws" + server.URL[4:] // Convert http:// to ws://
		client, err := ws.NewWebSocketClient(wsURL)
		require.NoError(t, err)
		defer client.Close()

		client.SetInitializeParams(&types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.ClientInfo{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		// Initialize the client
		err = client.Initialize(context.Background())
		require.NoError(t, err)

		// Send empty batch request
		responses, err := client.SendBatchRequest(context.Background(), nil)
		require.Error(t, err)
		require.Nil(t, responses)
		require.Contains(t, err.Error(), "empty batch request")

		// Close the client and wait for the server to finish
		client.Close()
		wg.Wait()
	})
}
