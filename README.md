# SlackRelay

A simple Go web service which consumes Slack API events and publishes to Redis pub/sub

## Features

- Receives and parses Slack Events API requests
- Verifies Slack request signatures using HMAC SHA256
- Handles URL verification challenges automatically
- Event filtering with configuration file support
- Publishes event payloads to event-specific Redis pub/sub channels
- Configurable log levels (DEBUG, INFO, WARN, ERROR)
- Configurable port via environment variable
- Configurable Redis connection via environment variables
- Docker and Docker Compose support for easy deployment

## Configuration

### Event Configuration

The service uses a JSON configuration file to map Slack event types to Redis pub/sub channels. Events not defined in the configuration file are ignored.

**Configuration File Format:**

Create a `config.json` file (or specify a custom path via `CONFIG_FILE` environment variable):

```json
[
  {
    "slack-event-type": "message",
    "channel": "slack-relay-message"
  },
  {
    "slack-event-type": "app_mention",
    "channel": "slack-relay-app-mention"
  }
]
```

**Environment Variables:**

- `CONFIG_FILE`: Path to the configuration file (default: `config.json`)

The server will examine the event type from incoming Slack Events API requests and publish to the corresponding Redis channel. If an event type is not configured, the event will be acknowledged but not processed.

**Example:**

```bash
# Use default config.json
./slack-relay

# Use custom configuration file
CONFIG_FILE=/path/to/my-config.json ./slack-relay
```

### Log Level Configuration

Control the verbosity of logging with the `LOG_LEVEL` environment variable.

**Available Log Levels:**

- `DEBUG`: Most verbose, includes event payloads
- `INFO`: Standard operational messages (default)
- `WARN`: Warning messages only
- `ERROR`: Error messages only

**Environment Variables:**

- `LOG_LEVEL`: Sets the logging level (default: `INFO`)

**Note:** Event payloads are only logged when `LOG_LEVEL` is set to `DEBUG`. This prevents sensitive data from appearing in logs during normal operation.

**Example:**

```bash
# Use INFO level (default)
./slack-relay

# Use DEBUG level to see event payloads
LOG_LEVEL=DEBUG ./slack-relay

# Use WARN level for minimal logging
LOG_LEVEL=WARN ./slack-relay
```

### Port Configuration

The server port can be configured via the `PORT` environment variable. If not set, it defaults to `8080`.

```bash
# Run on default port 8080
./slack-relay

# Run on custom port
PORT=3000 ./slack-relay
```

### Redis Configuration

The service publishes received events to Redis pub/sub channels based on the event configuration. Each event type is routed to its configured channel.

**Environment Variables:**

- `REDIS_HOST`: Redis server hostname (default: `localhost`)
- `REDIS_PORT`: Redis server port (default: `6379`)
- `REDIS_PASSWORD`: (Optional) Redis server password for authentication (default: unset)

**Note:** If the Redis connection fails, the application will log a warning and continue to work without Redis publishing. This ensures the service remains operational even if Redis is unavailable.

```bash
# Run with Redis configuration (with optional password)
REDIS_HOST=redis.example.com REDIS_PORT=6379 REDIS_PASSWORD=yourpassword ./slack-relay

# Run with default Redis settings (connects to localhost:6379, no password)
./slack-relay
```

### Slack Signing Secret

To enable Slack request signature verification:

1. Create a `.secret` file in the application directory
2. Add your Slack app's signing secret to this file (found in your Slack app's Basic Information page)
3. The application will automatically load this secret on startup

**Note:** If the `.secret` file is not found, the application will start but signature verification will be skipped (with a warning logged).

#### Setting up Slack Events API

1. Create a Slack app at https://api.slack.com/apps
2. Navigate to "Event Subscriptions" in your app settings
3. Enable Events and set the Request URL to your server's `/slack` endpoint
4. Slack will send a URL verification challenge - the service handles this automatically
5. Subscribe to the events you want to receive (e.g., `message.channels`, `app_mention`)
6. Get your signing secret from "Basic Information" → "App Credentials" → "Signing Secret"

Example `.secret` file:
```
your-signing-secret-here
```

**Security:** The `.secret` file is excluded from version control via `.gitignore`.

## Building and Running

### Local Development

```bash
# Initialize Go modules (first time only)
go mod download

# Build the application
go build -o slack-relay

# Run the server (requires config.json)
./slack-relay

# Run with custom configuration file
CONFIG_FILE=my-config.json ./slack-relay

# Run with custom port
PORT=3000 ./slack-relay

# Run with DEBUG logging
LOG_LEVEL=DEBUG ./slack-relay

# Run with Redis configuration
REDIS_HOST=redis.example.com REDIS_PORT=6379 ./slack-relay

# Run with all options
LOG_LEVEL=DEBUG CONFIG_FILE=config.json REDIS_HOST=localhost PORT=8080 ./slack-relay
```

### Using Docker

