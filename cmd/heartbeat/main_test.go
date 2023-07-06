package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/url"
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
	u, err := url.Parse(s.URL)
	rtx.Must(err, "could not parse server URL")

	flag.Set("heartbeat-url", s.URL)
	flag.Set("hostname", "ndt-mlab1-lga0t.mlab-sandbox.measurement-lab.org")
	flag.Set("experiment", "ndt")
	flag.Set("pod", "ndt-abcde")
	flag.Set("node", "mlab0-lga00.mlab-sandbox.measurement-lab.org")
	flag.Set("namespace", "default")
	flag.Set("registration-url", "file:./testdata/registration.json")
	flag.Set("services", "ndt/ndt7=ws://:"+u.Port()+"/ndt/v7/download")
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
