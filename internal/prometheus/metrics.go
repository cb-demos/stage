package prometheus

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Pre-compiled regexes for query parsing
var (
	rateRegex      = regexp.MustCompile(`rate\(([^[]+)\[`)
	histogramRegex = regexp.MustCompile(`histogram_quantile\(([\d.]+),`)
)

// Baseline request rate for converting error percentages to rates
const baselineRequestsPerSecond = 100.0

// PrometheusResponse represents the standard Prometheus API response
type PrometheusResponse struct {
	Status string           `json:"status"`
	Data   PrometheusData   `json:"data"`
	Error  string           `json:"error,omitempty"`
	ErrorType string        `json:"errorType,omitempty"`
}

// PrometheusData represents the data portion of a Prometheus response
type PrometheusData struct {
	ResultType string            `json:"resultType"`
	Result     []PrometheusResult `json:"result"`
}

// PrometheusResult represents a single result in a Prometheus response
type PrometheusResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// QueryHandler processes PromQL queries and returns mock data
type QueryHandler struct {
	mockServer *MockServer
}

// NewQueryHandler creates a new query handler
func NewQueryHandler(mockServer *MockServer) *QueryHandler {
	return &QueryHandler{
		mockServer: mockServer,
	}
}

// ExecuteQuery processes a PromQL query and returns a Prometheus response
func (qh *QueryHandler) ExecuteQuery(query string) PrometheusResponse {
	// Get current metrics
	metrics := qh.mockServer.GetCurrentMetrics()

	// Parse and execute the query
	value, err := qh.parseQuery(query, metrics)
	if err != nil {
		return PrometheusResponse{
			Status:    "error",
			ErrorType: "bad_data",
			Error:     err.Error(),
		}
	}

	// Create response in Prometheus format
	return PrometheusResponse{
		Status: "success",
		Data: PrometheusData{
			ResultType: "vector",
			Result: []PrometheusResult{
				{
					Metric: map[string]string{
						"job": "demo-app",
					},
					Value: []interface{}{
						float64(time.Now().Unix()),
						fmt.Sprintf("%.6f", value),
					},
				},
			},
		},
	}
}

// parseQuery parses a PromQL query and calculates the result
func (qh *QueryHandler) parseQuery(query string, metrics MetricValues) (float64, error) {
	query = strings.TrimSpace(query)

	// Handle rate() function - e.g., rate(http_requests_errors_total[5m])
	if strings.HasPrefix(query, "rate(") {
		return qh.handleRate(query, metrics)
	}

	// Handle histogram_quantile() - e.g., histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
	if strings.HasPrefix(query, "histogram_quantile(") {
		return qh.handleHistogramQuantile(query, metrics)
	}

	// Handle direct metric queries
	if strings.Contains(query, "http_requests_errors_total") {
		// Return error count (convert percentage to rate per second)
		return (metrics.ErrorRate / 100.0) * baselineRequestsPerSecond, nil
	}

	if strings.Contains(query, "http_request_duration_seconds") {
		// Return latency in seconds
		return metrics.Latency / 1000.0, nil
	}

	if strings.Contains(query, "up") {
		return metrics.Up, nil
	}

	// Unknown metric - return error
	return 0, fmt.Errorf("unsupported metric query: %s", query)
}

// handleRate processes rate() function queries
func (qh *QueryHandler) handleRate(query string, metrics MetricValues) (float64, error) {
	// Extract metric name from rate(metric_name[duration])
	matches := rateRegex.FindStringSubmatch(query)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid rate query format")
	}

	metricName := strings.TrimSpace(matches[1])

	if strings.Contains(metricName, "http_requests_errors_total") {
		// Return error rate per second: (error percentage / 100) * baseline RPS
		return (metrics.ErrorRate / 100.0) * baselineRequestsPerSecond, nil
	}

	if strings.Contains(metricName, "http_request_duration_seconds") {
		// For duration metrics in rate(), return the rate of change
		// This is a simplified mock - return latency/1000 as rate
		return metrics.Latency / 1000.0, nil
	}

	return 0, fmt.Errorf("unknown metric in rate query: %s", metricName)
}

