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
		prom prom.API
		want int
	}{
		{
			name: "success",
			prom: &fakePromAPI{
				queryResult: model.Vector{},
			},
			want: http.StatusOK,
		},
		{
			name: "e2e error",
			prom: &fakePromAPI{
				queryErr:    e2eQuery,
				queryResult: model.Vector{},
			},
			want: http.StatusAccepted,
		},
		{
			name: "gmx error",
			prom: &fakePromAPI{
				queryErr:    gmxQuery,
				queryResult: model.Vector{},
			},
			want: http.StatusAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memorystore := heartbeattest.FakeMemorystoreClient
			tracker := heartbeat.NewHeartbeatStatusTracker(&memorystore)
			locator := heartbeat.NewServerLocator(tracker)
			locator.StopImport()

			c := &Client{
				LocatorV2: locator,
				Prom:      tt.prom,
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
		prom    prom.API
		query   string
		label   model.LabelName
		f       func(float64) bool
		want    map[string]bool
		wantErr bool
	}{
		{
			name: "query-error",
			prom: &fakePromAPI{
				queryErr: "error",
			},
			query:   "error",
			wantErr: true,
		},
		{
			name: "cast-error",
			prom: &fakePromAPI{
				queryResult: model.Matrix{},
			},
			query:   "query",
			wantErr: true,
		},
		{
			name: "e2e",
			prom: &fakePromAPI{
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
			prom: &fakePromAPI{
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
				Prom: tt.prom,
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

type fakePromAPI struct {
	queryErr    string
	queryResult model.Value
}

func (p *fakePromAPI) Query(ctx context.Context, query string, ts time.Time, opts ...prom.Option) (model.Value, prom.Warnings, error) {
	if query == p.queryErr {
		return nil, prom.Warnings{}, errFakeQuery
	}

	return p.queryResult, prom.Warnings{}, nil
}

func (p *fakePromAPI) Alerts(ctx context.Context) (prom.AlertsResult, error) {
	return prom.AlertsResult{}, nil
}

func (p *fakePromAPI) AlertManagers(ctx context.Context) (prom.AlertManagersResult, error) {
	return prom.AlertManagersResult{}, nil
}

func (p *fakePromAPI) CleanTombstones(ctx context.Context) error {
	return nil
}

func (p *fakePromAPI) Config(ctx context.Context) (prom.ConfigResult, error) {
	return prom.ConfigResult{}, nil
}

func (p *fakePromAPI) DeleteSeries(ctx context.Context, matches []string, startTime, endTime time.Time) error {
	return nil
}

func (p *fakePromAPI) Flags(ctx context.Context) (prom.FlagsResult, error) {
	return prom.FlagsResult{}, nil
}

func (p *fakePromAPI) LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time) ([]string, prom.Warnings, error) {
	return []string{}, prom.Warnings{}, nil
}

func (p *fakePromAPI) LabelValues(ctx context.Context, label string, matches []string, startTime, endTime time.Time) (model.LabelValues, prom.Warnings, error) {
	return model.LabelValues{}, prom.Warnings{}, nil
}

func (p *fakePromAPI) QueryRange(ctx context.Context, query string, r prom.Range, opts ...prom.Option) (model.Value, prom.Warnings, error) {
	return nil, prom.Warnings{}, nil
}

func (p *fakePromAPI) QueryExemplars(ctx context.Context, query string, startTime, endTime time.Time) ([]prom.ExemplarQueryResult, error) {
	return []prom.ExemplarQueryResult{}, nil
}

func (p *fakePromAPI) Buildinfo(ctx context.Context) (prom.BuildinfoResult, error) {
	return prom.BuildinfoResult{}, nil
}

func (p *fakePromAPI) Runtimeinfo(ctx context.Context) (prom.RuntimeinfoResult, error) {
	return prom.RuntimeinfoResult{}, nil
}

func (p *fakePromAPI) Series(ctx context.Context, matches []string, startTime, endTime time.Time) ([]model.LabelSet, prom.Warnings, error) {
	return []model.LabelSet{}, prom.Warnings{}, nil
}

func (p *fakePromAPI) Snapshot(ctx context.Context, skipHead bool) (prom.SnapshotResult, error) {
	return prom.SnapshotResult{}, nil
}

func (p *fakePromAPI) Rules(ctx context.Context) (prom.RulesResult, error) {
	return prom.RulesResult{}, nil
}

func (p *fakePromAPI) Targets(ctx context.Context) (prom.TargetsResult, error) {
	return prom.TargetsResult{}, nil
}

func (p *fakePromAPI) TargetsMetadata(ctx context.Context, matchTarget, metric, limit string) ([]prom.MetricMetadata, error) {
	return []prom.MetricMetadata{}, nil
}

func (p *fakePromAPI) Metadata(ctx context.Context, metric, limit string) (map[string][]prom.Metadata, error) {
	return map[string][]prom.Metadata{}, nil
}

func (p *fakePromAPI) TSDB(ctx context.Context) (prom.TSDBResult, error) {
	return prom.TSDBResult{}, nil
}

func (p *fakePromAPI) WalReplay(ctx context.Context) (prom.WalReplayStatus, error) {
	return prom.WalReplayStatus{}, nil
}
