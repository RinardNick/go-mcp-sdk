package chat

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestChatClient(t *testing.T) {
	// Create a mock server executable
	serverPath := filepath.Join(t.TempDir(), "mock_server")
	if err := os.WriteFile(serverPath, []byte("#!/bin/sh\ncat"), 0755); err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}

	// Create stdio client
	client, err := stdio.NewStdioClient(serverPath)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Create test input/output buffers
	var stdin bytes.Buffer
	var stdout bytes.Buffer

	// Create session
	session, err := stdio.NewSession(&stdin, &stdout, client)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	// Test tool execution
	toolCall := types.ToolCall{
		Name: "test_tool",
		Parameters: map[string]interface{}{
			"param1": "test",
		},
	}

	result, err := session.ExecuteTool(context.Background(), toolCall)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	if result == nil {
		t.Fatal("Expected tool result, got nil")
	}
}
