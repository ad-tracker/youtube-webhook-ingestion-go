package middleware

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

const (
	headerAPIKey      = "X-API-Key"
	headerAuth        = "Authorization"
	bearerPrefix      = "Bearer "
	unauthorizedError = "Unauthorized"
)

// APIKeyAuth provides API key authentication middleware.
type APIKeyAuth struct {
	apiKeys map[string]bool
	logger  *slog.Logger
}

// NewAPIKeyAuth creates a new API key authentication middleware.
// The apiKeys parameter should be a slice of valid API keys.
// If no keys are provided, all requests will be rejected.
func NewAPIKeyAuth(apiKeys []string, logger *slog.Logger) *APIKeyAuth {
	if logger == nil {
		logger = slog.Default()
	}

	// Build a map for O(1) lookup
	keyMap := make(map[string]bool, len(apiKeys))
	for _, key := range apiKeys {
		if key != "" {
			keyMap[key] = true
		}
	}

	return &APIKeyAuth{
		apiKeys: keyMap,
		logger:  logger,
	}
}

// Middleware returns an HTTP middleware that validates API keys.
// It checks for API keys in the following order:
// 1. X-API-Key header
// 2. Authorization: Bearer <key> header
//
// If no valid API key is found, it returns 401 Unauthorized.
func (a *APIKeyAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from request
		apiKey := a.extractAPIKey(r)

		// Validate API key
		if !a.isValidAPIKey(apiKey) {
			a.logger.Warn("unauthorized request - invalid or missing API key",
				"path", r.URL.Path,
				"method", r.Method,
				"remote_addr", r.RemoteAddr,
			)
			a.sendUnauthorized(w)
			return
		}

		// API key is valid, continue to next handler
		next.ServeHTTP(w, r)
	})
}

// extractAPIKey extracts the API key from the request headers.
// It checks X-API-Key header first, then Authorization: Bearer header.
func (a *APIKeyAuth) extractAPIKey(r *http.Request) string {
	// Try X-API-Key header first
	if apiKey := r.Header.Get(headerAPIKey); apiKey != "" {
		return apiKey
	}

	// Try Authorization: Bearer header
	authHeader := r.Header.Get(headerAuth)
	if strings.HasPrefix(authHeader, bearerPrefix) {
		return strings.TrimPrefix(authHeader, bearerPrefix)
	}

	return ""
}

// isValidAPIKey validates the provided API key using constant-time comparison
// to prevent timing attacks.
func (a *APIKeyAuth) isValidAPIKey(providedKey string) bool {
	if providedKey == "" {
		return false
	}

	// If no API keys are configured, reject all requests
	if len(a.apiKeys) == 0 {
		return false
	}

	// Check if the provided key matches any valid key using constant-time comparison
	for validKey := range a.apiKeys {
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(validKey)) == 1 {
			return true
		}
	}

	return false
}

// sendUnauthorized sends a 401 Unauthorized response.
func (a *APIKeyAuth) sendUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	response := map[string]string{
		"error": unauthorizedError,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		a.logger.Error("failed to encode unauthorized response", "error", err)
	}
}
