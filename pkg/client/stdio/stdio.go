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
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

// StdioClient represents a client that communicates over stdio
type StdioClient struct {
	stdin            io.Writer
	stdout           io.Reader
	nextID           int64
	mu               sync.Mutex
	progressHandler  types.ProgressHandler
	notificationChan chan types.Notification
	pendingRequests  map[int64]chan *Response
	initializeParams *types.InitializeParams
	protocolVersion  string
}

// NewStdioClient creates a new StdioClient
func NewStdioClient(stdin io.Writer, stdout io.Reader) *StdioClient {
	return &StdioClient{
		stdin:            stdin,
		stdout:           stdout,
		nextID:           1,
		pendingRequests:  make(map[int64]chan *Response),
		notificationChan: make(chan types.Notification),
		initializeParams: nil,
		protocolVersion:  "",
	}
}

// OnProgress sets the handler for progress notifications
func (c *StdioClient) OnProgress(handler types.ProgressHandler) {
	c.progressHandler = handler
}

// handleNotification processes notifications from the server
func (c *StdioClient) handleNotification(notification types.Notification) error {
	switch notification.Method {
	case "progress":
		if c.progressHandler != nil {
			var progressNotif types.ProgressNotification
			if err := json.Unmarshal(notification.Params, &progressNotif); err != nil {
				return fmt.Errorf("failed to unmarshal progress notification: %w", err)
			}
			c.progressHandler(progressNotif.Progress)
		}
	default:
		// Forward notification to channel for other handlers
		select {
		case c.notificationChan <- notification:
		default:
			// Drop notification if channel is full
		}
	}
	return nil
}

// readLoop reads messages from the server
func (c *StdioClient) readLoop(ctx context.Context) {
	scanner := bufio.NewScanner(c.stdout)
	// Set a larger buffer size for the scanner
	const maxScannerSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScannerSize)
	scanner.Buffer(buf, maxScannerSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					fmt.Fprintf(os.Stderr, "Scanner error: %v\n", err)
				}
				return
			}
			line := scanner.Text()
			fmt.Fprintf(os.Stderr, "Received line: %s\n", line)

			var msg struct {
				Jsonrpc string           `json:"jsonrpc"`
				Method  string           `json:"method,omitempty"`
				Params  json.RawMessage  `json:"params,omitempty"`
				Result  json.RawMessage  `json:"result,omitempty"`
				Error   *types.MCPError  `json:"error,omitempty"`
				ID      *json.RawMessage `json:"id,omitempty"`
			}

			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to parse JSON: %v\n", err)
				continue
			}

			if msg.ID == nil {
				// This is a notification
				notification := types.Notification{
					Method: msg.Method,
					Params: msg.Params,
				}
				if err := c.handleNotification(notification); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to handle notification: %v\n", err)
				}
				continue
			}

			// This is a response to a request
			var id int64
			if err := json.Unmarshal(*msg.ID, &id); err != nil {
				// Try as float64 (JSON numbers are decoded as float64)
				var floatID float64
				if err := json.Unmarshal(*msg.ID, &floatID); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to parse response ID: %v\n", err)
					continue
				}
				id = int64(floatID)
			}

			c.mu.Lock()
			ch, ok := c.pendingRequests[id]
			if !ok {
				c.mu.Unlock()
				fmt.Fprintf(os.Stderr, "No pending request found for ID %d\n", id)
				continue
			}
			c.mu.Unlock()

			response := &Response{
				Jsonrpc: msg.Jsonrpc,
				Result:  msg.Result,
				Error:   msg.Error,
				ID:      id,
			}

			// Try to send the response with a timeout
			select {
			case ch <- response:
				c.mu.Lock()
				delete(c.pendingRequests, id)
				c.mu.Unlock()
			case <-time.After(100 * time.Millisecond):
				fmt.Fprintf(os.Stderr, "Timeout sending response for ID %d\n", id)
				c.mu.Lock()
				delete(c.pendingRequests, id)
				c.mu.Unlock()
			}
		}
	}
}

