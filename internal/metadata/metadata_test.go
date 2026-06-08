package metadata

import (
	"testing"
)

func TestGetString(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]interface{}
		key        string
		defaultVal string
		want       string
	}{
		{
			name:       "returns value when present",
			metadata:   map[string]interface{}{"key": "value"},
			key:        "key",
			defaultVal: "default",
			want:       "value",
		},
		{
			name:       "returns default when key missing",
			metadata:   map[string]interface{}{},
			key:        "key",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "returns default when value is empty string",
			metadata:   map[string]interface{}{"key": ""},
			key:        "key",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "returns default when value is wrong type",
			metadata:   map[string]interface{}{"key": 123},
			key:        "key",
			defaultVal: "default",
			want:       "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetString(tt.metadata, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]interface{}
		key        string
		defaultVal bool
		want       bool
	}{
		{
			name:       "returns true when present",
			metadata:   map[string]interface{}{"key": true},
			key:        "key",
			defaultVal: false,
			want:       true,
		},
		{
			name:       "returns false when present",
			metadata:   map[string]interface{}{"key": false},
			key:        "key",
			defaultVal: true,
			want:       false,
		},
		{
			name:       "returns default when key missing",
			metadata:   map[string]interface{}{},
			key:        "key",
			defaultVal: true,
			want:       true,
		},
		{
			name:       "returns default when value is wrong type",
			metadata:   map[string]interface{}{"key": "true"},
			key:        "key",
			defaultVal: false,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBool(tt.metadata, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]interface{}
		key        string
		defaultVal int
		want       int
	}{
		{
			name:       "returns int when present",
			metadata:   map[string]interface{}{"key": 42},
			key:        "key",
			defaultVal: 0,
			want:       42,
		},
		{
			name:       "returns default when key missing",
			metadata:   map[string]interface{}{},
			key:        "key",
			defaultVal: 100,
			want:       100,
		},
		{
			name:       "returns default when value is wrong type",
			metadata:   map[string]interface{}{"key": "42"},
			key:        "key",
			defaultVal: 0,
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetInt(tt.metadata, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	tests := []struct {
		name       string
		metadata   map[string]interface{}
		key        string
		want       []string
	}{
		{
			name:       "returns []string when present",
			metadata:   map[string]interface{}{"key": []string{"a", "b", "c"}},
			key:        "key",
			want:       []string{"a", "b", "c"},
		},
		{
			name:       "returns []any as []string",
			metadata:   map[string]interface{}{"key": []any{"x", "y"}},
			key:        "key",
			want:       []string{"x", "y"},
		},
		{
			name:       "returns nil when key missing",
			metadata:   map[string]interface{}{},
			key:        "key",
			want:       nil,
		},
		{
			name:       "converts numbers to strings",
			metadata:   map[string]interface{}{"key": []any{"a", 123, "b"}},
			key:        "key",
			want:       []string{"a", "123", "b"},
		},
		{
			name:       "handles float64 numbers from JSON",
			metadata:   map[string]interface{}{"key": []any{265598.0, 272093.0}},
			key:        "key",
			want:       []string{"265598", "272093"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStringSlice(tt.metadata, tt.key)
			if len(got) != len(tt.want) {
				t.Errorf("GetStringSlice() len = %d, want %d", len(got), len(tt.want))
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("GetStringSlice()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestGetStringPtr(t *testing.T) {
	t.Run("returns pointer when value present", func(t *testing.T) {
		metadata := map[string]interface{}{"key": "value"}
		got := GetStringPtr(metadata, "key")
		if got == nil {
			t.Fatal("GetStringPtr() = nil, want *string")
		}
		if *got != "value" {
			t.Errorf("GetStringPtr() = %q, want %q", *got, "value")
		}
	})

	t.Run("returns nil when key missing", func(t *testing.T) {
		metadata := map[string]interface{}{}
		got := GetStringPtr(metadata, "key")
		if got != nil {
			t.Errorf("GetStringPtr() = %v, want nil", *got)
		}
	})

	t.Run("returns nil when value is empty", func(t *testing.T) {
		metadata := map[string]interface{}{"key": ""}
		got := GetStringPtr(metadata, "key")
		if got != nil {
			t.Errorf("GetStringPtr() = %v, want nil", *got)
		}
	})
}