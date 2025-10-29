package prometheus

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed admin.html
var adminHTML string

// Handler manages HTTP handlers for the mock Prometheus server
type Handler struct {
	mockServer   *MockServer
	queryHandler *QueryHandler
}

// NewHandler creates a new Prometheus HTTP handler
func NewHandler(mockServer *MockServer) *Handler {
	return &Handler{
		mockServer:   mockServer,
		queryHandler: NewQueryHandler(mockServer),
	}
}

// HandleQuery handles Prometheus query API requests (both GET and POST)
// This implements /api/v1/query endpoint
func (h *Handler) HandleQuery(c *gin.Context) {
	var query string

	// Support both GET and POST (continuous-verification uses POST)
	if c.Request.Method == http.MethodPost {
		// Form-encoded: query=...
		query = c.PostForm("query")
	} else {
		// URL parameter: ?query=...
		query = c.Query("query")
	}

	if query == "" {
		c.JSON(http.StatusBadRequest, PrometheusResponse{
			Status:    "error",
			ErrorType: "bad_data",
			Error:     "query parameter is required",
		})
		return
	}

	// Execute the query
	response := h.queryHandler.ExecuteQuery(query)
	c.JSON(http.StatusOK, response)
}

// HandleMetrics handles the /metrics endpoint (Prometheus text format)
func (h *Handler) HandleMetrics(c *gin.Context) {
	metrics := h.queryHandler.FormatMetrics()
	c.Data(http.StatusOK, "text/plain; version=0.0.4", []byte(metrics))
}

// HandleGetScenario returns the current scenario status
func (h *Handler) HandleGetScenario(c *gin.Context) {
	status := h.mockServer.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   status,
	})
}

// SetScenarioRequest represents a request to change the scenario
type SetScenarioRequest struct {
	Scenario string `json:"scenario" binding:"required"`
}

// HandleSetScenario changes the current scenario
func (h *Handler) HandleSetScenario(c *gin.Context) {
	var req SetScenarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "invalid request: " + err.Error(),
		})
		return
	}

	// Validate scenario type
	scenarioType := ScenarioType(req.Scenario)
	validScenarios := ValidScenarioTypes()
	isValid := false
	for _, valid := range validScenarios {
		if req.Scenario == valid {
			isValid = true
			break
		}
	}

	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":          "error",
			"error":           "invalid scenario type",
			"valid_scenarios": validScenarios,
		})
		return
	}

	// Set the scenario
	h.mockServer.SetScenario(scenarioType)

	// Return new status
	status := h.mockServer.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   status,
	})
}

// HandleResetTimer resets the progression timer for the current scenario
func (h *Handler) HandleResetTimer(c *gin.Context) {
	h.mockServer.ResetTimer()

	status := h.mockServer.GetStatus()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "timer reset",
		"data":    status,
	})
}

// HandleAdmin serves the admin control panel HTML
func (h *Handler) HandleAdmin(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(adminHTML))
}

// HandleListScenarios returns all available scenarios
func (h *Handler) HandleListScenarios(c *gin.Context) {
	scenarios := AllScenarios()

	// Convert to a list for easier frontend consumption
	scenarioList := make([]map[string]interface{}, 0, len(scenarios))
	for _, scenario := range scenarios {
		scenarioList = append(scenarioList, map[string]interface{}{
			"type":        scenario.Type,
			"description": scenario.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   scenarioList,
	})
}
