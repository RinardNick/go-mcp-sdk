#!/bin/bash
set -e

echo "Setting up TypeScript SDK integration test environment..."

# Navigate to test directory
cd "$(dirname "$0")/../test/int_typescript_sdk"

# Clean up any existing setup
rm -rf ts || true
mkdir -p ts

cd ts

# Clone the official TypeScript SDK
echo "Cloning official TypeScript SDK..."
git clone https://github.com/modelcontextprotocol/typescript-sdk.git
cd typescript-sdk

# Install dependencies and build
echo "Building TypeScript SDK..."
npm install
npm run build

echo "Verifying TypeScript SDK build..."
ls -la

cd ..

# Create test client and server directories
mkdir -p client/src server/src

# Create client package.json
cat > client/package.json << EOL
{
  "name": "mcp-typescript-test-client",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "build": "tsc",
    "start": "node dist/client.js"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "file:../typescript-sdk",
    "zod": "^3.22.4"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0"
  }
}
EOL

# Create server package.json
cat > server/package.json << EOL
{
  "name": "mcp-typescript-test-server",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "build": "tsc",
    "start": "node dist/server.js"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "file:../typescript-sdk",
    "zod": "^3.22.4"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0"
  }
}
EOL

# Create tsconfig.json for both client and server
for dir in client server; do
  cat > $dir/tsconfig.json << EOL
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ES2020",
    "moduleResolution": "node",
    "esModuleInterop": true,
    "outDir": "dist",
    "strict": true,
    "skipLibCheck": true
  },
  "include": ["src/**/*"]
}
EOL
done

# Create server source file
cat > server/src/server.ts << EOL
import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { z } from 'zod';

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

const ListToolsRequestSchema = z.object({
    method: z.literal("tools/list")
});

const ListToolsResultSchema = z.object({
    tools: z.array(z.object({
        name: z.string(),
        description: z.string().optional(),
        inputSchema: z.object({
            type: z.literal("object"),
            properties: z.record(z.unknown()),
            required: z.array(z.string()).optional()
        })
    }))
});

const CallToolRequestSchema = z.object({
    method: z.literal("mcp/call_tool"),
    params: z.object({
        name: z.string(),
        parameters: z.record(z.unknown())
    })
});

const CallToolResultSchema = z.object({
    temperature: z.number(),
    condition: z.string(),
    location: z.string()
});

async function main() {
    console.error('Starting MCP TypeScript server...');

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

    // Handle tools/list
    server.setRequestHandler(ListToolsRequestSchema, async () => {
        return {
            tools: [weatherTool]
        };
    });

    // Handle mcp/call_tool
    server.setRequestHandler(CallToolRequestSchema, async (request) => {
        const { name, parameters } = request.params;

        if (name !== "get_weather") {
            throw new Error("Unknown tool");
        }

        const location = parameters.location;
        if (typeof location !== "string") {
            throw new Error("Invalid location parameter");
        }

        return {
            temperature: 72,
            condition: "sunny",
            location: location
        };
    });

    const transport = new StdioServerTransport();
    await server.connect(transport);

    console.error('Server started successfully');
}

main().catch(error => {
    console.error('Server error:', error);
    process.exit(1);
});
EOL

# Create client source file
cat > client/src/client.ts << EOL
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';
import { z } from 'zod';

const ListToolsRequestSchema = z.object({
    method: z.literal("tools/list")
});

const ListToolsResultSchema = z.object({
    tools: z.array(z.object({
        name: z.string(),
        description: z.string().optional(),
        inputSchema: z.object({
            type: z.literal("object"),
            properties: z.record(z.unknown()),
            required: z.array(z.string()).optional()
        })
    }))
});

const CallToolRequestSchema = z.object({
    method: z.literal("mcp/call_tool"),
    params: z.object({
        name: z.string(),
        parameters: z.record(z.unknown())
    })
});

const CallToolResultSchema = z.object({
    temperature: z.number(),
    condition: z.string(),
    location: z.string()
});

async function main() {
    console.error('Starting MCP TypeScript client...');

    const client = new Client(
        {
            name: "mcp-typescript-test-client",
            version: "1.0.0"
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

    const transport = new StdioClientTransport({
        command: process.argv[2] || "go-mcp-server",
        args: process.argv.slice(3)
    });

    await client.connect(transport);
    console.error('Client connected successfully');

    // List available tools
    const toolsResult = await client.request(
        { method: "tools/list" },
        ListToolsResultSchema
    );
    console.error('Available tools:', toolsResult.tools);

    // Call the weather tool
    const result = await client.request(
        {
            method: "mcp/call_tool",
            params: {
                name: "get_weather",
                parameters: {
                    location: "San Francisco"
                }
            }
        },
        CallToolResultSchema
    );
    console.error('Weather result:', result);

    await client.close();
}

main().catch(error => {
    console.error('Client error:', error);
    process.exit(1);
});
EOL

# Install dependencies for client and server
echo "Installing test client dependencies..."
cd client
npm install

echo "Installing test server dependencies..."
cd ../server
npm install

echo "TypeScript SDK integration test environment setup complete!" 