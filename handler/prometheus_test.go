package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/m-lab/locate/heartbeat"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func TestClient_Prometheus(t *testing.T) {
	tests := []struct {
		name string
		prom PrometheusClient
		want int
	}{
		{
			name: "success",
			prom: &fakePromClient{
				queryResult: model.Vector{},
			},
			want: http.StatusOK,
		},
		{
			name: "e2e error",
			prom: &fakePromClient{
				queryErr:    e2eQuery,
				queryResult: model.Vector{},
			},
			want: http.StatusInternalServerError,
		},
		{
			name: "gmx error",
			prom: &fakePromClient{
				queryErr:    gmxQuery,
				queryResult: model.Vector{},
			},
			want: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memorystore := heartbeattest.FakeMemorystoreClient
			tracker := heartbeat.NewHeartbeatStatusTracker(&memorystore)
			locator := heartbeat.NewServerLocator(tracker)
			locator.StopImport()

			c := &Client{
				LocatorV2:        locator,
				PrometheusClient: tt.prom,
			}
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v2/platform/prometheus", nil)
			c.Prometheus(rw, req)

			if tt.want != rw.Code {
				t.Errorf("Prometheus() expected status code: %d, got: %d", tt.want, rw.Code)
			}
		})
	}
}

func TestClient_query(t *testing.T) {
	tests := []struct {
		name    string
		prom    PrometheusClient
		query   string
		label   model.LabelName
		f       func(float64) bool
		want    map[string]bool
		wantErr bool
	}{
		{
			name: "query-error",
			prom: &fakePromClient{
				queryErr: "error",
			},
			query:   "error",
			wantErr: true,
		},
		{
			name: "cast-error",
			prom: &fakePromClient{
				queryResult: model.Matrix{},
			},
			query:   "query",
			wantErr: true,
		},
		{
			name: "e2e",
			prom: &fakePromClient{
				queryResult: model.Vector{
					{
						Metric: map[model.LabelName]model.LabelValue{
							e2eLabel: "success",
						},
						Value: 1,
					},
					{
						Metric: map[model.LabelName]model.LabelValue{
							e2eLabel: "failure",
						},
						Value: 0,
					},
				},
			},
			query: e2eQuery,
			label: e2eLabel,
			f:     e2eFunction,
			want: map[string]bool{
				"success": true,
				"failure": false,
			},
			wantErr: false,
		},
		{
			name: "gmx",
			prom: &fakePromClient{
				queryResult: model.Vector{
					{
						Metric: map[model.LabelName]model.LabelValue{
							gmxLabel: "not-gmx",
						},
						Value: 0,
					},
					{
						Metric: map[model.LabelName]model.LabelValue{
							gmxLabel: "gmx",
						},
						Value: 1,
					},
				},
			},
			query: gmxQuery,
			label: gmxLabel,
			f:     gmxFunction,
			want: map[string]bool{
				"not-gmx": true,
				"gmx":     false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				PrometheusClient: tt.prom,
			}

			got, err := c.query(context.TODO(), tt.query, tt.label, tt.f)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.query() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.query() = %v, want %v", got, tt.want)
			}
		})
	}
}

var errFakeQuery = errors.New("fake query error")

type fakePromClient struct {
	queryErr    string
	queryResult model.Value
}

func (p *fakePromClient) Query(ctx context.Context, query string, ts time.Time, opts ...prom.Option) (model.Value, prom.Warnings, error) {
	if query == p.queryErr {
		return nil, prom.Warnings{}, errFakeQuery
	}

	return p.queryResult, prom.Warnings{}, nil
}
