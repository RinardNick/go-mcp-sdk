package int_python_sdk

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestProtocolVersionNegotiation(t *testing.T) {
	// Create a virtual environment and install dependencies
	t.Log("Setting up Python environment...")
	weatherServerDir := "weather"
	err := setupPythonEnvironment(weatherServerDir)
	assert.NoError(t, err, "Failed to setup Python environment")

	// Test cases for different protocol versions
	testCases := []struct {
		name          string
		version       string
		expectedError bool
		errorContains string
		fallbackWorks bool
	}{
		{
			name:          "Unsupported Version",
			version:       "999.999.999",
			expectedError: false,
			fallbackWorks: true,
		},
		{
			name:          "Malformed Version",
			version:       "invalid",
			expectedError: false,
			fallbackWorks: true,
		},
		{
			name:          "Empty Version",
			version:       "",
			expectedError: true,
			errorContains: "invalid protocol version",
		},
		{
			name:          "Too Many Components",
			version:       "1.0.0.0",
			expectedError: false,
			fallbackWorks: true,
		},
		{
			name:          "Version Mismatch",
			version:       "2.0",
			expectedError: false,
			fallbackWorks: true,
		},
		{
			name:          "Fallback Version",
			version:       "1.0",
			expectedError: false,
			fallbackWorks: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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

			// Create client with test version
			client := stdio.NewStdioClient(stdin, stdout)

			// Set the protocol version for the test
			client.SetInitializeParams(&types.InitializeParams{
				ProtocolVersion: tc.version,
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
			})

			// Test initialization
			err = client.Initialize(context.Background())

			if tc.expectedError {
				assert.Error(t, err, "Expected error for version %s", tc.version)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains, "Error message mismatch")
				}
				return // Skip further tests if we expect an error
			}

			assert.NoError(t, err, "Failed to initialize with version %s", tc.version)

			// If fallback works, test that we can use the client
			if tc.fallbackWorks {
				// Try to list tools to verify the connection works
				tools, err := client.ListTools(context.Background())
				assert.NoError(t, err, "Failed to list tools after fallback")
				assert.NotEmpty(t, tools, "Expected non-empty tools list after fallback")

				// Try to execute a tool to verify full functionality
				result, err := client.ExecuteTool(context.Background(), types.ToolCall{
					Name: "get-alerts",
					Parameters: map[string]interface{}{
						"state": "CA",
					},
				})
				assert.NoError(t, err, "Failed to execute tool after fallback")
				assert.NotNil(t, result, "Expected non-nil result after fallback")
			}
		})
	}
}

func TestProtocolVersionValidation(t *testing.T) {
	tests := []struct {
		name            string
		protocolVersion string
		wantErr         bool
		errorContains   string
	}{
		{
			name:            "Valid version 0.1.0",
			protocolVersion: "0.1.0",
			wantErr:         false,
		},
		{
			name:            "Valid version 1.0",
			protocolVersion: "1.0",
			wantErr:         false,
		},
		{
			name:            "Valid date-based version",
			protocolVersion: "2024-01-01",
			wantErr:         false,
		},
		{
			name:            "Invalid empty version",
			protocolVersion: "",
			wantErr:         true,
			errorContains:   "invalid protocol version",
		},
		{
			name:            "Invalid version format",
			protocolVersion: "invalid",
			wantErr:         false,
		},
		{
			name:            "Invalid version with too many components",
			protocolVersion: "1.2.3.4",
			wantErr:         false,
		},
		{
			name:            "Unsupported version",
			protocolVersion: "2.0.0",
			wantErr:         false,
		},
		{
			name:            "Invalid date-based version (old)",
			protocolVersion: "2023-12-31",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start the Python server
			serverCmd := exec.Command(
				filepath.Join("weather", "venv", "bin", "python"),
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

			// Create client with test version
			client := stdio.NewStdioClient(stdin, stdout)

			// Set the protocol version for the test
			client.SetInitializeParams(&types.InitializeParams{
				ProtocolVersion: tt.protocolVersion,
				ClientInfo: types.Implementation{
					Name:    "test-client",
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
			err = client.Initialize(context.Background())

			if tt.wantErr {
				assert.Error(t, err, "Expected error for version %s", tt.protocolVersion)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "Error message mismatch")
				}
				return
			}

			assert.NoError(t, err, "Failed to initialize with version %s", tt.protocolVersion)

			// Try to list tools to verify the connection works
			tools, err := client.ListTools(context.Background())
			assert.NoError(t, err, "Failed to list tools")
			assert.NotEmpty(t, tools, "Expected non-empty tools list")
		})
	}
}
