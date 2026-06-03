package ingestion

// GetFloat64 extracts a float64 from job metadata with a default.
func GetFloat64(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return defaultVal
}

// GetString extracts a string from job metadata with a default.
func GetString(m map[string]interface{}, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

// GetStringSlice extracts a string slice from job metadata.
func GetStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	// Handle []interface{} from YAML parsing
	if v, ok := m[key].([]any); ok {
		result := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	return nil
}