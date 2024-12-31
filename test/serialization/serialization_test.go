package serialization

import (
	"encoding/json"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestWireFormatCompatibility(t *testing.T) {
	t.Run("Tool Serialization", func(t *testing.T) {
		// Create a tool with Go-style field names
		tool := types.Tool{
			Name:        "test_tool",
			Description: "A test tool",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"param1": {
						"type": "string",
						"description": "A test parameter"
					}
				},
				"required": ["param1"]
			}`),
		}

		// Serialize to JSON
		data, err := json.Marshal(tool)
		assert.NoError(t, err)

		// Verify wire format uses snake_case
		var wireFormat map[string]interface{}
		err = json.Unmarshal(data, &wireFormat)
		assert.NoError(t, err)

		// Check field names in wire format
		assert.Contains(t, wireFormat, "name")
		assert.Contains(t, wireFormat, "description")
		assert.Contains(t, wireFormat, "input_schema")
	})

	t.Run("Initialize Parameters", func(t *testing.T) {
		// Create initialization parameters with Go-style field names
		params := types.InitializeParams{
			ProtocolVersion: "0.1.0",
			ClientInfo: types.Implementation{
				Name:    "go-mcp-sdk",
				Version: "1.0.0",
			},
			Capabilities: types.ClientCapabilities{
				Tools: &types.ToolCapabilities{
					SupportsProgress:     true,
					SupportsCancellation: true,
				},
			},
		}

		// Serialize to JSON
		data, err := json.Marshal(params)
		assert.NoError(t, err)

		// Verify wire format uses snake_case
		var wireFormat map[string]interface{}
		err = json.Unmarshal(data, &wireFormat)
		assert.NoError(t, err)

		// Check field names in wire format
		assert.Contains(t, wireFormat, "protocol_version")
		assert.Contains(t, wireFormat, "client_info")
		assert.Contains(t, wireFormat, "capabilities")

		// Check nested fields
		clientInfo := wireFormat["client_info"].(map[string]interface{})
		assert.Contains(t, clientInfo, "name")
		assert.Contains(t, clientInfo, "version")

		capabilities := wireFormat["capabilities"].(map[string]interface{})
		tools := capabilities["tools"].(map[string]interface{})
		assert.Contains(t, tools, "supports_progress")
		assert.Contains(t, tools, "supports_cancellation")
	})

	t.Run("Deserialization Compatibility", func(t *testing.T) {
		// Test both snake_case and camelCase wire formats
		snakeCaseJSON := []byte(`{
			"protocol_version": "0.1.0",
			"client_info": {
				"name": "test-client",
				"version": "1.0.0"
			},
			"capabilities": {
				"tools": {
					"supports_progress": true,
					"supports_cancellation": true
				}
			}
		}`)

		camelCaseJSON := []byte(`{
			"protocolVersion": "0.1.0",
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			},
			"capabilities": {
				"tools": {
					"supportsProgress": true,
					"supportsCancellation": true
				}
			}
		}`)

		// Test snake_case deserialization
		var snakeParams types.InitializeParams
		err := json.Unmarshal(snakeCaseJSON, &snakeParams)
		assert.NoError(t, err)
		t.Logf("Snake case params: %+v", snakeParams)
		t.Logf("Snake case client info: %+v", snakeParams.ClientInfo)
		t.Logf("Snake case capabilities: %+v", snakeParams.Capabilities)
		if snakeParams.Capabilities.Tools != nil {
			t.Logf("Snake case tools: %+v", *snakeParams.Capabilities.Tools)
		}
		assert.Equal(t, "0.1.0", snakeParams.ProtocolVersion)
		assert.Equal(t, "test-client", snakeParams.ClientInfo.Name)
		assert.True(t, snakeParams.Capabilities.Tools.SupportsProgress)

		// Test camelCase deserialization
		var camelParams types.InitializeParams
		err = json.Unmarshal(camelCaseJSON, &camelParams)
		assert.NoError(t, err)
		t.Logf("Camel case params: %+v", camelParams)
		t.Logf("Camel case client info: %+v", camelParams.ClientInfo)
		t.Logf("Camel case capabilities: %+v", camelParams.Capabilities)
		if camelParams.Capabilities.Tools != nil {
			t.Logf("Camel case tools: %+v", *camelParams.Capabilities.Tools)
		}
		assert.Equal(t, "0.1.0", camelParams.ProtocolVersion)
		assert.Equal(t, "test-client", camelParams.ClientInfo.Name)
		assert.True(t, camelParams.Capabilities.Tools.SupportsProgress)
	})

	t.Run("Tool Result Compatibility", func(t *testing.T) {
		// Test both snake_case and camelCase wire formats for tool results
		snakeCaseJSON := []byte(`{
			"result": {
				"content": [
					{
						"type": "text",
						"text": "test output"
					}
				],
				"is_error": false
			}
		}`)

		camelCaseJSON := []byte(`{
			"result": {
				"content": [
					{
						"type": "text",
						"text": "test output"
					}
				],
				"isError": false
			}
		}`)

		// Test snake_case deserialization
		var snakeResult types.ToolResult
		err := json.Unmarshal(snakeCaseJSON, &snakeResult)
		assert.NoError(t, err)

		// Test camelCase deserialization
		var camelResult types.ToolResult
		err = json.Unmarshal(camelCaseJSON, &camelResult)
		assert.NoError(t, err)
	})
}
