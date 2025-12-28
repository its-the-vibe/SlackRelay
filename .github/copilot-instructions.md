# Copilot Instructions for SlackRelay

## Project Overview

SlackRelay is a Go web service that consumes Slack Events API requests and publishes events to Redis pub/sub channels. The service acts as a bridge between Slack's event system and Redis-based event consumers.

## Architecture

- **Single-file application**: All code is in `main.go` (except tests in `main_test.go`)
- **Event-driven**: Receives Slack webhook events, verifies signatures, and publishes to Redis
- **Configuration-based routing**: JSON config file maps Slack event types to Redis channels
- **Stateless design**: No database, all state is in Redis pub/sub

## Building and Testing

### Build Commands
```bash
# Download dependencies
go mod download

# Build the application
go build -o slack-relay

# Run the application (requires config.json)
./slack-relay

# Build Docker image
docker build -t slack-relay .
```

### Test Commands
```bash
# Run all tests
go test

# Run tests with verbose output
go test -v

# Run tests with coverage
go test -cover

# Run specific test
go test -run TestSlackHandlerURLVerification
```

### Running Locally
```bash
# Basic run (default: port 8080, INFO log level)
./slack-relay

# With custom configuration
LOG_LEVEL=DEBUG CONFIG_FILE=config.json REDIS_HOST=localhost REDIS_PORT=6379 ./slack-relay

# Using Docker Compose (recommended)
docker-compose up -d
docker-compose logs -f
```

## Code Conventions

### Go Style
- **Use standard Go formatting**: Code is formatted with `gofmt`
- **Explicit error handling**: Always check and handle errors explicitly
- **Prefer standard library**: Use standard library packages when possible
- **No external frameworks**: The project uses only `net/http` and `github.com/redis/go-redis/v9`

### Naming Conventions
- **Functions**: camelCase for private, PascalCase for public
- **Variables**: descriptive names, avoid single letters except in short loops
- **Constants**: PascalCase or UPPER_CASE for readability
- **Types**: PascalCase with clear, descriptive names

### Logging
- **Use custom log levels**: `logDebug()`, `logInfo()`, `logWarn()`, `logError()`
- **DEBUG level only for sensitive data**: Event payloads logged only at DEBUG level
- **INFO level for operations**: Standard operational messages
- **WARN level for non-fatal issues**: Missing configs, Redis connection failures
- **ERROR level for failures**: Configuration errors, critical failures

### Error Handling
- **Return errors, don't panic**: Except in `main()` for fatal startup errors
- **Log before returning errors**: Provide context in logs
- **Graceful degradation**: Service continues without Redis if connection fails
- **HTTP status codes**: Use appropriate codes (200, 400, 401, 405)

## Configuration Files

### config.json (Required)
Event-to-channel mapping configuration:
```json
[
  {
    "slack-event-type": "message",
    "channel": "slack-relay-message"
  },
  {
    "slack-event-type": "view_submission",
    "channel": "slack-relay-view-submission",
    "response": {"response_action": "clear"}
  }
]
```

- `slack-event-type`: The Slack event type to match
- `channel`: The Redis pub/sub channel to publish to
- `response` (optional): JSON response to send back to Slack

### .secret (Optional)
Contains the Slack signing secret for request verification. If missing, signature verification is skipped (with warning).

## Environment Variables

- `PORT`: Server port (default: `8080`)
- `LOG_LEVEL`: Logging verbosity - `DEBUG`, `INFO`, `WARN`, `ERROR` (default: `INFO`)
- `CONFIG_FILE`: Path to config file (default: `config.json`)
- `REDIS_HOST`: Redis hostname (default: `localhost`)
- `REDIS_PORT`: Redis port (default: `6379`)
- `REDIS_PASSWORD`: Redis password (optional, default: empty)

## Security Considerations

### Slack Signature Verification
- **HMAC SHA256 verification**: All requests are verified using Slack's signature format
- **Timestamp validation**: Requests older than 5 minutes are rejected to prevent replay attacks
- **Secret from file**: Signing secret loaded from `.secret` file, never hardcoded
- **Graceful fallback**: If `.secret` is missing, verification is skipped with a warning

### Sensitive Data
- **No logging by default**: Event payloads only logged at DEBUG level
- **Secrets excluded from git**: `.secret` file is in `.gitignore`
- **Read-only containers**: Docker Compose uses `read_only: true`

## Testing Approach

### Test Structure
- **Use httptest**: All handler tests use `httptest.NewRequest` and `httptest.NewRecorder`
- **Setup function**: `setupTestEnvironment()` initializes test config
- **Disable auth in tests**: `signingSecret = []byte{}` to skip signature verification
- **Test both content types**: JSON and URL-encoded form data
- **Minimal logging**: Tests run at ERROR level to reduce noise

### Test Coverage Areas
- Application/JSON content type handling
- URL-encoded form data handling  
- URL verification challenges
- Event routing with optional responses
- Error cases (missing parameters, invalid payloads)

### Adding New Tests
When adding features:
1. Create test setup with relevant event configs
2. Build test payload (use `json.Marshal`)
3. Create `httptest.NewRequest` with headers
4. Call handler with `httptest.NewRecorder`
5. Assert status codes and response bodies

## Common Tasks

### Adding a New Event Type
1. Update `config.json` to add the event type and channel mapping
2. No code changes needed - the system is configuration-driven
3. Restart the service to reload configuration

### Adding New Endpoints
1. Add handler function following `slackHandler` pattern
2. Register with `http.HandleFunc` in `main()`
3. Add tests in `main_test.go`
4. Update README.md API documentation

### Modifying Slack Verification
1. All verification logic is in `verifySlackSignature()`
2. Follows Slack's documented signature format: `v0=<HMAC-SHA256>`
3. Uses constant `slackTimestampToleranceSeconds` for replay protection

### Changing Log Behavior
1. Modify `logDebug()`, `logInfo()`, `logWarn()`, `logError()` functions
2. Consider impact on security (sensitive data exposure)
3. Update `parseLogLevel()` if adding new levels

## Dependencies

### Direct Dependencies
- `github.com/redis/go-redis/v9`: Redis client library
  - Used for pub/sub functionality
  - Connection is optional - service works without Redis

### Standard Library Usage
- `net/http`: HTTP server and client
- `encoding/json`: JSON parsing and generation
- `crypto/hmac` and `crypto/sha256`: Signature verification
- `io`, `os`, `log`: Standard I/O and logging

## Project Structure Notes

- **Inspired by github-webhook**: Similar architecture and patterns
- **Single purpose**: Does one thing well - relay Slack events to Redis
- **Minimal dependencies**: Intentionally keeps dependency footprint small
- **Container-first**: Designed for containerized deployment
- **Event filtering**: Only configured events are processed, others are acknowledged but ignored
