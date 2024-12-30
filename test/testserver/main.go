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
		tools := []types.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: schemaBytes,
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

	case "resources/list":
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

	case "tools/call":
		log.Printf("Handling tools/call request")
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = types.InvalidParamsError(err.Error())
			break
		}

		if params.Name == "test_tool" {
			// Validate required param1 parameter
			param1, ok := params.Arguments["param1"].(string)
			if !ok {
				resp.Error = types.InvalidParamsError("param1 parameter must be a string")
				break
			}
			if param1 == "" {
				resp.Error = types.InvalidParamsError("param1 parameter cannot be empty")
				break
			}

			result := map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "test output",
					},
				},
				"isError": false,
			}
			toolResult, err := types.NewToolResult(result)
			if err != nil {
				resp.Error = types.InternalError(err.Error())
				break
			}
			resp.Result = toolResult.Result
		} else {
			resp.Error = types.MethodNotFoundError(fmt.Sprintf("Unknown tool: %s", params.Name))
		}

	case "test_notification":
		log.Printf("Handling test_notification request")
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
