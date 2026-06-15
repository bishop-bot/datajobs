package historical

import (
	"testing"
)

func TestParseHistoricalParams(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     historicalParams
	}{
		{
			name:     "uses defaults when empty",
			metadata: map[string]interface{}{},
			want: historicalParams{
				Period:     defaultPeriod,
				Bar:        defaultBar,
				OutsideRth: defaultOutsideRth,
				Instruments: nil,
			},
		},
		{
			name: "uses provided values",
			metadata: map[string]interface{}{
				"period":      "1y",
				"bar":         "1hour",
				"outsideRth":  true,
				"instruments": []any{"123", "456"},
			},
			want: historicalParams{
				Period:      "1y",
				Bar:         "1hour",
				OutsideRth:  true,
				Instruments: []string{"123", "456"},
			},
		},
		{
			name: "partial values use defaults",
			metadata: map[string]interface{}{
				"period": "2y",
			},
			want: historicalParams{
				Period:     "2y",
				Bar:        defaultBar,
				OutsideRth: defaultOutsideRth,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHistoricalParams(tt.metadata)
			if got.Period != tt.want.Period {
				t.Errorf("Period = %q, want %q", got.Period, tt.want.Period)
			}
			if got.Bar != tt.want.Bar {
				t.Errorf("Bar = %q, want %q", got.Bar, tt.want.Bar)
			}
			if got.OutsideRth != tt.want.OutsideRth {
				t.Errorf("OutsideRth = %v, want %v", got.OutsideRth, tt.want.OutsideRth)
			}
			if len(got.Instruments) != len(tt.want.Instruments) {
				t.Errorf("Instruments len = %d, want %d", len(got.Instruments), len(tt.want.Instruments))
			}
		})
	}
}

func TestHistoricalParamsDefaults(t *testing.T) {
	if defaultPeriod != "5y" {
		t.Errorf("defaultPeriod = %q, want %q", defaultPeriod, "5y")
	}
	if defaultBar != "1d" {
		t.Errorf("defaultBar = %q, want %q", defaultBar, "1d")
	}
	if defaultOutsideRth != false {
		t.Errorf("defaultOutsideRth = %v, want false", defaultOutsideRth)
	}
	if defaultPublisher != "IB" {
		t.Errorf("defaultPublisher = %q, want %q", defaultPublisher, "IB")
	}
	if upsertBatchSize != 1000 {
		t.Errorf("upsertBatchSize = %d, want %d", upsertBatchSize, 1000)
	}
}

func TestHistoricalParams(t *testing.T) {
	t.Run("empty instruments when not provided", func(t *testing.T) {
		params := historicalParams{}
		if len(params.Instruments) != 0 {
			t.Errorf("expected empty Instruments, got %v", params.Instruments)
		}
	})
}