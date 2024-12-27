package int_typescript_sdk

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestTypeScriptSDKIntegration(t *testing.T) {
	// Setup TypeScript server
	t.Log("Setting up TypeScript SDK...")
	setupCmd := exec.Command("bash", "../../scripts/setup_ts_integration.sh")
	setupCmd.Stdout = os.Stdout
	setupCmd.Stderr = os.Stderr
	err := setupCmd.Run()
	require.NoError(t, err, "Failed to setup TypeScript SDK")

	// Start the TypeScript server
	t.Log("Starting TypeScript server...")
	serverDir := filepath.Join("ts")
	cmd := exec.Command("node", "dist/server.js")
	cmd.Dir = serverDir

	// Setup pipes for communication
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err, "Failed to create stdin pipe")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err, "Failed to create stdout pipe")
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err, "Failed to create stderr pipe")

	// Start server process
	err = cmd.Start()
	require.NoError(t, err, "Failed to start TypeScript server")

	// Copy stderr to a buffer for debugging
	go func() {
		io.Copy(os.Stderr, stderr)
	}()

	// Ensure server is killed after tests
	defer func() {
		t.Log("Cleaning up TypeScript server...")
		if err := cmd.Process.Kill(); err != nil {
			t.Logf("Warning: Failed to kill TypeScript server: %v", err)
		}
	}()

	// Create Go SDK client
	client := stdio.NewClient(stdin, stdout)

	// Give server time to start
	time.Sleep(2 * time.Second)

	// Run subtests
	t.Run("Protocol", func(t *testing.T) {
		t.Run("Initialize", func(t *testing.T) {
			ctx := context.Background()
			err := client.Initialize(ctx)
			require.NoError(t, err, "Failed to initialize client")
		})
	})

	t.Run("Tools", func(t *testing.T) {
		t.Run("ListTools", func(t *testing.T) {
			ctx := context.Background()
			tools, err := client.ListTools(ctx)
			require.NoError(t, err, "Failed to list tools")
			require.Len(t, tools, 1, "Expected exactly one tool")

			weatherTool := tools[0]
			assert.Equal(t, "get_weather", weatherTool.Name)
			assert.Contains(t, weatherTool.Description, "weather")

			// Verify tool parameters schema
			var schema map[string]interface{}
			err = json.Unmarshal(weatherTool.InputSchema, &schema)
			require.NoError(t, err, "Failed to unmarshal tool parameters")

			assert.Equal(t, "object", schema["type"])
			properties, ok := schema["properties"].(map[string]interface{})
			require.True(t, ok, "Expected properties in schema")

			location, ok := properties["location"].(map[string]interface{})
			require.True(t, ok, "Expected location in properties")
			assert.Equal(t, "string", location["type"])

			required, ok := schema["required"].([]interface{})
			require.True(t, ok, "Expected required fields in schema")
			assert.Contains(t, required, "location")
		})

		t.Run("ExecuteTool", func(t *testing.T) {
			ctx := context.Background()
			params := map[string]interface{}{
				"location": "San Francisco",
			}

			result, err := client.ExecuteTool(ctx, types.ToolCall{
				Name:       "get_weather",
				Parameters: params,
			})
			require.NoError(t, err, "Failed to execute tool")

			var weather struct {
				Temperature int    `json:"temperature"`
				Condition   string `json:"condition"`
				Location    string `json:"location"`
			}
			err = json.Unmarshal(result.Result, &weather)
			require.NoError(t, err, "Failed to unmarshal weather result")

			assert.Equal(t, "San Francisco", weather.Location)
			assert.Equal(t, 72, weather.Temperature)
			assert.Equal(t, "sunny", weather.Condition)
		})
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		t.Run("InvalidToolName", func(t *testing.T) {
			ctx := context.Background()
			params := map[string]interface{}{
				"location": "San Francisco",
			}

			_, err = client.ExecuteTool(ctx, types.ToolCall{
				Name:       "invalid_tool",
				Parameters: params,
			})
			require.Error(t, err, "Expected error for invalid tool")
			assert.Contains(t, err.Error(), "invalid")
		})

		t.Run("InvalidParameters", func(t *testing.T) {
			ctx := context.Background()
			params := map[string]interface{}{
				"invalid": "parameter",
			}

			_, err = client.ExecuteTool(ctx, types.ToolCall{
				Name:       "get_weather",
				Parameters: params,
			})
			require.Error(t, err, "Expected error for invalid parameters")
			assert.Contains(t, err.Error(), "Invalid location parameter")
		})

		t.Run("MissingParameters", func(t *testing.T) {
			ctx := context.Background()
			params := map[string]interface{}{}

			_, err = client.ExecuteTool(ctx, types.ToolCall{
				Name:       "get_weather",
				Parameters: params,
			})
			require.Error(t, err, "Expected error for missing parameters")
			assert.Contains(t, err.Error(), "Invalid location parameter")
		})
	})
}
