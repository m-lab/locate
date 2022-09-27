package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
				srv := testHealthServer(tt.code)
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

func testHealthServer(code int) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}))
	s := httptest.NewServer(mux)
	healthAddress = s.URL + "/health"
	return s
}
