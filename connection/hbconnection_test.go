package connection

import (
	"math"
	"net/http"
	"testing"

	"github.com/gorilla/websocket"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/connection/testdata"
)

func Test_HeartbeatConnDial(t *testing.T) {
	tests := []struct {
		name    string
		dialMsg v2.Registration
		wantErr bool
	}{
		{
			name:    "successful-encoding",
			dialMsg: testdata.FakeRegistration,
			wantErr: false,
		},
		{
			name: "invalid-input",
			dialMsg: v2.Registration{
				Latitude: math.Inf(1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hbc := NewHeartbeatConn()
			defer hbc.Close()

			fh := testdata.FakeHandler{}
			s := testdata.FakeServer(fh.Upgrade)
			defer s.Close()

			gotErr := hbc.Dial(s.URL, http.Header{}, tt.dialMsg)

			if (gotErr != nil) != tt.wantErr {
				t.Errorf("HeartbeatConn.Dial() error: %v, want: %v", gotErr, tt.wantErr)
			}
		})
	}
}

func Test_HeartbeatWriteMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     v2.Health
		wantErr bool
	}{
		{
			name:    "successful-write",
			msg:     testdata.FakeHealth,
			wantErr: false,
		},
		{
			name: "invalid-input",
			msg: v2.Health{
				Score: math.Inf(1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hbc := NewHeartbeatConn()
			defer hbc.Close()

			fh := testdata.FakeHandler{}
			s := testdata.FakeServer(fh.Upgrade)
			defer s.Close()

			hbc.Dial(s.URL, http.Header{}, testdata.FakeRegistration)

			gotErr := hbc.WriteMessage(websocket.TextMessage, tt.msg)

			if (gotErr != nil) != tt.wantErr {
				t.Errorf("HeartbeatConn.WriteMessage() error: %v, want: %v", gotErr, tt.wantErr)
			}
		})
	}
}
