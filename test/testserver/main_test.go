package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestMethodNameCompatibility(t *testing.T) {
	// Create requests for both method names
	toolsListReq := request{
		Jsonrpc: "2.0",
		Method:  "tools/list",
		ID:      1,
	}

	mcpListToolsReq := request{
		Jsonrpc: "2.0",
		Method:  "mcp/list_tools",
		ID:      2,
	}

	// Handle both requests
	toolsListResp := handleRequest(toolsListReq)
	mcpListToolsResp := handleRequest(mcpListToolsReq)

	// Both responses should be successful
	if toolsListResp.Error != nil {
		t.Errorf("tools/list request failed: %v", toolsListResp.Error)
	}
	if mcpListToolsResp.Error != nil {
		t.Errorf("mcp/list_tools request failed: %v", mcpListToolsResp.Error)
	}

	// Decode and compare the results
	var toolsListResult, mcpListToolsResult struct {
		Tools []types.Tool `json:"tools"`
	}

	if err := json.Unmarshal(toolsListResp.Result, &toolsListResult); err != nil {
		t.Fatalf("Failed to unmarshal tools/list response: %v", err)
	}
	if err := json.Unmarshal(mcpListToolsResp.Result, &mcpListToolsResult); err != nil {
		t.Fatalf("Failed to unmarshal mcp/list_tools response: %v", err)
	}

	// Compare the number of tools
	if len(toolsListResult.Tools) != len(mcpListToolsResult.Tools) {
		t.Errorf("Different number of tools returned: tools/list=%d, mcp/list_tools=%d",
			len(toolsListResult.Tools), len(mcpListToolsResult.Tools))
	}

	// Compare each tool
	for i := range toolsListResult.Tools {
		if i >= len(mcpListToolsResult.Tools) {
			break
		}
		tool1 := toolsListResult.Tools[i]
		tool2 := mcpListToolsResult.Tools[i]

		if tool1.Name != tool2.Name {
			t.Errorf("Tool %d has different names: %s vs %s", i, tool1.Name, tool2.Name)
		}
		if tool1.Description != tool2.Description {
			t.Errorf("Tool %d has different descriptions: %s vs %s", i, tool1.Description, tool2.Description)
		}
		// Compare input schemas if needed
		if !bytes.Equal(tool1.InputSchema, tool2.InputSchema) {
			t.Errorf("Tool %d has different input schemas", i)
		}
	}
}
