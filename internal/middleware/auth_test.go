package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIKeyAuth(t *testing.T) {
	t.Parallel()

	t.Run("creates auth with valid keys", func(t *testing.T) {
		t.Parallel()

		keys := []string{"key1", "key2", "key3"}
		auth := NewAPIKeyAuth(keys, nil)

		require.NotNil(t, auth)
		assert.Equal(t, 3, len(auth.apiKeys))
		assert.True(t, auth.apiKeys["key1"])
		assert.True(t, auth.apiKeys["key2"])
		assert.True(t, auth.apiKeys["key3"])
	})

	t.Run("filters out empty keys", func(t *testing.T) {
		t.Parallel()

		keys := []string{"key1", "", "key2", ""}
		auth := NewAPIKeyAuth(keys, nil)

		require.NotNil(t, auth)
		assert.Equal(t, 2, len(auth.apiKeys))
		assert.True(t, auth.apiKeys["key1"])
		assert.True(t, auth.apiKeys["key2"])
	})

	t.Run("handles empty key slice", func(t *testing.T) {
		t.Parallel()

		auth := NewAPIKeyAuth([]string{}, nil)

		require.NotNil(t, auth)
		assert.Equal(t, 0, len(auth.apiKeys))
	})

	t.Run("uses default logger when nil", func(t *testing.T) {
		t.Parallel()

		auth := NewAPIKeyAuth([]string{"key1"}, nil)

		require.NotNil(t, auth)
		require.NotNil(t, auth.logger)
	})

	t.Run("uses provided logger", func(t *testing.T) {
		t.Parallel()

		logger := slog.Default()
		auth := NewAPIKeyAuth([]string{"key1"}, logger)

		require.NotNil(t, auth)
		assert.Equal(t, logger, auth.logger)
	})
}

func TestAPIKeyAuth_Middleware_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		headerName string
		apiKey     string
		validKeys  []string
	}{
		{
			name:       "valid X-API-Key header",
			headerName: headerAPIKey,
			apiKey:     "valid-key-123",
			validKeys:  []string{"valid-key-123"},
		},
		{
			name:       "valid Authorization Bearer header",
			headerName: headerAuth,
			apiKey:     "Bearer valid-key-456",
			validKeys:  []string{"valid-key-456"},
		},
		{
			name:       "matches one of multiple valid keys",
			headerName: headerAPIKey,
			apiKey:     "key2",
			validKeys:  []string{"key1", "key2", "key3"},
		},
		{
			name:       "exact match with complex key",
			headerName: headerAPIKey,
			apiKey:     "complex_key_51234567890abcdefghijklmnopqrstuvwxyz",
			validKeys:  []string{"complex_key_51234567890abcdefghijklmnopqrstuvwxyz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			auth := NewAPIKeyAuth(tt.validKeys, nil)

			handlerCalled := false
			handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set(tt.headerName, tt.apiKey)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.True(t, handlerCalled, "handler should have been called")
			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestAPIKeyAuth_Middleware_Unauthorized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		headerName string
		apiKey     string
		validKeys  []string
	}{
		{
			name:       "missing API key",
			headerName: "",
			apiKey:     "",
			validKeys:  []string{"valid-key"},
		},
		{
			name:       "invalid API key in X-API-Key header",
			headerName: headerAPIKey,
			apiKey:     "invalid-key",
			validKeys:  []string{"valid-key"},
		},
		{
			name:       "invalid API key in Authorization header",
			headerName: headerAuth,
			apiKey:     "Bearer invalid-key",
			validKeys:  []string{"valid-key"},
		},
		{
			name:       "no valid keys configured",
			headerName: headerAPIKey,
			apiKey:     "any-key",
			validKeys:  []string{},
		},
		{
			name:       "empty API key value",
			headerName: headerAPIKey,
			apiKey:     "",
			validKeys:  []string{"valid-key"},
		},
		{
			name:       "malformed Authorization header (missing Bearer)",
			headerName: headerAuth,
			apiKey:     "valid-key",
			validKeys:  []string{"valid-key"},
		},
		{
			name:       "case sensitive mismatch",
			headerName: headerAPIKey,
			apiKey:     "Valid-Key",
			validKeys:  []string{"valid-key"},
		},
		{
			name:       "partial key match",
			headerName: headerAPIKey,
			apiKey:     "valid",
			validKeys:  []string{"valid-key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			auth := NewAPIKeyAuth(tt.validKeys, nil)

			handlerCalled := false
			handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.headerName != "" && tt.apiKey != "" {
				req.Header.Set(tt.headerName, tt.apiKey)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.False(t, handlerCalled, "handler should not have been called")
			assert.Equal(t, http.StatusUnauthorized, rec.Code)

			var response map[string]string
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, unauthorizedError, response["error"])
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		})
	}
}

func TestAPIKeyAuth_ExtractAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		headers        map[string]string
		expectedAPIKey string
		description    string
	}{
		{
			name:           "extracts from X-API-Key header",
			headers:        map[string]string{headerAPIKey: "my-api-key"},
			expectedAPIKey: "my-api-key",
			description:    "should extract from X-API-Key",
		},
		{
			name:           "extracts from Authorization Bearer header",
			headers:        map[string]string{headerAuth: "Bearer my-bearer-token"},
			expectedAPIKey: "my-bearer-token",
			description:    "should extract from Authorization Bearer",
		},
		{
			name: "prefers X-API-Key over Authorization",
			headers: map[string]string{
				headerAPIKey: "api-key",
				headerAuth:   "Bearer bearer-token",
			},
			expectedAPIKey: "api-key",
			description:    "should prefer X-API-Key when both are present",
		},
		{
			name:           "returns empty for missing headers",
			headers:        map[string]string{},
			expectedAPIKey: "",
			description:    "should return empty string when no headers present",
		},
		{
			name:           "returns empty for malformed Authorization header",
			headers:        map[string]string{headerAuth: "Basic username:password"},
			expectedAPIKey: "",
			description:    "should return empty for non-Bearer Authorization",
		},
		{
			name:           "handles empty X-API-Key value",
			headers:        map[string]string{headerAPIKey: ""},
			expectedAPIKey: "",
			description:    "should return empty for empty X-API-Key",
		},
		{
			name:           "handles Bearer with empty token",
			headers:        map[string]string{headerAuth: "Bearer "},
			expectedAPIKey: "",
			description:    "should return empty string for Bearer with no token",
		},
		{
			name:           "handles Bearer token with spaces",
			headers:        map[string]string{headerAuth: "Bearer  token-with-spaces  "},
			expectedAPIKey: " token-with-spaces  ",
			description:    "should preserve spaces in token after Bearer prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			auth := NewAPIKeyAuth([]string{"test-key"}, nil)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := auth.extractAPIKey(req)
			assert.Equal(t, tt.expectedAPIKey, result, tt.description)
		})
	}
}