```bash
# Build the Docker image
docker build -t slack-relay .

# Run the container (mount config.json)
docker run -p 8080:8080 -v $(pwd)/config.json:/app/config.json:ro slack-relay

# Run with custom log level
docker run -p 8080:8080 -e LOG_LEVEL=DEBUG -v $(pwd)/config.json:/app/config.json:ro slack-relay

# Run with custom port
docker run -p 3000:8080 -e PORT=8080 -v $(pwd)/config.json:/app/config.json:ro slack-relay

# Run with secret file
docker run -p 8080:8080 -v $(pwd)/.secret:/app/.secret:ro -v $(pwd)/config.json:/app/config.json:ro slack-relay

# Run with Redis configuration (connecting to Redis on host machine)
docker run -p 8080:8080 -e REDIS_HOST=host.docker.internal -e REDIS_PORT=6379 -v $(pwd)/config.json:/app/config.json:ro slack-relay
```

### Using Docker Compose

The easiest way to run the application:

```bash
# Start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

To use custom configuration with Docker Compose, you can set environment variables:

```bash
# Custom port
PORT=3000 docker-compose up -d

# Custom log level
LOG_LEVEL=DEBUG docker-compose up -d

# Custom configuration file
CONFIG_FILE=/path/to/config.json docker-compose up -d

# Redis configuration
REDIS_HOST=192.168.1.100 REDIS_PORT=6379 docker-compose up -d
```

The docker-compose configuration automatically mounts the `.secret` and `config.json` files if they exist.

## API Endpoints

### POST /slack

Accepts Slack Events API requests and publishes event payloads to Redis based on event type.

**Headers:**
- `X-Slack-Request-Timestamp`: Unix timestamp when the request was sent - **required**
- `X-Slack-Signature`: HMAC signature for request verification (verified if signing secret is configured)

**Request Body:**

The service handles two types of Slack requests:

1. **URL Verification** (during initial setup):
```json
{
  "type": "url_verification",
  "challenge": "3eZbrw1aBm2rZgRNFdxV2595E9CY3gmdALWMmHkvFXO7tYXAYM8P",
  "token": "Jhj5dZrVaK7ZwHHjRyZWjbDl"
}
```
Response:
```json
{
  "challenge": "3eZbrw1aBm2rZgRNFdxV2595E9CY3gmdALWMmHkvFXO7tYXAYM8P"
}
```

2. **Event Callback** (normal operation):
```json
{
  "type": "event_callback",
  "team_id": "T1H9RESGL",
  "event": {
    "type": "message",
    "channel": "C2147483705",
    "user": "U2147483697",
    "text": "Hello world",
    "ts": "1355517523.000005"
  }
}
```

**Response:**
- `200 OK`: Event received and processed successfully
- `200 OK` (with message): Event received but event type not configured (event ignored)
- `401 Unauthorized`: Invalid request signature
- `405 Method Not Allowed`: Non-POST request
- `400 Bad Request`: Invalid JSON or request body error

## Testing

### Manual Testing with curl

```bash
# Test URL verification (initial Slack setup)
curl -X POST http://localhost:8080/slack \
  -H "Content-Type: application/json" \
  -d '{"type":"url_verification","challenge":"test_challenge","token":"test_token"}'

# Test with a configured event type (message)
curl -X POST http://localhost:8080/slack \
  -H "Content-Type: application/json" \
  -H "X-Slack-Request-Timestamp: $(date +%s)" \
  -H "X-Slack-Signature: v0=test" \
  -d '{"type":"event_callback","event":{"type":"message","text":"test"}}'

# Test with an unconfigured event type (will be ignored)
curl -X POST http://localhost:8080/slack \
  -H "Content-Type: application/json" \
  -H "X-Slack-Request-Timestamp: $(date +%s)" \
  -H "X-Slack-Signature: v0=test" \
  -d '{"type":"event_callback","event":{"type":"user_change"}}'
```

**Note:** Without a `.secret` file, signature verification is skipped for testing purposes.

### Testing Redis Integration

If you have Redis running locally, you can subscribe to channels and see events being published:

```bash
# Subscribe to the message event channel
redis-cli
127.0.0.1:6379> SUBSCRIBE slack-relay-message

# In another terminal, send a test event to the service
curl -X POST http://localhost:8080/slack \
  -H "Content-Type: application/json" \
  -H "X-Slack-Request-Timestamp: $(date +%s)" \
  -d '{"type":"event_callback","event":{"type":"message","text":"Hello Redis!"}}'
```

## Development

The project follows standard Go conventions:
- Use `gofmt` for code formatting
- Explicit error handling
- Standard library packages preferred

## Architecture

This service is conceptually similar to [github-webhook](https://github.com/its-the-vibe/github-webhook) and follows the same project structure and layout, adapted for Slack Events API instead of GitHub webhooks.

**Key Differences from GitHub Webhooks:**
- Uses Slack's signature verification format (`v0=<hash>` with timestamp)
- Handles URL verification challenges required by Slack
- Extracts event types from nested `event` objects in event callbacks
- Uses `/slack` endpoint instead of `/webhook`

## License

MIT
