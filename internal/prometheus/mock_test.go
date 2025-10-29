package prometheus

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

func TestNewMockServer(t *testing.T) {
	ms := NewMockServer(ScenarioHealthy, testLogger())
	defer ms.Stop()

	if ms == nil {
		t.Fatal("expected mock server to be created")
	}

	status := ms.GetStatus()
	if status.Type != ScenarioHealthy {
		t.Errorf("expected scenario type %s, got %s", ScenarioHealthy, status.Type)
	}
}

func TestGetCurrentMetrics(t *testing.T) {
	tests := []struct {
		name     string
		scenario ScenarioType
	}{
		{"healthy", ScenarioHealthy},
		{"high-errors", ScenarioHighErrors},
		{"latency-spike", ScenarioLatencySpike},
		{"gradual-degradation", ScenarioGradualDegradation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMockServer(tt.scenario, testLogger())
			defer ms.Stop()

			metrics := ms.GetCurrentMetrics()

			// Verify metrics are within valid ranges
			if metrics.ErrorRate < 0 || metrics.ErrorRate > 100 {
				t.Errorf("error rate out of range: %f", metrics.ErrorRate)
			}

			if metrics.Latency < 0 {
				t.Errorf("latency cannot be negative: %f", metrics.Latency)
			}

			if metrics.Up != 0 && metrics.Up != 1 {
				t.Errorf("up metric must be 0 or 1, got %f", metrics.Up)
			}
		})
	}
}

func TestSetScenario(t *testing.T) {
	ms := NewMockServer(ScenarioHealthy, testLogger())
	defer ms.Stop()

	// Change scenario
	ms.SetScenario(ScenarioHighErrors)

	status := ms.GetStatus()
	if status.Type != ScenarioHighErrors {
		t.Errorf("expected scenario type %s, got %s", ScenarioHighErrors, status.Type)
	}

	// Verify timer was reset
	elapsed := time.Since(status.StartTime)
	if elapsed > 100*time.Millisecond {
		t.Errorf("expected timer to be recent, elapsed: %v", elapsed)
	}
}

func TestResetTimer(t *testing.T) {
	ms := NewMockServer(ScenarioHealthy, testLogger())
	defer ms.Stop()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Get status before reset
	status1 := ms.GetStatus()

	// Reset timer
	ms.ResetTimer()

	// Get status after reset
	status2 := ms.GetStatus()

	// Start time should be more recent after reset
	if !status2.StartTime.After(status1.StartTime) {
		t.Error("expected start time to be reset")
	}
}

func TestProgressionOverTime(t *testing.T) {
	ms := NewMockServer(ScenarioHighErrors, testLogger())
	defer ms.Stop()

	// Get initial metrics
	metrics1 := ms.GetCurrentMetrics()

	// Wait for progression
	time.Sleep(100 * time.Millisecond)

	// Get metrics after some time
	metrics2 := ms.GetCurrentMetrics()

	// For high errors scenario, error rate should increase over time
	// (or stay the same if we're at the start)
	if metrics2.ErrorRate < metrics1.ErrorRate {
		t.Errorf("expected error rate to stay same or increase, got %f -> %f",
			metrics1.ErrorRate, metrics2.ErrorRate)
	}
}

func TestConcurrentAccess(t *testing.T) {
	ms := NewMockServer(ScenarioHealthy, testLogger())
	defer ms.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10
	numIterations := 100

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				_ = ms.GetCurrentMetrics()
				_ = ms.GetStatus()
			}
		}()
	}

	// Test concurrent writes
	scenarios := []ScenarioType{
		ScenarioHealthy,
		ScenarioHighErrors,
		ScenarioLatencySpike,
		ScenarioGradualDegradation,
	}

	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				scenario := scenarios[j%len(scenarios)]
				ms.SetScenario(scenario)
				if j%10 == 0 {
					ms.ResetTimer()
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestStop(t *testing.T) {
	ms := NewMockServer(ScenarioHealthy, testLogger())

	// Stop should not panic
	ms.Stop()

	// Calling stop again should also not panic
	ms.Stop()
}

func TestScenarioStatus(t *testing.T) {
	ms := NewMockServer(ScenarioGradualDegradation, testLogger())
	defer ms.Stop()

	status := ms.GetStatus()

	if status.Type != ScenarioGradualDegradation {
		t.Errorf("expected type %s, got %s", ScenarioGradualDegradation, status.Type)
	}

	if status.Description == "" {
		t.Error("expected non-empty description")
	}

	if status.Elapsed == "" {
		t.Error("expected non-empty elapsed time")
	}

	if status.StartTime.IsZero() {
		t.Error("expected non-zero start time")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{5 * time.Second, "5s"},
		{65 * time.Second, "1m 5s"},
		{3665 * time.Second, "1h 1m 5s"},
		{120 * time.Second, "2m 0s"},
		{3600 * time.Second, "1h 0m 0s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
