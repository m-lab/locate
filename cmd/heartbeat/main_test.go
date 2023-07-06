package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/connection/testdata"
)

func Test_main(t *testing.T) {
	mainCtx, mainCancel = context.WithCancel(context.Background())
	fh := testdata.FakeHandler{}
	s := testdata.FakeServer(fh.Upgrade)
	defer s.Close()

	flag.Set("heartbeat-url", s.URL)
	flag.Set("hostname", "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org")
	flag.Set("experiment", "ndt")
	flag.Set("pod", "ndt-abcde")
	flag.Set("node", "mlab0-lga00.mlab-sandbox.measurement-lab.org")
	flag.Set("namespace", "default")
	flag.Set("kubernetes-url", "https://localhost:1234")
	flag.Set("registration-url", "file:./testdata/registration.json")
	flag.Set("services", "ndt/ndt7=ws:///ndt/v7/download,ws:///ndt/v7/upload")
	kubernetesAuth = "health/testdata"

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

		p, err := os.FindProcess(os.Getpid())
		rtx.Must(err, "could not get the current process")
		err = p.Signal(syscall.SIGTERM)
		rtx.Must(err, "could not send signal")
		msg, err = fh.Read()
		if err != nil {
			t.Errorf("write() did not send message")
		}
		var hbm v2.HeartbeatMessage
		err = json.Unmarshal(msg, &hbm)
		rtx.Must(err, "could not unmarshal message")
		if hbm.Health.Score != 0 {
			t.Errorf("write() did not send 0 health message")
		}

		mainCancel()
	}()

	main()
}
