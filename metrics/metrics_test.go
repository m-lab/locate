package metrics

import (
	"testing"

	"github.com/m-lab/go/prometheusx/promtest"
)

func TestLintMetrics(t *testing.T) {
	RequestsTotal.WithLabelValues("type", "status")
	AppEngineTotal.WithLabelValues("country")
	CurrentHeartbeatConnections.Set(0)
	PortChecksTotal.WithLabelValues("status")
	KubernetesRequestsTotal.WithLabelValues("status")
	KubernetesRequestTimeHistogram.WithLabelValues("healthy")
	promtest.LintMetrics(nil)
}
