package main

// import (
// 	"context"
// 	"flag"
// 	"testing"
// 	"time"

// 	"github.com/m-lab/locate/connection/testdata"
// )

// func Test_main(t *testing.T) {
// 	mainCtx, mainCancel = context.WithCancel(context.Background())
// 	fh := testdata.FakeHandler{}
// 	s := testdata.FakeServer(fh.Upgrade)
// 	defer s.Close()

// 	flag.Set("locate-url", s.URL)

// 	timer := time.NewTimer(time.Second)
// 	go func() {
// 		<-timer.C
// 		if locate != s.URL {
// 			t.Errorf("main() incorrect locate URL; got: %s, want: %s",
// 				locate, s.URL)
// 		}
// 		mainCancel()
// 	}()

// 	main()
// }
