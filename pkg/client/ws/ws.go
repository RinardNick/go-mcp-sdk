package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/gorilla/websocket"
)

type connectionState int32

const (
	stateDisconnected connectionState = iota
	stateConnecting
	stateConnected
)

type WebSocketClient struct {
	url              string
	conn             *websocket.Conn
	nextID           atomic.Int64
	initializeParams *types.InitializeParams
	responseChan     chan []byte
	errorChan        chan error
	mu               sync.Mutex
	listToolsMethod  string
	state            int32
	reconnectBackoff time.Duration
	maxBackoff       time.Duration
	done             chan struct{}
}

// BatchRequest represents a single request in a batch
type BatchRequest struct {
	Method string
	Params interface{}
}

// BatchResponse represents a single response in a batch
type BatchResponse struct {
	Result json.RawMessage
	Error  *types.MCPError
}

func NewWebSocketClient(url string) (*WebSocketClient, error) {
	client := &WebSocketClient{
		url:              url,
		responseChan:     make(chan []byte),
		errorChan:        make(chan error),
		reconnectBackoff: time.Second,
		maxBackoff:       time.Minute,
		done:             make(chan struct{}),
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	// Start the connection manager
	go client.connectionManager()

	return client, nil
}

func (c *WebSocketClient) connect() error {
	if !atomic.CompareAndSwapInt32(&c.state, int32(stateDisconnected), int32(stateConnecting)) {
		return fmt.Errorf("connection already in progress")
	}

	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		atomic.StoreInt32(&c.state, int32(stateDisconnected))
		return fmt.Errorf("failed to connect to WebSocket server: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	atomic.StoreInt32(&c.state, int32(stateConnected))

	// Start reading messages in a goroutine
	go c.readLoop()

	return nil
}

func (c *WebSocketClient) connectionManager() {
	backoff := c.reconnectBackoff
	for {
		select {
		case <-c.done:
			return
		case <-c.errorChan:
			if atomic.LoadInt32(&c.state) == int32(stateConnected) {
				// Connection error occurred, try to reconnect
				c.mu.Lock()
				if c.conn != nil {
					_ = c.conn.Close()
					c.conn = nil
				}
				c.mu.Unlock()
				atomic.StoreInt32(&c.state, int32(stateDisconnected))

				// Wait before attempting to reconnect
				select {
				case <-c.done:
					return
				case <-time.After(backoff):
					if err := c.connect(); err != nil {
						// Increase backoff time for next attempt
						backoff *= 2
						if backoff > c.maxBackoff {
							backoff = c.maxBackoff
						}
						continue
					}
					// Reset backoff on successful connection
					backoff = c.reconnectBackoff
				}
			}
		}
	}
}

func (c *WebSocketClient) readLoop() {
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			c.errorChan <- fmt.Errorf("failed to read message: %w", err)
			return
		}
		c.responseChan <- message
	}
}

func (c *WebSocketClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.done:
		// Channel is already closed
	default:
		close(c.done)
	}

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *WebSocketClient) writeMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || atomic.LoadInt32(&c.state) != int32(stateConnected) {
		return fmt.Errorf("not connected")
	}

	return c.conn.WriteMessage(messageType, data)
}

func (c *WebSocketClient) SetInitializeParams(params *types.InitializeParams) {
	c.initializeParams = params
}

func (c *WebSocketClient) Initialize(ctx context.Context) error {
	if c.initializeParams == nil {
		return fmt.Errorf("initialize params not set")
	}

	id := c.nextID.Add(1)
	request := struct {
		Jsonrpc string                  `json:"jsonrpc"`
		Method  string                  `json:"method"`
		Params  *types.InitializeParams `json:"params"`
		ID      int64                   `json:"id"`
	}{
		Jsonrpc: "2.0",
		Method:  "initialize",
		Params:  c.initializeParams,
		ID:      id,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal initialize request: %w", err)
	}

	if err := c.writeMessage(websocket.TextMessage, requestBytes); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-c.errorChan:
		return err
	case response := <-c.responseChan:
		var resp struct {
			Jsonrpc string          `json:"jsonrpc"`
			Result  json.RawMessage `json:"result"`
			Error   *types.MCPError `json:"error"`
			ID      int64           `json:"id"`
		}
		if err := json.Unmarshal(response, &resp); err != nil {
			return fmt.Errorf("failed to unmarshal initialize response: %w", err)
		}
		if resp.Error != nil {
			return resp.Error
		}
		return nil
	}
}

func (c *WebSocketClient) ListTools(ctx context.Context) ([]types.Tool, error) {
	// Try cached method first
	if c.listToolsMethod != "" {
		tools, err := c.listToolsWithMethod(ctx, c.listToolsMethod)
		if err == nil {
			return tools, nil
		}
	}

	// Try tools/list first
	tools, err := c.listToolsWithMethod(ctx, "tools/list")
	if err == nil {
		c.listToolsMethod = "tools/list"
		return tools, nil
	}

	// Fall back to mcp/list_tools
	tools, err = c.listToolsWithMethod(ctx, "mcp/list_tools")
	if err == nil {
		c.listToolsMethod = "mcp/list_tools"
		return tools, nil
	}

	return nil, fmt.Errorf("both tools/list and mcp/list_tools methods failed")
}

