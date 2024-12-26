# Go MCP SDK

A Go implementation of the Model Context Protocol (MCP) SDK. This SDK provides a client and server implementation for the MCP protocol, enabling communication between language models and tools.

## Features

- Full MCP protocol implementation
- Multiple transport options:
  - STDIO (for subprocess communication)
  - WebSocket (for persistent connections)
  - Server-Sent Events (SSE) (for one-way streaming)
- Built-in validation
- Comprehensive test coverage
- Thread-safe implementation
- Support for batch operations
- Automatic reconnection handling

## Installation

```bash
go get github.com/RinardNick/go-mcp-sdk
```

## Quick Start

### Client Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
    "github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func main() {
    // Create a new STDIO client
    client, err := stdio.NewStdioClient("path/to/server")
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // List available tools
    tools, err := client.ListTools(context.Background())
    if err != nil {
        log.Fatalf("Failed to list tools: %v", err)
    }

    // Execute a tool
    toolCall := types.ToolCall{
        Name: "example_tool",
        Parameters: map[string]interface{}{
            "param1": "value1",
        },
    }
    result, err := client.ExecuteTool(context.Background(), toolCall)
    if err != nil {
        log.Fatalf("Failed to execute tool: %v", err)
    }
    fmt.Printf("Tool result: %+v\n", result)
}
```

### Server Usage

```go
package main

import (
    "context"
    "log"

    "github.com/RinardNick/go-mcp-sdk/pkg/server"
    "github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func main() {
    // Create a new server
    s := server.NewServer(&server.InitializationOptions{
        Version: "1.0",
        Capabilities: map[string]interface{}{
            "tools":     true,
            "resources": true,
        },
    })

    // Register a tool
    tool := types.Tool{
        Name:        "example_tool",
        Description: "An example tool",
        Parameters: map[string]any{
            "param1": map[string]any{
                "type":        "string",
                "description": "A parameter",
            },
        },
    }
    if err := s.RegisterTool(tool); err != nil {
        log.Fatalf("Failed to register tool: %v", err)
    }

    // Register tool handler
    if err := s.RegisterToolHandler("example_tool", func(ctx context.Context, params map[string]any) (*types.ToolResult, error) {
        return &types.ToolResult{
            Result: map[string]interface{}{
                "output": params["param1"],
            },
        }, nil
    }); err != nil {
        log.Fatalf("Failed to register tool handler: %v", err)
    }

    // Start server (implementation depends on transport)
    // ...
}
```

## Transport Options

### STDIO

STDIO transport is useful for subprocess communication, particularly when integrating with language models that execute tools as subprocesses.

```go
import "github.com/RinardNick/go-mcp-sdk/pkg/client/stdio"
```

### WebSocket

WebSocket transport provides persistent, bidirectional communication.

```go
import "github.com/RinardNick/go-mcp-sdk/pkg/client/ws"
```

### Server-Sent Events (SSE)

SSE transport is ideal for one-way streaming scenarios.

```go
import "github.com/RinardNick/go-mcp-sdk/pkg/client/sse"
```

## Testing

Run the test suite:

```bash
./scripts/run_tests.sh
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 