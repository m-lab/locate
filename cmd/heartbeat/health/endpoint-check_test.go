package health

import (
	"net/http"
	"testing"
	"time"

	"github.com/m-lab/locate/cmd/heartbeat/health/healthtest"
)

func Test_checkHealthEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		timeout time.Duration
		want    bool
		wantErr bool
	}{
		{
			name:    "200-status",
			code:    http.StatusOK,
			timeout: time.Second,
			want:    true,
			wantErr: false,
		},
		{
			name:    "timeout",
			code:    http.StatusOK,
			timeout: 0,
			want:    false,
			wantErr: true,
		},
		{
			name:    "500-status",
			code:    http.StatusInternalServerError,
			timeout: time.Second,
			want:    false,
			wantErr: false,
		},
		{
			name:    "error",
			timeout: time.Second,
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantErr {
				srv := healthtest.TestHealthServer(tt.code)
				healthAddress = srv.URL + "/health"
				defer srv.Close()
			}

			hc := NewEndpointClient(time.Second)
			got, err := hc.checkHealthEndpoint()
			if (err != nil) != tt.wantErr {
				t.Errorf("checkHealthEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("checkHealthEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}
