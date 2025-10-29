package prometheus

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MockServer manages the mock Prometheus state and auto-progression
type MockServer struct {
	mu              sync.RWMutex
	currentScenario Scenario
	startTime       time.Time
	logger          *slog.Logger
}

// MetricValues holds the current calculated metric values
type MetricValues struct {
	ErrorRate float64 // Percentage (0-100)
	Latency   float64 // Milliseconds
	Up        float64 // 0 or 1
}

// ScenarioStatus represents the current status of the mock server
type ScenarioStatus struct {
	Type        ScenarioType `json:"type"`
	Description string       `json:"description"`
	StartTime   time.Time    `json:"start_time"`
	Elapsed     string       `json:"elapsed"`
	Metrics     MetricValues `json:"metrics"`
}

// NewMockServer creates a new mock Prometheus server
func NewMockServer(initialScenario ScenarioType, logger *slog.Logger) *MockServer {
	scenario := GetScenario(initialScenario)

	ms := &MockServer{
		currentScenario: scenario,
		startTime:       time.Now(),
		logger:          logger,
	}

	ms.logger.Info("mock prometheus server initialized",
		"scenario", scenario.Type,
		"description", scenario.Description)

	return ms
}

// calculateCurrentMetrics computes metrics without holding locks
func (ms *MockServer) calculateCurrentMetrics(startTime time.Time, scenario Scenario) MetricValues {
	elapsed := time.Since(startTime)
	return MetricValues{
		ErrorRate: scenario.CalculateErrorRate(elapsed),
		Latency:   scenario.CalculateLatency(elapsed),
		Up:        scenario.CalculateUp(),
	}
}

// GetCurrentMetrics returns the current metric values based on elapsed time
func (ms *MockServer) GetCurrentMetrics() MetricValues {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return ms.calculateCurrentMetrics(ms.startTime, ms.currentScenario)
}

// GetStatus returns the current scenario status
func (ms *MockServer) GetStatus() ScenarioStatus {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	elapsed := time.Since(ms.startTime)

	return ScenarioStatus{
		Type:        ms.currentScenario.Type,
		Description: ms.currentScenario.Description,
		StartTime:   ms.startTime,
		Elapsed:     formatDuration(elapsed),
		Metrics:     ms.calculateCurrentMetrics(ms.startTime, ms.currentScenario),
	}
}

// SetScenario changes the current scenario and resets the timer
func (ms *MockServer) SetScenario(scenarioType ScenarioType) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	scenario := GetScenario(scenarioType)
	ms.currentScenario = scenario
	ms.startTime = time.Now()

	ms.logger.Info("scenario changed",
		"scenario", scenario.Type,
		"description", scenario.Description)
}

// ResetTimer resets the progression timer for the current scenario
func (ms *MockServer) ResetTimer() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.startTime = time.Now()
	ms.logger.Info("scenario timer reset", "scenario", ms.currentScenario.Type)
}

// Stop gracefully stops the mock server
func (ms *MockServer) Stop() {
	ms.logger.Info("mock prometheus server stopped")
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
