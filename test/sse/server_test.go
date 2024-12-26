package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	serversse "github.com/RinardNick/go-mcp-sdk/pkg/server/sse"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/r3labs/sse/v2"
)

type testLogger struct {
	buf bytes.Buffer
}

func (l *testLogger) Printf(format string, v ...interface{}) {
	fmt.Fprintf(&l.buf, format+"\n", v...)
}

func TestSSETransport(t *testing.T) {
	// Create a server with some test tools and resources
	s := server.NewServer(nil)

	// Register test tool
	tool := types.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]any{
			"param1": map[string]any{
				"type":        "string",
				"description": "A test parameter",
			},
		},
	}
	if err := s.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Register test handler
	handler := func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
		return &types.ToolResult{
			Result: map[string]interface{}{
				"output": params["param1"],
			},
		}, nil
	}
	if err := s.RegisterToolHandler("test_tool", handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Register test resource
	resource := types.Resource{
		URI:  "test://resource",
		Name: "test_resource",
	}
	if err := s.RegisterResource(resource); err != nil {
		t.Fatalf("Failed to register resource: %v", err)
	}

	// Create transport with test logger
	logger := &testLogger{}
	config := &serversse.Config{
		Address:    ":0", // Let OS choose port
		EventsPath: "/events",
		RPCPath:    "/rpc",
		Logger:     logger,
	}
	transport := serversse.NewTransport(s, config)

	// Start server
	server := httptest.NewServer(transport.GetHandler())
	defer server.Close()

	// Log startup message
	transport.Start()

	// Test RPC endpoint
	t.Run("List Tools", func(t *testing.T) {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "mcp/list_tools",
			"id":      1,
		}
		resp := testRPCRequest(t, server.URL+config.RPCPath, req)

		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result struct {
			Tools []types.Tool `json:"tools"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if len(result.Tools) != 1 {
			t.Fatalf("Expected 1 tool, got %d", len(result.Tools))
		}
		if result.Tools[0].Name != tool.Name {
			t.Errorf("Expected tool name %s, got %s", tool.Name, result.Tools[0].Name)
		}
	})

	t.Run("List Resources", func(t *testing.T) {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "mcp/list_resources",
			"id":      2,
		}
		resp := testRPCRequest(t, server.URL+config.RPCPath, req)

		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result struct {
			Resources []types.Resource `json:"resources"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		if len(result.Resources) != 1 {
			t.Fatalf("Expected 1 resource, got %d", len(result.Resources))
		}
		if result.Resources[0].URI != resource.URI {
			t.Errorf("Expected resource URI %s, got %s", resource.URI, result.Resources[0].URI)
		}
	})

	t.Run("Call Tool", func(t *testing.T) {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "mcp/call_tool",
			"params": map[string]interface{}{
				"name": "test_tool",
				"parameters": map[string]interface{}{
					"param1": "test input",
				},
			},
			"id": 3,
		}
		resp := testRPCRequest(t, server.URL+config.RPCPath, req)

		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}

		var result types.ToolResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}

		output, ok := result.Result.(map[string]interface{})["output"]
		if !ok {
			t.Fatal("Expected output in result")
		}
		if output != "test input" {
			t.Errorf("Expected output 'test input', got %v", output)
		}
	})

	// Test SSE endpoint
	t.Run("SSE Events", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client := sse.NewClient(server.URL + config.EventsPath)
		client.Headers = map[string]string{
			"Cache-Control": "no-cache",
			"Accept":        "text/event-stream",
			"Connection":    "keep-alive",
		}
		client.ReconnectStrategy = nil // Disable reconnection for test

		events := make(chan *sse.Event)
		errChan := make(chan error, 1)

		// Subscribe with context
		go func() {
			if err := client.SubscribeWithContext(ctx, "events", func(msg *sse.Event) {
				select {
				case events <- msg:
				case <-ctx.Done():
				}
			}); err != nil && err != context.Canceled {
				errChan <- fmt.Errorf("subscription error: %v", err)
			}
			close(errChan)
		}()

		// Wait for connection
		time.Sleep(100 * time.Millisecond)

		// Check for subscription errors
		select {
		case err := <-errChan:
			if err != nil {
				t.Fatalf("Failed to subscribe: %v", err)
			}
		default:
			// No error
		}

		// Publish test event
		testEvent := map[string]string{"message": "test"}
		if err := transport.Publish("test", testEvent); err != nil {
			t.Fatalf("Failed to publish event: %v", err)
		}

		// Wait for event with timeout
		select {
		case event := <-events:
			if string(event.Event) != "test" {
				t.Errorf("Expected event type 'test', got '%s'", string(event.Event))
			}

			var data map[string]string
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatalf("Failed to unmarshal event data: %v", err)
			}

			if data["message"] != "test" {
				t.Errorf("Expected message 'test', got '%s'", data["message"])
			}

		case err := <-errChan:
			if err != nil {
				t.Fatalf("Subscription error: %v", err)
			}
			t.Fatal("Subscription ended without receiving event")

		case <-ctx.Done():
			t.Fatal("Timeout waiting for event")
		}

		// Cleanup
		cancel()
		close(events)
	})

	// Verify logging
	if !strings.Contains(logger.buf.String(), "Starting SSE server") {
		t.Error("Expected startup log message")
	}
	if !strings.Contains(logger.buf.String(), "New SSE connection") {
		t.Error("Expected connection log message")
	}
}

func testRPCRequest(t *testing.T, url string, req interface{}) *types.Response {
	reqBody, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var jsonRPCResp types.Response
	if err := json.NewDecoder(resp.Body).Decode(&jsonRPCResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return &jsonRPCResp
}
