package ws_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	serverws "github.com/RinardNick/go-mcp-sdk/pkg/server/ws"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/gorilla/websocket"
)

func TestWebSocketTransportReconnection(t *testing.T) {
	s := server.NewServer(nil)
	config := &serverws.Config{
		Address:      ":0",
		WSPath:       "/ws",
		PingInterval: 100 * time.Millisecond,
		PongWait:     200 * time.Millisecond,
	}
	transport := serverws.NewTransport(s, config)

	// Create test server
	server := httptest.NewServer(transport.GetHandler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Test connection drop and reconnect
	t.Run("Connection Drop and Reconnect", func(t *testing.T) {
		// First connection
		conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect to WebSocket: %v", err)
		}

		// Wait for connection setup
		time.Sleep(50 * time.Millisecond)

		// Force close the connection
		conn1.Close()

		// Wait a bit
		time.Sleep(50 * time.Millisecond)

		// New connection should work
		conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to reconnect to WebSocket: %v", err)
		}
		defer conn2.Close()

		// Test that new connection works
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "tools/list",
			"id":      1,
		}
		if err := conn2.WriteJSON(req); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		var resp types.Response
		if err := conn2.ReadJSON(&resp); err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if resp.Error != nil {
			t.Fatalf("Unexpected error: %v", resp.Error)
		}
	})
}

func TestWebSocketTransportConnectionStates(t *testing.T) {
	s := server.NewServer(nil)
	config := &serverws.Config{
		Address:      ":0",
		WSPath:       "/ws",
		PingInterval: 100 * time.Millisecond,
		PongWait:     200 * time.Millisecond,
	}
	transport := serverws.NewTransport(s, config)
	ctx := context.Background()
	go transport.Start(ctx)
	defer transport.Stop()

	// Create test server
	server := httptest.NewServer(transport.GetHandler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Test connection state transitions
	t.Run("Connection State Transitions", func(t *testing.T) {
		// Connect
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect to WebSocket: %v", err)
		}

		// Wait for connection setup
		time.Sleep(50 * time.Millisecond)

		// Check connection state
		states := transport.GetConnectionStates()
		if len(states) != 1 {
			t.Errorf("Expected 1 connection, got %d", len(states))
		}
		for _, state := range states {
			if state != serverws.StateConnected {
				t.Errorf("Expected state %s, got %s", serverws.StateConnected, state)
			}
		}

		// Close connection
		conn.Close()

		// Wait for disconnection
		time.Sleep(50 * time.Millisecond)

		// Check disconnected state
		states = transport.GetConnectionStates()
		if len(states) != 0 {
			t.Errorf("Expected 0 connections after close, got %d", len(states))
		}
	})

	// Test multiple connections
	t.Run("Multiple Connections", func(t *testing.T) {
		// Create multiple connections
		var conns []*websocket.Conn
		for i := 0; i < 3; i++ {
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("Failed to connect to WebSocket: %v", err)
			}
			conns = append(conns, conn)
			time.Sleep(10 * time.Millisecond) // Small delay between connections
		}

		// Wait for connections to stabilize
		time.Sleep(50 * time.Millisecond)

		// Check all connections are in connected state
		states := transport.GetConnectionStates()
		if len(states) != 3 {
			t.Errorf("Expected 3 connections, got %d", len(states))
		}
		for _, state := range states {
			if state != serverws.StateConnected {
				t.Errorf("Expected state %s, got %s", serverws.StateConnected, state)
			}
		}

		// Close connections one by one
		for i, conn := range conns {
			conn.Close()
			time.Sleep(50 * time.Millisecond)

			states = transport.GetConnectionStates()
			expectedConns := len(conns) - (i + 1)
			if len(states) != expectedConns {
				t.Errorf("Expected %d connections, got %d", expectedConns, len(states))
			}
		}
	})

	// Test graceful shutdown
	t.Run("Graceful Shutdown", func(t *testing.T) {
		// Create a connection
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect to WebSocket: %v", err)
		}
		defer conn.Close()

		// Wait for connection setup
		time.Sleep(50 * time.Millisecond)

		// Stop the transport
		if err := transport.Stop(); err != nil {
			t.Fatalf("Failed to stop transport: %v", err)
		}

		// Wait for disconnection
		time.Sleep(50 * time.Millisecond)

		// Check all connections are closed
		states := transport.GetConnectionStates()
		if len(states) != 0 {
			t.Errorf("Expected 0 connections after shutdown, got %d", len(states))
		}
	})
}

