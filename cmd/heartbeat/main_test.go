package main

import (
	"context"
	"encoding/json"
	"flag"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/m-lab/locate/connection/testdata"
)

func Test_main(t *testing.T) {
	mainCtx, mainCancel = context.WithCancel(context.Background())
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer s.Close()

	flag.Set("heartbeat-url", s.URL)
	flag.Set("hostname", "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org")
	flag.Set("registration-url", "file:./registration/testdata/registration.json")

	heartbeatPeriod = 2 * time.Second
	timer := time.NewTimer(2 * heartbeatPeriod)
	go func() {
		<-timer.C
		if heartbeatURL != s.URL {
			t.Errorf("main() incorrect locate URL; got: %s, want: %s",
				heartbeatURL, s.URL)
		}

		msg, err := fh.Read()
		if msg == nil || err != nil {
			t.Errorf("write() did not send heartbeat message")
		}

		mainCancel()
	}()

	main()
}
func Test_constructHeartbeatMsg(t *testing.T) {
	tests := []struct {
		name    string
		msg     json.RawMessage
		wantErr bool
		wantMsg []byte
	}{
		{
			name: "success-message",
			msg: json.RawMessage(`{
				"Hostname": "fakeHostname",
				"Score": 1
			}`),
			wantErr: false,
			wantMsg: []byte(`{"msgType":"fakeType","msg":{"Hostname":"fakeHostname","Score":1}}`),
		},
		{
			name:    "invalid-json-message",
			msg:     json.RawMessage("foo"),
			wantErr: true,
			wantMsg: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, gotErr := constructHeartbeatMsg("fakeType", tt.msg)

			if (gotErr != nil) != tt.wantErr {
				t.Errorf("constructHeartbeatMsg() error: %v, want: %v", gotErr, tt.wantErr)
			}

			if diff := deep.Equal(gotMsg, tt.wantMsg); diff != nil {
				t.Errorf("constructHeartbeatMsg() message did not match; got: %s, want: %s", string(gotMsg), string(tt.wantMsg))
			}
		})
	}
}
