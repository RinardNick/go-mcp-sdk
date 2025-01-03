package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	serverio "github.com/RinardNick/go-mcp-sdk/pkg/server/stdio"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamInterruption(t *testing.T) {
	// Create pipes for communication
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	// Create server with streaming capability
	s := server.NewServer(&server.InitializationOptions{
		Version: "1.0",
		Capabilities: map[string]interface{}{
			"streaming": true,
		},
	})

	// Register a streaming tool that supports interruption
	err := s.RegisterTool(types.Tool{
		Name: "interruptible_stream",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"delay": {
					"type": "number",
					"description": "Delay between stream events in milliseconds",
					"minimum": 100
				},
				"count": {
					"type": "number",
					"description": "Number of events to stream",
					"minimum": 1
				}
			},
			"required": ["delay", "count"]
		}`),
	})
	require.NoError(t, err)

	// Register tool handler
	err = s.RegisterToolHandler("interruptible_stream", func(ctx context.Context, params map[string]interface{}) (*types.ToolResult, error) {
		delay := int(params["delay"].(float64))
		count := int(params["count"].(float64))

		// Send stream events with delay
		for i := 1; i <= count; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// Send stream event
				event := types.StreamEvent{
					Method: "tools/stream",
					Params: types.StreamData{
						ToolID: "interruptible_stream",
						Data:   fmt.Sprintf("Stream data %d of %d", i, count),
					},
				}
				eventBytes, err := json.Marshal(event)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal stream event: %w", err)
				}

				// Write event to stdout
				if _, err := fmt.Fprintf(serverWriter, "%s\n", eventBytes); err != nil {
					return nil, fmt.Errorf("failed to write stream event: %w", err)
				}

				// Wait for delay
				time.Sleep(time.Duration(delay) * time.Millisecond)
			}
		}

		return &types.ToolResult{
			Result: json.RawMessage(`{"status":"completed","message":"Stream completed successfully"}`),
		}, nil
	})
	require.NoError(t, err)

	// Create and start the transport
	transport := serverio.NewTransport(s)
	transport.Reader = serverReader
	transport.Writer = serverWriter

	// Create client
	client := stdio.NewStdioClient(clientWriter, clientReader)

	// Set initialization parameters with streaming support
	client.SetInitializeParams(&types.InitializeParams{
		ProtocolVersion: "1.0",
		ClientInfo: types.ClientInfo{
			Name:    "test-client",
			Version: "1.0",
		},
		Capabilities: types.ClientCapabilities{
			Streaming: &types.StreamingCapabilities{
				Supported: true,
			},
		},
	})

	// Start transport and initialize client
	ctx := context.Background()
	err = transport.Start(ctx)
	require.NoError(t, err)
	err = client.Initialize(ctx)
	require.NoError(t, err)

	t.Run("Stream Interruption", func(t *testing.T) {
		var streamData []string
		streamMutex := sync.Mutex{}

		// Set up stream handler
		client.OnStream(func(data []byte) error {
			var streamEvent types.StreamEvent
			if err := json.Unmarshal(data, &streamEvent); err != nil {
				return err
			}

			streamMutex.Lock()
			streamData = append(streamData, streamEvent.Params.Data)
			streamMutex.Unlock()
			return nil
		})

		// Create a context that will be cancelled mid-stream
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// Execute streaming tool with parameters that ensure it will be interrupted
		_, err := client.ExecuteTool(ctx, types.ToolCall{
			Name: "interruptible_stream",
			Parameters: map[string]interface{}{
				"delay": 100, // 100ms between events
				"count": 10,  // 10 events total (would take 1s without interruption)
			},
		})

		// Verify that we got a context cancellation error
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)

		// Wait a bit to ensure no more events are received after cancellation
		time.Sleep(200 * time.Millisecond)

		// Verify stream data
		streamMutex.Lock()
		defer streamMutex.Unlock()

		// We should have received some events but not all
		assert.Greater(t, len(streamData), 0, "Should have received some events")
		assert.Less(t, len(streamData), 10, "Should not have received all events")

		// Verify the events we did receive are in order
		for i, data := range streamData {
			expected := fmt.Sprintf("Stream data %d of 10", i+1)
			assert.Equal(t, expected, data, "Unexpected stream data")
		}
	})

	// Clean up
	clientReader.Close()
	clientWriter.Close()
	serverReader.Close()
	serverWriter.Close()
}
