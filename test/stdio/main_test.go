package stdio

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/RinardNick/go-mcp-sdk/test/testutil"
)

func TestStdioClient(t *testing.T) {
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

	// Test cases
	t.Run("Basic RPC", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tools, err := client.ListTools(ctx)
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}

		if len(tools) == 0 {
			t.Fatal("Expected at least one tool")
		}

		if tools[0].Name != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got '%s'", tools[0].Name)
		}
	})

	t.Run("Tool Call", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.ExecuteTool(ctx, types.ToolCall{
			Name: "test_tool",
			Parameters: map[string]interface{}{
				"param1": "test",
			},
		})
		if err != nil {
			t.Fatalf("ExecuteTool failed: %v", err)
		}

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

		if len(resultMap.Content) == 0 {
			t.Fatal("Expected non-empty content")
		}
		if resultMap.Content[0].Text != "test output" {
			t.Errorf("Expected output 'test output', got '%s'", resultMap.Content[0].Text)
		}
		if resultMap.IsError {
			t.Error("Expected isError to be false")
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.ListTools(ctx)
		if err != context.Canceled {
			t.Fatalf("Expected context.Canceled error, got: %v", err)
		}
	})
}
