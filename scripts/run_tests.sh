#!/bin/bash

# Exit on error
set -e

echo -e "\nRunning integration tests..."
go test -v ./test/int_python_sdk/...

echo "Running server tests..."
go test -v ./test/server/...

echo -e "\nRunning STDIO transport tests..."
go test -v ./test/stdio/...

echo -e "\nRunning SSE transport tests..."
go test -v ./test/sse/...

echo -e "\nRunning WebSocket transport tests..."
go test -v ./test/ws/...

echo -e "\nRunning validation tests..."
go test -v ./pkg/validation/...

echo -e "\nAll tests completed successfully!" 
