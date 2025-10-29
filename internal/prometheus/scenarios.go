package prometheus

import (
	"math"
	"time"
)

// ScenarioType represents the different mock metric scenarios
type ScenarioType string

const (
	ScenarioHealthy            ScenarioType = "healthy"
	ScenarioHighErrors         ScenarioType = "high-errors"
	ScenarioLatencySpike       ScenarioType = "latency-spike"
	ScenarioGradualDegradation ScenarioType = "gradual-degradation"
)

// Scenario defines the behavior and progression rules for a mock scenario
type Scenario struct {
	Type        ScenarioType
	Description string

	// Error rate configuration (as a percentage, 0.0 to 100.0)
	ErrorRateStart    float64
	ErrorRateEnd      float64
	ErrorRateDuration time.Duration

	// Latency configuration (in milliseconds)
	LatencyStart    float64
	LatencyEnd      float64
	LatencyDuration time.Duration

	// Uptime (0 or 1)
	Up float64
}

// AllScenarios returns all available scenarios
func AllScenarios() map[ScenarioType]Scenario {
	return map[ScenarioType]Scenario{
		ScenarioHealthy: {
			Type:              ScenarioHealthy,
			Description:       "Healthy application with minimal errors and low latency",
			ErrorRateStart:    0.1,
			ErrorRateEnd:      0.1,
			ErrorRateDuration: 0, // Static
			LatencyStart:      100,
			LatencyEnd:        100,
			LatencyDuration:   0, // Static
			Up:                1,
		},
		ScenarioHighErrors: {
			Type:              ScenarioHighErrors,
			Description:       "High error rate that progressively increases",
			ErrorRateStart:    5.0,
			ErrorRateEnd:      25.0,
			ErrorRateDuration: 5 * time.Minute,
			LatencyStart:      200,
			LatencyEnd:        200,
			LatencyDuration:   0, // Static
			Up:                1,
		},
		ScenarioLatencySpike: {
			Type:              ScenarioLatencySpike,
			Description:       "Latency spike with gradual increase",
			ErrorRateStart:    0.5,
			ErrorRateEnd:      0.5,
			ErrorRateDuration: 0, // Static
			LatencyStart:      150,
			LatencyEnd:        2000,
			LatencyDuration:   3 * time.Minute,
			Up:                1,
		},
		ScenarioGradualDegradation: {
			Type:              ScenarioGradualDegradation,
			Description:       "Both errors and latency degrade over time",
			ErrorRateStart:    0.5,
			ErrorRateEnd:      15.0,
			ErrorRateDuration: 10 * time.Minute,
			LatencyStart:      120,
			LatencyEnd:        800,
			LatencyDuration:   10 * time.Minute,
			Up:                1,
		},
	}
}

// GetScenario returns a scenario by type, or the healthy scenario if not found
func GetScenario(scenarioType ScenarioType) Scenario {
	scenarios := AllScenarios()
	if scenario, ok := scenarios[scenarioType]; ok {
		return scenario
	}
	return scenarios[ScenarioHealthy]
}

// CalculateErrorRate calculates the current error rate based on elapsed time
func (s *Scenario) CalculateErrorRate(elapsed time.Duration) float64 {
	if s.ErrorRateDuration == 0 {
		return s.ErrorRateStart
	}

	progress := float64(elapsed) / float64(s.ErrorRateDuration)
	if progress >= 1.0 {
		return s.ErrorRateEnd
	}

	// Linear interpolation
	return s.ErrorRateStart + (s.ErrorRateEnd-s.ErrorRateStart)*progress
}

// CalculateLatency calculates the current p99 latency based on elapsed time
func (s *Scenario) CalculateLatency(elapsed time.Duration) float64 {
	if s.LatencyDuration == 0 {
		return s.LatencyStart
	}

	progress := float64(elapsed) / float64(s.LatencyDuration)
	if progress >= 1.0 {
		return s.LatencyEnd
	}

	// Exponential curve for latency spikes (feels more realistic)
	return s.LatencyStart + (s.LatencyEnd-s.LatencyStart)*math.Pow(progress, 2)
}

// CalculateUp returns the uptime value
func (s *Scenario) CalculateUp() float64 {
	return s.Up
}

// ValidScenarioTypes returns all valid scenario type strings
func ValidScenarioTypes() []string {
	return []string{
		string(ScenarioHealthy),
		string(ScenarioHighErrors),
		string(ScenarioLatencySpike),
		string(ScenarioGradualDegradation),
	}
}
