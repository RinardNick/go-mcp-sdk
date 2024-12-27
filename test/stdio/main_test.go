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
	client := stdio.NewClient(stdin, stdout)

	// Copy server stderr to os.Stderr for debugging
	go func() {
		io.Copy(os.Stderr, stderr)
	}()

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

		var resultMap map[string]interface{}
		if err := json.Unmarshal(result.Result, &resultMap); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		output, ok := resultMap["output"].(string)
		if !ok {
			t.Fatal("Expected string output in result")
		}

		if output != "test output" {
			t.Errorf("Expected output 'test output', got '%s'", output)
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