func TestWebSocketTransportConcurrentClients(t *testing.T) {
	s := server.NewServer(nil)

	// Register test tool
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"param1": map[string]interface{}{
				"type":        "string",
				"description": "A test parameter",
			},
		},
		"required": []string{"param1"},
	}
	schemaBytes, err := types.NewToolInputSchema(inputSchema)
	if err != nil {
		t.Fatalf("Failed to create input schema: %v", err)
	}

	tool := types.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: schemaBytes,
	}
	if err := s.RegisterTool(tool); err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	handler := func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
		// Create a new result for each request to avoid sharing state
		result, err := types.NewToolResult(map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": params["param1"].(string),
				},
			},
			"isError": false,
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	if err := s.RegisterToolHandler("test_tool", handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	config := &serverws.Config{
		Address: ":0",
		WSPath:  "/ws",
	}
	transport := serverws.NewTransport(s, config)

	// Create test server
	server := httptest.NewServer(transport.GetHandler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Test concurrent clients
	t.Run("Concurrent Clients", func(t *testing.T) {
		numClients := 10
		var wg sync.WaitGroup
		wg.Add(numClients)

		// Create a channel to collect errors
		errChan := make(chan error, numClients)

		// Add a delay between client connections
		for i := 0; i < numClients; i++ {
			go func(clientID int) {
				defer wg.Done()

				// Add a small delay between client connections
				time.Sleep(time.Duration(clientID) * 10 * time.Millisecond)

				// Connect
				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				if err != nil {
					errChan <- fmt.Errorf("Client %d failed to connect: %v", clientID, err)
					return
				}
				defer conn.Close()

				// Send tool call request
				req := map[string]interface{}{
					"jsonrpc": "2.0",
					"method":  "tools/call",
					"params": map[string]interface{}{
						"name": "test_tool",
						"Parameters": map[string]interface{}{
							"param1": fmt.Sprintf("test input %d", clientID),
						},
					},
					"id": clientID,
				}
				if err := conn.WriteJSON(req); err != nil {
					errChan <- fmt.Errorf("Client %d failed to write request: %v", clientID, err)
					return
				}

				// Read response with timeout
				done := make(chan struct{})
				var resp types.Response
				go func() {
					if err := conn.ReadJSON(&resp); err != nil {
						errChan <- fmt.Errorf("Client %d failed to read response: %v", clientID, err)
						return
					}
					close(done)
				}()

				select {
				case <-done:
					// Response received successfully
				case <-time.After(5 * time.Second):
					errChan <- fmt.Errorf("Client %d timed out waiting for response", clientID)
					return
				}

				if resp.Error != nil {
					errChan <- fmt.Errorf("Client %d got unexpected error: %v", clientID, resp.Error)
					return
				}

				// Verify response
				var resultMap struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
					IsError bool `json:"isError"`
				}
				if err := json.Unmarshal(resp.Result, &resultMap); err != nil {
					errChan <- fmt.Errorf("Client %d failed to unmarshal result: %v", clientID, err)
					return
				}

				if len(resultMap.Content) == 0 {
					errChan <- fmt.Errorf("Client %d: Expected non-empty content", clientID)
					return
				}

				expected := fmt.Sprintf("test input %d", clientID)
				if resultMap.Content[0].Text != expected {
					errChan <- fmt.Errorf("Client %d: Expected text %q, got %v", clientID, expected, resultMap.Content[0].Text)
					return
				}

				if resultMap.IsError {
					errChan <- fmt.Errorf("Client %d: Expected isError to be false", clientID)
					return
				}
			}(i)
		}

		// Wait for all goroutines to finish
		wg.Wait()
		close(errChan)

		// Check for any errors
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			for _, err := range errors {
				t.Error(err)
			}
		}
	})
}

