#!/bin/bash
set -e

echo "Setting up TypeScript SDK integration test environment..."

# Create test directory if it doesn't exist
TEST_DIR="test/int_typescript_sdk"
mkdir -p "${TEST_DIR}/ts"

# Clone the TypeScript SDK if not already present
if [ ! -d "${TEST_DIR}/ts/typescript-sdk" ]; then
    echo "Cloning TypeScript SDK..."
    git clone https://github.com/modelcontextprotocol/typescript-sdk.git "${TEST_DIR}/ts/typescript-sdk"
fi

# Install dependencies and build TypeScript SDK
echo "Building TypeScript SDK..."
cd "${TEST_DIR}/ts/typescript-sdk"
npm install
npm run build

# Verify the build output
echo "Verifying TypeScript SDK build..."
ls -la dist/server/

# Create example server that uses the SDK
echo "Creating test server..."
cd ..
cat > server.ts << 'EOL'
import { Server } from '@modelcontextprotocol/typescript-sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/typescript-sdk/server/stdio.js';
import { z } from 'zod';
import { Request, Tool } from '@modelcontextprotocol/typescript-sdk/types.js';

function log(message: string) {
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

        const weatherTool: Tool = {
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
        const server = new Server(
            {
                name: "mcp-typescript-test-server",
                version: "1.0.0",
                protocolVersion: "0.1.0"
            },
            {
                capabilities: {
                    tools: {
                        supportsProgress: true,
                        supportsCancellation: true
                    }
                }
            }
        );

        log('Server created, setting up request handlers...');

        // Implement tools/call handler
        server.setRequestHandler(z.object({ method: z.literal("tools/execute") }), async (request: Request) => {
            const params = request.params;
            log('Executing tool with params: ' + JSON.stringify(params));

            if (!params || !params.name || params.name !== weatherTool.name) {
                throw new Error("Invalid tool name");
            }

            const parameters = params.parameters as { location: string };
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
    } catch (error) {
        log('Server error: ' + error);
        process.exit(1);
    }
}

log('Starting main...');
main().catch(error => {
    log('Unhandled error: ' + error);
    process.exit(1);
});
EOL

# Create package.json for the test server
echo "Creating package.json..."
cat > package.json << 'EOL'
{
    "name": "mcp-typescript-test-server",
    "version": "1.0.0",
    "type": "module",
    "scripts": {
        "build": "tsc",
        "start": "node dist/server.js"
    },
    "dependencies": {
        "zod": "^3.22.4",
        "@modelcontextprotocol/typescript-sdk": "file:typescript-sdk"
    },
    "devDependencies": {
        "typescript": "^5.3.3",
        "@types/node": "^20.10.0"
    }
}
EOL

# Create tsconfig.json
echo "Creating tsconfig.json..."
cat > tsconfig.json << 'EOL'
{
    "compilerOptions": {
        "target": "ES2022",
        "module": "ES2022",
        "moduleResolution": "bundler",
        "esModuleInterop": true,
        "strict": true,
        "skipLibCheck": true,
        "allowJs": true,
        "outDir": "dist",
        "baseUrl": ".",
        "paths": {
            "@modelcontextprotocol/typescript-sdk/*": ["typescript-sdk/dist/*"]
        }
    },
    "include": ["server.ts"]
}
EOL

# Install dependencies for the test server
echo "Installing test server dependencies..."
npm install

# Build the server
echo "Building test server..."
npm run build

echo "TypeScript SDK integration test environment setup complete!" 