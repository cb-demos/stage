package prometheus

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestHandler() (*Handler, *MockServer) {
	ms := NewMockServer(ScenarioHealthy, testLogger())
	h := NewHandler(ms)
	return h, ms
}

func TestHandleQuery_GET(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/query", handler.HandleQuery)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?query=up", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response PrometheusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("expected status 'success', got %s", response.Status)
	}

	if len(response.Data.Result) == 0 {
		t.Error("expected at least one result")
	}
}

func TestHandleQuery_POST(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/query", handler.HandleQuery)

	// Test with form-encoded body (as used by continuous-verification)
	body := strings.NewReader("query=rate(http_requests_errors_total[5m])")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response PrometheusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("expected status 'success', got %s", response.Status)
	}
}

func TestHandleQuery_MissingParameter(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/query", handler.HandleQuery)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/query", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleQuery_ErrorRate(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	// Set to high errors scenario
	ms.SetScenario(ScenarioHighErrors)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/query", handler.HandleQuery)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?query=rate(http_requests_errors_total[5m])", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response PrometheusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("expected status 'success', got %s", response.Status)
	}

	// Verify we get a numeric value
	if len(response.Data.Result) > 0 {
		value := response.Data.Result[0].Value[1].(string)
		if value == "" {
			t.Error("expected non-empty value")
		}
	}
}

func TestHandleQuery_HistogramQuantile(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/query", handler.HandleQuery)

	query := "histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))"
	// URL encode the query parameter
	encodedQuery := url.QueryEscape(query)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/query?query="+encodedQuery, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response PrometheusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("expected status 'success', got %s", response.Status)
	}
}

func TestHandleMetrics(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/metrics", handler.HandleMetrics)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected text/plain content type, got %s", contentType)
	}

	body := w.Body.String()

	// Verify Prometheus text format structure
	if !strings.Contains(body, "# HELP") {
		t.Error("expected HELP comments in metrics output")
	}

	if !strings.Contains(body, "# TYPE") {
		t.Error("expected TYPE comments in metrics output")
	}

	if !strings.Contains(body, "http_requests_errors_total") {
		t.Error("expected error metric in output")
	}

	if !strings.Contains(body, "http_request_duration_seconds") {
		t.Error("expected latency metric in output")
	}

	if !strings.Contains(body, "up") {
		t.Error("expected up metric in output")
	}
}

func TestHandleGetScenario(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/prometheus/api/scenario", handler.HandleGetScenario)

	req := httptest.NewRequest(http.MethodGet, "/prometheus/api/scenario", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("expected status 'success', got %v", response["status"])
	}

	data := response["data"].(map[string]interface{})
	if data["type"] != string(ScenarioHealthy) {
		t.Errorf("expected type %s, got %v", ScenarioHealthy, data["type"])
	}
}

func TestHandleSetScenario(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/prometheus/api/scenario", handler.HandleSetScenario)

	reqBody := SetScenarioRequest{
		Scenario: string(ScenarioHighErrors),
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/prometheus/api/scenario", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify scenario was changed
	status := ms.GetStatus()
	if status.Type != ScenarioHighErrors {
		t.Errorf("expected scenario to be changed to %s, got %s", ScenarioHighErrors, status.Type)
	}
}

func TestHandleSetScenario_InvalidScenario(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/prometheus/api/scenario", handler.HandleSetScenario)

	reqBody := SetScenarioRequest{
		Scenario: "invalid-scenario",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/prometheus/api/scenario", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleSetScenario_MissingField(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/prometheus/api/scenario", handler.HandleSetScenario)

	req := httptest.NewRequest(http.MethodPost, "/prometheus/api/scenario", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleResetTimer(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/prometheus/api/scenario/reset", handler.HandleResetTimer)

	req := httptest.NewRequest(http.MethodPost, "/prometheus/api/scenario/reset", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("expected status 'success', got %v", response["status"])
	}
}

func TestHandleAdmin(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/prometheus/admin", handler.HandleAdmin)

	req := httptest.NewRequest(http.MethodGet, "/prometheus/admin", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected HTML document")
	}

	if !strings.Contains(body, "Prometheus Mock") {
		t.Error("expected admin page title")
	}
}

func TestHandleListScenarios(t *testing.T) {
	handler, ms := setupTestHandler()
	defer ms.Stop()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/prometheus/api/scenarios", handler.HandleListScenarios)

	req := httptest.NewRequest(http.MethodGet, "/prometheus/api/scenarios", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["status"] != "success" {
		t.Errorf("expected status 'success', got %v", response["status"])
	}

	data := response["data"].([]interface{})
	if len(data) != 4 {
		t.Errorf("expected 4 scenarios, got %d", len(data))
	}
}
