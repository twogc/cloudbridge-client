package performance

import (
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewOptimizer(t *testing.T) {
	optimizer := NewOptimizer(true)
	if !optimizer.enabled {
		t.Error("Expected optimizer to be enabled")
	}

	disabled := NewOptimizer(false)
	if disabled.enabled {
		t.Error("Expected optimizer to be disabled")
	}
}

func TestOptimizeForHighThroughput(t *testing.T) {
	optimizer := NewOptimizer(true)
	optimizer.OptimizeForHighThroughput()

	// Check that GOMAXPROCS is set to number of CPUs
	if runtime.GOMAXPROCS(0) != runtime.NumCPU() {
		t.Errorf("Expected GOMAXPROCS to be %d, got %d", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	}
}

func TestOptimizeForLowLatency(t *testing.T) {
	optimizer := NewOptimizer(true)
	optimizer.OptimizeForLowLatency()

	// Check that GOMAXPROCS is set to number of CPUs
	if runtime.GOMAXPROCS(0) != runtime.NumCPU() {
		t.Errorf("Expected GOMAXPROCS to be %d, got %d", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	}
}

func TestSetGCPercent(t *testing.T) {
	optimizer := NewOptimizer(true)
	optimizer.SetGCPercent(150)

	// Note: We can't easily test the actual GC percent without internal access
	// The function should not panic and should complete successfully
}

func TestSetMemoryLimit(t *testing.T) {
	optimizer := NewOptimizer(true)
	optimizer.SetMemoryLimit(1024 * 1024 * 1024) // 1GB

	// Note: We can't easily test the actual memory limit without internal access
	// The function should not panic and should complete successfully
}

func TestGetPerformanceStats(t *testing.T) {
	optimizer := NewOptimizer(true)
	stats := optimizer.GetPerformanceStats()

	// Check that all expected fields are present
	expectedFields := []string{
		"goroutines", "cpu_count", "max_procs",
		"memory_alloc", "memory_total_alloc", "memory_sys",
		"memory_heap_alloc", "memory_heap_sys",
		"gc_cycles", "gc_pause_total_ns",
	}

	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected field %s to be present in stats", field)
		}
	}

	// Check that numeric values are reasonable
	if stats["cpu_count"].(int) <= 0 {
		t.Error("Expected cpu_count to be positive")
	}
	if stats["max_procs"].(int) <= 0 {
		t.Error("Expected max_procs to be positive")
	}
}

func TestMonitorPerformance(t *testing.T) {
	optimizer := NewOptimizer(true)

	var callCount int64
	callback := func(stats map[string]interface{}) {
		atomic.AddInt64(&callCount, 1)
		if stats == nil {
			t.Error("Expected stats to be non-nil")
		}
	}

	// Start monitoring with short interval
	stop := optimizer.MonitorPerformance(10*time.Millisecond, callback)

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)

	// Stop monitoring
	stop()

	// Wait a bit more to ensure it's stopped
	time.Sleep(20 * time.Millisecond)

	initialCount := atomic.LoadInt64(&callCount)

	// Wait a bit more - should not increase
	time.Sleep(20 * time.Millisecond)

	finalCount := atomic.LoadInt64(&callCount)
	if finalCount != initialCount {
		t.Errorf("Expected callCount to remain %d, got %d", initialCount, finalCount)
	}
}

func TestMonitorPerformanceDisabled(t *testing.T) {
	optimizer := NewOptimizer(false)

	callback := func(stats map[string]interface{}) {
		t.Error("Callback should not be called when optimizer is disabled")
	}

	stop := optimizer.MonitorPerformance(10*time.Millisecond, callback)

	// Should return immediately
	time.Sleep(20 * time.Millisecond)

	// Stop should be safe to call
	stop()
}

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"1G", 1024 * 1024 * 1024, false},
		{"512M", 512 * 1024 * 1024, false},
		{"1024K", 1024 * 1024, false},
		{"1048576", 1048576, false},
		{"", 0, false},
		{"invalid", 0, true},
		{"1X", 0, false}, // parseMemoryLimit returns 0 for unknown suffix, not error
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseMemoryLimit(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input %s", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %s: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d for input %s", tt.expected, result, tt.input)
				}
			}
		})
	}
}

func TestNewOptimizerWithEnv(t *testing.T) {
	// Test with GOMEMLIMIT
	os.Setenv("GOMEMLIMIT", "512M")
	os.Setenv("GOGC", "150")
	defer func() {
		os.Unsetenv("GOMEMLIMIT")
		os.Unsetenv("GOGC")
	}()

	optimizer := NewOptimizerWithEnv(true)
	if !optimizer.enabled {
		t.Error("Expected optimizer to be enabled")
	}

	// Test with disabled optimizer
	disabled := NewOptimizerWithEnv(false)
	if disabled.enabled {
		t.Error("Expected optimizer to be disabled")
	}
}

func TestDisabledOptimizer(t *testing.T) {
	optimizer := NewOptimizer(false)

	// All methods should return without doing anything
	optimizer.OptimizeForHighThroughput()
	optimizer.OptimizeForLowLatency()
	optimizer.SetGCPercent(100)
	optimizer.SetMemoryLimit(1024)

	// GetPerformanceStats should still work
	stats := optimizer.GetPerformanceStats()
	if stats == nil {
		t.Error("Expected stats to be returned even when disabled")
	}

	// MonitorPerformance should return a no-op stop function
	stop := optimizer.MonitorPerformance(time.Second, func(map[string]interface{}) {})
	if stop == nil {
		t.Error("Expected stop function to be returned")
	}
	stop() // Should be safe to call
}
