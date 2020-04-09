package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/go-test/deep"
	"github.com/m-lab/access/controller"
	"github.com/m-lab/go/rtx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

func TestClient_Monitoring(t *testing.T) {
	tests := []struct {
		name    string
		claim   *jwt.Claims
		signer  Signer
		locator Locator
		path    string
		wantErr *v2.Error
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
			locator: &fakeLocator{
				machines: []string{"mlab1-lga0t.measurement-lab.org"},
			},
			path: "ndt/ndt5",
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
			c := NewClient("mlab-sandbox", tt.signer, tt.locator)
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v2/monitoring/"+tt.path, nil)
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
		})
	}
}
