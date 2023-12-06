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
			path: "testdata/config.yml",
			want: Agents{
				"foo": NewCron("* * * * *", time.Minute),
				"bar": NewCron("7,8 0,15,30,45 * * * * *", time.Minute),
			},
			wantErr: false,
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
