package main

import (
	"context"
	"flag"
	"testing"
	"time"

	"github.com/m-lab/locate/connection/testdata"
)

func Test_main(t *testing.T) {
	mainCtx, mainCancel = context.WithCancel(context.Background())
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer s.Close()

	flag.Set("heartbeat-url", s.URL)
	flag.Set("hostname", "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org")
	flag.Set("registration-url", "file:./messaging/testdata/registration.json")

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
