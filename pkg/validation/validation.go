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
func ValidateParameters(params map[string]any, schema map[string]any) error {
	if schema["type"] != "object" {
		return fmt.Errorf("schema must be of type object")
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid schema: properties must be an object")
	}

	// Check required fields
	required, _ := schema["required"].([]interface{})
	requiredSet := make(map[string]bool)
	for _, r := range required {
		if name, ok := r.(string); ok {
			requiredSet[name] = true
			if _, exists := params[name]; !exists {
				return fmt.Errorf("missing required field: %s", name)
			}
		}
	}

	// Validate each parameter against its schema
	for name, value := range params {
		propSchema, ok := properties[name].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unknown parameter: %s", name)
		}

		if err := validateNestedParameter(name, value, propSchema); err != nil {
			return err
		}
	}

	return nil
}

// validateNestedParameter validates a parameter and its nested fields against a schema
func validateNestedParameter(path string, value any, schema map[string]interface{}) error {
	paramType, ok := schema["type"].(string)
	if !ok {
		return fmt.Errorf("invalid schema: type not specified for parameter: %s", path)
	}

	switch paramType {
	case "object":
		objValue, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("parameter %s: expected object, got %T", path, value)
		}

		properties, ok := schema["properties"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid schema: properties must be an object for parameter: %s", path)
		}

		// Check required fields
		required, _ := schema["required"].([]interface{})
		for _, r := range required {
			if name, ok := r.(string); ok {
				if _, exists := objValue[name]; !exists {
					return fmt.Errorf("missing required field: %s.%s", path, name)
				}
			}
		}

		// Validate nested fields
		for name, nestedValue := range objValue {
			propSchema, ok := properties[name].(map[string]interface{})
			if !ok {
				return fmt.Errorf("unknown field: %s.%s", path, name)
			}

			if err := validateNestedParameter(fmt.Sprintf("%s.%s", path, name), nestedValue, propSchema); err != nil {
				return err
			}
		}

	case "array":
		// Handle both []interface{} and concrete slice types
		v := reflect.ValueOf(value)
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
			return fmt.Errorf("parameter %s: expected array, got %T", path, value)
		}

		// Validate array items if items schema is provided
		if items, ok := schema["items"].(map[string]interface{}); ok {
			for i := 0; i < v.Len(); i++ {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				item := v.Index(i).Interface()
				if err := validateNestedParameter(itemPath, item, items); err != nil {
					return err
				}
			}
		}

	default:
		// For primitive types, use the existing ValidateParameter function
		if err := ValidateParameter(path, value, schema); err != nil {
			return err
		}
	}

	return nil
}
