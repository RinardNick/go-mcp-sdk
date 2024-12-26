package stdio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/RinardNick/go-mcp-sdk/pkg/client"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

// StdioClient implements the Client interface using stdio communication
type StdioClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex
	nextID int64

	// Response handling
	respMu      sync.Mutex
	respReaders map[int64]chan *types.Response
	done        chan struct{}
}

// request represents a JSON-RPC request
type request struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int64       `json:"id"`
}

// notification represents a JSON-RPC notification (no ID)
type notification struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// batchRequest represents a JSON-RPC batch request
type batchRequest struct {
	Requests []request `json:"requests"`
}

// batchResponse represents a JSON-RPC batch response
type batchResponse struct {
	Responses []types.Response `json:"responses"`
}

// NewStdioClient creates a new stdio client
func NewStdioClient(command string, args ...string) (*StdioClient, error) {
	cmd := exec.Command(command, args...)

	// Set up pipes
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	client := &StdioClient{
		cmd:         cmd,
		stdin:       stdinPipe,
		stdout:      stdoutPipe,
		respReaders: make(map[int64]chan *types.Response),
		done:        make(chan struct{}),
	}

	// Start response reader goroutine
	go client.readResponses()

	// Set up stderr logging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	return client, nil
}

// readResponses reads responses from stdout and routes them to the appropriate reader
func (c *StdioClient) readResponses() {
	defer func() {
		c.respMu.Lock()
		for _, ch := range c.respReaders {
			close(ch)
		}
		c.respReaders = nil
		c.respMu.Unlock()
	}()

	decoder := json.NewDecoder(c.stdout)
	for {
		select {
		case <-c.done:
			return
		default:
			// Try to decode the response
			var rawMsg json.RawMessage
			if err := decoder.Decode(&rawMsg); err != nil {
				if err == io.EOF {
					return
				}
				fmt.Printf("STDIO: Error decoding response: %v\n", err)
				continue
			}

			// Try to decode as batch response
			var batchResp batchResponse
			if err := json.Unmarshal(rawMsg, &batchResp); err == nil && len(batchResp.Responses) > 0 {
				c.respMu.Lock()
				for _, resp := range batchResp.Responses {
					if ch, ok := c.respReaders[resp.ID]; ok {
						ch <- &resp
						delete(c.respReaders, resp.ID)
					}
				}
				c.respMu.Unlock()
				continue
			}

			// Try single response
			var resp types.Response
			if err := json.Unmarshal(rawMsg, &resp); err != nil {
				fmt.Printf("STDIO: Error decoding response: %v\n", err)
				continue
			}

			c.respMu.Lock()
			if ch, ok := c.respReaders[resp.ID]; ok {
				ch <- &resp
				delete(c.respReaders, resp.ID)
			}
			c.respMu.Unlock()
		}
	}
}

// NewSession creates a new session with this client
func (c *StdioClient) NewSession(reader io.Reader, writer io.Writer) (*client.Session, error) {
	return client.NewSession(reader, writer, c)
}

// SendRequest sends a JSON-RPC request and returns the response
func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (*types.Response, error) {
	c.mu.Lock()
	id := atomic.AddInt64(&c.nextID, 1)
	req := request{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}

	// Create response channel
	respCh := make(chan *types.Response, 1)
	c.respMu.Lock()
	c.respReaders[id] = respCh
	c.respMu.Unlock()
	c.mu.Unlock()

	// Send request
	fmt.Printf("STDIO: Sending request %d: %+v\n", id, req)
	if err := json.NewEncoder(c.stdin).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}
	fmt.Printf("STDIO: Request %d sent\n", id)

	// Wait for response or context cancellation
	select {
	case <-ctx.Done():
		c.respMu.Lock()
		delete(c.respReaders, id)
		c.respMu.Unlock()
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp, nil
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (c *StdioClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	notif := notification{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
	}

	return json.NewEncoder(c.stdin).Encode(notif)
}

// SendBatchRequest sends multiple JSON-RPC requests as a batch
func (c *StdioClient) SendBatchRequest(ctx context.Context, methods []string, params []interface{}) ([]types.Response, error) {
	if len(methods) != len(params) {
		return nil, fmt.Errorf("number of methods does not match number of parameters")
	}

	c.mu.Lock()
	var requests []request
	var respChannels []chan *types.Response

	// Create requests and response channels
	for i, method := range methods {
		id := atomic.AddInt64(&c.nextID, 1)
		requests = append(requests, request{
			Jsonrpc: "2.0",
			Method:  method,
			Params:  params[i],
			ID:      id,
		})

		// Create response channel for each request
		respCh := make(chan *types.Response, 1)
		c.respMu.Lock()
		c.respReaders[id] = respCh
		c.respMu.Unlock()
		respChannels = append(respChannels, respCh)
	}

	// Send batch request
	batch := batchRequest{Requests: requests}
	if err := json.NewEncoder(c.stdin).Encode(batch); err != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to encode batch request: %w", err)
	}
	c.mu.Unlock()

	// Wait for all responses or context cancellation
	responses := make([]types.Response, 0, len(requests))
	for i, ch := range respChannels {
		select {
		case <-ctx.Done():
			// Clean up remaining response channels
			c.respMu.Lock()
			for j := i; j < len(requests); j++ {
				delete(c.respReaders, requests[j].ID)
			}
			c.respMu.Unlock()
			return nil, ctx.Err()
		case resp := <-ch:
			if resp == nil {
				return nil, fmt.Errorf("response channel closed before receiving response")
			}
			responses = append(responses, *resp)
		}
	}

	return responses, nil
}

// ListResources implements the Client interface
func (c *StdioClient) ListResources(ctx context.Context) ([]types.Resource, error) {
	resp, err := c.SendRequest(ctx, "mcp/list_resources", nil)
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

// ListTools implements the Client interface
func (c *StdioClient) ListTools(ctx context.Context) ([]types.Tool, error) {
	resp, err := c.SendRequest(ctx, "mcp/list_tools", nil)
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

// ExecuteTool implements the Client interface
func (c *StdioClient) ExecuteTool(ctx context.Context, call types.ToolCall) (*types.ToolResult, error) {
	resp, err := c.SendRequest(ctx, "mcp/call_tool", call)
	if err != nil {
		return nil, err
	}

	var result types.ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	return &result, nil
}

// Close shuts down the client and cleans up resources
func (c *StdioClient) Close() error {
	select {
	case <-c.done:
		// Already closed
		return nil
	default:
		close(c.done)
	}

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		// Send interrupt signal first
		if err := c.cmd.Process.Signal(os.Interrupt); err != nil {
			// If interrupt fails, force kill
			c.cmd.Process.Kill()
		}
		// Wait for process to exit
		c.cmd.Wait()
	}
	return nil
}
