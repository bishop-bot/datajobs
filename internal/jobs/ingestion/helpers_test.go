package ingestion

import (
	"testing"
)

func TestGetFloat64(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]interface{}
		key        string
		defaultVal float64
		expected   float64
	}{
		{
			name:       "existing key returns value",
			metadata:   map[string]interface{}{"key": float64(100)},
			key:        "key",
			defaultVal: 50,
			expected:   100,
		},
		{
			name:       "missing key returns default",
			metadata:   map[string]interface{}{"other": float64(100)},
			key:        "key",
			defaultVal: 50,
			expected:   50,
		},
		{
			name:       "nil metadata returns default",
			metadata:   nil,
			key:        "key",
			defaultVal: 50,
			expected:   50,
		},
		{
			name:       "zero value returns zero",
			metadata:   map[string]interface{}{"key": float64(0)},
			key:        "key",
			defaultVal: 50,
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFloat64(tt.metadata, tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]interface{}
		key        string
		defaultVal string
		expected   string
	}{
		{
			name:       "existing key returns value",
			metadata:   map[string]interface{}{"key": "value"},
			key:        "key",
			defaultVal: "default",
			expected:   "value",
		},
		{
			name:       "missing key returns default",
			metadata:   map[string]interface{}{"other": "value"},
			key:        "key",
			defaultVal: "default",
			expected:   "default",
		},
		{
			name:       "nil metadata returns default",
			metadata:   nil,
			key:        "key",
			defaultVal: "default",
			expected:   "default",
		},
		{
			name:       "empty string returns empty",
			metadata:   map[string]interface{}{"key": ""},
			key:        "key",
			defaultVal: "default",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetString(tt.metadata, tt.key, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]interface{}
		key        string
		expected   []string
	}{
		{
			name:       "[]string returns value",
			metadata:   map[string]interface{}{"key": []string{"a", "b", "c"}},
			key:        "key",
			expected:   []string{"a", "b", "c"},
		},
		{
			name:       "[]any returns values",
			metadata:   map[string]interface{}{"key": []any{"x", "y", "z"}},
			key:        "key",
			expected:   []string{"x", "y", "z"},
		},
		{
			name:       "missing key returns nil",
			metadata:   map[string]interface{}{"other": []string{"a"}},
			key:        "key",
			expected:   nil,
		},
		{
			name:       "nil metadata returns nil",
			metadata:   nil,
			key:        "key",
			expected:   nil,
		},
		{
			name:       "mixed []any returns strings only",
			metadata:   map[string]interface{}{"key": []any{"a", "b", 123}},
			key:        "key",
			expected:   []string{"a", "b", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringSlice(tt.metadata, tt.key)
			if len(result) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(result))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("element %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}