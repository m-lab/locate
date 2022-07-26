package locatetest

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/m-lab/go/testingx"
	"github.com/m-lab/locate/api/locate"
)

func TestLocateServer_Success(t *testing.T) {
	tests := []struct {
		name string
		srv  *httptest.Server
		path string
		want int
	}{
		{
			name: "success-locate-server",
			srv: NewLocateServer(&Locator{
				Servers: []string{"127.0.0.1"},
			}),
			path: "/v2/nearest",
			want: 1,
		},
		{
			name: "success-locate-server-v2",
			srv: NewLocateServerV2(&LocatorV2{
				Servers: []string{"127.0.0.1"},
			}),
			path: "/v2beta2/nearest",
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := locate.NewClient("fake-user-agent")
			u, err := url.Parse(tt.srv.URL)
			testingx.Must(t, err, "failed to parse locatetest url")
			u.Path = tt.path
			c.BaseURL = u

			ctx := context.Background()
			// NOTE: only known services (e.g. ndt/ndt7) are supported by the locate API.
			targets, err := c.Nearest(ctx, "ndt/ndt7")
			testingx.Must(t, err, "failed to get response from locatetest server")

			if tt.want != len(targets) {
				t.Errorf("NewLocateServer() = got %d, want %d", len(targets), tt.want)
			}
		})
	}
}

func TestLocateServer_Error(t *testing.T) {
	tests := []struct {
		name string
		srv  *httptest.Server
		path string
	}{
		{
			name: "error-locate-server",
			srv: NewLocateServer(&Locator{
				Err: errors.New("fake error"),
			}),
			path: "/v2/nearest",
		},
		{
			name: "error-locate-server-v2",
			srv: NewLocateServerV2(&LocatorV2{
				Err: errors.New("fake error"),
			}),
			path: "/v2beta2/nearest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := locate.NewClient("fake-user-agent")
			u, err := url.Parse(tt.srv.URL)
			testingx.Must(t, err, "failed to parse locatetest url")
			u.Path = tt.path
			c.BaseURL = u

			ctx := context.Background()
			// NOTE: only known services (e.g. ndt/ndt7) are supported by the locate API.
			targets, err := c.Nearest(ctx, "ndt/ndt7")
			if err == nil {
				t.Errorf("expected error, got %#v", targets)
			}
		})
	}
}