func TestWSServer(t *testing.T) {
	// Create test server
	s := server.NewServer(&server.InitializationOptions{
		Version: "1.0",
		Capabilities: map[string]interface{}{
			"tools":     true,
			"resources": true,
		},
	})

	// Add test tool
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"param1": map[string]interface{}{
				"type":        "string",
				"description": "A test parameter",
			},
		},
		"required": []string{"param1"},
	}
	schemaBytes, err := types.NewToolInputSchema(inputSchema)
	if err != nil {
		t.Fatalf("Failed to create input schema: %v", err)
	}

	err = s.RegisterTool(types.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: schemaBytes,
	})
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// Register tool handler
	err = s.RegisterToolHandler("test_tool", func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
		result, err := types.NewToolResult(map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": params["param1"],
				},
			},
			"isError": false,
		})
		if err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		t.Fatalf("Failed to register tool handler: %v", err)
	}

	// Test tool call
	toolCall := types.ToolCall{
		Name: "test_tool",
		Parameters: map[string]interface{}{
			"param1": "test",
		},
	}

	result, err := s.HandleToolCall(context.Background(), toolCall)
	if err != nil {
		t.Fatalf("Failed to handle tool call: %v", err)
	}

	// Check result
	var resultMap struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result.Result, &resultMap); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(resultMap.Content) == 0 {
		t.Error("Expected non-empty content")
	} else if resultMap.Content[0].Text != "test" {
		t.Errorf("Expected text 'test', got %v", resultMap.Content[0].Text)
	}

	if resultMap.IsError {
		t.Error("Expected isError to be false")
	}
}

