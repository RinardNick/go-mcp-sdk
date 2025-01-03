package client_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestClientMethodNameCompatibility(t *testing.T) {
	t.Run("Client Automatically Tries Both Method Names", func(t *testing.T) {
		// Create pipes for communication
		clientReader, serverWriter := io.Pipe()
		serverReader, clientWriter := io.Pipe()

		// Create a mock server that handles initialization and tool listing
		go func() {
			decoder := json.NewDecoder(serverReader)
			encoder := json.NewEncoder(serverWriter)

			// Handle initialization request
			var initReq struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
				ID      int64           `json:"id"`
			}
			err := decoder.Decode(&initReq)
			require.NoError(t, err)
			require.Equal(t, "initialize", initReq.Method)

			// Return initialization response
			initResult := types.InitializeResult{
				ProtocolVersion: "1.0",
				Capabilities: types.ServerCapabilities{
					Tools: &types.ToolsCapability{
						ListChanged: false,
					},
				},
				ServerInfo: types.Implementation{
					Name:    "test-server",
					Version: "1.0",
				},
			}
			resultBytes, err := json.Marshal(initResult)
			require.NoError(t, err)

			err = encoder.Encode(&types.Response{
				Jsonrpc: "2.0",
				Result:  resultBytes,
				ID:      initReq.ID,
			})
			require.NoError(t, err)

			// Handle initialized notification
			var initNotif struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
			}
			err = decoder.Decode(&initNotif)
			require.NoError(t, err)
			require.Equal(t, "notifications/initialized", initNotif.Method)

			// Read first request (tools/list)
			var req1 struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
				ID      int64           `json:"id"`
			}
			err = decoder.Decode(&req1)
			require.NoError(t, err)
			require.Equal(t, "tools/list", req1.Method)

			// Return method not found error
			err = encoder.Encode(&types.Response{
				Jsonrpc: "2.0",
				Error:   types.MethodNotFoundError("Method not found: tools/list"),
				ID:      req1.ID,
			})
			require.NoError(t, err)

			// Read second request (mcp/list_tools)
			var req2 struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
				ID      int64           `json:"id"`
			}
			err = decoder.Decode(&req2)
			require.NoError(t, err)
			require.Equal(t, "mcp/list_tools", req2.Method)

			// Return success response with tools
			tools := []types.Tool{
				{
					Name:        "test-tool",
					Description: "A test tool",
					InputSchema: []byte("{}"),
				},
			}
			toolsBytes, err := json.Marshal(tools)
			require.NoError(t, err)

			err = encoder.Encode(&types.Response{
				Jsonrpc: "2.0",
				Result:  toolsBytes,
				ID:      req2.ID,
			})
			require.NoError(t, err)
		}()

		// Create and initialize the client
		client := stdio.NewStdioClient(clientWriter, clientReader)
		client.SetInitializeParams(&types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.ClientInfo{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		err := client.Initialize(context.Background())
		require.NoError(t, err)

		// Try to list tools - should automatically try both method names
		tools, err := client.ListTools(context.Background())
		require.NoError(t, err)
		require.Len(t, tools, 1)
		require.Equal(t, "test-tool", tools[0].Name)

		// Clean up
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})

	t.Run("Both Method Names Fail", func(t *testing.T) {
		// Create pipes for communication
		clientReader, serverWriter := io.Pipe()
		serverReader, clientWriter := io.Pipe()

		// Create a mock server that handles initialization and fails both tool listing methods
		go func() {
			decoder := json.NewDecoder(serverReader)
			encoder := json.NewEncoder(serverWriter)

			// Handle initialization request
			var initReq struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
				ID      int64           `json:"id"`
			}
			err := decoder.Decode(&initReq)
			require.NoError(t, err)
			require.Equal(t, "initialize", initReq.Method)

			// Return initialization response
			initResult := types.InitializeResult{
				ProtocolVersion: "1.0",
				Capabilities: types.ServerCapabilities{
					Tools: &types.ToolsCapability{
						ListChanged: false,
					},
				},
				ServerInfo: types.Implementation{
					Name:    "test-server",
					Version: "1.0",
				},
			}
			resultBytes, err := json.Marshal(initResult)
			require.NoError(t, err)

			err = encoder.Encode(&types.Response{
				Jsonrpc: "2.0",
				Result:  resultBytes,
				ID:      initReq.ID,
			})
			require.NoError(t, err)

			// Handle initialized notification
			var initNotif struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
			}
			err = decoder.Decode(&initNotif)
			require.NoError(t, err)
			require.Equal(t, "notifications/initialized", initNotif.Method)

			// Read first request (tools/list)
			var req1 struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
				ID      int64           `json:"id"`
			}
			err = decoder.Decode(&req1)
			require.NoError(t, err)
			require.Equal(t, "tools/list", req1.Method)

			// Return method not found error for tools/list
			err = encoder.Encode(&types.Response{
				Jsonrpc: "2.0",
				Error:   types.MethodNotFoundError("Method not found: tools/list"),
				ID:      req1.ID,
			})
			require.NoError(t, err)

			// Read second request (mcp/list_tools)
			var req2 struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
				ID      int64           `json:"id"`
			}
			err = decoder.Decode(&req2)
			require.NoError(t, err)
			require.Equal(t, "mcp/list_tools", req2.Method)

			// Return method not found error for mcp/list_tools
			err = encoder.Encode(&types.Response{
				Jsonrpc: "2.0",
				Error:   types.MethodNotFoundError("Method not found: mcp/list_tools"),
				ID:      req2.ID,
			})
			require.NoError(t, err)
		}()

		// Create and initialize the client
		client := stdio.NewStdioClient(clientWriter, clientReader)
		client.SetInitializeParams(&types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.ClientInfo{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		err := client.Initialize(context.Background())
		require.NoError(t, err)

		// Try to list tools - should fail with error indicating both methods failed
		_, err = client.ListTools(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to list tools")

		// Clean up
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})

	t.Run("Caches Successful Method Name", func(t *testing.T) {
		// Create pipes for communication
		clientReader, serverWriter := io.Pipe()
		serverReader, clientWriter := io.Pipe()

		// Track the number of times each method is called
		methodCalls := make(map[string]int)

		// Create a mock server that handles initialization and tool listing
		go func() {
			decoder := json.NewDecoder(serverReader)
			encoder := json.NewEncoder(serverWriter)

			// Handle initialization request
			var initReq struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
				ID      int64           `json:"id"`
			}
			err := decoder.Decode(&initReq)
			require.NoError(t, err)
			require.Equal(t, "initialize", initReq.Method)

			// Return initialization response
			initResult := types.InitializeResult{
				ProtocolVersion: "1.0",
				Capabilities: types.ServerCapabilities{
					Tools: &types.ToolsCapability{
						ListChanged: false,
					},
				},
				ServerInfo: types.Implementation{
					Name:    "test-server",
					Version: "1.0",
				},
			}
			resultBytes, err := json.Marshal(initResult)
			require.NoError(t, err)

			err = encoder.Encode(&types.Response{
				Jsonrpc: "2.0",
				Result:  resultBytes,
				ID:      initReq.ID,
			})
			require.NoError(t, err)

			// Handle initialized notification
			var initNotif struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
			}
			err = decoder.Decode(&initNotif)
			require.NoError(t, err)
			require.Equal(t, "notifications/initialized", initNotif.Method)

			// Handle tool listing requests
			for {
				var req struct {
					Jsonrpc string          `json:"jsonrpc"`
					Method  string          `json:"method"`
					Params  json.RawMessage `json:"params,omitempty"`
					ID      int64           `json:"id"`
				}
				err = decoder.Decode(&req)
				if err != nil {
					return
				}

				methodCalls[req.Method]++

				switch req.Method {
				case "tools/list":
					// Return method not found error
					err = encoder.Encode(&types.Response{
						Jsonrpc: "2.0",
						Error:   types.MethodNotFoundError("Method not found: tools/list"),
						ID:      req.ID,
					})
					require.NoError(t, err)

				case "mcp/list_tools":
					// Return success response
					tools := []types.Tool{
						{
							Name:        "test-tool",
							Description: "A test tool",
							InputSchema: []byte("{}"),
						},
					}
					toolsBytes, err := json.Marshal(tools)
					require.NoError(t, err)

					err = encoder.Encode(&types.Response{
						Jsonrpc: "2.0",
						Result:  toolsBytes,
						ID:      req.ID,
					})
					require.NoError(t, err)
				}
			}
		}()

		// Create and initialize the client
		client := stdio.NewStdioClient(clientWriter, clientReader)
		client.SetInitializeParams(&types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.ClientInfo{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		err := client.Initialize(context.Background())
		require.NoError(t, err)

		// First call should try both methods
		tools, err := client.ListTools(context.Background())
		require.NoError(t, err)
		require.Len(t, tools, 1)
		require.Equal(t, "test-tool", tools[0].Name)
		require.Equal(t, 1, methodCalls["tools/list"], "tools/list should be called once")
		require.Equal(t, 1, methodCalls["mcp/list_tools"], "mcp/list_tools should be called once")

		// Second call should only use the cached successful method
		tools, err = client.ListTools(context.Background())
		require.NoError(t, err)
		require.Len(t, tools, 1)
		require.Equal(t, "test-tool", tools[0].Name)
		require.Equal(t, 1, methodCalls["tools/list"], "tools/list should still be called once")
		require.Equal(t, 2, methodCalls["mcp/list_tools"], "mcp/list_tools should be called twice")

		// Clean up
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})

	t.Run("Other Methods Not Affected", func(t *testing.T) {
		// Create pipes for communication
		clientReader, serverWriter := io.Pipe()
		serverReader, clientWriter := io.Pipe()

		// Track the number of times each method is called
		methodCalls := make(map[string]int)

		// Create a mock server that handles initialization and tool execution
		go func() {
			decoder := json.NewDecoder(serverReader)
			encoder := json.NewEncoder(serverWriter)

			// Handle initialization request
			var initReq struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
				ID      int64           `json:"id"`
			}
			err := decoder.Decode(&initReq)
			require.NoError(t, err)
			require.Equal(t, "initialize", initReq.Method)

			// Return initialization response
			initResult := types.InitializeResult{
				ProtocolVersion: "1.0",
				Capabilities: types.ServerCapabilities{
					Tools: &types.ToolsCapability{
						ListChanged: false,
					},
				},
				ServerInfo: types.Implementation{
					Name:    "test-server",
					Version: "1.0",
				},
			}
			resultBytes, err := json.Marshal(initResult)
			require.NoError(t, err)

			err = encoder.Encode(&types.Response{
				Jsonrpc: "2.0",
				Result:  resultBytes,
				ID:      initReq.ID,
			})
			require.NoError(t, err)

			// Handle initialized notification
			var initNotif struct {
				Jsonrpc string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params,omitempty"`
			}
			err = decoder.Decode(&initNotif)
			require.NoError(t, err)
			require.Equal(t, "notifications/initialized", initNotif.Method)

			// Handle subsequent requests
			for {
				var req struct {
					Jsonrpc string          `json:"jsonrpc"`
					Method  string          `json:"method"`
					Params  json.RawMessage `json:"params,omitempty"`
					ID      int64           `json:"id"`
				}
				err = decoder.Decode(&req)
				if err != nil {
					return
				}

				methodCalls[req.Method]++

				switch req.Method {
				case "tools/call":
					// Verify it's not trying any alternative method names
					var params struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments"`
					}
					err = json.Unmarshal(req.Params, &params)
					require.NoError(t, err)
					require.Equal(t, "test-tool", params.Name)

					// Return success response
					result := map[string]interface{}{
						"result": "success",
					}
					resultBytes, err := json.Marshal(result)
					require.NoError(t, err)

					err = encoder.Encode(&types.Response{
						Jsonrpc: "2.0",
						Result:  resultBytes,
						ID:      req.ID,
					})
					require.NoError(t, err)

				case "mcp/call_tool":
					// This should never be called
					t.Error("mcp/call_tool was called when it shouldn't have been")
				}
			}
		}()

		// Create and initialize the client
		client := stdio.NewStdioClient(clientWriter, clientReader)
		client.SetInitializeParams(&types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.ClientInfo{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		err := client.Initialize(context.Background())
		require.NoError(t, err)

		// Execute a tool - should only try tools/call
		toolCall := types.ToolCall{
			Name:       "test-tool",
			Parameters: map[string]interface{}{},
		}
		result, err := client.ExecuteTool(context.Background(), toolCall)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify only tools/call was used
		require.Equal(t, 1, methodCalls["tools/call"], "tools/call should be called exactly once")
		require.Equal(t, 0, methodCalls["mcp/call_tool"], "mcp/call_tool should not be called")

		// Clean up
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})
}