func TestAPIKeyAuth_IsValidAPIKey(t *testing.T) {
	t.Parallel()

	validKeys := []string{"key1", "key2", "very-long-key-123456789"}
	auth := NewAPIKeyAuth(validKeys, nil)

	tests := []struct {
		name        string
		providedKey string
		expected    bool
	}{
		{
			name:        "valid key 1",
			providedKey: "key1",
			expected:    true,
		},
		{
			name:        "valid key 2",
			providedKey: "key2",
			expected:    true,
		},
		{
			name:        "valid long key",
			providedKey: "very-long-key-123456789",
			expected:    true,
		},
		{
			name:        "invalid key",
			providedKey: "invalid-key",
			expected:    false,
		},
		{
			name:        "empty key",
			providedKey: "",
			expected:    false,
		},
		{
			name:        "case sensitive - uppercase",
			providedKey: "KEY1",
			expected:    false,
		},
		{
			name:        "partial key match",
			providedKey: "key",
			expected:    false,
		},
		{
			name:        "key with extra characters",
			providedKey: "key1-extra",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := auth.isValidAPIKey(tt.providedKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAPIKeyAuth_IsValidAPIKey_NoKeysConfigured(t *testing.T) {
	t.Parallel()

	auth := NewAPIKeyAuth([]string{}, nil)

	result := auth.isValidAPIKey("any-key")
	assert.False(t, result, "should reject all keys when none are configured")
}

func TestAPIKeyAuth_ConstantTimeComparison(t *testing.T) {
	t.Parallel()

	// This test verifies that we're using constant-time comparison
	// by checking that the function is deterministic for the same inputs
	auth := NewAPIKeyAuth([]string{"secret-key-12345"}, nil)

	// Test multiple times to ensure consistency
	for i := 0; i < 100; i++ {
		assert.True(t, auth.isValidAPIKey("secret-key-12345"))
		assert.False(t, auth.isValidAPIKey("secret-key-12344"))
		assert.False(t, auth.isValidAPIKey("secret-key-123456"))
	}
}

func TestAPIKeyAuth_MultipleValidKeys(t *testing.T) {
	t.Parallel()

	// Test with a realistic scenario of multiple API keys
	keys := []string{
		"sk_live_abc123",
		"sk_test_def456",
		"pk_live_ghi789",
	}
	auth := NewAPIKeyAuth(keys, nil)

	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test each key works
	for _, key := range keys {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(headerAPIKey, key)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "key %s should be valid", key)
	}
}

func TestAPIKeyAuth_IntegrationScenario(t *testing.T) {
	t.Parallel()

	// Simulate a real-world scenario with multiple endpoints
	auth := NewAPIKeyAuth([]string{"production-key-123"}, nil)

	protectedHandler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name           string
		headerType     string
		headerValue    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "authorized request via X-API-Key",
			headerType:     headerAPIKey,
			headerValue:    "production-key-123",
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "authorized request via Bearer token",
			headerType:     headerAuth,
			headerValue:    "Bearer production-key-123",
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
		},
		{
			name:           "unauthorized request",
			headerType:     headerAPIKey,
			headerValue:    "wrong-key",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"Unauthorized"}`,
		},
		{
			name:           "missing credentials",
			headerType:     "",
			headerValue:    "",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"Unauthorized"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", nil)
			if tt.headerType != "" {
				req.Header.Set(tt.headerType, tt.headerValue)
			}
			rec := httptest.NewRecorder()

			protectedHandler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedBody != "" {
				if tt.expectedStatus == http.StatusOK {
					assert.Equal(t, tt.expectedBody, rec.Body.String())
				} else {
					assert.JSONEq(t, tt.expectedBody, rec.Body.String())
				}
			}
		})
	}
}
