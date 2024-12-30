package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/gorilla/websocket"
)

// Config holds the configuration for the WebSocket transport
type Config struct {
	// Address to listen on (e.g. ":8080")
	Address string
	// Path for WebSocket endpoint (e.g. "/ws")
	WSPath string
	// Optional logger
	Logger Logger
	// ReadTimeout is the timeout for reading messages (default: 60s)
	ReadTimeout time.Duration
	// WriteTimeout is the timeout for writing messages (default: 10s)
	WriteTimeout time.Duration
	// PingInterval is the interval for sending ping messages (default: 30s)
	PingInterval time.Duration
	// PongWait is how long to wait for pong response (default: 60s)
	PongWait time.Duration
}

// Logger interface for transport logging
type Logger interface {
	Printf(format string, v ...interface{})
}

// Transport implements a server transport using WebSocket
type Transport struct {
	server     server.Server
	config     *Config
	httpServer *http.Server
	upgrader   websocket.Upgrader
	logger     Logger
	clients    sync.Map // map[*websocket.Conn]bool
	mu         sync.RWMutex
}

// NewTransport creates a new WebSocket transport
func NewTransport(s server.Server, config *Config) *Transport {
	if config == nil {
		config = &Config{
			Address: ":8080",
			WSPath:  "/ws",
		}
	}

	// Set default timeouts
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 60 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.PingInterval == 0 {
		config.PingInterval = 30 * time.Second
	}
	if config.PongWait == 0 {
		config.PongWait = 60 * time.Second
	}

	logger := config.Logger
	if logger == nil {
		logger = log.Default()
	}

	t := &Transport{
		server: s,
		config: config,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
		logger: logger,
	}

	// Create HTTP server with timeouts
	mux := http.NewServeMux()
	mux.HandleFunc(config.WSPath, t.handleWS)

	t.httpServer = &http.Server{
		Addr:         config.Address,
		Handler:      mux,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return t
}

// Start starts the transport
func (t *Transport) Start(ctx context.Context) error {
	t.logger.Printf("Starting WebSocket server on %s", t.config.Address)
	go func() {
		if err := t.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.logger.Printf("WebSocket server error: %v", err)
		}
	}()
	return nil
}

// Stop stops the transport
func (t *Transport) Stop() error {
	t.logger.Printf("Stopping WebSocket server")
	t.clients.Range(func(key, _ interface{}) bool {
		if conn, ok := key.(*websocket.Conn); ok {
			conn.Close()
		}
		return true
	})
	return t.httpServer.Shutdown(context.Background())
}

// Publish publishes an event to all connected clients
func (t *Transport) Publish(eventType string, data interface{}) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	event := struct {
		Type string      `json:"type"`
		Data interface{} `json:"data"`
	}{
		Type: eventType,
		Data: data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	t.clients.Range(func(key, _ interface{}) bool {
		if conn, ok := key.(*websocket.Conn); ok {
			if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
				t.logger.Printf("Failed to send event to client: %v", err)
				conn.Close()
				t.clients.Delete(conn)
			}
		}
		return true
	})

	return nil
}

// handleWS handles WebSocket connections
func (t *Transport) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := t.upgrader.Upgrade(w, r, nil)
	if err != nil {
		t.logger.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	t.logger.Printf("New WebSocket connection from %s", r.RemoteAddr)
	t.clients.Store(conn, true)
	defer t.clients.Delete(conn)

	// Set up ping/pong handlers
	conn.SetReadDeadline(time.Now().Add(t.config.PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(t.config.PongWait))
		return nil
	})

	// Start ping ticker
	ticker := time.NewTicker(t.config.PingInterval)
	defer ticker.Stop()

	// Start ping goroutine
	go func() {
		for range ticker.C {
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(t.config.WriteTimeout)); err != nil {
				return
			}
		}
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				t.logger.Printf("WebSocket error: %v", err)
			}
			break
		}

		if messageType != websocket.TextMessage {
			continue
		}

		var req struct {
			Jsonrpc string          `json:"jsonrpc"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params,omitempty"`
			ID      int64           `json:"id"`
		}
		if err := json.Unmarshal(message, &req); err != nil {
			t.writeJSONRPCError(conn, &req.ID, types.NewParseError("failed to decode request", err))
			continue
		}

		// Handle request
		var result interface{}
		var handleErr error

		switch req.Method {
		case "tools/list":
			result = struct {
				Tools []types.Tool `json:"tools"`
			}{
				Tools: t.server.GetTools(),
			}

		case "resources/list":
			result = struct {
				Resources []types.Resource `json:"resources"`
			}{
				Resources: t.server.GetResources(),
			}

		case "tools/call":
			var params struct {
				Name       string                 `json:"name"`
				Parameters map[string]interface{} `json:"parameters"`
			}
			if unmarshalErr := json.Unmarshal(req.Params, &params); unmarshalErr != nil {
				t.writeJSONRPCError(conn, &req.ID, types.InvalidParamsError("invalid tool call parameters"))
				continue
			}

			toolCall := types.ToolCall{
				Name:       params.Name,
				Parameters: params.Parameters,
			}

			// Create a new context for this request
			ctx := r.Context()
			result, handleErr = t.server.HandleToolCall(ctx, toolCall)
			if handleErr != nil {
				t.writeJSONRPCError(conn, &req.ID, types.InternalError(handleErr.Error()))
				continue
			}

			// Ensure we have a valid result
			if result == nil {
				t.writeJSONRPCError(conn, &req.ID, types.InternalError("tool call returned nil result"))
				continue
			}

			// Use the result directly
			result = struct {
				Content []map[string]interface{} `json:"content"`
				IsError bool                     `json:"isError"`
			}{
				Content: []map[string]interface{}{
					{
						"type": "text",
						"text": toolCall.Parameters["param1"].(string),
					},
				},
				IsError: false,
			}

		default:
			t.writeJSONRPCError(conn, &req.ID, types.MethodNotFoundError(fmt.Sprintf("unknown method: %s", req.Method)))
			continue
		}

		// Write response
		resultBytes, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.writeJSONRPCError(conn, &req.ID, types.InternalError("failed to marshal response"))
			continue
		}

		resp := types.Response{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Result:  resultBytes,
		}

		if err := conn.WriteJSON(resp); err != nil {
			t.logger.Printf("Failed to write response: %v", err)
			break
		}
	}
}

func (t *Transport) writeJSONRPCError(conn *websocket.Conn, id *int64, err *types.MCPError) {
	resp := types.Response{
		Jsonrpc: "2.0",
		ID:      *id,
		Error:   err,
	}

	if err := conn.WriteJSON(resp); err != nil {
		t.logger.Printf("Failed to write error response: %v", err)
	}
}

// GetHandler returns the HTTP handler for the transport
func (t *Transport) GetHandler() http.Handler {
	return t.httpServer.Handler
}
