// Package proxy issues requests to the legacy mlab-ns service and parses responses.
package proxy

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// TODO(soltesz): move proxy to another package.

// ErrNoContent is returned when mlab-ns returns http.StatusNoContent.
var ErrNoContent = errors.New("no content from server")

// UnmarshalResponse reads the response from the given request and unmarshals
// the value into the given result.
func UnmarshalResponse(req *http.Request, result interface{}) (*http.Response, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode == http.StatusNoContent {
		// Cannot unmarshal empty content.
		return resp, ErrNoContent
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, err
	}
	return resp, json.Unmarshal(b, result)
}
