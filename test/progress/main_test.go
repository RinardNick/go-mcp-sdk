package progress

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/RinardNick/go-mcp-sdk/test/testutil"
)

func TestProgressReporting(t *testing.T) {
	// Get test server binary
	serverBin, err := testutil.GetTestServer()
	if err != nil {
		t.Fatalf("Failed to get test server: %v", err)
	}

	// Start the server process
	cmd := exec.Command(serverBin)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to get stderr pipe: %v", err)
	}

	// Start the server
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer cmd.Process.Kill()

	// Create stdio client
	client := stdio.NewStdioClient(stdin, stdout)

	// Set initialization parameters with progress support
	client.SetInitializeParams(&types.InitializeParams{
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
	})

	// Copy server stderr to os.Stderr for debugging
	go func() {
		io.Copy(os.Stderr, stderr)
	}()

	// Initialize the client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	// Test progress reporting
	t.Run("Progress Notifications", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		var progressUpdates []types.Progress
		progressMutex := sync.Mutex{}

		// Set up progress handler
		client.OnProgress(func(p types.Progress) {
			progressMutex.Lock()
			progressUpdates = append(progressUpdates, p)
			progressMutex.Unlock()
			if p.Current == p.Total {
				wg.Done()
			}
		})

		// Execute a long-running tool that reports progress
		result, err := client.ExecuteTool(ctx, types.ToolCall{
			Name: "long_operation",
			Parameters: map[string]interface{}{
				"steps": 5,
			},
		})
		if err != nil {
			t.Fatalf("Failed to execute tool: %v", err)
		}

		// Wait for all progress updates
		wg.Wait()

		// Verify progress updates
		progressMutex.Lock()
		defer progressMutex.Unlock()

		if len(progressUpdates) == 0 {
			t.Fatal("Expected progress updates, got none")
		}

		// Verify progress sequence
		for i, p := range progressUpdates {
			if p.ToolID != "long_operation" {
				t.Errorf("Expected tool ID 'long_operation', got '%s'", p.ToolID)
			}
			if p.Current != i+1 {
				t.Errorf("Expected current progress %d, got %d", i+1, p.Current)
			}
			if p.Total != 5 {
				t.Errorf("Expected total progress 5, got %d", p.Total)
			}
			if p.Message == "" {
				t.Error("Expected non-empty progress message")
			}
		}

		// Verify final result
		var resultMap struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(result.Result, &resultMap); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if resultMap.IsError {
			t.Error("Expected success response")
		}

		if len(resultMap.Content) == 0 {
			t.Fatal("Expected non-empty content")
		}

		if resultMap.Content[0].Text != "Operation completed" {
			t.Errorf("Expected 'Operation completed', got '%s'", resultMap.Content[0].Text)
		}
	})
}
