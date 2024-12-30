package server

import (
	"context"
	"strings"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestProtocolVersionNegotiation(t *testing.T) {
	t.Run("Reject unsupported protocol version", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{
					"supportsProgress":     true,
					"supportsCancellation": true,
				},
			},
		})

		// Try to initialize with an unsupported version
		initParams := types.InitializeParams{
			ProtocolVersion: "999.0.0",
			ClientInfo: types.Implementation{
				Name:    "test-client",
				Version: "1.0.0",
			},
			Capabilities: types.ClientCapabilities{
				Tools: &types.ToolCapabilities{
					SupportsProgress:     true,
					SupportsCancellation: true,
				},
			},
		}

		result, err := s.HandleInitialize(context.Background(), initParams)
		if err == nil {
			t.Error("Expected error for unsupported protocol version, got nil")
			return
		}
		if result != nil {
			t.Errorf("Expected nil result for unsupported protocol version, got %v", result)
		}

		// Check that the error is of type MCPError
		mcpErr, ok := err.(*types.MCPError)
		if !ok {
			t.Errorf("Expected error of type *types.MCPError, got %T", err)
			return
		}

		// Check error code and message
		if mcpErr.Code != types.ErrInvalidParams {
			t.Errorf("Expected error code %d, got %d", types.ErrInvalidParams, mcpErr.Code)
		}
		if mcpErr.Message != "Invalid params" {
			t.Errorf("Expected error message 'Invalid params', got %q", mcpErr.Message)
		}

		// Check error data contains version information
		errData, ok := mcpErr.Data.(string)
		if !ok {
			t.Errorf("Expected error data to be string, got %T", mcpErr.Data)
			return
		}

		expectedParts := []string{
			"unsupported protocol version",
			"999.0.0",
			"1.0",
		}
		for _, part := range expectedParts {
			if !strings.Contains(errData, part) {
				t.Errorf("Expected error data to contain %q, got: %v", part, errData)
			}
		}
	})

	t.Run("Reject malformed protocol version", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
		})

		testCases := []string{
			"",        // Empty string
			"invalid", // Non-semver string
			"1.0.0.0", // Too many components
			"v1.0.0",  // Invalid format
			"1",       // Incomplete version
			"latest",  // Invalid keyword
		}

		for _, version := range testCases {
			result, err := s.HandleInitialize(context.Background(), types.InitializeParams{
				ProtocolVersion: version,
				ClientInfo: types.Implementation{
					Name:    "test-client",
					Version: "1.0",
				},
			})

			if err == nil {
				t.Errorf("Expected error for malformed version %q, got nil", version)
			}

			if result != nil {
				t.Errorf("Expected nil result for malformed version %q, got %v", version, result)
			}
		}
	})

	t.Run("Protocol version mismatch error details", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
		})

		result, err := s.HandleInitialize(context.Background(), types.InitializeParams{
			ProtocolVersion: "2.0", // Different major version
			ClientInfo: types.Implementation{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		if err == nil {
			t.Error("Expected error for version mismatch, got nil")
		}

		// Check that the error is of type MCPError
		mcpErr, ok := err.(*types.MCPError)
		if !ok {
			t.Errorf("Expected error of type *types.MCPError, got %T", err)
		}

		// Check error details
		if mcpErr != nil {
			if mcpErr.Code != types.ErrInvalidParams {
				t.Errorf("Expected error code %d, got %d", types.ErrInvalidParams, mcpErr.Code)
			}

			expectedMsg := "Invalid params"
			if mcpErr.Message != expectedMsg {
				t.Errorf("Expected error message %q, got %q", expectedMsg, mcpErr.Message)
			}

			expectedData := "unsupported protocol version: 2.0. Supported versions: [1.0]"
			if mcpErr.Data != expectedData {
				t.Errorf("Expected error data %q, got %q", expectedData, mcpErr.Data)
			}
		}

		if result != nil {
			t.Errorf("Expected nil result for version mismatch, got %v", result)
		}
	})

	t.Run("Protocol version fallback", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "2.0",
			SupportedVersions: []string{
				"2.0",
				"1.5",
				"1.0",
			},
		})

		// Test that we can fall back to an older version
		result, err := s.HandleInitialize(context.Background(), types.InitializeParams{
			ProtocolVersion: "1.0",
			ClientInfo: types.Implementation{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		if err != nil {
			t.Errorf("Expected no error for supported older version, got %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result for supported older version")
		}

		if result.ProtocolVersion != "1.0" {
			t.Errorf("Expected protocol version 1.0, got %s", result.ProtocolVersion)
		}

		// Test that we still reject unsupported versions
		result, err = s.HandleInitialize(context.Background(), types.InitializeParams{
			ProtocolVersion: "0.5",
			ClientInfo: types.Implementation{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		if err == nil {
			t.Error("Expected error for unsupported version")
		}

		if result != nil {
			t.Errorf("Expected nil result for unsupported version, got %v", result)
		}
	})

	t.Run("Fallback to older supported version", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version:           "2.0",
			SupportedVersions: []string{"1.0", "1.5", "0.9"}, // Ordered from newest to oldest
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{
					"supportsProgress":     true,
					"supportsCancellation": true,
				},
			},
		})

		// Try to initialize with a version that should trigger fallback
		initParams := types.InitializeParams{
			ProtocolVersion: "1.0", // Should match one of the supported versions
			ClientInfo: types.Implementation{
				Name:    "test-client",
				Version: "1.0.0",
			},
			Capabilities: types.ClientCapabilities{
				Tools: &types.ToolCapabilities{
					SupportsProgress:     true,
					SupportsCancellation: true,
				},
			},
		}

		result, err := s.HandleInitialize(context.Background(), initParams)
		if err != nil {
			t.Errorf("Expected successful fallback to version 1.0, got error: %v", err)
			return
		}
		if result == nil {
			t.Error("Expected non-nil result after successful fallback")
			return
		}

		// Verify the server responded with the fallback version
		if result.ProtocolVersion != "1.0" {
			t.Errorf("Expected server to use fallback version 1.0, got %s", result.ProtocolVersion)
		}

		// Try a version that's not in the supported list
		initParams.ProtocolVersion = "1.1"
		result, err = s.HandleInitialize(context.Background(), initParams)
		if err == nil {
			t.Error("Expected error for unsupported version 1.1")
		}
	})
}
