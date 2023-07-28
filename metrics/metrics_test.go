package metrics

import (
	"testing"

	"github.com/m-lab/go/prometheusx/promtest"
)

func TestLintMetrics(t *testing.T) {
	RequestsTotal.WithLabelValues("type", "condition", "status")
	AppEngineTotal.WithLabelValues("country")
	CurrentHeartbeatConnections.WithLabelValues("experiment").Set(0)
	LocateHealthStatus.WithLabelValues("experiment").Set(0)
	LocateMemorystoreRequestDuration.WithLabelValues("type", "field", "status")
	ImportMemorystoreTotal.WithLabelValues("status")
	PrometheusHealthCollectionDuration.WithLabelValues("code")
	ServerDistanceRanking.WithLabelValues("index")
	MetroDistanceRanking.WithLabelValues("index")
	ConnectionRequestsTotal.WithLabelValues("status")
	PortChecksTotal.WithLabelValues("status")
	KubernetesRequestsTotal.WithLabelValues("type", "status")
	KubernetesRequestTimeHistogram.WithLabelValues("healthy")
	RegistrationUpdateTime.Set(0)
	HealthTransmissionDuration.WithLabelValues("score")
	promtest.LintMetrics(nil)
}
