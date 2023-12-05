// Package proxy issues requests to the legacy mlab-ns service and parses responses.
package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeNS struct {
	content     []byte
	status      int
	breakReader bool
}

func (f *fakeNS) defaultHandler(rw http.ResponseWriter, req *http.Request) {
	if f.breakReader {
		// Speicyfing a content length larger than the actual response generates
		// a read error in the client.
		rw.Header().Set("Content-Length", "8000")
	}
	rw.WriteHeader(f.status)
	rw.Write([]byte(f.content))
}

func Test_UnmarshalResponse(t *testing.T) {
	type fakeObject struct {
		Message string
	}
	tests := []struct {
		name        string
		url         string
		result      interface{}
		content     string
		status      int
		breakReader bool
		wantErr     bool
	}{
		{
			name:    "success",
			result:  &fakeObject{},
			content: `{"Message":"success"}`,
			status:  http.StatusOK,
		},
		{
			name:    "error-response",
			url:     "http://fake/this-does-not-exist",
			wantErr: true,
		},
		{
			name:    "error-no-content",
			content: "",
			status:  http.StatusNoContent,
			wantErr: true,
		},
		{
			name:        "error-reader",
			status:      http.StatusOK,
			breakReader: true,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeNS{
				content:     []byte(tt.content),
				status:      tt.status,
				breakReader: tt.breakReader,
			}
			srv := httptest.NewServer(http.HandlerFunc(f.defaultHandler))
			url := srv.URL
			if tt.url != "" {
				// Override url with test url.
				url = tt.url
			}

			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				t.Errorf("failed to create request")
			}
			resp, err := UnmarshalResponse(req, tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if resp.StatusCode != tt.status {
				t.Errorf("UnmarshalResponse() got %d, want %d", resp.StatusCode, tt.status)
			}
			obj := tt.result.(*fakeObject)
			if obj.Message != "success" {
				t.Errorf("Result did not decode message: got %q, want 'success'", obj.Message)
			}
		})
	}
}
