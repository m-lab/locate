package locatetest

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/m-lab/go/testingx"
	"github.com/m-lab/locate/api/locate"
)

func TestNewLocateServer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		loc := &Locator{
			Servers: []string{"127.0.0.1"},
		}
		srv := NewLocateServer(loc)

		c := locate.NewClient("fake-user-agent")
		u, err := url.Parse(srv.URL)
		testingx.Must(t, err, "failed to parse locatetest url")
		u.Path = "/v2/nearest/"
		c.BaseURL = u

		ctx := context.Background()
		// NOTE: only known services (e.g. ndt/ndt7) are supported by the locate API.
		targets, err := c.Nearest(ctx, "ndt/ndt7")
		testingx.Must(t, err, "failed to get response from locatetest server")

		if len(loc.Servers) != len(targets) {
			t.Errorf("NewLocateServer() = got %d, want %d", len(targets), len(loc.Servers))
		}
	})
	t.Run("error", func(t *testing.T) {
		loc := &Locator{
			Err: errors.New("fake error"),
		}
		srv := NewLocateServer(loc)

		c := locate.NewClient("fake-user-agent")
		u, err := url.Parse(srv.URL)
		testingx.Must(t, err, "failed to parse locatetest url")
		u.Path = "/v2/nearest/"
		c.BaseURL = u

		ctx := context.Background()
		// NOTE: only known services (e.g. ndt/ndt7) are supported by the locate API.
		targets, err := c.Nearest(ctx, "ndt/ndt7")
		if err == nil {
			t.Errorf("expected error, got %#v", targets)
		}
	})
}
