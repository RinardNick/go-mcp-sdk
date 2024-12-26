package stdio

import (
	"context"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/test/testutil"
)

func TestStdioClient(t *testing.T) {
	// Get test server binary
	serverBin, err := testutil.GetTestServer()
	if err != nil {
		t.Fatalf("Failed to get test server: %v", err)
	}

	// Create stdio client
	client, err := stdio.NewStdioClient(serverBin)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

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

	t.Run("Invalid JSON-RPC Version", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Send request with invalid version
		resp, err := client.SendRequest(ctx, "test_method", nil)
		if err == nil {
			t.Fatal("Expected error for invalid JSON-RPC version")
		}
		if resp != nil {
			t.Fatal("Expected nil response for invalid JSON-RPC version")
		}
	})

	t.Run("Batch Request", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		methods := []string{"mcp/list_tools", "mcp/list_resources"}
		params := []interface{}{nil, nil}

		responses, err := client.SendBatchRequest(ctx, methods, params)
		if err != nil {
			t.Fatalf("Batch request failed: %v", err)
		}

		if len(responses) != len(methods) {
			t.Fatalf("Expected %d responses, got %d", len(methods), len(responses))
		}

		// Verify each response
		for i, resp := range responses {
			if resp.Jsonrpc != "2.0" {
				t.Errorf("Response %d: expected JSON-RPC version 2.0, got %s", i, resp.Jsonrpc)
			}
			if resp.Error != nil {
				t.Errorf("Response %d: unexpected error: %v", i, resp.Error)
			}
		}
	})

	t.Run("Notification", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.SendNotification(ctx, "test_notification", map[string]string{
			"message": "test",
		})
		if err != nil {
			t.Fatalf("Failed to send notification: %v", err)
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
