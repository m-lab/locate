package main

import (
	"flag"
	"testing"
	"time"

	"github.com/m-lab/locate/connection/testdata"
)

func Test_main(t *testing.T) {
	s := testdata.FakeServer()
	defer s.Close()

	flag.Set("locate-url", s.URL)

	timer := time.NewTimer(time.Second)
	go func() {
		<-timer.C
		if locate != s.URL {
			t.Error("main() incorrect locate URL")
		}
		done <- true
	}()

	main()
}