func TestWebSocketTransportConnectionAttempts(t *testing.T) {
	s := server.NewServer(nil)
	config := &serverws.Config{
		Address:      ":0",
		WSPath:       "/ws",
		PingInterval: 100 * time.Millisecond,
		PongWait:     200 * time.Millisecond,
	}
	transport := serverws.NewTransport(s, config)
	ctx := context.Background()
	go transport.Start(ctx)
	defer transport.Stop()

	// Create test server
	server := httptest.NewServer(transport.GetHandler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Test simultaneous connection attempts
	t.Run("Simultaneous Connection Attempts", func(t *testing.T) {
		numConnections := 50
		var wg sync.WaitGroup
		wg.Add(numConnections)

		// Create a channel to collect errors and successful connections
		errChan := make(chan error, numConnections)
		connChan := make(chan *websocket.Conn, numConnections)

		// Start all connections at roughly the same time
		for i := 0; i < numConnections; i++ {
			go func(id int) {
				defer wg.Done()

				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				if err != nil {
					errChan <- fmt.Errorf("connection %d failed: %v", id, err)
					return
				}
				connChan <- conn

				// Send a simple ping to verify connection is working
				if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
					errChan <- fmt.Errorf("connection %d failed to send ping: %v", id, err)
				}
			}(i)
		}

		wg.Wait()
		close(errChan)
		close(connChan)

		// Collect all connections
		var conns []*websocket.Conn
		for conn := range connChan {
			conns = append(conns, conn)
		}

		// Clean up connections at the end
		defer func() {
			for _, conn := range conns {
				conn.Close()
			}
		}()

		// Check for connection errors
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			for _, err := range errors {
				t.Error(err)
			}
		}

		// Wait a bit for connection states to stabilize
		time.Sleep(100 * time.Millisecond)

		// Verify connection states
		states := transport.GetConnectionStates()
		if len(states) != len(conns) {
			t.Errorf("Expected %d connections, got %d", len(conns), len(states))
		}
		for _, state := range states {
			if state != serverws.StateConnected {
				t.Errorf("Expected state %s, got %s", serverws.StateConnected, state)
			}
		}
	})

	// Test connection attempts during high load
	t.Run("Connection Attempts During High Load", func(t *testing.T) {
		// Create initial connections to generate load
		numInitialConns := 20
		initialConns := make([]*websocket.Conn, 0, numInitialConns)
		for i := 0; i < numInitialConns; i++ {
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("Failed to create initial connection %d: %v", i, err)
			}
			initialConns = append(initialConns, conn)
		}
		defer func() {
			for _, conn := range initialConns {
				conn.Close()
			}
		}()

		// Generate load by sending messages on all connections
		var wg sync.WaitGroup
		for _, conn := range initialConns {
			wg.Add(1)
			go func(c *websocket.Conn) {
				defer wg.Done()
				for i := 0; i < 100; i++ {
					if err := c.WriteMessage(websocket.TextMessage, []byte("test message")); err != nil {
						return
					}
					time.Sleep(time.Millisecond)
				}
			}(conn)
		}

		// Try to establish new connections while under load
		numNewConns := 10
		newConns := make([]*websocket.Conn, 0, numNewConns)
		errChan := make(chan error, numNewConns)
		connChan := make(chan *websocket.Conn, numNewConns)

		var connWg sync.WaitGroup
		connWg.Add(numNewConns)
		for i := 0; i < numNewConns; i++ {
			go func(id int) {
				defer connWg.Done()
				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				if err != nil {
					errChan <- fmt.Errorf("new connection %d failed: %v", id, err)
					return
				}
				connChan <- conn
			}(i)
		}

		connWg.Wait()
		close(errChan)
		close(connChan)

		// Collect new connections
		for conn := range connChan {
			newConns = append(newConns, conn)
		}

		// Wait for load generation to complete
		wg.Wait()

		// Check for connection errors
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			for _, err := range errors {
				t.Error(err)
			}
		}

		// Clean up new connections
		for _, conn := range newConns {
			conn.Close()
		}
	})

	// Test connection attempts during shutdown
	t.Run("Connection Attempts During Shutdown", func(t *testing.T) {
		// Create some initial connections
		numInitialConns := 5
		initialConns := make([]*websocket.Conn, 0, numInitialConns)
		for i := 0; i < numInitialConns; i++ {
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("Failed to create initial connection %d: %v", i, err)
			}
			initialConns = append(initialConns, conn)
		}

		// Start shutdown
		shutdownStarted := make(chan struct{})
		shutdownComplete := make(chan struct{})
		go func() {
			close(shutdownStarted)
			transport.Stop()
			close(shutdownComplete)
		}()

		<-shutdownStarted

		// Try to connect during shutdown
		var wg sync.WaitGroup
		numAttempts := 5
		wg.Add(numAttempts)
		errChan := make(chan error, numAttempts)

		for i := 0; i < numAttempts; i++ {
			go func(id int) {
				defer wg.Done()
				conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
				if err == nil {
					conn.Close()
					errChan <- fmt.Errorf("connection %d succeeded during shutdown", id)
				}
			}(i)
		}

		wg.Wait()
		<-shutdownComplete

		// Wait a bit for connections to fully close
		time.Sleep(100 * time.Millisecond)

		// Clean up initial connections
		for _, conn := range initialConns {
			conn.Close()
		}

		close(errChan)

		// We expect some connection attempts to fail during shutdown
		errors := make([]error, 0)
		for err := range errChan {
			errors = append(errors, err)
		}

		// Verify no connections are left
		states := transport.GetConnectionStates()
		if len(states) != 0 {
			t.Errorf("Expected 0 connections after shutdown, got %d", len(states))
		}
	})

	// Test connection attempts with invalid parameters
	t.Run("Connection Attempts With Invalid Parameters", func(t *testing.T) {
		// Create an HTTP client that doesn't follow redirects
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		// Test cases for invalid connection attempts
		testCases := []struct {
			name    string
			headers http.Header
			path    string
		}{
			{
				name:    "Invalid WebSocket Version",
				headers: http.Header{"Sec-WebSocket-Version": []string{"1"}},
				path:    "/ws",
			},
			{
				name:    "Missing Upgrade Header",
				headers: http.Header{},
				path:    "/ws",
			},
			{
				name:    "Invalid Path",
				headers: http.Header{"Upgrade": []string{"websocket"}},
				path:    "/invalid",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				url := "http" + strings.TrimPrefix(server.URL, "http") + tc.path
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				// Add headers
				for k, v := range tc.headers {
					req.Header[k] = v
				}

				resp, err := client.Do(req)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()

				// All invalid attempts should fail
				if resp.StatusCode < 400 {
					t.Errorf("Expected error status code, got %d", resp.StatusCode)
				}
			})
		}
	})
}
