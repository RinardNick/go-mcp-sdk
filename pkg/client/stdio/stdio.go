package stdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

// StdioClient represents a client that communicates over stdio
type StdioClient struct {
	stdin  io.Writer
	stdout io.Reader
	nextID int64
	mu     sync.Mutex
}

// NewClient creates a new stdio client
func NewClient(stdin io.Writer, stdout io.Reader) *StdioClient {
	return &StdioClient{
		stdin:  stdin,
		stdout: stdout,
	}
}

// Initialize sends the initialization request to the server
func (c *StdioClient) Initialize(ctx context.Context) error {
	fmt.Fprintf(os.Stderr, "Initializing MCP client...\n")
	params := &types.InitializeParams{
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
	}

	fmt.Fprintf(os.Stderr, "Sending initialize request with params: %+v\n", params)
	resp, err := c.SendRequest(ctx, "initialize", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Initialization failed: %v\n", err)
		return fmt.Errorf("initialization failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Initialize response: %+v\n", resp)

	// Send initialized notification
	fmt.Fprintf(os.Stderr, "Sending initialized notification...\n")
	err = c.SendNotification(ctx, "notifications/initialized", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send initialized notification: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Initialized notification sent successfully\n")
	}
	return err
}

// ListTools returns a list of available tools
func (c *StdioClient) ListTools(ctx context.Context) ([]types.Tool, error) {
	resp, err := c.SendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []types.Tool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools: %w", err)
	}

	return result.Tools, nil
}

// ExecuteTool executes a tool with the given parameters
func (c *StdioClient) ExecuteTool(ctx context.Context, toolCall types.ToolCall) (*types.ToolResult, error) {
	params := struct {
		Name       string                 `json:"name"`
		Parameters map[string]interface{} `json:"parameters"`
	}{
		Name:       toolCall.Name,
		Parameters: toolCall.Parameters,
	}

	resp, err := c.SendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			IsError bool   `json:"isError"`
		} `json:"content"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty response from server")
	}

	if result.Content[0].IsError {
		return nil, fmt.Errorf("tool execution failed: %s", result.Content[0].Text)
	}

	return &types.ToolResult{
		Result: resp.Result,
	}, nil
}

// SendRequest sends a JSON-RPC request and returns the response
func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (*Response, error) {
	id := atomic.AddInt64(&c.nextID, 1)

	request := struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      int64       `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		Jsonrpc: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Log request JSON
	requestJSON, _ := json.Marshal(request)
	fmt.Fprintf(os.Stderr, "Sending request: %s\n", string(requestJSON))

	if err := json.NewEncoder(c.stdin).Encode(request); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response, skipping non-JSON lines
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		line := scanner.Text()
		// Log raw line for debugging
		fmt.Fprintf(os.Stderr, "Received line: %s\n", line)
		// Try to decode as JSON
		var response Response
		if err := json.Unmarshal([]byte(line), &response); err == nil {
			// Found valid JSON response
			fmt.Fprintf(os.Stderr, "Found valid JSON response\n")
			if response.Error != nil {
				return nil, response.Error
			}
			return &response, nil
		}
		// Not JSON, treat as log message
		fmt.Fprintf(os.Stderr, "Server log: %s\n", line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return nil, fmt.Errorf("no valid JSON response received")
}

// SendNotification sends a JSON-RPC notification
func (c *StdioClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	notification := struct {
		Jsonrpc string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := json.NewEncoder(c.stdin).Encode(notification); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// Response represents a JSON-RPC response
type Response struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *types.MCPError `json:"error,omitempty"`
}

// Close implements the Client interface
func (c *StdioClient) Close() error {
	// Nothing to close for basic stdin/stdout
	return nil
}

// ListResources returns a list of available resources
func (c *StdioClient) ListResources(ctx context.Context) ([]types.Resource, error) {
	resp, err := c.SendRequest(ctx, "resources/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Resources []types.Resource `json:"resources"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resources: %w", err)
	}

	return result.Resources, nil
}
