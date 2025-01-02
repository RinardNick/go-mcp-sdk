package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

type request struct {
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      int64           `json:"id"`
}

type batchRequest struct {
	Requests []request `json:"requests"`
}

type batchResponse struct {
	Responses []types.Response `json:"responses"`
}

func writeNotification(w io.Writer, notification interface{}) error {
	log.Printf("Writing notification: %+v", notification)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(notification); err != nil {
		log.Printf("Error encoding notification: %v", err)
		return fmt.Errorf("failed to encode notification: %w", err)
	}
	log.Println("Notification written successfully")
	return nil
}

func handleRequest(req request) *types.Response {
	log.Printf("Handling request: %+v", req)
	resp := &types.Response{
		Jsonrpc: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		log.Printf("Handling initialize request")
		result := map[string]interface{}{
			"protocolVersion": "0.1.0",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"supportsProgress":     true,
					"supportsCancellation": true,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "test_server",
				"version": "1.0.0",
			},
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			resp.Error = types.InternalError(err)
			return resp
		}
		resp.Result = resultBytes
		return resp

	case "tools/list":
		log.Printf("Handling list_tools request")
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
			resp.Error = types.InternalError(err)
			return resp
		}

		// Add long_operation tool
		longOpSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"steps": map[string]interface{}{
					"type":        "integer",
					"description": "Number of steps in the operation",
					"minimum":     1,
					"maximum":     10,
				},
			},
			"required": []string{"steps"},
		}
		longOpSchemaBytes, err := types.NewToolInputSchema(longOpSchema)
		if err != nil {
			resp.Error = types.InternalError(err)
			return resp
		}

		tools := []types.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: schemaBytes,
			},
			{
				Name:        "long_operation",
				Description: "A long-running operation that reports progress",
				InputSchema: longOpSchemaBytes,
			},
		}
		result := map[string]interface{}{
			"tools": tools,
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			resp.Error = types.InternalError(err)
			return resp
		}
		resp.Result = resultBytes
		return resp

	case "tools/call":
		log.Printf("Handling tool call request")
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = types.InvalidParamsError("invalid tool call parameters")
			return resp
		}

		// Handle the tool call
		switch params.Name {
		case "test_tool":
			// Validate parameters
			if params.Arguments == nil {
				resp.Error = types.InvalidParamsError("missing parameters")
				return resp
			}
			_, ok := params.Arguments["param1"].(string)
			if !ok {
				resp.Error = types.InvalidParamsError("param1 must be a string")
				return resp
			}

			// Create the result
			result := map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "test output",
					},
				},
				"isError": false,
			}
			resultBytes, err := json.Marshal(result)
			if err != nil {
				resp.Error = types.InternalError(err)
				return resp
			}
			resp.Result = resultBytes
			return resp

		case "long_operation":
			// Validate parameters
			if params.Arguments == nil {
				resp.Error = types.InvalidParamsError("missing parameters")
				return resp
			}
			steps, ok := params.Arguments["steps"].(float64)
			if !ok {
				resp.Error = types.InvalidParamsError("steps must be a number")
				return resp
			}
			if steps < 1 || steps > 10 {
				resp.Error = types.InvalidParamsError("steps must be between 1 and 10")
				return resp
			}

			// Send progress notifications
			for i := 1; i <= int(steps); i++ {
				progress := map[string]interface{}{
					"jsonrpc": "2.0",
					"method":  "tools/progress",
					"params": map[string]interface{}{
						"toolID":  "long_operation",
						"current": i,
						"total":   int(steps),
						"message": fmt.Sprintf("Step %d of %d", i, int(steps)),
					},
				}
				if err := writeNotification(os.Stdout, progress); err != nil {
					resp.Error = types.InternalError(err)
					return resp
				}
			}

			// Create the result
			result := map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Operation completed",
					},
				},
				"isError": false,
			}
			resultBytes, err := json.Marshal(result)
			if err != nil {
				resp.Error = types.InternalError(err)
				return resp
			}
			resp.Result = resultBytes
			return resp

		default:
			resp.Error = types.InvalidParamsError(fmt.Sprintf("unknown tool: %s", params.Name))
			return resp
		}

	case "mcp/list_resources":
		log.Printf("Handling list_resources request")
		result := map[string]interface{}{
			"resources": []types.Resource{
				{
					ID:          "test_resource",
					Name:        "test_resource",
					Description: "A test resource",
					Type:        "test",
					URI:         "test://resource",
					Metadata:    map[string]interface{}{},
				},
			},
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			resp.Error = types.InternalError(fmt.Sprintf("failed to marshal result: %v", err))
			break
		}
		resp.Result = json.RawMessage(resultBytes)
		log.Printf("List resources response: %+v", resp)

	case "test_notification":
		log.Printf("Handling test_notification request")
		// No response for notifications
		return nil

	case "notifications/initialized":
		log.Printf("Handling initialized notification")
		// No response for notifications
		return nil

	default:
		log.Printf("Unknown method: %s", req.Method)
		resp.Error = types.MethodNotFoundError(fmt.Sprintf("Unknown method: %s", req.Method))
	}

	return resp
}

func writeResponse(w io.Writer, resp interface{}) error {
	log.Printf("Writing response: %+v", resp)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(resp); err != nil {
		// Check for broken pipe
		if pathErr, ok := err.(*os.PathError); ok {
			if pathErr.Err == syscall.EPIPE {
				// Client closed connection, exit gracefully
				log.Println("Client closed pipe")
				os.Exit(0)
			}
		}
		log.Printf("Error encoding response: %v", err)
		return fmt.Errorf("failed to encode response: %w", err)
	}
	log.Println("Response written successfully")

	// Flush stdout
	if f, ok := w.(*os.File); ok {
		if err := f.Sync(); err != nil {
			log.Printf("Error syncing output: %v", err)
			return fmt.Errorf("failed to sync output: %w", err)
		}
	}

	return nil
}

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("testserver: ")
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	decoder := json.NewDecoder(os.Stdin)
	for {
		// Try to decode the request
		var rawMsg json.RawMessage
		if err := decoder.Decode(&rawMsg); err != nil {
			if err == io.EOF {
				log.Println("Client closed connection")
				return
			}
			log.Printf("Error decoding request: %v", err)
			continue
		}

		// Try to decode as batch request
		var batch batchRequest
		if err := json.Unmarshal(rawMsg, &batch); err == nil && len(batch.Requests) > 0 {
			log.Printf("Received batch request with %d requests", len(batch.Requests))
			// Handle batch request
			var responses []types.Response
			for _, req := range batch.Requests {
				if resp := handleRequest(req); resp != nil {
					responses = append(responses, *resp)
				}
			}
			if err := writeResponse(os.Stdout, batchResponse{Responses: responses}); err != nil {
				log.Printf("Error writing batch response: %v", err)
				continue
			}
			continue
		}

		// Try single request
		var req request
		if err := json.Unmarshal(rawMsg, &req); err != nil {
			log.Printf("Error decoding single request: %v", err)
			continue
		}

		log.Printf("Received single request: %+v", req)
		// Handle single request
		if resp := handleRequest(req); resp != nil {
			if err := writeResponse(os.Stdout, resp); err != nil {
				log.Printf("Error writing response: %v", err)
				continue
			}
		}
	}
}
