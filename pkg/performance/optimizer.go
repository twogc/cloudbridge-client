package performance

import (
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"
)

// Optimizer provides performance optimization features
type Optimizer struct {
	enabled bool
}

// NewOptimizer creates a new performance optimizer
func NewOptimizer(enabled bool) *Optimizer {
	return &Optimizer{enabled: enabled}
}

// OptimizeForHighThroughput tunes runtime for higher throughput.
// Heuristics:
// - Use all CPUs
// - Relax GC to reduce GC CPU overhead (GOGC ~ 200)
// - No artificial ballast (prefer memory limit if needed)
func (o *Optimizer) OptimizeForHighThroughput() {
	if !o.enabled {
		return
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Higher GOGC => larger heap, fewer collections => better throughput
	_ = debug.SetGCPercent(200)
	// Keep default memory limit (no hard cap). Users can override via SetMemoryLimit.
}

// OptimizeForLowLatency tunes runtime for lower latency.
// Heuristics:
// - Use all CPUs
// - Slightly tighter GC to keep heap smaller and reduce background GC debt (GOGC ~ 75)
// - Optional memory ceiling can be applied via SetMemoryLimit by caller
func (o *Optimizer) OptimizeForLowLatency() {
	if !o.enabled {
		return
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Lower GOGC => more frequent GC, smaller heap; in practice Go pauses малы,
	// но такой режим снижает риск burst-реструктуризаций под нагрузкой
	_ = debug.SetGCPercent(75)
}

// SetGCPercent sets GOGC (% growth over live heap before next GC).
// Use values like 50 (more frequent GC), 100 (default), 200+ (fewer collections).
func (o *Optimizer) SetGCPercent(percent int) {
	if !o.enabled {
		return
	}
	_ = debug.SetGCPercent(percent)
}

// SetMemoryLimit sets a soft memory limit for the Go runtime (bytes).
// Pass -1 to clear the limit. Requires Go 1.19+.
func (o *Optimizer) SetMemoryLimit(bytes int64) {
	if !o.enabled {
		return
	}
	// If bytes < 0, SetMemoryLimit(-1) clears the limit (restore default).
	debug.SetMemoryLimit(bytes)
}

// GetPerformanceStats returns current performance statistics
func (o *Optimizer) GetPerformanceStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"goroutines":         runtime.NumGoroutine(),
		"cpu_count":          runtime.NumCPU(),
		"max_procs":          runtime.GOMAXPROCS(0),
		"memory_alloc":       m.Alloc,
		"memory_total_alloc": m.TotalAlloc,
		"memory_sys":         m.Sys,
		"memory_heap_alloc":  m.HeapAlloc,
		"memory_heap_sys":    m.HeapSys,
		"gc_cycles":          m.NumGC,
		"gc_pause_total_ns":  m.PauseTotalNs,
		// NB: debug.ReadGCStats deprecated; MemStats достаточно для базового мониторинга
	}
}

// MonitorPerformance starts periodic performance reporting.
// Returns a stop func that cancels monitoring.
func (o *Optimizer) MonitorPerformance(interval time.Duration, callback func(map[string]interface{})) (stop func()) {
	if !o.enabled {
		return func() {}
	}

	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				callback(o.GetPerformanceStats())
			case <-stopCh:
				return
			}
		}
	}()
	return func() { close(stopCh) }
}

// NewOptimizerWithEnv creates a new optimizer with settings from environment variables
func NewOptimizerWithEnv(enabled bool) *Optimizer {
	optimizer := NewOptimizer(enabled)

	if !enabled {
		return optimizer
	}

	// Apply GOMEMLIMIT from environment
	if memLimit := os.Getenv("GOMEMLIMIT"); memLimit != "" {
		if bytes, err := parseMemoryLimit(memLimit); err == nil {
			optimizer.SetMemoryLimit(bytes)
		}
	}

	// Apply GOGC from environment
	if gogc := os.Getenv("GOGC"); gogc != "" {
		if percent, err := strconv.Atoi(gogc); err == nil && percent > 0 {
			optimizer.SetGCPercent(percent)
		}
	}

	return optimizer
}

// parseMemoryLimit parses memory limit from environment variable
// Supports formats like "1G", "512M", "1024K", "1048576" (bytes)
func parseMemoryLimit(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}

	// Handle numeric values (bytes)
	if bytes, err := strconv.ParseInt(s, 10, 64); err == nil {
		return bytes, nil
	}

	// Handle suffixes
	suffix := s[len(s)-1]
	value, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
	if err != nil {
		return 0, err
	}

	switch suffix {
	case 'G', 'g':
		return value * 1024 * 1024 * 1024, nil
	case 'M', 'm':
		return value * 1024 * 1024, nil
	case 'K', 'k':
		return value * 1024, nil
	default:
		return 0, nil
	}
}
