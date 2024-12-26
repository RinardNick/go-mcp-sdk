package anthropic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/llm"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/joho/godotenv"
)

func TestAnthropicIntegration(t *testing.T) {
	// Load .env file
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("Failed to load .env file: %v", err)
	}

	// Check if API key is set
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}
	if !strings.HasPrefix(apiKey, "sk-ant") {
		t.Fatal("Invalid API key format")
	}

	// Build test server
	serverPath := filepath.Join("..", "testserver", "main.go")
	if err := runCommand("go", "build", "-o", "testserver", serverPath); err != nil {
		t.Fatalf("Failed to build test server: %v", err)
	}
	defer os.Remove("testserver")

	// Create stdio client
	client, err := stdio.NewStdioClient("./testserver")
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

	// Create Anthropic client
	anthropicClient, err := llm.NewAnthropicClient()
	if err != nil {
		t.Fatalf("Failed to create Anthropic client: %v", err)
	}

	// Get available tools
	tools := session.GetTools()
	if len(tools) == 0 {
		t.Fatal("No tools available from server")
	}
	t.Logf("Available tools: %+v", tools)

	// Test cases
	testCases := []struct {
		name     string
		message  string
		wantTool string
		timeout  time.Duration
	}{
		{
			name:     "Weather query",
			message:  "What's the weather like in San Francisco?",
			wantTool: "get_weather",
			timeout:  30 * time.Second,
		},
		{
			name:     "Weather with location",
			message:  "Tell me the current temperature in New York",
			wantTool: "get_weather",
			timeout:  30 * time.Second,
		},
		{
			name:     "Casual conversation",
			message:  "Hello, how are you?",
			wantTool: "", // No tool call expected
			timeout:  30 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()

			// Create test message
			messages := []types.Message{
				{
					Role:    "user",
					Content: tc.message,
				},
			}

			t.Logf("Sending message: %s", tc.message)

			// Process message
			response, err := anthropicClient.ProcessMessage(ctx, messages, tools)
			if err != nil {
				t.Fatalf("Failed to process message: %v", err)
			}

			t.Logf("Received response: %s", response.Content)

			// Check tool call
			if tc.wantTool == "" {
				if response.ToolCall != nil {
					t.Errorf("Expected no tool call, got %s", response.ToolCall.Name)
				}
				return
			}

			if response.ToolCall == nil {
				t.Fatalf("Expected tool call %s, got none", tc.wantTool)
			}

			if response.ToolCall.Name != tc.wantTool {
				t.Errorf("Expected tool %s, got %s", tc.wantTool, response.ToolCall.Name)
			}

			t.Logf("Tool call: %+v", response.ToolCall)

			// Execute tool if present
			if response.ToolCall != nil {
				result, err := session.ExecuteTool(ctx, *response.ToolCall)
				if err != nil {
					t.Fatalf("Failed to execute tool: %v", err)
				}

				weatherResult, ok := result.Result.(map[string]interface{})
				if !ok {
					t.Fatal("Expected weather result to be a map")
				}

				// Verify required fields
				requiredFields := []string{"temperature", "condition", "location"}
				for _, field := range requiredFields {
					if _, ok := weatherResult[field]; !ok {
						t.Errorf("Missing required field %s in weather result", field)
					}
				}

				t.Logf("Weather result: %+v", weatherResult)
			}
		})

		// Add delay between tests to avoid rate limiting
		time.Sleep(2 * time.Second)
	}
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
