package pathselect

import "sync/atomic"

// Counters for Phase C path selection observability (package-level; Prometheus later).
var (
	metricPathSelected  atomic.Int64
	metricPathFail      atomic.Int64
	metricFailoverTotal atomic.Int64
)

// MetricsSnapshot is a point-in-time view of pathselect counters.
type MetricsSnapshot struct {
	PathSelected  int64 `json:"path_selected"`
	PathFail      int64 `json:"path_fail"`
	FailoverTotal int64 `json:"failover_total"`
}

// SnapshotMetrics returns current counter values.
func SnapshotMetrics() MetricsSnapshot {
	return MetricsSnapshot{
		PathSelected:  metricPathSelected.Load(),
		PathFail:      metricPathFail.Load(),
		FailoverTotal: metricFailoverTotal.Load(),
	}
}

// ResetMetrics zeros counters (tests only).
func ResetMetrics() {
	metricPathSelected.Store(0)
	metricPathFail.Store(0)
	metricFailoverTotal.Store(0)
}

func notePathSelected()  { metricPathSelected.Add(1) }
func notePathFail()      { metricPathFail.Add(1) }
func noteFailover()      { metricFailoverTotal.Add(1) }
