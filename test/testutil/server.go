package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var (
	serverBinaryPath string
	buildOnce        sync.Once
	buildError       error
)

// GetTestServer ensures the test server is built and returns the path to the binary
func GetTestServer() (string, error) {
	buildOnce.Do(func() {
		// Get absolute path to test server source
		srcPath, err := filepath.Abs(filepath.Join("..", "testserver", "main.go"))
		if err != nil {
			buildError = fmt.Errorf("failed to get test server source path: %w", err)
			return
		}
		fmt.Printf("Building test server from: %s\n", srcPath)

		// Create binary in temp directory
		serverBinaryPath = filepath.Join(os.TempDir(), "mcp_testserver")
		fmt.Printf("Building test server to: %s\n", serverBinaryPath)

		// Build the server
		cmd := exec.Command("go", "build", "-o", serverBinaryPath, srcPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			buildError = fmt.Errorf("failed to build test server: %w", err)
			return
		}
		fmt.Printf("Test server built successfully\n")
	})

	if buildError != nil {
		return "", buildError
	}

	return serverBinaryPath, nil
}

// Cleanup removes the test server binary
func Cleanup() {
	if serverBinaryPath != "" {
		os.Remove(serverBinaryPath)
	}
}
