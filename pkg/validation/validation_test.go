package validation

import (
	"testing"
)

func TestValidateParameter(t *testing.T) {
	tests := []struct {
		name      string
		paramName string
		value     any
		schema    map[string]any
		wantErr   bool
	}{
		{
			name:      "valid string",
			paramName: "test",
			value:     "hello",
			schema:    map[string]any{"type": "string"},
			wantErr:   false,
		},
		{
			name:      "invalid string",
			paramName: "test",
			value:     123,
			schema:    map[string]any{"type": "string"},
			wantErr:   true,
		},
		{
			name:      "valid number",
			paramName: "test",
			value:     123,
			schema:    map[string]any{"type": "number"},
			wantErr:   false,
		},
		{
			name:      "valid float",
			paramName: "test",
			value:     123.45,
			schema:    map[string]any{"type": "number"},
			wantErr:   false,
		},
		{
			name:      "invalid number",
			paramName: "test",
			value:     "123",
			schema:    map[string]any{"type": "number"},
			wantErr:   true,
		},
		{
			name:      "valid boolean",
			paramName: "test",
			value:     true,
			schema:    map[string]any{"type": "boolean"},
			wantErr:   false,
		},
		{
			name:      "invalid boolean",
			paramName: "test",
			value:     "true",
			schema:    map[string]any{"type": "boolean"},
			wantErr:   true,
		},
		{
			name:      "valid array",
			paramName: "test",
			value:     []string{"a", "b", "c"},
			schema:    map[string]any{"type": "array"},
			wantErr:   false,
		},
		{
			name:      "invalid array",
			paramName: "test",
			value:     "not an array",
			schema:    map[string]any{"type": "array"},
			wantErr:   true,
		},
		{
			name:      "valid object",
			paramName: "test",
			value:     map[string]any{"key": "value"},
			schema:    map[string]any{"type": "object"},
			wantErr:   false,
		},
		{
			name:      "invalid object",
			paramName: "test",
			value:     []string{"not", "an", "object"},
			schema:    map[string]any{"type": "object"},
			wantErr:   true,
		},
		{
			name:      "required parameter present",
			paramName: "test",
			value:     "value",
			schema:    map[string]any{"type": "string", "required": true},
			wantErr:   false,
		},
		{
			name:      "required parameter missing",
			paramName: "test",
			value:     nil,
			schema:    map[string]any{"type": "string", "required": true},
			wantErr:   true,
		},
		{
			name:      "valid enum value",
			paramName: "test",
			value:     "a",
			schema:    map[string]any{"type": "string", "enum": []any{"a", "b", "c"}},
			wantErr:   false,
		},
		{
			name:      "invalid enum value",
			paramName: "test",
			value:     "d",
			schema:    map[string]any{"type": "string", "enum": []any{"a", "b", "c"}},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParameter(tt.paramName, tt.value, tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParameter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]any
		schema  map[string]any
		wantErr bool
	}{
		{
			name: "valid parameters",
			params: map[string]any{
				"name":    "test",
				"age":     30,
				"active":  true,
				"tags":    []string{"a", "b"},
				"details": map[string]any{"key": "value"},
			},
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
					"age": map[string]any{
						"type": "number",
					},
					"active": map[string]any{
						"type": "boolean",
					},
					"tags": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
					"details": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"key": map[string]any{
								"type": "string",
							},
						},
					},
				},
				"required": []any{"name"},
			},
			wantErr: false,
		},
		{
			name: "missing required parameter",
			params: map[string]any{
				"age": 30,
			},
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
					"age": map[string]any{
						"type": "number",
					},
				},
				"required": []any{"name"},
			},
			wantErr: true,
		},
		{
			name: "unknown parameter",
			params: map[string]any{
				"unknown": "value",
			},
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid parameter type",
			params: map[string]any{
				"name": 123,
			},
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type": "string",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParameters(tt.params, tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateComplexSchema(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]any
		schema  map[string]any
		wantErr bool
	}{
		{
			name: "nested object validation",
			params: map[string]any{
				"user": map[string]any{
					"name": "John",
					"address": map[string]any{
						"street": "123 Main St",
						"city":   "San Francisco",
						"zip":    "94105",
					},
				},
			},
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type": "string",
							},
							"address": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"street": map[string]any{"type": "string"},
									"city":   map[string]any{"type": "string"},
									"zip":    map[string]any{"type": "string"},
								},
								"required": []any{"street", "city", "zip"},
							},
						},
						"required": []any{"name", "address"},
					},
				},
				"required": []any{"user"},
			},
			wantErr: false,
		},
		{
			name: "invalid nested object",
			params: map[string]any{
				"user": map[string]any{
					"name": "John",
					"address": map[string]any{
						"street": "123 Main St",
						"city":   123, // Should be string
						"zip":    "94105",
					},
				},
			},
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type": "string",
							},
							"address": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"street": map[string]any{"type": "string"},
									"city":   map[string]any{"type": "string"},
									"zip":    map[string]any{"type": "string"},
								},
								"required": []any{"street", "city", "zip"},
							},
						},
						"required": []any{"name", "address"},
					},
				},
				"required": []any{"user"},
			},
			wantErr: true,
		},
		{
			name: "missing required nested field",
			params: map[string]any{
				"user": map[string]any{
					"name": "John",
					"address": map[string]any{
						"street": "123 Main St",
						// Missing required city field
						"zip": "94105",
					},
				},
			},
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type": "string",
							},
							"address": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"street": map[string]any{"type": "string"},
									"city":   map[string]any{"type": "string"},
									"zip":    map[string]any{"type": "string"},
								},
								"required": []any{"street", "city", "zip"},
							},
						},
						"required": []any{"name", "address"},
					},
				},
				"required": []any{"user"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateParameters(tt.params, tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
