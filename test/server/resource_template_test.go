package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RinardNick/go-mcp-sdk/pkg/server"
	"github.com/RinardNick/go-mcp-sdk/pkg/types"
)

func TestResourceTemplates(t *testing.T) {
	// Create test server
	s := server.NewServer(&server.InitializationOptions{
		Version: "1.0",
		Capabilities: map[string]interface{}{
			"tools":     true,
			"resources": true,
		},
	})

	// Register a test resource template
	templateSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"username": map[string]interface{}{
				"type":        "string",
				"description": "GitHub username",
			},
			"repo": map[string]interface{}{
				"type":        "string",
				"description": "Repository name",
			},
		},
		"required": []string{"username", "repo"},
	}

	schemaBytes, err := json.Marshal(templateSchema)
	if err != nil {
		t.Fatalf("Failed to marshal template schema: %v", err)
	}

	template := types.Resource{
		ID:          "github-repo",
		Name:        "GitHub Repository",
		Description: "Template for GitHub repository resources",
		Type:        "git",
		URI:         "https://github.com/{username}/{repo}",
		IsTemplate:  true,
		Schema:      json.RawMessage(schemaBytes),
		Parameters: map[string]interface{}{
			"username": "",
			"repo":     "",
		},
	}

	if err := s.RegisterResource(template); err != nil {
		t.Fatalf("Failed to register resource template: %v", err)
	}

	// Test getting resource templates
	t.Run("GetResourceTemplates", func(t *testing.T) {
		templates := s.GetResourceTemplates()
		if len(templates) != 1 {
			t.Fatalf("Expected 1 template, got %d", len(templates))
		}

		if templates[0].ID != "github-repo" {
			t.Errorf("Expected template ID 'github-repo', got '%s'", templates[0].ID)
		}

		if !templates[0].IsTemplate {
			t.Error("Expected IsTemplate to be true")
		}
	})

	// Test applying resource template
	t.Run("ApplyResourceTemplate", func(t *testing.T) {
		templateReq := types.ResourceTemplate{
			ResourceID: "github-repo",
			Parameters: map[string]interface{}{
				"username": "octocat",
				"repo":     "Hello-World",
			},
		}

		result, err := s.ApplyResourceTemplate(context.Background(), templateReq)
		if err != nil {
			t.Fatalf("Failed to apply template: %v", err)
		}

		if result.Error != nil {
			t.Fatalf("Template application returned error: %v", result.Error)
		}

		if result.Resource.IsTemplate {
			t.Error("Result should not be a template")
		}

		expectedURI := "https://github.com/octocat/Hello-World"
		if result.Resource.URI != expectedURI {
			t.Errorf("Expected URI '%s', got '%s'", expectedURI, result.Resource.URI)
		}

		if result.Resource.Schema != nil {
			t.Error("Result should not include schema")
		}

		if result.Resource.Parameters != nil {
			t.Error("Result should not include parameters")
		}
	})

	// Test validation
	t.Run("TemplateValidation", func(t *testing.T) {
		testCases := []struct {
			name      string
			template  types.ResourceTemplate
			wantError bool
		}{
			{
				name: "Valid parameters",
				template: types.ResourceTemplate{
					ResourceID: "github-repo",
					Parameters: map[string]interface{}{
						"username": "octocat",
						"repo":     "Hello-World",
					},
				},
				wantError: false,
			},
			{
				name: "Missing required parameter",
				template: types.ResourceTemplate{
					ResourceID: "github-repo",
					Parameters: map[string]interface{}{
						"username": "octocat",
					},
				},
				wantError: true,
			},
			{
				name: "Invalid parameter type",
				template: types.ResourceTemplate{
					ResourceID: "github-repo",
					Parameters: map[string]interface{}{
						"username": 123,
						"repo":     "Hello-World",
					},
				},
				wantError: true,
			},
			{
				name: "Non-existent template",
				template: types.ResourceTemplate{
					ResourceID: "non-existent",
					Parameters: map[string]interface{}{
						"username": "octocat",
						"repo":     "Hello-World",
					},
				},
				wantError: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := s.ApplyResourceTemplate(context.Background(), tc.template)
				if tc.wantError {
					if err == nil {
						t.Error("Expected error, got nil")
					}
				} else {
					if err != nil {
						t.Errorf("Unexpected error: %v", err)
					}
					if result == nil {
						t.Error("Expected result, got nil")
					}
				}
			})
		}
	})
}
