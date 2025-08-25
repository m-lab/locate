package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/connection"
	"github.com/m-lab/locate/connection/testdata"
)

func Test_main(t *testing.T) {
	// Override the JWT token function for testing
	getJWTTokenFunc = func(apiKey, tokenExchangeURL string) (string, error) {
		return "fake-jwt-token", nil
	}
	defer func() {
		getJWTTokenFunc = getJWTToken // restore original function
	}()

	mainCtx, mainCancel = context.WithCancel(context.Background())
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer s.Close()
	u, err := url.Parse(s.URL)
	rtx.Must(err, "could not parse server URL")

	lbPath = "/tmp/loadbalanced"
	os.WriteFile(lbPath, []byte("false"), 0644)
	defer os.Remove(lbPath)

	flag.Set("heartbeat-url", s.URL)
	flag.Set("hostname", "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org")
	flag.Set("experiment", "ndt")
	flag.Set("pod", "ndt-abcde")
	flag.Set("node", "mlab0-lga00.mlab-sandbox.measurement-lab.org")
	flag.Set("namespace", "default")
	flag.Set("registration-url", "file:./registration/testdata/registration.json")
	flag.Set("services", "ndt/ndt7=ws://:"+u.Port()+"/ndt/v7/download")
	flag.Set("api-key", "test-api-key")
	flag.Set("token-exchange-url", "http://fake-token-exchange.example.com/token")

	heartbeatPeriod = 2 * time.Second
	timer := time.NewTimer(2 * heartbeatPeriod)
	go func() {
		<-timer.C
		if heartbeatURL != s.URL {
			t.Errorf("main() incorrect locate URL; got: %s, want: %s",
				heartbeatURL, s.URL)
		}

		msg, err := fh.Read()
		rtx.Must(err, "could not read registration message")
		var hbm v2.HeartbeatMessage
		err = json.Unmarshal(msg, &hbm)
		rtx.Must(err, "could not unmarshal registration message")
		if hbm.Registration == nil {
			t.Errorf("main() did not send registration message")
		}

		msg, err = fh.Read()
		rtx.Must(err, "could not read health message")
		err = json.Unmarshal(msg, &hbm)
		rtx.Must(err, "could not unmarshal health message")
		if hbm.Health.Score != 1 {
			t.Errorf("write() did not send healthy (Score: 1) message")
		}

		mainCancel()
	}()

	main()
}

func Test_sendMessage(t *testing.T) {
	tests := []struct {
		name        string
		msg         v2.HeartbeatMessage
		msgType     string
		wantDialMsg interface{}
	}{
		{
			name:        "registration-change-dial-msg",
			msg:         v2.HeartbeatMessage{Registration: &v2.Registration{City: "changed"}},
			msgType:     "registration",
			wantDialMsg: v2.HeartbeatMessage{Registration: &v2.Registration{City: "changed"}},
		},
		{
			name:        "health-no-change",
			msg:         v2.HeartbeatMessage{Health: &v2.Health{}},
			msgType:     "health",
			wantDialMsg: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := connection.NewConn()
			defer ws.Close()

			sendMessage(ws, tt.msg, tt.msgType)
			if !reflect.DeepEqual(ws.DialMessage, tt.wantDialMsg) {
				t.Errorf("sendMessage() error updating websocket dial message; got: %v, want: %v",
					ws.DialMessage, tt.wantDialMsg)
			}
		})
	}
}