// handleHistogramQuantile processes histogram_quantile() function queries
func (qh *QueryHandler) handleHistogramQuantile(query string, metrics MetricValues) (float64, error) {
	// Extract quantile value - e.g., histogram_quantile(0.99, ...)
	matches := histogramRegex.FindStringSubmatch(query)
	if len(matches) < 2 {
		return 0, fmt.Errorf("invalid histogram_quantile format")
	}

	quantile, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid quantile value: %s", matches[1])
	}

	if quantile < 0 || quantile > 1 {
		return 0, fmt.Errorf("quantile must be between 0 and 1, got: %f", quantile)
	}

	// For realistic distributions, higher quantiles have higher latencies
	// p50 ≈ 0.8x mean, p95 ≈ 1.5x mean, p99 ≈ 2.5x mean
	meanLatency := metrics.Latency / 1000.0

	var multiplier float64
	switch {
	case quantile >= 0.99:
		multiplier = 2.5
	case quantile >= 0.95:
		// Linear interpolation between p95 (1.5x) and p99 (2.5x)
		multiplier = 1.5 + (quantile-0.95)/(0.99-0.95)*(2.5-1.5)
	case quantile >= 0.50:
		// Linear interpolation between p50 (0.8x) and p95 (1.5x)
		multiplier = 0.8 + (quantile-0.50)/(0.95-0.50)*(1.5-0.8)
	default:
		// Below p50, scale linearly from 0 to 0.8
		multiplier = quantile / 0.50 * 0.8
	}

	return meanLatency * multiplier, nil
}

// FormatMetrics returns metrics in Prometheus text exposition format
func (qh *QueryHandler) FormatMetrics() string {
	metrics := qh.mockServer.GetCurrentMetrics()

	var sb strings.Builder

	// Help and type declarations
	sb.WriteString("# HELP http_requests_errors_total Total number of HTTP request errors\n")
	sb.WriteString("# TYPE http_requests_errors_total counter\n")
	errorCount := (metrics.ErrorRate / 100.0) * baselineRequestsPerSecond
	sb.WriteString(fmt.Sprintf("http_requests_errors_total{job=\"demo-app\"} %.2f\n", errorCount))
	sb.WriteString("\n")

	sb.WriteString("# HELP http_request_duration_seconds HTTP request latency\n")
	sb.WriteString("# TYPE http_request_duration_seconds histogram\n")

	// Generate histogram buckets for latency
	latencySeconds := metrics.Latency / 1000.0

	// Guard against zero or negative latency
	if latencySeconds <= 0 {
		latencySeconds = 0.001 // 1ms minimum
	}

	buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	count := baselineRequestsPerSecond // Simulated request count

	for _, bucket := range buckets {
		// Cumulative count using square root curve for realistic distribution
		// Most requests cluster near the mean latency
		var cumCount float64
		if bucket >= latencySeconds {
			cumCount = count
		} else {
			// Square root curve: gentler distribution than linear
			ratio := bucket / latencySeconds
			cumCount = count * math.Pow(ratio, 0.5)
		}
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds_bucket{job=\"demo-app\",le=\"%.3f\"} %.0f\n", bucket, cumCount))
	}
	sb.WriteString(fmt.Sprintf("http_request_duration_seconds_bucket{job=\"demo-app\",le=\"+Inf\"} %.0f\n", count))
	sb.WriteString(fmt.Sprintf("http_request_duration_seconds_sum{job=\"demo-app\"} %.3f\n", latencySeconds*count))
	sb.WriteString(fmt.Sprintf("http_request_duration_seconds_count{job=\"demo-app\"} %.0f\n", count))
	sb.WriteString("\n")

	sb.WriteString("# HELP up Service is up\n")
	sb.WriteString("# TYPE up gauge\n")
	sb.WriteString(fmt.Sprintf("up{job=\"demo-app\"} %.0f\n", metrics.Up))

	return sb.String()
}
