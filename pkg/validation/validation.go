package validation

import (
	"fmt"
	"reflect"
)

// ValidateParameter validates a parameter value against its schema
func ValidateParameter(name string, value any, schema map[string]any) error {
	if schema == nil {
		return fmt.Errorf("no schema defined for parameter: %s", name)
	}

	paramType, ok := schema["type"].(string)
	if !ok {
		return fmt.Errorf("invalid schema: type not specified for parameter: %s", name)
	}

	switch paramType {
	case "string":
		if value == nil {
			return fmt.Errorf("parameter %s: expected string, got nil", name)
		}
		if _, ok := value.(string); !ok {
			return fmt.Errorf("parameter %s: expected string, got %T", name, value)
		}
	case "number":
		if value == nil {
			return fmt.Errorf("parameter %s: expected number, got nil", name)
		}
		switch v := value.(type) {
		case float64, float32, int, int32, int64:
			// Valid number types
		default:
			return fmt.Errorf("parameter %s: expected number, got %T", name, v)
		}
	case "boolean":
		if value == nil {
			return fmt.Errorf("parameter %s: expected boolean, got nil", name)
		}
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("parameter %s: expected boolean, got %T", name, value)
		}
	case "array":
		if value == nil {
			return fmt.Errorf("parameter %s: expected array, got nil", name)
		}
		v := reflect.ValueOf(value)
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
			return fmt.Errorf("parameter %s: expected array, got %T", name, value)
		}
		// TODO: Add validation for array item types if specified in schema
	case "object":
		if value == nil {
			return fmt.Errorf("parameter %s: expected object, got nil", name)
		}
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("parameter %s: expected object, got %T", name, value)
		}
		// TODO: Add validation for object properties if specified in schema
	default:
		return fmt.Errorf("unsupported parameter type: %s", paramType)
	}

	// Check required field if specified
	if required, ok := schema["required"].(bool); ok && required && value == nil {
		return fmt.Errorf("parameter %s is required", name)
	}

	// Check enum if specified
	if enum, ok := schema["enum"].([]any); ok {
		valid := false
		for _, e := range enum {
			if reflect.DeepEqual(value, e) {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("parameter %s: value not in enum", name)
		}
	}

	return nil
}

// ValidateParameters validates a set of parameters against their schemas
func ValidateParameters(params map[string]any, schemas map[string]any) error {
	// Check for required parameters
	for name, schema := range schemas {
		schemaMap, ok := schema.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid schema for parameter: %s", name)
		}

		if required, ok := schemaMap["required"].(bool); ok && required {
			if _, exists := params[name]; !exists {
				return fmt.Errorf("missing required parameter: %s", name)
			}
		}
	}

	// Validate provided parameters
	for name, value := range params {
		schema, ok := schemas[name].(map[string]any)
		if !ok {
			return fmt.Errorf("unknown parameter: %s", name)
		}

		if err := ValidateParameter(name, value, schema); err != nil {
			return err
		}
	}

	return nil
}
