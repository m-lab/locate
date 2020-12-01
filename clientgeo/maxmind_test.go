package clientgeo

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/m-lab/go/content"
	"github.com/m-lab/go/rtx"
)

// Networks taken from https://github.com/maxmind/MaxMind-DB/blob/master/source-data/GeoIP2-City-Test.json
var localIP = "175.16.199.3"
var remoteIP = "2.125.160.216" // includes multiple subdivision annotations.

func TestNewMaxmindLocator(t *testing.T) {
	var localRawfile content.Provider

	tests := []struct {
		name       string
		useHeaders map[string]string
		remoteIP   string
		want       *Location
		filename   string
		reloadDB   bool
		wantErr    bool
	}{
		{
			name: "success-using-X-Forwarded-For-header",
			useHeaders: map[string]string{
				"X-Forwarded-For": "2.125.160.216, 192.168.0.2",
			},
			remoteIP: remoteIP + ":1234",
			want: &Location{
				Latitude:  "51.750000",
				Longitude: "-1.250000",
				Headers: http.Header{
					hLocateClientlatlon:       []string{"51.750000,-1.250000"},
					hLocateClientlatlonMethod: []string{"maxmind-remoteip"},
				},
			},
			filename: "file:./testdata/fake.tar.gz",
		},
		{
			name:     "success-using-remote-ip",
			remoteIP: remoteIP + ":1234",
			want: &Location{
				Latitude:  "51.750000",
				Longitude: "-1.250000",
				Headers: http.Header{
					hLocateClientlatlon:       []string{"51.750000,-1.250000"},
					hLocateClientlatlonMethod: []string{"maxmind-remoteip"},
				},
			},
			filename: "file:./testdata/fake.tar.gz",
		},
		{
			name:     "error-remote-ip-split-error",
			remoteIP: "invalid-ip-1234",
			filename: "file:./testdata/fake.tar.gz",
			wantErr:  true,
		},
		{
			name:     "error-remote-ip-parses-as-nil",
			remoteIP: "invalid-ip:1234",
			filename: "file:./testdata/fake.tar.gz",
			wantErr:  true,
		},
		{
			name:     "error-maxmind-db-error",
			remoteIP: remoteIP + ":1234",
			reloadDB: true,
			filename: "file:./testdata/fake.tar.gz",
			wantErr:  true,
		},
		{
			name:     "error-wrong-db-type",
			remoteIP: "127.0.0.1:1234",
			filename: "file:./testdata/wrongtype.tar.gz",
			wantErr:  true,
		},
		{
			name:     "error-empty-response-from-db",
			remoteIP: "127.0.0.1:1234",
			filename: "file:./testdata/fake.tar.gz",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.filename)
			rtx.Must(err, "Could not parse URL")
			localRawfile, err = content.FromURL(context.Background(), u)
			rtx.Must(err, "Could not create content.Provider")

			ctx := context.Background()
			locator := NewMaxmindLocator(ctx, localRawfile)
			if tt.reloadDB {
				// This will result in a null maxmind db, b/c the localRawfile has not changed.
				locator = NewMaxmindLocator(ctx, localRawfile)
			}

			req := httptest.NewRequest(http.MethodGet, "/anytarget", nil)
			for key, value := range tt.useHeaders {
				req.Header.Set(key, value)
			}
			req.RemoteAddr = tt.remoteIP

			l, err := locator.Locate(req)
			if (err != nil) && !tt.wantErr {
				t.Errorf("MaxmindLocator.Locate got error: %v", err)
			}
			// fmt.Printf("%#v\n", l)
			if !reflect.DeepEqual(l, tt.want) {
				t.Errorf("NewMaxmindLocator() = %v, want %v", l, tt.want)
			}
		})
	}
}

// workOnceProvider returns an error the second reload.
type workOnceProvider struct {
	provider content.Provider
	called   bool
}

func (w *workOnceProvider) Get(ctx context.Context) ([]byte, error) {
	if !w.called {
		w.called = true
		return w.provider.Get(ctx)
	}
	return nil, errors.New("fake error on second load")
}

// emptyReloadProvider returns an empty archive on the second reload.
type emptyReloadProvider struct {
	provider content.Provider
	called   bool
}

func (e *emptyReloadProvider) Get(ctx context.Context) ([]byte, error) {
	if !e.called {
		e.called = true
		return e.provider.Get(ctx)
	}
	return []byte("bad-content"), nil
}

func loadProvider(name string) content.Provider {
	u, err := url.Parse(name)
	rtx.Must(err, "Could not parse URL")
	localRawfile, err := content.FromURL(context.Background(), u)
	rtx.Must(err, "Could not create content.Provider")
	return localRawfile
}

func TestMaxmindLocator_Reload(t *testing.T) {
	tests := []struct {
		name       string
		workOnce   bool
		emptyAfter bool
	}{
		{
			name: "success",
		},
		{
			name:     "success-fail-second-reload-error",
			workOnce: true,
		},
		{
			name:       "success-fail-second-reload-data",
			emptyAfter: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			localRawfile := loadProvider("file:./testdata/fake.tar.gz")

			if tt.workOnce {
				localRawfile = &workOnceProvider{
					provider: localRawfile,
				}
			}
			if tt.emptyAfter {
				localRawfile = &emptyReloadProvider{
					provider: localRawfile,
				}
			}

			ctx := context.Background()
			mml := NewMaxmindLocator(ctx, localRawfile)
			mml.Reload(ctx)
		})
	}
}
