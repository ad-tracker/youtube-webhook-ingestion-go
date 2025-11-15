package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewHealthHandler(t *testing.T) {
	handler := NewHealthHandler(nil, nil)

	if handler == nil {
		t.Fatal("NewHealthHandler() returned nil")
	}
}

func TestHealthHandler_LivenessProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHealthHandler(nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/health/live", nil)

	handler.LivenessProbe(c)

	if w.Code != http.StatusOK {
		t.Errorf("LivenessProbe() status = %d, want %d", w.Code, http.StatusOK)
	}

	// Check response contains status
	body := w.Body.String()
	if body == "" {
		t.Error("LivenessProbe() returned empty body")
	}
}
