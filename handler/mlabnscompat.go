package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/m-lab/go/host"
	"github.com/m-lab/go/rtx"
	"github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/metrics"
)

// MLabNSCompat handles mlab-ns `/ndt` and returns a compatible response.
//
// # Motivation
//
// According to https://www.measurementlab.net/blog/retiring-ndt5-raw/ ndt5 should
// have discontinued in January, 2024 along with mlab-ns. However, for technical
// reasons, there are still known mlab-ns integrators that needs a minimal amount
// of mlab-ns backward compatibility to finish their migrations.
//
// What they specifically need is for these URLs:
//
//	https://mlab-ns.appspot.com/ndt?format=json&policy=metro&metro={metro}
//	https://mlab-ns.appspot.com/ndt?format=json
//
// to return a correctly formatted response refererring to ndt7 servers on either
// the bare-metal or the auto-joined platform.
//
// Traffic from mlab-ns.appspot.com is routed to locate via dispatch.yaml.
//
// # Implementation
//
// We accept without erroring out the following query parameters:
//
//  1. format="" and format="json"
//  2. policy="", policy="geo", and policy="metro"
//  3. metro with any non-empty value
//
// However, the policy="metro" is silently ignored and remapped to the
// "geo" policy. This is acceptable because the only requirement is that
// the output does not break existing clients.
//
// The returned status code is as follows:
//
//  1. [http.StatusNotImplemented] if the method is not GET.
//  2. [http.StatusBadRequet] if we do not support the URL options.
//  3. [http.StatusBadGateway] if the inner call to `/v2/nearest/ndt/ndt7` fails.
//  4. [http.StatusNoContent] if no servers are available.
//  5. [http.StatusOK] on success.
//
// On success, we return the following JSON structure:
//
//	{
//	  "city": "",
//	  "country": "",
//	  "fqdn": "",
//	  "ip": ["127.0.0.1", "::1"],
//	  "site": "",
//	  "url": "",
//	}
//
// where:
//
//   - "city" is set to the *actual* city of the ndt7 server.
//   - "country" is set to the *actual* country of the ndt7 server.
//   - "fqdn" is set to the *actual* hostname of the ndt7 server.
//   - "ip" is set to contain specific sentinel values.
//   - "site" is set to the *actual* site of the ndt7 server.
//   - "url" is set to the *actual* HTTPS URL of the ndt7 server.
//
// This output response has been constructed to avoid breaking parsing of the
// existing consumer who need the output for documentational purposes.
//
// CAVEAT: the returned URL is not actionable to run ndt7 tests.
//
// # Metrics
//
// This handler emits [metrics.RequestsTotal] with type="mlabns". Because the
// implementation internally calls [*Client.Nearest] via [httptest.NewRecorder],
// the inner call also emits its own "nearest" metrics. Therefore, each
// successful mlab-ns compat request produces two [metrics.RequestsTotal]
// increments: one for "mlabns" and one for "nearest". This double-counting
// is intentional: it allows monitoring mlab-ns compat traffic independently
// while the "nearest" counter reflects the actual load on that code path.
//
// # Impact
//
// This API endpoint implements the required functionality and effectively
// cripples all the other existing users of mlab-ns and ndt5. This is completely
// fine since the support has been extended for two years beyond EOL.
//
// # Removal
//
// TODO(https://github.com/m-lab/locate/issues/185): this endpoint should not be
// removed until the affected integrators confirm that it is fine.
func (c *Client) MLabNSCompat(rw http.ResponseWriter, outerReq *http.Request) {
	// 1. Set the common headers
	setHeaders(rw)

	// 2. Transform the outerReq to an innerReq
	innerReq, ok := mlabnsCompatNewInnerRequest(rw, outerReq)
	if !ok {
		return // status code already set
	}

	// 3. Pass the request to [*Client.Nearest].
	respRec := httptest.NewRecorder()
	c.Nearest(respRec, innerReq)

	// 4. Serialize and send the response.
	mlabnsCompatSerializeAndSendResponse(rw, respRec)
}

// mlabnsCompatSerializeAndSendResponse writes a response from the [*httptest.ResponseRecorder] that
// captured the [*Client.Nearest] response. Separated for unit-testability.
func mlabnsCompatSerializeAndSendResponse(rw http.ResponseWriter, respRec *httptest.ResponseRecorder) {
	compatResp, ok := mlabnsCompatNewResponse(rw, respRec)
	if !ok {
		return // status code already set
	}

	rw.Header().Set("content-type", "application/json")

	respBody, err := json.Marshal(compatResp)
	rtx.PanicOnError(err, "Failed to format result") // json.Marshal cannot fail here
	rw.Write(respBody)

	metrics.RequestsTotal.WithLabelValues("mlabns", "success", http.StatusText(http.StatusOK)).Inc()
}