// Initialize initializes the client with the server
func (c *StdioClient) Initialize(ctx context.Context) error {
	// Start the read loop
	go c.readLoop(ctx)

	if c.initializeParams == nil {
		return fmt.Errorf("initialize params not set")
	}

	if c.initializeParams.ProtocolVersion == "" {
		return fmt.Errorf("invalid protocol version: version cannot be empty")
	}

	resp, err := c.SendRequest(ctx, "initialize", c.initializeParams)
	if err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Parse the response
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools struct {
				ListChanged bool `json:"listChanged"`
			} `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}

	// Store the negotiated protocol version
	c.protocolVersion = result.ProtocolVersion

	// Send initialized notification
	if err := c.SendNotification(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return nil
}

// ListTools lists all available tools
func (c *StdioClient) ListTools(ctx context.Context) ([]types.Tool, error) {
	resp, err := c.SendRequest(ctx, "mcp/list_tools", nil)
	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded {
			return nil, err
		}
		return nil, fmt.Errorf("failed to list tools: %w", err)
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
func (c *StdioClient) ExecuteTool(ctx context.Context, call types.ToolCall) (*types.ToolResult, error) {
	// Create the request parameters
	params := map[string]interface{}{
		"name":      call.Name,
		"arguments": call.Parameters,
	}

	resp, err := c.SendRequest(ctx, "mcp/call_tool", params)
	if err != nil {
		return nil, fmt.Errorf("failed to send tool call request: %w", err)
	}

	// Parse the response
	var result struct {
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			IsError bool   `json:"isError"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	// Check for error in the response content
	if result.IsError {
		errorMessage := "tool execution failed"
		if len(result.Content) > 0 {
			errorMessage = result.Content[0].Text
		}
		return &types.ToolResult{
			Result: resp.Result,
		}, fmt.Errorf("tool execution failed: %s", errorMessage)
	}

	return &types.ToolResult{
		Result: resp.Result,
	}, nil
}

// SendRequest sends a JSON-RPC request and returns the response
func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (*Response, error) {
	id := atomic.AddInt64(&c.nextID, 1)

	// Create response channel with buffer
	responseChan := make(chan *Response, 1)
	c.mu.Lock()
	c.pendingRequests[id] = responseChan
	c.mu.Unlock()

	// Clean up on function exit
	defer func() {
		c.mu.Lock()
		delete(c.pendingRequests, id)
		close(responseChan)
		c.mu.Unlock()
	}()

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

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Sending request: %s\n", requestBytes)

	c.mu.Lock()
	_, err = fmt.Fprintf(c.stdin, "%s\n", requestBytes)
	c.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response with timeout
	select {
	case response := <-responseChan:
		if response.Error != nil {
			return nil, response.Error
		}
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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

// SendProgress sends a progress notification
func (c *StdioClient) SendProgress(ctx context.Context, progress types.Progress) error {
	return c.SendNotification(ctx, "notifications/progress", progress)
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

// SetInitializeParams sets the initialization parameters for the client
func (c *StdioClient) SetInitializeParams(params *types.InitializeParams) {
	c.initializeParams = params
}

// GetResourceTemplates returns a list of available resource templates
func (c *StdioClient) GetResourceTemplates(ctx context.Context) ([]types.Resource, error) {
	resp, err := c.SendRequest(ctx, "resources/templates/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Templates []types.Resource `json:"templates"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource templates: %w", err)
	}

	return result.Templates, nil
}

// ApplyResourceTemplate applies a resource template with the given parameters
func (c *StdioClient) ApplyResourceTemplate(ctx context.Context, template types.ResourceTemplate) (*types.ResourceTemplateResult, error) {
	resp, err := c.SendRequest(ctx, "resources/templates/apply", template)
	if err != nil {
		return nil, err
	}

	var result types.ResourceTemplateResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template result: %w", err)
	}

	return &result, nil
}
