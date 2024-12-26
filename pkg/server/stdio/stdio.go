package stdio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

// Transport implements a STDIO transport for the MCP server
type Transport struct {
	server *server.BaseServer
	Reader io.Reader
	Writer io.Writer
	logger *log.Logger
	wg     sync.WaitGroup
	done   chan struct{}
}

// Request represents a JSON-RPC request
type Request struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      int64           `json:"id"`
}

// BatchRequest represents a batch of JSON-RPC requests
type BatchRequest struct {
	Requests []Request `json:"requests"`
}

// BatchResponse represents a batch of JSON-RPC responses
type BatchResponse struct {
	Responses []types.Response `json:"responses"`
}

// NewTransport creates a new STDIO transport
func NewTransport(s *server.BaseServer) *Transport {
	return &Transport{
		server: s,
		Reader: os.Stdin,
		Writer: os.Stdout,
		logger: log.New(os.Stderr, "stdio: ", log.LstdFlags|log.Lmicroseconds),
		done:   make(chan struct{}),
	}
}

// Start starts the transport
func (t *Transport) Start(ctx context.Context) error {
	t.wg.Add(1)
	go t.readLoop(ctx)
	return nil
}

// Stop stops the transport
func (t *Transport) Stop(ctx context.Context) error {
	close(t.done)
	t.wg.Wait()
	return nil
}

func (t *Transport) readLoop(ctx context.Context) {
	defer t.wg.Done()

	decoder := json.NewDecoder(t.Reader)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		default:
			var rawMsg json.RawMessage
			if err := decoder.Decode(&rawMsg); err != nil {
				if err == io.EOF {
					t.logger.Println("Client closed connection")
					return
				}
				t.logger.Printf("Error decoding request: %v", err)
				continue
			}

			// Try to decode as batch request
			var batch BatchRequest
			if err := json.Unmarshal(rawMsg, &batch); err == nil && len(batch.Requests) > 0 {
				t.logger.Printf("Received batch request with %d requests", len(batch.Requests))
				t.handleBatchRequest(ctx, batch)
				continue
			}

			// Try single request
			var req Request
			if err := json.Unmarshal(rawMsg, &req); err != nil {
				t.logger.Printf("Error decoding single request: %v", err)
				continue
			}

			t.logger.Printf("Received single request: %+v", req)
			if resp := t.handleRequest(ctx, req); resp != nil {
				if err := t.writeResponse(resp); err != nil {
					t.logger.Printf("Error writing response: %v", err)
				}
			}
		}
	}
}

func (t *Transport) handleRequest(ctx context.Context, req Request) *types.Response {
	resp := &types.Response{
		Jsonrpc: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "mcp/list_tools":
		tools := t.server.GetTools()
		result := map[string]interface{}{
			"tools": tools,
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			resp.Error = types.InternalError(fmt.Sprintf("failed to marshal result: %v", err))
			break
		}
		resp.Result = json.RawMessage(resultBytes)

	case "mcp/list_resources":
		resources := t.server.GetResources()
		result := map[string]interface{}{
			"resources": resources,
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			resp.Error = types.InternalError(fmt.Sprintf("failed to marshal result: %v", err))
			break
		}
		resp.Result = json.RawMessage(resultBytes)

	case "mcp/call_tool":
		var toolCall types.ToolCall
		if err := json.Unmarshal(req.Params, &toolCall); err != nil {
			resp.Error = types.InvalidParamsError(err.Error())
			break
		}

		result, err := t.server.HandleToolCall(ctx, toolCall)
		if err != nil {
			resp.Error = types.InternalError(err.Error())
			break
		}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			resp.Error = types.InternalError(fmt.Sprintf("failed to marshal result: %v", err))
			break
		}
		resp.Result = json.RawMessage(resultBytes)

	default:
		resp.Error = types.MethodNotFoundError(fmt.Sprintf("Unknown method: %s", req.Method))
	}

	return resp
}

func (t *Transport) handleBatchRequest(ctx context.Context, batch BatchRequest) {
	var responses []types.Response
	for _, req := range batch.Requests {
		if resp := t.handleRequest(ctx, req); resp != nil {
			responses = append(responses, *resp)
		}
	}

	if err := t.writeResponse(BatchResponse{Responses: responses}); err != nil {
		t.logger.Printf("Error writing batch response: %v", err)
	}
}

func (t *Transport) writeResponse(resp interface{}) error {
	t.logger.Printf("Writing response: %+v", resp)
	if err := json.NewEncoder(t.Writer).Encode(resp); err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}
	t.logger.Println("Response written successfully")
	return nil
}
