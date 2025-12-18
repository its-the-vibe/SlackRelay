package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// EventConfig represents the configuration for a Slack event type
type EventConfig struct {
	EventType string `json:"slack-event-type"`
	Channel   string `json:"channel"`
}

var signingSecret []byte
var redisClient *redis.Client
var currentLogLevel LogLevel = INFO
var eventConfigs []EventConfig
var eventChannelMap map[string]string

// parseLogLevel converts a string to LogLevel
func parseLogLevel(level string) LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

// logDebug logs a message at DEBUG level
func logDebug(format string, v ...interface{}) {
	if currentLogLevel <= DEBUG {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// logInfo logs a message at INFO level
func logInfo(format string, v ...interface{}) {
	if currentLogLevel <= INFO {
		log.Printf("[INFO] "+format, v...)
	}
}

// logWarn logs a message at WARN level
func logWarn(format string, v ...interface{}) {
	if currentLogLevel <= WARN {
		log.Printf("[WARN] "+format, v...)
	}
}

// logError logs a message at ERROR level
func logError(format string, v ...interface{}) {
	if currentLogLevel <= ERROR {
		log.Printf("[ERROR] "+format, v...)
	}
}

// loadEventConfig loads the event configuration from a JSON file
func loadEventConfig(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &eventConfigs)
	if err != nil {
		return err
	}

	// Build a map for quick lookup
	eventChannelMap = make(map[string]string)
	for _, config := range eventConfigs {
		eventChannelMap[config.EventType] = config.Channel
	}

	return nil
}

func verifySlackSignature(body []byte, timestamp string, signature string) bool {
	if len(signingSecret) == 0 {
		// No secret configured, skip verification
		return true
	}

	if signature == "" || timestamp == "" {
		return false
	}

	// Check timestamp to prevent replay attacks (should be within 5 minutes)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	now := time.Now().Unix()
	if abs(now-ts) > 300 { // 5 minutes
		logWarn("Request timestamp too old or too far in the future")
		return false
	}

	// Slack sends signature as "v0=<hash>"
	if !strings.HasPrefix(signature, "v0=") {
		return false
	}

	signatureHash := strings.TrimPrefix(signature, "v0=")

	// Compute expected signature: v0:<timestamp>:<body>
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, signingSecret)
	mac.Write([]byte(baseString))
	expectedMAC := mac.Sum(nil)
	expectedSignature := hex.EncodeToString(expectedMAC)

	return hmac.Equal([]byte(signatureHash), []byte(expectedSignature))
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func slackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// Verify Slack request signature
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")
	if !verifySlackSignature(body, timestamp, signature) {
		logWarn("Invalid Slack signature")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse the JSON payload
	var payload map[string]interface{}
	err = json.Unmarshal(body, &payload)
	if err != nil {
		http.Error(w, "Error parsing JSON", http.StatusBadRequest)
		return
	}

	// Handle URL verification challenge
	if payload["type"] == "url_verification" {
		challenge, ok := payload["challenge"].(string)
		if !ok {
			http.Error(w, "Invalid challenge", http.StatusBadRequest)
			return
		}
		logInfo("Responding to URL verification challenge")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(fmt.Sprintf(`{"challenge":"%s"}`, challenge))); err != nil {
			logError("Error writing response: %v", err)
		}
		return
	}

	// Get the Slack event type
	var eventType string
	if payload["type"] == "event_callback" {
		// Extract event type from nested event object
		if event, ok := payload["event"].(map[string]interface{}); ok {
			if et, ok := event["type"].(string); ok {
				eventType = et
			}
		}
	} else {
		// For other types, use the top-level type
		if et, ok := payload["type"].(string); ok {
			eventType = et
		}
	}

	if eventType == "" {
		logWarn("Could not determine event type from payload")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Event received but type unknown")); err != nil {
			logError("Error writing response: %v", err)
		}
		return
	}

	logInfo("Received Slack event: %s", eventType)

	// Check if event is configured
	channel, ok := eventChannelMap[eventType]
	if !ok {
		logInfo("Event type '%s' not configured, ignoring", eventType)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Event received but event type not configured")); err != nil {
			logError("Error writing response: %v", err)
		}
		return
	}

	// Only log payload at DEBUG level
	if currentLogLevel <= DEBUG {
		jsonOutput, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			logError("Error formatting JSON: %v", err)
			fmt.Println(string(body))
		} else {
			logDebug("Slack event payload:\n%s", string(jsonOutput))
		}
	}

	// Publish to Redis if client is configured
	if redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = redisClient.Publish(ctx, channel, body).Err()
		if err != nil {
			logError("Error publishing to Redis channel '%s': %v", channel, err)
			// Don't fail the request if Redis publish fails
		} else {
			logInfo("Published event to Redis channel: %s", channel)
		}
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Event received")); err != nil {
		logError("Error writing response: %v", err)
	}
}

func main() {
	// Set log level from environment variable
	logLevelStr := os.Getenv("LOG_LEVEL")
	if logLevelStr == "" {
		logLevelStr = "INFO"
	}
	currentLogLevel = parseLogLevel(logLevelStr)
	logInfo("Log level set to: %s", strings.ToUpper(logLevelStr))

	// Load event configuration
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	err := loadEventConfig(configFile)
	if err != nil {
		logError("Error loading configuration file '%s': %v", configFile, err)
		logError("Please create a configuration file with event-to-channel mappings")
		os.Exit(1)
	}
	logInfo("Loaded %d event configuration(s) from %s", len(eventConfigs), configFile)

	// Load Slack signing secret from .secret file
	secretData, err := os.ReadFile(".secret")
	if err != nil {
		logWarn(".secret file not found. Slack signature verification will be skipped.")
		logWarn("To enable verification, create a .secret file with your Slack signing secret.")
	} else {
		signingSecret = []byte(strings.TrimSpace(string(secretData)))
		logInfo("Slack signing secret loaded. Signature verification enabled.")
	}

	// Configure Redis connection
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	// Set defaults
	if redisHost == "" {
		redisHost = "localhost"
	}
	if redisPort == "" {
		redisPort = "6379"
	}

	// Initialize Redis client
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test Redis connection
	ctx := context.Background()
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		logWarn("Could not connect to Redis at %s: %v", redisAddr, err)
		logWarn("Redis publishing will be disabled. Service will continue to work without Redis.")
		redisClient = nil
	} else {
		logInfo("Connected to Redis at %s", redisAddr)
	}

	http.HandleFunc("/slack", slackHandler)

	// Get port from environment variable, default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Ensure port has colon prefix
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	logInfo("Starting Slack event server on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
