package int_python_sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	client := stdio.NewClient(stdin, stdout)

	// Initialize client
	err = client.Initialize(context.Background())
	assert.NoError(t, err, "Failed to initialize client")

	// List tools
	tools, err := client.ListTools(context.Background())
	assert.NoError(t, err, "Failed to list tools")
	assert.Len(t, tools, 2, "Expected exactly two tools")

	// Call get-alerts tool
	toolCall := types.ToolCall{
		Name: "get-alerts",
		Parameters: map[string]interface{}{
			"state": "CA",
		},
	}
	result, err := client.ExecuteTool(context.Background(), toolCall)
	assert.NoError(t, err, "Failed to call get-alerts tool")

	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	err = json.Unmarshal(result.Result, &response)
	assert.NoError(t, err, "Failed to parse alerts result")
	assert.NotEmpty(t, response.Content, "Expected non-empty alerts response")
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
