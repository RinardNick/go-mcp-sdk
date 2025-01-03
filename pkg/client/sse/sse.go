package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/client"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/r3labs/sse/v2"
	"gopkg.in/cenkalti/backoff.v1"
)

// SSEClient implements the Client interface using Server-Sent Events
type SSEClient struct {
	baseURL    string
	httpClient *http.Client
	sseClient  *sse.Client
	mu         sync.Mutex
	nextID     int64
	ctx        context.Context
	cancel     context.CancelFunc
	eventSub   chan *sse.Event
}

// request represents a JSON-RPC request
type request struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int64       `json:"id"`
}

// response represents a JSON-RPC response
type response struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *types.MCPError `json:"error,omitempty"`
	ID      int64           `json:"id"`
}

// NewSSEClient creates a new SSE client
func NewSSEClient(baseURL string) (*SSEClient, error) {
	sseClient := sse.NewClient(baseURL + "/events")

	// Configure exponential backoff
	backOff := backoff.NewExponentialBackOff()
	backOff.InitialInterval = 1 * time.Second
	backOff.MaxInterval = 30 * time.Second
	backOff.Multiplier = 2
	backOff.MaxElapsedTime = 5 * time.Minute
	sseClient.ReconnectStrategy = backOff

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan *sse.Event)

	client := &SSEClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		sseClient:  sseClient,
		ctx:        ctx,
		cancel:     cancel,
		eventSub:   events,
	}

	// Start event subscription
	go func() {
		err := sseClient.SubscribeChanWithContext(ctx, "", events)
		if err != nil && err != context.Canceled {
			// Log error but don't fail - the client can still work without events
			fmt.Printf("Failed to subscribe to events: %v\n", err)
		}
	}()

	// Start event processing
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-events:
				// Handle events as needed
				if event != nil {
					// Process event
				}
			}
		}
	}()

	return client, nil
}

// NewSession creates a new session with this client
func (c *SSEClient) NewSession(reader io.Reader, writer io.Writer) (*client.Session, error) {
	return client.NewSession(reader, writer, c)
}

// sendRequest sends a JSON-RPC request and returns the response
func (c *SSEClient) sendRequest(ctx context.Context, method string, params interface{}) (*response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := atomic.AddInt64(&c.nextID, 1)
	req := request{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      id,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/rpc", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var jsonRPCResp response
	if err := json.NewDecoder(resp.Body).Decode(&jsonRPCResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if jsonRPCResp.Error != nil {
		return nil, jsonRPCResp.Error
	}

	return &jsonRPCResp, nil
}

// ListResources implements the Client interface
func (c *SSEClient) ListResources(ctx context.Context) ([]types.Resource, error) {
	resp, err := c.sendRequest(ctx, "mcp/list_resources", nil)
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
func (c *SSEClient) ListTools(ctx context.Context) ([]types.Tool, error) {
	resp, err := c.sendRequest(ctx, "tools/list", nil)
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
func (c *SSEClient) ExecuteTool(ctx context.Context, call types.ToolCall) (*types.ToolResult, error) {
	// Create the request parameters
	params := map[string]interface{}{
		"name":      call.Name,
		"arguments": call.Parameters,
	}

	resp, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	// Create a new ToolResult with the response result
	return &types.ToolResult{
		Result: resp.Result,
	}, nil
}

// SendBatchRequest implements the Client interface
func (c *SSEClient) SendBatchRequest(ctx context.Context, methods []string, params []interface{}) ([]types.Response, error) {
	if len(methods) != len(params) {
		return nil, fmt.Errorf("number of methods does not match number of parameters")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var requests []request
	for i, method := range methods {
		id := atomic.AddInt64(&c.nextID, 1)
		requests = append(requests, request{
			Jsonrpc: "2.0",
			Method:  method,
			Params:  params[i],
			ID:      id,
		})
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"requests": requests,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/rpc", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var batchResp struct {
		Responses []types.Response `json:"responses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode batch response: %w", err)
	}

	// Verify responses match requests
	if len(batchResp.Responses) != len(requests) {
		return nil, fmt.Errorf("number of responses does not match number of requests")
	}

	for i, resp := range batchResp.Responses {
		if resp.ID != requests[i].ID {
			return nil, fmt.Errorf("response ID does not match request ID in batch")
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
	}

	return batchResp.Responses, nil
}

// SendNotification implements the Client interface
func (c *SSEClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	notif := struct {
		Jsonrpc string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
	}

	reqBody, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/rpc", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notification failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close implements the Client interface
func (c *SSEClient) Close() error {
	c.cancel()        // Stop event subscription and processing
	close(c.eventSub) // Close event channel
	return nil
}

// SendRequest implements the Client interface
func (c *SSEClient) SendRequest(ctx context.Context, method string, params interface{}) (*types.Response, error) {
	resp, err := c.sendRequest(ctx, method, params)
	if err != nil {
		return nil, err
	}
	return &types.Response{
		Jsonrpc: resp.Jsonrpc,
		Result:  resp.Result,
		Error:   resp.Error,
		ID:      resp.ID,
	}, nil
}

// Initialize sends the initialization request to the server
func (c *SSEClient) Initialize(ctx context.Context) error {
	params := types.InitializeParams{
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

	_, err := c.sendRequest(ctx, "initialize", params)
	return err
}

// ApplyResourceTemplate implements the Client interface
func (c *SSEClient) ApplyResourceTemplate(ctx context.Context, template types.ResourceTemplate) (*types.ResourceTemplateResult, error) {
	resp, err := c.sendRequest(ctx, "mcp/apply_resource_template", template)
	if err != nil {
		return nil, err
	}

	var result types.ResourceTemplateResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template result: %w", err)
	}

	return &result, nil
}

// GetResourceTemplates implements the Client interface
func (c *SSEClient) GetResourceTemplates(ctx context.Context) ([]types.Resource, error) {
	resp, err := c.sendRequest(ctx, "mcp/list_resource_templates", nil)
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
