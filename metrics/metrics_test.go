package metrics

import (
	"testing"

	"github.com/m-lab/go/prometheusx/promtest"
)

func TestLintMetrics(t *testing.T) {
	RequestsTotal.WithLabelValues("status")
	AppEngineTotal.WithLabelValues("country")
	HeartbeatConnections.Set(0)
	promtest.LintMetrics(nil)
}
