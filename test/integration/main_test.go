package integration

import (
	"context"
	"os"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/llm"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/RinardNick/go-mcp-sdk/test/testutil"
)

func TestWeatherFlow(t *testing.T) {
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

	// Create session
	session, err := stdio.NewSession(os.Stdin, os.Stdout, client)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	// List tools
	tools := session.GetTools()
	if len(tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "test_tool" {
		t.Fatalf("Expected tool name 'test_tool', got '%s'", tools[0].Name)
	}

	// Create test message
	messages := []types.Message{
		{
			Role:    "user",
			Content: "What's the weather like in San Francisco?",
		},
	}

	// Create mock client
	mockClient := llm.NewMockClient()

	// Process message
	response, err := mockClient.ProcessMessage(context.Background(), messages, tools)
	if err != nil {
		t.Fatalf("Failed to process message: %v", err)
	}

	// Verify tool call
	if response.ToolCall == nil {
		t.Fatal("Expected tool call in response")
	}
	if response.ToolCall.Name != "test_tool" {
		t.Fatalf("Expected tool call to 'test_tool', got '%s'", response.ToolCall.Name)
	}

	// Execute tool
	result, err := session.ExecuteTool(context.Background(), *response.ToolCall)
	if err != nil {
		t.Fatalf("Failed to execute tool: %v", err)
	}

	// Verify result
	toolResult, ok := result.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected tool result to be a map")
	}
	if output, ok := toolResult["output"].(string); !ok || output != "test output" {
		t.Fatalf("Expected output 'test output', got '%v'", toolResult["output"])
	}
}