func (c *WebSocketClient) listToolsWithMethod(ctx context.Context, method string) ([]types.Tool, error) {
	id := c.nextID.Add(1)
	request := struct {
		Jsonrpc string `json:"jsonrpc"`
		Method  string `json:"method"`
		ID      int64  `json:"id"`
	}{
		Jsonrpc: "2.0",
		Method:  method,
		ID:      id,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal list tools request: %w", err)
	}

	if err := c.writeMessage(websocket.TextMessage, requestBytes); err != nil {
		return nil, fmt.Errorf("failed to send list tools request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-c.errorChan:
		return nil, err
	case response := <-c.responseChan:
		var resp struct {
			Jsonrpc string          `json:"jsonrpc"`
			Result  json.RawMessage `json:"result"`
			Error   *types.MCPError `json:"error"`
			ID      int64           `json:"id"`
		}
		if err := json.Unmarshal(response, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal list tools response: %w", err)
		}
		if resp.Error != nil {
			return nil, resp.Error
		}

		// Try to unmarshal as array first (mcp/list_tools format)
		var tools []types.Tool
		if err := json.Unmarshal(resp.Result, &tools); err == nil {
			return tools, nil
		}

		// If that fails, try to unmarshal as struct (tools/list format)
		var result struct {
			Tools []types.Tool `json:"tools"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tools: %w", err)
		}

		return result.Tools, nil
	}
}

func (c *WebSocketClient) CallTool(ctx context.Context, name string, args interface{}) (interface{}, error) {
	id := c.nextID.Add(1)
	request := struct {
		Jsonrpc string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params"`
		ID      int64       `json:"id"`
	}{
		Jsonrpc: "2.0",
		Method:  "tools/call",
		Params: struct {
			Name      string      `json:"name"`
			Arguments interface{} `json:"arguments"`
		}{
			Name:      name,
			Arguments: args,
		},
		ID: id,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal call tool request: %w", err)
	}

	if err := c.writeMessage(websocket.TextMessage, requestBytes); err != nil {
		return nil, fmt.Errorf("failed to send call tool request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-c.errorChan:
		return nil, err
	case response := <-c.responseChan:
		var resp struct {
			Jsonrpc string          `json:"jsonrpc"`
			Result  json.RawMessage `json:"result"`
			Error   *types.MCPError `json:"error"`
			ID      int64           `json:"id"`
		}
		if err := json.Unmarshal(response, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal call tool response: %w", err)
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (c *WebSocketClient) ExecuteTool(ctx context.Context, toolCall types.ToolCall) (*types.ToolResult, error) {
	result, err := c.CallTool(ctx, toolCall.Name, toolCall.Parameters)
	if err != nil {
		return nil, err
	}

	// Convert the raw result to a ToolResult
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool result: %w", err)
	}

	var toolResult types.ToolResult
	if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	return &toolResult, nil
}

// SendBatchRequest sends multiple requests in a single batch
func (c *WebSocketClient) SendBatchRequest(ctx context.Context, requests []BatchRequest) ([]BatchResponse, error) {
	if len(requests) == 0 {
		return nil, fmt.Errorf("empty batch request")
	}

	// Create batch request
	batch := make([]interface{}, len(requests))
	for i, req := range requests {
		batch[i] = struct {
			Jsonrpc string      `json:"jsonrpc"`
			Method  string      `json:"method"`
			Params  interface{} `json:"params,omitempty"`
			ID      int64       `json:"id"`
		}{
			Jsonrpc: "2.0",
			Method:  req.Method,
			Params:  req.Params,
			ID:      c.nextID.Add(1),
		}
	}

	// Marshal batch request
	requestBytes, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	// Send request
	if err := c.writeMessage(websocket.TextMessage, requestBytes); err != nil {
		return nil, fmt.Errorf("failed to send batch request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-c.errorChan:
		return nil, err
	case response := <-c.responseChan:
		var batchResp []struct {
			Jsonrpc string          `json:"jsonrpc"`
			Result  json.RawMessage `json:"result,omitempty"`
			Error   *types.MCPError `json:"error,omitempty"`
			ID      int64           `json:"id"`
		}
		if err := json.Unmarshal(response, &batchResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal batch response: %w", err)
		}

		// Convert to BatchResponse slice
		responses := make([]BatchResponse, len(batchResp))
		for i, resp := range batchResp {
			responses[i] = BatchResponse{
				Result: resp.Result,
				Error:  resp.Error,
			}
		}

		return responses, nil
	}
}
