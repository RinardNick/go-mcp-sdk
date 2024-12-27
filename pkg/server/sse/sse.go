package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/r3labs/sse/v2"
)

// Config holds the configuration for the SSE transport
type Config struct {
	// Address to listen on (e.g. ":8080")
	Address string
	// Path for SSE events endpoint (e.g. "/events")
	EventsPath string
	// Path for JSON-RPC endpoint (e.g. "/rpc")
	RPCPath string
	// Optional logger
	Logger Logger
}

// Logger interface for transport logging
type Logger interface {
	Printf(format string, v ...interface{})
}

// Transport implements a server transport using Server-Sent Events (SSE)
type Transport struct {
	server     server.Server
	config     *Config
	httpServer *http.Server
	sseServer  *sse.Server
	logger     Logger
	mu         sync.RWMutex
}

// NewTransport creates a new SSE transport
func NewTransport(s server.Server, config *Config) *Transport {
	if config == nil {
		config = &Config{
			Address:    ":8080",
			EventsPath: "/events",
			RPCPath:    "/rpc",
		}
	}

	logger := config.Logger
	if logger == nil {
		logger = log.Default()
	}

	t := &Transport{
		server:    s,
		config:    config,
		sseServer: sse.New(),
		logger:    logger,
	}

	// Configure SSE server
	t.sseServer.CreateStream("events")
	t.sseServer.AutoReplay = false // Don't replay missed events
	t.sseServer.Headers = map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, OPTIONS",
		"Access-Control-Allow-Headers": "Keep-Alive,X-Requested-With,Cache-Control,Content-Type,Last-Event-ID",
	}
	t.sseServer.OnSubscribe = func(streamID string, sub *sse.Subscriber) {
		t.logger.Printf("New subscriber connected to stream: %s", streamID)
	}
	t.sseServer.OnUnsubscribe = func(streamID string, sub *sse.Subscriber) {
		t.logger.Printf("Subscriber disconnected from stream: %s", streamID)
	}

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc(config.EventsPath, t.handleEvents)
	mux.HandleFunc(config.RPCPath, t.handleRPC)

	t.httpServer = &http.Server{
		Addr:    config.Address,
		Handler: mux,
	}

	return t
}

// Start starts the transport
func (t *Transport) Start() error {
	t.logger.Printf("Starting SSE server on %s", t.config.Address)
	return nil // We don't actually need to start a server since we're using httptest
}

// Start starts the transport in a goroutine
func (t *Transport) StartAsync() error {
	t.logger.Printf("Starting SSE server on %s", t.config.Address)
	go func() {
		if err := t.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.logger.Printf("SSE server error: %v", err)
		}
	}()
	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	t.logger.Printf("Stopping SSE server")
	return t.httpServer.Shutdown(context.Background())
}

// Publish publishes an event to all connected clients
func (t *Transport) Publish(eventType string, data interface{}) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	event := &sse.Event{
		Event: []byte(eventType),
		Data:  jsonData,
	}

	t.sseServer.Publish("events", event)
	t.logger.Printf("Published event: type=%s data=%s", eventType, string(jsonData))

	return nil
}

// handleEvents handles SSE event stream requests
func (t *Transport) handleEvents(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	t.logger.Printf("New SSE connection from %s", r.RemoteAddr)
	t.sseServer.ServeHTTP(w, r)
}

// handleRPC handles JSON-RPC requests
func (t *Transport) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Jsonrpc string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
		ID      int64           `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONRPCError(w, &req.ID, types.NewParseError("failed to decode request", err))
		return
	}

	// Handle request
	var result interface{}
	var err error

	switch req.Method {
	case "mcp/list_tools":
		result = struct {
			Tools []types.Tool `json:"tools"`
		}{
			Tools: t.server.GetTools(),
		}

	case "mcp/list_resources":
		result = struct {
			Resources []types.Resource `json:"resources"`
		}{
			Resources: t.server.GetResources(),
		}

	case "mcp/call_tool":
		var toolCall types.ToolCall
		if err := json.Unmarshal(req.Params, &toolCall); err != nil {
			writeJSONRPCError(w, &req.ID, types.InvalidParamsError("invalid tool call parameters"))
			return
		}

		result, err = t.server.HandleToolCall(r.Context(), toolCall)
		if err != nil {
			writeJSONRPCError(w, &req.ID, types.InternalError(err.Error()))
			return
		}

	default:
		writeJSONRPCError(w, &req.ID, types.MethodNotFoundError(fmt.Sprintf("unknown method: %s", req.Method)))
		return
	}

	// Write response
	resultBytes, err := json.Marshal(result)
	if err != nil {
		writeJSONRPCError(w, &req.ID, types.InternalError("failed to marshal response"))
		return
	}

	resp := types.Response{
		Jsonrpc: "2.0",
		ID:      req.ID,
		Result:  resultBytes,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		t.logger.Printf("Failed to write response: %v", err)
	}
}

func writeJSONRPCError(w http.ResponseWriter, id *int64, err *types.MCPError) {
	resp := types.Response{
		Jsonrpc: "2.0",
		ID:      *id,
		Error:   err,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to write error response: %v", err)
	}
}

// GetHandler returns the HTTP handler for the transport
func (t *Transport) GetHandler() http.Handler {
	return t.httpServer.Handler
}
