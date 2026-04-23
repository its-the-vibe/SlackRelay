package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func setupTestEnvironment() {
	eventConfigs = []EventConfig{
		{EventType: "message", Channel: "test-channel"},
	}
	eventChannelMap = make(map[string]string)
	eventResponseMap = make(map[string]map[string]interface{})
	for _, config := range eventConfigs {
		eventChannelMap[config.EventType] = config.Channel
		if config.Response != nil {
			eventResponseMap[config.EventType] = config.Response
		}
	}
	signingSecret = []byte{} // Disable signature verification for tests
}

func buildEventMaps() {
	eventChannelMap = make(map[string]string)
	eventResponseMap = make(map[string]map[string]interface{})
	for _, config := range eventConfigs {
		eventChannelMap[config.EventType] = config.Channel
		if config.Response != nil {
			eventResponseMap[config.EventType] = config.Response
		}
	}
}

// computeTestSignature builds a valid Slack HMAC-SHA256 signature for testing.
func computeTestSignature(body []byte, timestamp string, secret []byte) string {
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(baseString))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func TestSlackHandlerApplicationJSON(t *testing.T) {
	setupTestEnvironment()

	// Create test payload
	payload := map[string]interface{}{
		"type": "event_callback",
		"event": map[string]interface{}{
			"type": "message",
			"text": "Hello world",
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

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
	setupTestEnvironment()

	// Create test payload
	payload := map[string]interface{}{
		"type": "event_callback",
		"event": map[string]interface{}{
			"type": "message",
			"text": "Hello world",
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

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
	setupTestEnvironment()

	// Create test payload
	payload := map[string]interface{}{
		"type":      "url_verification",
		"challenge": "test_challenge",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

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
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Errorf("failed to decode response: %v", err)
	}
	if response["challenge"] != "test_challenge" {
		t.Errorf("handler returned wrong challenge: got %v want %v", response["challenge"], "test_challenge")
	}
}

func TestSlackHandlerURLVerificationURLEncoded(t *testing.T) {
	setupTestEnvironment()

	// Create test payload
	payload := map[string]interface{}{
		"type":      "url_verification",
		"challenge": "test_challenge_urlencoded",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

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
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Errorf("failed to decode response: %v", err)
	}
	if response["challenge"] != "test_challenge_urlencoded" {
		t.Errorf("handler returned wrong challenge: got %v want %v", response["challenge"], "test_challenge_urlencoded")
	}
}

func TestSlackHandlerMissingPayloadParameter(t *testing.T) {
	setupTestEnvironment()

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

func TestSlackHandlerWithOptionalResponse(t *testing.T) {
	// Setup test environment with a response configured
	eventConfigs = []EventConfig{
		{
			EventType: "view_submission",
			Channel:   "test-channel",
			Response:  map[string]interface{}{"response_action": "clear"},
		},
	}
	buildEventMaps()
	signingSecret = []byte{} // Disable signature verification for tests

	// Create test payload
	payload := map[string]interface{}{
		"type": "view_submission",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=test")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	slackHandler(rr, req)

	// Check response status
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check response body
	var response map[string]interface{}
	err = json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Errorf("failed to decode response: %v", err)
	}
	if response["response_action"] != "clear" {
		t.Errorf("handler returned wrong response: got %v want %v", response["response_action"], "clear")
	}
}

func TestSlackHandlerWithoutOptionalResponse(t *testing.T) {
	// Setup test environment without a response configured
	eventConfigs = []EventConfig{
		{
			EventType: "message",
			Channel:   "test-channel",
		},
	}
	buildEventMaps()
	signingSecret = []byte{} // Disable signature verification for tests

	// Create test payload
	payload := map[string]interface{}{
		"type": "event_callback",
		"event": map[string]interface{}{
			"type": "message",
			"text": "Hello world",
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=test")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	slackHandler(rr, req)

	// Check response status
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check response body
	body := rr.Body.String()
	if body != "Event received" {
		t.Errorf("handler returned wrong response: got %v want %v", body, "Event received")
	}
}

func TestSlackHandlerMethodNotAllowed(t *testing.T) {
	setupTestEnvironment()

	req := httptest.NewRequest(http.MethodGet, "/slack", nil)
	rr := httptest.NewRecorder()

	slackHandler(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusMethodNotAllowed)
	}
}

func TestSlackHandlerInvalidSignature(t *testing.T) {
	setupTestEnvironment()
	signingSecret = []byte("test-secret") // Enable signature verification

	payload := map[string]interface{}{"type": "event_callback"}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", "1234567890")
	req.Header.Set("X-Slack-Signature", "v0=invalidsignature")

	rr := httptest.NewRecorder()
	slackHandler(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}
}

func TestSlackHandlerUnconfiguredEventType(t *testing.T) {
	setupTestEnvironment()

	// Send an event type that is not in the config
	payload := map[string]interface{}{
		"type": "event_callback",
		"event": map[string]interface{}{
			"type": "app_mention",
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	slackHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	body := rr.Body.String()
	if body != "Event received but event type not configured" {
		t.Errorf("handler returned wrong body: got %v", body)
	}
}

func TestSlackHandlerUnknownEventType(t *testing.T) {
	setupTestEnvironment()

	// Payload with no type field
	payload := map[string]interface{}{
		"text": "hello",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	slackHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	body := rr.Body.String()
	if body != "Event received but type unknown" {
		t.Errorf("handler returned wrong body: got %v", body)
	}
}

func TestSlackHandlerInvalidJSON(t *testing.T) {
	setupTestEnvironment()

	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader([]byte("not-json{")))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	slackHandler(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestSlackHandlerInvalidJSONInFormPayload(t *testing.T) {
	setupTestEnvironment()

	formData := url.Values{}
	formData.Set("payload", "not-json{")
	encodedPayload := formData.Encode()

	req := httptest.NewRequest(http.MethodPost, "/slack", bytes.NewReader([]byte(encodedPayload)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	slackHandler(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", DEBUG},
		{"debug", DEBUG},
		{"INFO", INFO},
		{"info", INFO},
		{"WARN", WARN},
		{"warn", WARN},
		{"ERROR", ERROR},
		{"error", ERROR},
		{"unknown", INFO},
		{"", INFO},
	}

	for _, tt := range tests {
		got := parseLogLevel(tt.input)
		if got != tt.expected {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestLoadEventConfig(t *testing.T) {
	configContent := `[{"slack-event-type":"message","channel":"test-channel"},{"slack-event-type":"view_submission","channel":"test-view-channel","response":{"response_action":"clear"}}]`
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	if err := loadEventConfig(tmpFile.Name()); err != nil {
		t.Fatalf("loadEventConfig returned error: %v", err)
	}

	if eventChannelMap["message"] != "test-channel" {
		t.Errorf("expected channel 'test-channel' for 'message', got %v", eventChannelMap["message"])
	}
	if eventChannelMap["view_submission"] != "test-view-channel" {
		t.Errorf("expected channel 'test-view-channel' for 'view_submission', got %v", eventChannelMap["view_submission"])
	}
	if eventResponseMap["view_submission"]["response_action"] != "clear" {
		t.Errorf("expected response_action 'clear', got %v", eventResponseMap["view_submission"]["response_action"])
	}
}

func TestLoadEventConfigFileNotFound(t *testing.T) {
	err := loadEventConfig("/tmp/nonexistent-config-file.json")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLoadEventConfigInvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("not-valid-json"); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	err = loadEventConfig(tmpFile.Name())
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestVerifySlackSignature(t *testing.T) {
	secret := []byte("test-signing-secret")
	body := []byte(`{"type":"event_callback"}`)

	t.Run("valid signature", func(t *testing.T) {
		signingSecret = secret
		ts := fmt.Sprintf("%d", time.Now().Unix())
		sig := computeTestSignature(body, ts, secret)
		if !verifySlackSignature(body, ts, sig) {
			t.Error("expected valid signature to pass verification")
		}
	})

	t.Run("empty secret skips verification", func(t *testing.T) {
		signingSecret = []byte{}
		if !verifySlackSignature(body, "any", "any") {
			t.Error("expected verification to be skipped with empty secret")
		}
	})

	t.Run("missing timestamp", func(t *testing.T) {
		signingSecret = secret
		if verifySlackSignature(body, "", "v0=something") {
			t.Error("expected false for missing timestamp")
		}
	})

	t.Run("missing signature", func(t *testing.T) {
		signingSecret = secret
		if verifySlackSignature(body, "1234567890", "") {
			t.Error("expected false for missing signature")
		}
	})

	t.Run("old timestamp", func(t *testing.T) {
		signingSecret = secret
		if verifySlackSignature(body, "1000000000", "v0=something") {
			t.Error("expected false for old timestamp")
		}
	})

	t.Run("signature without v0 prefix", func(t *testing.T) {
		signingSecret = secret
		ts := fmt.Sprintf("%d", time.Now().Unix())
		if verifySlackSignature(body, ts, "noprefixsig") {
			t.Error("expected false for signature without v0= prefix")
		}
	})

	t.Run("wrong signature", func(t *testing.T) {
		signingSecret = secret
		ts := fmt.Sprintf("%d", time.Now().Unix())
		if verifySlackSignature(body, ts, "v0=wrongsignature") {
			t.Error("expected false for wrong signature")
		}
	})
}

func TestAbsInt64(t *testing.T) {
	tests := []struct {
		input    int64
		expected int64
	}{
		{5, 5},
		{-5, 5},
		{0, 0},
		{-100, 100},
	}
	for _, tt := range tests {
		got := absInt64(tt.input)
		if got != tt.expected {
			t.Errorf("absInt64(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestMain(m *testing.M) {
	// Setup test environment
	currentLogLevel = ERROR // Reduce logging noise during tests
	os.Exit(m.Run())
}
