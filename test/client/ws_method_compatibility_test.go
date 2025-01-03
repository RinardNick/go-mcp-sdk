package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/ws"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

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

				// Parse request
				var req struct {
					Jsonrpc string          `json:"jsonrpc"`
					Method  string          `json:"method"`
					Params  json.RawMessage `json:"params,omitempty"`
					ID      int64           `json:"id"`
				}
				err = json.Unmarshal(msg, &req)
				require.NoError(t, err)

				// Handle request
				var result json.RawMessage
				var respErr error
				if req.Method == "initialize" {
					result, respErr = mockServer.HandleInitialize(req.Params)
				} else {
					result, respErr = mockServer.HandleRequest(req.Method, req.Params)
				}

				// Create response
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

				// Send response
				responseBytes, err := json.Marshal(response)
				require.NoError(t, err)
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