// mlabnsCompatNewInnnerRequest parses the `GET /ndt?format=json` request and converts it to
// the corresponding `GET /v2/nearest/ndt/ndt7` request to send to [*Client.Nearest].
//
// This method returns a non-nil request and true, on success, and nil and false, on failure.
//
// On failure, the response status code indicating error is set.
func mlabnsCompatNewInnerRequest(rw http.ResponseWriter, outerReq *http.Request) (*http.Request, bool) {
	// 1. Parse the request method
	if outerReq.Method != http.MethodGet {
		status := http.StatusNotImplemented
		rw.WriteHeader(status)
		metrics.RequestsTotal.WithLabelValues("mlabns", "method", http.StatusText(status)).Inc()
		return nil, false
	}

	// 2. Parse the query string
	//
	// Based on the historical analysis in https://github.com/m-lab/locate/pull/184
	// most requests include no parameters. When requests included parameters, the
	// following are most frequent:
	//
	//   - no parameters                    0.813
	//   - format=json                      0.068
	//   - format=bt&ip=&policy=geo_options 0.059
	//   - format=json&policy=geo           0.022
	//   - policy=geo                       0.012
	//   - policy=geo_options               0.011
	//   - policy=random                    0.003
	//   - address_family=ipv4&format=json  0.002
	//   - address_family=ipv6&format=json  0.002
	//   - format=json&metro=&policy=metro  0.001
	//
	// Supported options:
	//   - format=json        (default)
	//   - policy=geo         (default)
	//   - policy=metro
	//
	// In particular, the `policy=metro` is accepted to avoid erroring
	// clients but not acted upon. The option has no effect.
	var (
		format = outerReq.URL.Query().Get("format")
		policy = outerReq.URL.Query().Get("policy")
		metro  = outerReq.URL.Query().Get("metro")
		ok     = true
	)
	switch {
	case format != "" && format != "json": // "json" is the default
		ok = false

	case policy != "" && policy != "geo" && policy != "metro": // "geo" is the default
		ok = false

	case policy == "metro" && metro == "": // integrators use this policy w/ a valid metro
		ok = false
	}
	if !ok {
		status := http.StatusBadRequest
		rw.WriteHeader(status)
		metrics.RequestsTotal.WithLabelValues("mlabns", "query", http.StatusText(status)).Inc()
		return nil, false
	}

	// 3. Create the inner HTTP request w/ original context
	innerReq := &http.Request{
		Method: http.MethodGet,
		URL: &url.URL{
			Scheme: outerReq.URL.Scheme,
			Host:   outerReq.URL.Host,
			Path:   "/v2/nearest/ndt/ndt7",
		},
		Header:     outerReq.Header, // including app-engine headers
		Host:       outerReq.Host,
		RemoteAddr: outerReq.RemoteAddr,
		RequestURI: outerReq.RequestURI,
		TLS:        outerReq.TLS,
	}
	innerReq = innerReq.WithContext(outerReq.Context())
	return innerReq, true
}

// mlabnsCompatResponse is the response returned by `GET /ndt?format=json`.
type mlabnsCompatResponse struct {
	City    string   `json:"city"`
	Country string   `json:"country"`
	FQDN    string   `json:"fqdn"`
	IP      []string `json:"ip"`
	Site    string   `json:"site"`
	URL     string   `json:"url"`
}

// mlabnsCompatNewResponse parses the recorded response and returns the [*mlabnsCompatResponse].
//
// This method returns a non-nil response and true, on success, and nil and false, on failure.
//
// On failure, the response status code indicating error is set.
func mlabnsCompatNewResponse(rw http.ResponseWriter, respRec *httptest.ResponseRecorder) (*mlabnsCompatResponse, bool) {
	// 1. Handle failure in the inner call.
	//
	// Implementation note: we *could* remap 429 to 204 but, in such a case, the clients
	// would retry after a short delay and we don't want this to happen.
	if respRec.Code != http.StatusOK {
		status := http.StatusBadGateway
		rw.WriteHeader(status)
		metrics.RequestsTotal.WithLabelValues("mlabns", "inner status", http.StatusText(status)).Inc()
		return nil, false
	}

	// 2. Parse the inner call body.
	//
	// [http.StatusNoContent] is historically used by mlab-ns to indicate that no servers
	// are available; see https://github.com/m-lab/locate/pull/184.
	v2Result := v2.NearestResult{}
	if err := json.Unmarshal(respRec.Body.Bytes(), &v2Result); err != nil {
		status := http.StatusBadGateway
		rw.WriteHeader(status)
		metrics.RequestsTotal.WithLabelValues("mlabns", "inner json", http.StatusText(status)).Inc()
		return nil, false
	}
	if len(v2Result.Results) <= 0 || v2Result.Results[0].Location == nil {
		status := http.StatusNoContent
		rw.WriteHeader(status)
		metrics.RequestsTotal.WithLabelValues("mlabns", "inner empty", http.StatusText(status)).Inc()
		return nil, false
	}
	res0 := v2Result.Results[0]

	// 3. Assemble the compatibility response
	ph, err := host.Parse(res0.Machine)
	if err != nil {
		status := http.StatusNoContent
		rw.WriteHeader(status)
		metrics.RequestsTotal.WithLabelValues("mlabns", "machine", http.StatusText(status)).Inc()
		return nil, false
	}
	fqdn := res0.Hostname
	compat := &mlabnsCompatResponse{
		City:    res0.Location.City,
		Country: res0.Location.Country,
		FQDN:    fqdn,
		IP:      []string{"127.0.0.1", "::1"}, // sentinel values
		Site:    ph.Site,
		URL:     (&url.URL{Scheme: "https", Host: fqdn, Path: "/"}).String(),
	}
	return compat, true
}
