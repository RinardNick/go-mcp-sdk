import { Server } from '@modelcontextprotocol/typescript-sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/typescript-sdk/server/stdio.js';
import { z } from 'zod';
function log(message) {
    console.error(message);
}
process.on('uncaughtException', (error) => {
    log('Uncaught exception: ' + error);
    process.exit(1);
});
process.on('unhandledRejection', (error) => {
    log('Unhandled rejection: ' + error);
    process.exit(1);
});
async function main() {
    try {
        log('Starting MCP server...');
        const weatherTool = {
            name: "get_weather",
            description: "Get the weather for a location",
            inputSchema: {
                type: "object",
                properties: {
                    location: {
                        type: "string",
                        description: "The location to get weather for"
                    }
                },
                required: ["location"]
            }
        };
        // Create server with protocol version and capabilities
        const server = new Server({
            name: "mcp-typescript-test-server",
            version: "1.0.0",
            protocolVersion: "0.1.0"
        }, {
            capabilities: {
                tools: {
                    supportsProgress: true,
                    supportsCancellation: true
                }
            }
        });
        log('Server created, setting up request handlers...');
        // Implement tools/call handler
        server.setRequestHandler(z.object({ method: z.literal("tools/execute") }), async (request) => {
            const params = request.params;
            log('Executing tool with params: ' + JSON.stringify(params));
            if (!params || !params.name || params.name !== weatherTool.name) {
                throw new Error("Invalid tool name");
            }
            const parameters = params.parameters;
            if (!parameters || typeof parameters.location !== 'string') {
                throw new Error("Invalid location parameter");
            }
            return {
                result: {
                    temperature: 72,
                    condition: "sunny",
                    location: parameters.location
                }
            };
        });
        // Implement tools/list handler
        server.setRequestHandler(z.object({ method: z.literal("tools/list") }), async () => {
            return {
                tools: [weatherTool]
            };
        });
        // Implement initialize handler
        server.setRequestHandler(z.object({ method: z.literal("initialize") }), async () => {
            return {
                name: "mcp-typescript-test-server",
                version: "1.0.0",
                protocolVersion: "0.1.0",
                capabilities: {
                    tools: {
                        supportsProgress: true,
                        supportsCancellation: true
                    }
                }
            };
        });
        log('Request handlers configured, connecting transport...');
        // Start server
        const transport = new StdioServerTransport();
        await server.connect(transport);
        log('Server started successfully');
    }
    catch (error) {
        log('Server error: ' + error);
        process.exit(1);
    }
}
log('Starting main...');
main().catch(error => {
    log('Unhandled error: ' + error);
    process.exit(1);
});
