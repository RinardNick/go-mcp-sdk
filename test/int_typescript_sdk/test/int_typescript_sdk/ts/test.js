import { spawn } from 'child_process';
import { fileURLToPath } from 'url';
import { dirname } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const server = spawn('node', ['--loader', 'ts-node/esm', 'server.ts']);

server.stdout.on('data', (data) => {
    console.log(`stdout: ${data}`);
});

server.stderr.on('data', (data) => {
    console.error(`stderr: ${data}`);
});

server.on('close', (code) => {
    console.log(`Server process exited with code ${code}`);
});

// Send initialize request
const initializeRequest = {
    jsonrpc: '2.0',
    id: 1,
    method: 'initialize',
    params: {
        protocolVersion: '0.1.0',
        clientInfo: {
            name: 'test-client',
            version: '1.0.0'
        },
        capabilities: {
            tools: {
                supportsProgress: true,
                supportsCancellation: true
            }
        }
    }
};

server.stdin.write(JSON.stringify(initializeRequest) + '\n');

// Wait for response
server.stdout.once('data', (data) => {
    console.log('Received response:', data.toString());
    server.kill();
});
