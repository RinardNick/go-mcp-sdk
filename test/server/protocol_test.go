package server

import (
	"context"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestProtocolVersionNegotiation(t *testing.T) {
	t.Run("Reject unsupported protocol version", func(t *testing.T) {
		s := server.NewServer(&server.InitializationOptions{
			Version: "1.0",
		})

		// Try to initialize with an unsupported version
		result, err := s.HandleInitialize(context.Background(), types.InitializeParams{
			ProtocolVersion: "999.999.999", // Unsupported version
			ClientInfo: types.Implementation{
				Name:    "test-client",
				Version: "1.0",
			},
		})

		if err == nil {
			t.Error("Expected error for unsupported protocol version, got nil")
		}

		if result != nil {
			t.Errorf("Expected nil result for unsupported protocol version, got %v", result)
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
}
