package server_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/server/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestMethodNameCompatibility(t *testing.T) {
	t.Run("Server Returns Identical Response For Both Method Names", func(t *testing.T) {
		// Create a server with a test tool
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Capabilities: map[string]interface{}{
				"tools": true,
			},
		})

		// Create input schema for the test tool
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
		assert.NoError(t, err)

		// Register a test tool
		tool := types.Tool{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: schemaBytes,
		}
		err = s.RegisterTool(tool)
		assert.NoError(t, err)

		// Create pipes for communication
		clientReader, serverWriter := io.Pipe()
		serverReader, clientWriter := io.Pipe()

		// Create and start the transport
		transport := stdio.NewTransport(s)
		transport.Reader = serverReader
		transport.Writer = serverWriter

		ctx := context.Background()
		err = transport.Start(ctx)
		assert.NoError(t, err)

		// Helper function to send request and get response
		sendRequest := func(method string, id int64) *types.Response {
			req := &stdio.Request{
				Jsonrpc: "2.0",
				Method:  method,
				ID:      id,
			}
			err := json.NewEncoder(clientWriter).Encode(req)
			assert.NoError(t, err)

			var resp types.Response
			err = json.NewDecoder(clientReader).Decode(&resp)
			assert.NoError(t, err)
			return &resp
		}

		// Test both method names
		toolsListResp := sendRequest("tools/list", 1)
		mcpListToolsResp := sendRequest("mcp/list_tools", 2)

		// Both should succeed
		assert.Nil(t, toolsListResp.Error)
		assert.Nil(t, mcpListToolsResp.Error)

		// Decode and compare the results
		var toolsList, mcpList struct {
			Tools []types.Tool `json:"tools"`
		}
		err = json.Unmarshal(toolsListResp.Result, &toolsList)
		assert.NoError(t, err)
		err = json.Unmarshal(mcpListToolsResp.Result, &mcpList)
		assert.NoError(t, err)

		// Results should be identical
		assert.Equal(t, toolsList, mcpList)
		assert.Equal(t, 1, len(toolsList.Tools))
		assert.Equal(t, "test_tool", toolsList.Tools[0].Name)

		// Clean up
		err = transport.Stop(ctx)
		assert.NoError(t, err)
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})

	t.Run("Server Returns Error When Neither Method Is Supported", func(t *testing.T) {
		// Create a server with no tools support
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Capabilities: map[string]interface{}{
				"tools": false,
			},
		})

		// Create pipes for communication
		clientReader, serverWriter := io.Pipe()
		serverReader, clientWriter := io.Pipe()

		// Create and start the transport
		transport := stdio.NewTransport(s)
		transport.Reader = serverReader
		transport.Writer = serverWriter

		ctx := context.Background()
		err := transport.Start(ctx)
		assert.NoError(t, err)

		// Helper function to send request and get response
		sendRequest := func(method string, id int64) *types.Response {
			req := &stdio.Request{
				Jsonrpc: "2.0",
				Method:  method,
				ID:      id,
			}
			err := json.NewEncoder(clientWriter).Encode(req)
			assert.NoError(t, err)

			var resp types.Response
			err = json.NewDecoder(clientReader).Decode(&resp)
			assert.NoError(t, err)
			return &resp
		}

		// Test both method names
		toolsListResp := sendRequest("tools/list", 1)
		mcpListToolsResp := sendRequest("mcp/list_tools", 2)

		// Both should fail with method not found error
		assert.NotNil(t, toolsListResp.Error)
		assert.NotNil(t, mcpListToolsResp.Error)
		assert.Equal(t, types.ErrMethodNotFound, toolsListResp.Error.Code)
		assert.Equal(t, types.ErrMethodNotFound, mcpListToolsResp.Error.Code)

		// Clean up
		err = transport.Stop(ctx)
		assert.NoError(t, err)
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})

	t.Run("Other Methods Not Affected By Method Name Compatibility", func(t *testing.T) {
		// Create a server with tools support
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Capabilities: map[string]interface{}{
				"tools": true,
			},
		})

		// Create pipes for communication
		clientReader, serverWriter := io.Pipe()
		serverReader, clientWriter := io.Pipe()

		// Create and start the transport
		transport := stdio.NewTransport(s)
		transport.Reader = serverReader
		transport.Writer = serverWriter

		ctx := context.Background()
		err := transport.Start(ctx)
		assert.NoError(t, err)

		// Helper function to send request and get response
		sendRequest := func(method string, id int64) *types.Response {
			req := &stdio.Request{
				Jsonrpc: "2.0",
				Method:  method,
				ID:      id,
			}
			err := json.NewEncoder(clientWriter).Encode(req)
			assert.NoError(t, err)

			var resp types.Response
			err = json.NewDecoder(clientReader).Decode(&resp)
			assert.NoError(t, err)
			return &resp
		}

		// Test a method that should not be affected
		resp := sendRequest("tools/call", 1)

		// Should fail with method not found since no tool is registered
		assert.NotNil(t, resp.Error)
		assert.Equal(t, types.ErrMethodNotFound, resp.Error.Code)

		// Clean up
		err = transport.Stop(ctx)
		assert.NoError(t, err)
		clientReader.Close()
		clientWriter.Close()
		serverReader.Close()
		serverWriter.Close()
	})
}
