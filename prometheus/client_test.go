package prometheus

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "success",
			addr:    "valid-url",
			wantErr: false,
		},
		{
			name:    "invalid-url",
			addr:    "invalid-url%",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(&Credentials{}, tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
