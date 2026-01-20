package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/go-test/deep"
	"github.com/m-lab/access/controller"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/clientgeo"
	"github.com/m-lab/locate/static"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
)

func TestClient_Monitoring(t *testing.T) {
	tests := []struct {
		name            string
		claim           *jwt.Claims
		signer          Signer
		locator         LocatorV2
		path            string
		wantTokenPrefix string
		wantKey         string
		wantErr         *v2.Error
	}{
		{
			name: "success-machine",
			claim: &jwt.Claims{
				Issuer:   static.IssuerMonitoring,
				Subject:  "mlab1-lga0t.mlab-oti.measurement-lab.org",
				Audience: jwt.Audience{static.AudienceLocate},
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
			signer: &fakeSigner{},
			locator: &fakeLocatorV2{
				targets: []v2.Target{{Machine: "mlab1-lga0t.measurement-lab.org"}},
			},
			path:    "ndt/ndt5",
			wantKey: "wss://:3010/ndt_protocol",
			// The fakeSigner generates synthetic access tokens based on the claim constructed by the handler.
			// The audience (machine), the subject (monitoring), and issuer (locate). The suffix is the timestamp, which varies.
			wantTokenPrefix: "mlab1-lga0t.mlab-oti.measurement-lab.org--monitoring--locate--",
		},
		{
			name:  "error-no-claim",
			claim: nil,
			path:  "ndt/ndt5",
			wantErr: &v2.Error{
				Type:   "claim",
				Title:  "Must provide access_token",
				Status: http.StatusBadRequest,
			},
		},
		{
			name: "error-bad-subject",
			claim: &jwt.Claims{
				Issuer:   static.IssuerMonitoring,
				Subject:  "this-is-an-invalid-hostname",
				Audience: jwt.Audience{static.AudienceLocate},
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
			path: "ndt/ndt5",
			wantErr: &v2.Error{
				Type:   "subject",
				Title:  "Subject must be specified",
				Status: http.StatusBadRequest,
			},
		},
		{
			name: "error-invalid-service-path",
			claim: &jwt.Claims{
				Issuer:   static.IssuerMonitoring,
				Subject:  "mlab1-lga0t.mlab-oti.measurement-lab.org",
				Audience: jwt.Audience{static.AudienceLocate},
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Minute)),
			},
			path: "ndt/this-is-an-invalid-service-name",
			wantErr: &v2.Error{
				Type:   "config",
				Title:  "Unknown service: ndt/this-is-an-invalid-service-name",
				Status: http.StatusBadRequest,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := clientgeo.NewAppEngineLocator()
			c := NewClient("mlab-sandbox", tt.signer, tt.locator, cl, prom.NewAPI(nil), nil, nil, nil, nil, nil)
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v2/platform/monitoring/"+tt.path, nil)
			req = req.Clone(controller.SetClaim(req.Context(), tt.claim))

			c.Monitoring(rw, req)

			q := v2.MonitoringResult{}
			err := json.Unmarshal(rw.Body.Bytes(), &q)
			rtx.Must(err, "Failed to unmarshal")

			if tt.wantErr != nil {
				if q.Error == nil {
					t.Fatal("Monitoring() expected error, got nil")
				}
				if diff := deep.Equal(q.Error, tt.wantErr); diff != nil {
					t.Errorf("Monitoring() expected error: got: %#v", diff)
				}
				return
			}
			if q.Target == nil {
				t.Fatalf("Monitoring() returned nil Target")
			}
			if q.Target.Machine != tt.claim.Subject {
				t.Errorf("Monitoring() returned different machine than claim subject; got %s, want %s",
					q.Target.Machine, tt.claim.Subject)
			}
			if len(q.Target.URLs) != len(static.Configs[tt.path]) {
				t.Errorf("Monitoring() returned incomplete urls; got %d, want %d",
					len(q.Target.URLs), len(static.Configs[tt.path]))
			}
			if q.AccessToken == "" {
				t.Errorf("Monitoring() expected AccessToken, got empty string")
			}
			if strings.Contains(tt.wantTokenPrefix, q.AccessToken) {
				t.Errorf("Monitoring() did not get access token;\ngot %s,\nwant %s", q.AccessToken, tt.wantTokenPrefix)
			}
			if _, ok := q.Target.URLs[tt.wantKey]; !ok {
				t.Errorf("Monitoring() result missing URLs key; want %q", tt.wantKey)
			}
		})
	}
}
