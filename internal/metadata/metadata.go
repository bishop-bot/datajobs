package metadata

import "fmt"

// GetString extracts a string from metadata with default.
func GetString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return defaultVal
}

// GetBool extracts a bool from metadata with default.
func GetBool(m map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultVal
}

// GetInt extracts an int from metadata with default.
func GetInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	// Handle float64 from YAML parsing
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}

// GetStringSlice extracts a string slice from metadata.
func GetStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	// Handle []any from JSON/YAML parsing (mixed types)
	if v, ok := m[key].([]any); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			// Handle string
			if s, ok := item.(string); ok {
				result = append(result, s)
				continue
			}
			// Handle numbers (convert to string)
			if f, ok := item.(float64); ok {
				result = append(result, formatFloatAsString(f))
				continue
			}
			// Handle int
			if i, ok := item.(int); ok {
				result = append(result, formatIntAsString(i))
				continue
			}
		}
		return result
	}
	return nil
}

// formatFloatAsString converts a float64 to string without decimal point
// if it's a whole number, otherwise includes decimal.
func formatFloatAsString(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%v", f)
}

// formatIntAsString converts an int to string.
func formatIntAsString(i int) string {
	return fmt.Sprintf("%d", i)
}

// GetStringPtr extracts a string and returns pointer, or nil if not present.
func GetStringPtr(m map[string]interface{}, key string) *string {
	if v, ok := m[key].(string); ok && v != "" {
		return &v
	}
	return nil
}