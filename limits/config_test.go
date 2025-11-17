package limits

import (
	"reflect"
	"testing"
	"time"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    Agents
		wantErr bool
	}{
		{
			name: "success",
			path: "testdata/config.yaml",
			want: Agents{
				"foo": NewCron("* * * * *", time.Minute),
				"bar": NewCron("7,8 0,15,30,45 * * * * *", time.Minute),
			},
			wantErr: false,
		},
		{
			name:    "file-error",
			path:    "",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfig(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseFullConfig(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantAgents   Agents
		wantTiers    TierLimits
		wantErr      bool
	}{
		{
			name: "success-with-tiers",
			path: "config.yaml",
			wantAgents: Agents{
				"node-fetch/1.0 (+https://github.com/bitinn/node-fetch)": NewCron("7,8 0,15,30,45 * * * * *", time.Minute),
			},
			wantTiers: TierLimits{
				0: LimitConfig{Interval: time.Hour, MaxEvents: 100},
				1: LimitConfig{Interval: time.Hour, MaxEvents: 500},
				2: LimitConfig{Interval: time.Hour, MaxEvents: 1000},
				3: LimitConfig{Interval: time.Hour, MaxEvents: 5000},
			},
			wantErr: false,
		},
		{
			name:       "file-error",
			path:       "nonexistent.yaml",
			wantAgents: nil,
			wantTiers:  nil,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAgents, gotTiers, err := ParseFullConfig(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFullConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Check agents
				if len(gotAgents) != len(tt.wantAgents) {
					t.Errorf("ParseFullConfig() got %d agents, want %d", len(gotAgents), len(tt.wantAgents))
				}
				// Check tiers
				if !reflect.DeepEqual(gotTiers, tt.wantTiers) {
					t.Errorf("ParseFullConfig() tiers = %+v, want %+v", gotTiers, tt.wantTiers)
				}
			}
		})
	}
}
