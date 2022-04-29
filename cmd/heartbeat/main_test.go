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

	flag.Set("locate-url", s.URL)

	heartbeatPeriod = 2 * time.Second
	timer := time.NewTimer(2 * heartbeatPeriod)
	go func() {
		<-timer.C
		if locate != s.URL {
			t.Errorf("main() incorrect locate URL; got: %s, want: %s",
				locate, s.URL)
		}

		msg, err := fh.Read()
		if msg == nil || err != nil {
			t.Errorf("write() did not send heartbeat message")
		}

		mainCancel()
	}()

	main()
}
