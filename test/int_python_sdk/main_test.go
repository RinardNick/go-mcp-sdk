package int_python_sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestWeatherServerIntegration(t *testing.T) {
	// Create a virtual environment and install dependencies
	t.Log("Setting up Python environment...")
	weatherServerDir := "weather"
	err := setupPythonEnvironment(weatherServerDir)
	assert.NoError(t, err, "Failed to setup Python environment")

	// Start the Python server
	serverCmd := exec.Command(
		filepath.Join(weatherServerDir, "venv", "bin", "python"),
		"-m", "weather",
	)
	serverCmd.Stderr = os.Stderr

	stdin, err := serverCmd.StdinPipe()
	assert.NoError(t, err, "Failed to get stdin pipe")

	stdout, err := serverCmd.StdoutPipe()
	assert.NoError(t, err, "Failed to get stdout pipe")

	err = serverCmd.Start()
	assert.NoError(t, err, "Failed to start server")
	defer serverCmd.Process.Kill()

	// Create client
	client := stdio.NewStdioClient(stdin, stdout)

	// Set initialization parameters
	client.SetInitializeParams(&types.InitializeParams{
		ProtocolVersion: "0.1.0",
		ClientInfo: types.ClientInfo{
			Name:    "go-mcp-sdk",
			Version: "1.0.0",
		},
		Capabilities: types.ClientCapabilities{
			Tools: &types.ToolCapabilities{
				SupportsProgress:     true,
				SupportsCancellation: true,
			},
		},
	})

	// Test initialization
	t.Run("Initialization", func(t *testing.T) {
		err = client.Initialize(context.Background())
		assert.NoError(t, err, "Failed to initialize client")
	})

	// Test tool listing
	t.Run("List Tools", func(t *testing.T) {
		tools, err := client.ListTools(context.Background())
		assert.NoError(t, err, "Failed to list tools")
		assert.Len(t, tools, 2, "Expected exactly two tools")

		// Verify tool schemas
		for _, tool := range tools {
			assert.NotEmpty(t, tool.Name, "Tool name should not be empty")
			assert.NotEmpty(t, tool.Description, "Tool description should not be empty")
			assert.NotNil(t, tool.InputSchema, "Tool input schema should not be nil")
		}
	})

	// Test tool call with valid parameters
	t.Run("Execute Tool Success", func(t *testing.T) {
		result, err := client.ExecuteTool(context.Background(), types.ToolCall{
			Name: "get-alerts",
			Parameters: map[string]interface{}{
				"state": "CA",
			},
		})
		if err == nil {
			// Success case
			var resultMap struct {
				Content []struct {
					Type    string `json:"type"`
					Text    string `json:"text"`
					IsError bool   `json:"isError"`
				} `json:"content"`
				IsError bool `json:"isError"`
			}
			if err := json.Unmarshal(result.Result, &resultMap); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			if len(resultMap.Content) == 0 {
				t.Fatal("Expected non-empty content")
			}

			if resultMap.IsError {
				t.Errorf("Expected success response, got error: %s", resultMap.Content[0].Text)
			}

			// The response could be either alerts or an error message
			text := resultMap.Content[0].Text
			if !strings.Contains(text, "alerts for CA") && !strings.Contains(text, "Failed to retrieve alerts data") {
				t.Errorf("Expected response about alerts for CA or API error, got: %s", text)
			}
		} else {
			t.Errorf("Failed to execute tool: %v", err)
		}
	})

	// Test tool call with invalid parameters
	t.Run("Execute Tool Invalid Parameters", func(t *testing.T) {
		result, err := client.ExecuteTool(context.Background(), types.ToolCall{
			Name: "get-alerts",
			Parameters: map[string]interface{}{
				"invalid": "parameter",
			},
		})
		if err != nil {
			if !strings.Contains(err.Error(), "Missing state parameter") {
				t.Errorf("Expected missing state parameter error, got: %v", err)
			}
			return
		}

		var resultMap struct {
			Content []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				IsError bool   `json:"isError"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(result.Result, &resultMap); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if !resultMap.IsError {
			t.Error("Expected error response for invalid parameters")
		}

		if len(resultMap.Content) == 0 {
			t.Fatal("Expected non-empty content")
		}

		if !strings.Contains(resultMap.Content[0].Text, "Missing state parameter") {
			t.Errorf("Expected missing state parameter error, got: %s", resultMap.Content[0].Text)
		}
	})

	// Test invalid tool call
	t.Run("Execute Invalid Tool", func(t *testing.T) {
		result, err := client.ExecuteTool(context.Background(), types.ToolCall{
			Name:       "invalid_tool",
			Parameters: map[string]interface{}{},
		})
		if err != nil {
			if !strings.Contains(err.Error(), "Missing arguments") {
				t.Errorf("Expected missing arguments error, got: %v", err)
			}
			return
		}

		var resultMap struct {
			Content []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				IsError bool   `json:"isError"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(result.Result, &resultMap); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if !resultMap.IsError {
			t.Error("Expected error response for invalid tool")
		}

		if len(resultMap.Content) == 0 {
			t.Fatal("Expected non-empty content")
		}

		if !strings.Contains(resultMap.Content[0].Text, "Missing arguments") {
			t.Errorf("Expected missing arguments error, got: %s", resultMap.Content[0].Text)
		}
	})

	// Test tool call with progress
	t.Run("Execute Tool With Progress", func(t *testing.T) {
		result, err := client.ExecuteTool(context.Background(), types.ToolCall{
			Name: "get-forecast",
			Parameters: map[string]interface{}{
				"latitude":  37.7749,
				"longitude": -122.4194,
			},
		})
		if err == nil {
			var resultMap struct {
				Content []struct {
					Type    string `json:"type"`
					Text    string `json:"text"`
					IsError bool   `json:"isError"`
				} `json:"content"`
				IsError bool `json:"isError"`
			}
			if err := json.Unmarshal(result.Result, &resultMap); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			if resultMap.IsError {
				t.Errorf("Expected success response, got error: %s", resultMap.Content[0].Text)
			}

			if len(resultMap.Content) == 0 {
				t.Fatal("Expected non-empty content")
			}

			// The response could be either a forecast or an error message
			text := resultMap.Content[0].Text
			if !strings.Contains(text, "Weather Forecast") && !strings.Contains(text, "Failed to retrieve forecast data") {
				t.Errorf("Expected forecast response or API error, got: %s", text)
			}
		} else {
			t.Errorf("Failed to execute tool: %v", err)
		}
	})

	// Test tool call with cancellation
	t.Run("Execute Tool With Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result, err := client.ExecuteTool(ctx, types.ToolCall{
			Name: "get-forecast",
			Parameters: map[string]interface{}{
				"latitude":  37.7749,
				"longitude": -122.4194,
			},
		})
		if err == nil {
			t.Error("Expected error due to cancellation")
		} else {
			if !strings.Contains(err.Error(), "context canceled") {
				t.Errorf("Expected context canceled error, got: %v", err)
			}
		}
		if result != nil {
			t.Error("Expected nil result for cancelled request")
		}
	})
}

func setupPythonEnvironment(pythonServerDir string) error {
	// Create virtual environment
	cmd := exec.Command("python3", "-m", "venv", filepath.Join(pythonServerDir, "venv"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create virtual environment: %w", err)
	}

	// Install dependencies
	cmd = exec.Command(
		filepath.Join(pythonServerDir, "venv", "bin", "pip"),
		"install",
		"-e",
		pythonServerDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	return nil
}
