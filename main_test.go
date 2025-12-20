package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func TestSlackHandlerApplicationJSON(t *testing.T) {
	// Setup
	eventConfigs = []EventConfig{
		{EventType: "message", Channel: "test-channel"},
	}
	eventChannelMap = make(map[string]string)
	for _, config := range eventConfigs {
		eventChannelMap[config.EventType] = config.Channel
	}
	signingSecret = []byte{} // Disable signature verification for tests

	// Create test payload
	payload := map[string]interface{}{
		"type": "event_callback",
		"event": map[string]interface{}{
			"type": "message",
			"text": "Hello world",
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=test")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	slackHandler(rr, req)

	// Check response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestSlackHandlerURLEncoded(t *testing.T) {
	// Setup
	eventConfigs = []EventConfig{
		{EventType: "message", Channel: "test-channel"},
	}
	eventChannelMap = make(map[string]string)
	for _, config := range eventConfigs {
		eventChannelMap[config.EventType] = config.Channel
	}
	signingSecret = []byte{} // Disable signature verification for tests

	// Create test payload
	payload := map[string]interface{}{
		"type": "event_callback",
		"event": map[string]interface{}{
			"type": "message",
			"text": "Hello world",
		},
	}
	payloadBytes, _ := json.Marshal(payload)

	// Create URL-encoded form data
	formData := url.Values{}
	formData.Set("payload", string(payloadBytes))
	encodedPayload := formData.Encode()

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader([]byte(encodedPayload)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=test")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	slackHandler(rr, req)

	// Check response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestSlackHandlerURLVerification(t *testing.T) {
	// Setup
	signingSecret = []byte{} // Disable signature verification for tests

	// Create test payload
	payload := map[string]interface{}{
		"type":      "url_verification",
		"challenge": "test_challenge",
	}
	payloadBytes, _ := json.Marshal(payload)

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=test")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	slackHandler(rr, req)

	// Check response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check challenge response
	var response map[string]string
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Errorf("failed to decode response: %v", err)
	}
	if response["challenge"] != "test_challenge" {
		t.Errorf("handler returned wrong challenge: got %v want %v", response["challenge"], "test_challenge")
	}
}

func TestSlackHandlerURLVerificationURLEncoded(t *testing.T) {
	// Setup
	signingSecret = []byte{} // Disable signature verification for tests

	// Create test payload
	payload := map[string]interface{}{
		"type":      "url_verification",
		"challenge": "test_challenge_urlencoded",
	}
	payloadBytes, _ := json.Marshal(payload)

	// Create URL-encoded form data
	formData := url.Values{}
	formData.Set("payload", string(payloadBytes))
	encodedPayload := formData.Encode()

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader([]byte(encodedPayload)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=test")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	slackHandler(rr, req)

	// Check response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check challenge response
	var response map[string]string
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Errorf("failed to decode response: %v", err)
	}
	if response["challenge"] != "test_challenge_urlencoded" {
		t.Errorf("handler returned wrong challenge: got %v want %v", response["challenge"], "test_challenge_urlencoded")
	}
}

func TestSlackHandlerMissingPayloadParameter(t *testing.T) {
	// Setup
	signingSecret = []byte{} // Disable signature verification for tests

	// Create URL-encoded form data without payload parameter
	formData := url.Values{}
	formData.Set("other", "value")
	encodedPayload := formData.Encode()

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader([]byte(encodedPayload)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=test")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	slackHandler(rr, req)

	// Check response - should be 400 Bad Request
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestMain(m *testing.M) {
	// Setup test environment
	currentLogLevel = ERROR // Reduce logging noise during tests
	os.Exit(m.Run())
}
