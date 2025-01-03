package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	conn             *websocket.Conn
	nextID           atomic.Int64
	initializeParams *types.InitializeParams
	responseChan     chan []byte
	errorChan        chan error
	mu               sync.Mutex
	listToolsMethod  string
}

func NewWebSocketClient(url string) (*WebSocketClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket server: %w", err)
	}

	client := &WebSocketClient{
		conn:         conn,
		responseChan: make(chan []byte),
		errorChan:    make(chan error),
	}

	// Start reading messages in a goroutine
	go client.readLoop()

	return client, nil
}

func (c *WebSocketClient) readLoop() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}
			c.errorChan <- fmt.Errorf("failed to read message: %w", err)
			return
		}
		c.responseChan <- message
	}
}

func (c *WebSocketClient) Close() error {
	return c.conn.Close()
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

	c.mu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, requestBytes)
	c.mu.Unlock()
	if err != nil {
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

	c.mu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, requestBytes)
	c.mu.Unlock()
	if err != nil {
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

	c.mu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, requestBytes)
	c.mu.Unlock()
	if err != nil {
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
