package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

var (
	// ErrSubscriptionFailed is returned when the PubSubHubbub hub rejects the subscription.
	ErrSubscriptionFailed = errors.New("subscription request failed")

	// ErrInvalidHubResponse is returned when the hub returns an unexpected response.
	ErrInvalidHubResponse = errors.New("invalid hub response")
)

// HTTPClient defines the interface for making HTTP requests.
// This allows for easy mocking in tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// PubSubHub defines the interface for interacting with PubSubHubbub hubs.
type PubSubHub interface {
	Subscribe(ctx context.Context, req *SubscribeRequest) (*SubscribeResponse, error)
	Unsubscribe(ctx context.Context, req *SubscribeRequest) (*SubscribeResponse, error)
}

// PubSubHubService handles interactions with the PubSubHubbub hub.
type PubSubHubService struct {
	client HTTPClient
	logger *slog.Logger
}

// NewPubSubHubService creates a new PubSubHubService.
func NewPubSubHubService(client HTTPClient, logger *slog.Logger) *PubSubHubService {
	if client == nil {
		client = &http.Client{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PubSubHubService{
		client: client,
		logger: logger,
	}
}

// SubscribeRequest contains the parameters for subscribing to a topic.
type SubscribeRequest struct {
	HubURL       string
	TopicURL     string
	CallbackURL  string
	LeaseSeconds int
	Secret       *string
}

// SubscribeResponse contains the response from the hub.
type SubscribeResponse struct {
	Accepted     bool
	StatusCode   int
	ResponseBody string
	LeaseSeconds int
}

// Subscribe sends a subscription request to the PubSubHubbub hub.
// The hub should respond with 202 Accepted if the request is valid.
func (s *PubSubHubService) Subscribe(ctx context.Context, req *SubscribeRequest) (*SubscribeResponse, error) {
	if err := s.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Build form data
	formData := url.Values{}
	formData.Set("hub.mode", "subscribe")
	formData.Set("hub.topic", req.TopicURL)
	formData.Set("hub.callback", req.CallbackURL)
	formData.Set("hub.lease_seconds", fmt.Sprintf("%d", req.LeaseSeconds))

	if req.Secret != nil && *req.Secret != "" {
		formData.Set("hub.secret", *req.Secret)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		req.HubURL,
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s.logger.Info("sending subscription request to hub",
		"hub_url", req.HubURL,
		"topic_url", req.TopicURL,
		"callback_url", req.CallbackURL,
		"lease_seconds", req.LeaseSeconds,
	)

	// Send request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	response := &SubscribeResponse{
		StatusCode:   resp.StatusCode,
		ResponseBody: string(body),
		LeaseSeconds: req.LeaseSeconds,
	}

	// Handle response codes
	switch resp.StatusCode {
	case http.StatusAccepted:
		// 202 Accepted - subscription request accepted
		response.Accepted = true
		s.logger.Info("subscription request accepted by hub",
			"status_code", resp.StatusCode,
		)
	case http.StatusNoContent:
		// 204 No Content - also considered success
		response.Accepted = true
		s.logger.Info("subscription request accepted by hub",
			"status_code", resp.StatusCode,
		)
	case http.StatusBadRequest:
		// 400 Bad Request - invalid parameters
		s.logger.Warn("subscription request rejected - bad request",
			"status_code", resp.StatusCode,
			"response_body", string(body),
		)
		return response, fmt.Errorf("%w: bad request - %s", ErrSubscriptionFailed, string(body))
	case http.StatusNotFound:
		// 404 Not Found - topic or hub not found
		s.logger.Warn("subscription request rejected - not found",
			"status_code", resp.StatusCode,
			"response_body", string(body),
		)
		return response, fmt.Errorf("%w: not found - %s", ErrSubscriptionFailed, string(body))
	default:
		// Unexpected response
		s.logger.Error("unexpected response from hub",
			"status_code", resp.StatusCode,
			"response_body", string(body),
		)
		return response, fmt.Errorf("%w: unexpected status code %d - %s", ErrInvalidHubResponse, resp.StatusCode, string(body))
	}

	return response, nil
}

// Unsubscribe sends an unsubscription request to the PubSubHubbub hub.
func (s *PubSubHubService) Unsubscribe(ctx context.Context, req *SubscribeRequest) (*SubscribeResponse, error) {
	if err := s.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Build form data
	formData := url.Values{}
	formData.Set("hub.mode", "unsubscribe")
	formData.Set("hub.topic", req.TopicURL)
	formData.Set("hub.callback", req.CallbackURL)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		req.HubURL,
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s.logger.Info("sending unsubscription request to hub",
		"hub_url", req.HubURL,
		"topic_url", req.TopicURL,
		"callback_url", req.CallbackURL,
	)

	// Send request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	response := &SubscribeResponse{
		StatusCode:   resp.StatusCode,
		ResponseBody: string(body),
	}

	// Handle response codes
	switch resp.StatusCode {
	case http.StatusAccepted, http.StatusNoContent:
		response.Accepted = true
		s.logger.Info("unsubscription request accepted by hub",
			"status_code", resp.StatusCode,
		)
	default:
		s.logger.Warn("unsubscription request failed",
			"status_code", resp.StatusCode,
			"response_body", string(body),
		)
		return response, fmt.Errorf("%w: status code %d - %s", ErrSubscriptionFailed, resp.StatusCode, string(body))
	}

	return response, nil
}

// validateRequest validates the subscription request parameters.
func (s *PubSubHubService) validateRequest(req *SubscribeRequest) error {
	if req == nil {
		return errors.New("request is nil")
	}
	if req.HubURL == "" {
		return errors.New("hub URL is required")
	}
	if req.TopicURL == "" {
		return errors.New("topic URL is required")
	}
	if req.CallbackURL == "" {
		return errors.New("callback URL is required")
	}
	if req.LeaseSeconds < 0 {
		return errors.New("lease seconds must be non-negative")
	}

	// Validate URLs
	if _, err := url.Parse(req.HubURL); err != nil {
		return fmt.Errorf("invalid hub URL: %w", err)
	}
	if _, err := url.Parse(req.TopicURL); err != nil {
		return fmt.Errorf("invalid topic URL: %w", err)
	}
	if _, err := url.Parse(req.CallbackURL); err != nil {
		return fmt.Errorf("invalid callback URL: %w", err)
	}

	return nil
}
