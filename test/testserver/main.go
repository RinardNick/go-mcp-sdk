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
	case "mcp/list_tools":
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

	case "mcp/list_resources":
		log.Printf("Handling list_resources request")
		result := map[string]interface{}{
			"resources": []types.Resource{
				{
					URI:  "test://resource",
					Name: "test_resource",
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

	case "mcp/call_tool":
		var toolCall types.ToolCall
		if err := json.Unmarshal(req.Params, &toolCall); err != nil {
			resp.Error = types.InvalidParamsError(err.Error())
			break
		}

		if toolCall.Name == "test_tool" {
			_, ok := toolCall.Parameters["param1"].(string)
			if !ok {
				resp.Error = types.InvalidParamsError("param1 parameter must be a string")
				break
			}

			result, err := types.NewToolResult(map[string]interface{}{
				"output": "test output",
			})
			if err != nil {
				resp.Error = types.InternalError(err.Error())
				break
			}
			resultBytes, err := json.Marshal(result)
			if err != nil {
				resp.Error = types.InternalError(err.Error())
				break
			}
			resp.Result = resultBytes
		} else {
			resp.Error = types.MethodNotFoundError(fmt.Sprintf("Unknown tool: %s", toolCall.Name))
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
	if err := json.NewEncoder(w).Encode(resp); err != nil {
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
