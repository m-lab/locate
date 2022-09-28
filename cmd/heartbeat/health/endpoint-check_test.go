package health

import (
	"net/http"
	"testing"

	"github.com/m-lab/locate/cmd/heartbeat/health/healthtest"
)

func Test_checkHealthEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		want    bool
		wantErr bool
	}{
		{
			name:    "200-status",
			code:    http.StatusOK,
			want:    true,
			wantErr: false,
		},
		{
			name:    "500-status",
			code:    http.StatusInternalServerError,
			want:    false,
			wantErr: false,
		},
		{
			name:    "error",
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

			got, err := checkHealthEndpoint()
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
