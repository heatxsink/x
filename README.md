# x

A collection of Go utility packages and experimental modules for various applications.

## Description

This is my `x` repository - a curated collection of reusable Go packages that provide common functionality for web applications, IoT devices, cloud services, and system administration tasks. The repository is organized into stable production-ready modules and experimental work-in-progress features.

## Installation

```bash
go get github.com/heatxsink/x
```

## Architecture

The repository is organized into two main categories:

### Stable Modules (Production Ready)
These modules are well-tested, stable, and ready for production use.

### Experimental Modules (`exp/`)
These modules are work-in-progress and should be considered experimental. Use with caution in production environments.

## Stable Modules

### `gcs/` - Google Cloud Storage
Utilities for interacting with Google Cloud Storage buckets and objects.

### `gravatar/` - Gravatar Integration
Client for generating Gravatar URLs and fetching user avatars based on email addresses.

### `shell/` - Shell Command Execution
Safe utilities for executing shell commands with proper error handling and output capture.

### `ssh/` - SSH Client Utilities
SSH client wrapper with connection management and remote command execution capabilities.

### `systemd/` - systemd Service Management
Tools for managing systemd services, including start, stop, status, and configuration operations.

### `term/` - Terminal Utilities
Terminal and console utilities for interactive command-line applications.

### `times/` - Time Manipulation
Enhanced time and date manipulation utilities beyond the standard library.

### `webhook/` - HTTP Webhook Client
HTTP client specifically designed for sending webhook payloads with retry logic, timeouts, and context support.

## Experimental Modules (`exp/`)

### `config/` - Configuration Management
Multi-source configuration loader supporting file, Google Cloud Storage, and Google Secret Manager backends via URI-based configuration.

**Features:**
- File-based configuration (`file://`)
- Google Cloud Storage configuration (`gs://`)
- Google Secret Manager integration (`secret://`)
- Unified URI-based interface

**Example:**
```go
ctx := context.Background()
config, err := config.FromURI(ctx, "gs://my-bucket/config.json")
if err != nil {
    log.Fatal(err)
}
```

### `discord/` - Discord Bot Utilities
Discord bot integration utilities for creating and managing Discord bots.

### `http/` - HTTP Server Utilities
Comprehensive HTTP server middleware and utilities.

#### `http/clients/` - HTTP Client Configurations
Pre-configured HTTP clients with sensible defaults, timeouts, and proxy support.

#### `http/handlers/` - HTTP Middleware
Production-ready HTTP middleware including:
- **CORS**: Cross-origin resource sharing with configurable policies
- **Recovery**: Panic recovery with structured logging
- **Compression**: Gzip compression with configurable levels
- **Minification**: HTML, CSS, and JavaScript minification
- **Logging**: Request/response logging and tracing
- **Rate Limiting**: Request throttling and rate limiting

**Example:**
```go
mux := http.NewServeMux()
mux.HandleFunc("/api", apiHandler)

handler := handlers.Patch(mux, 
    []string{"https://example.com"}, // allowed origins
    handlers.DefaultAllowedMethods,   // allowed methods
    handlers.DefaultAllowedHeaders,   // allowed headers
)

log.Fatal(http.ListenAndServe(":8080", handler))
```

#### `http/healthz/` - Health Check Endpoints
Standard health check endpoints for monitoring and load balancing.

#### `http/responses/` - HTTP Response Utilities
Standardized HTTP response helpers for JSON, errors, and common status codes.

#### `http/throttled/` - Rate Limiting
Advanced rate limiting middleware with multiple algorithms and storage backends.

#### `http/tracer/` - Request Tracing
Request tracing and correlation ID management for distributed systems.

### `iot/` - IoT Device Management
Internet of Things device integration and management utilities.

#### `iot/ezplug/` - Smart Plug Control
Control and monitoring for EZPlug smart outlets and power management devices.

#### `iot/wled/` - WLED Lighting Control
Integration with WLED (WiFi LED) controllers for managing addressable LED strips and matrices.

### `logger/` - Structured Logging
Advanced logging utilities built on top of Uber's Zap logger.

**Features:**
- File-based logging with rotation
- stderr logging for development
- HTTP middleware integration
- Context-aware logging
- Structured and sugared logger interfaces

**Example:**
```go
// Create a file-based logger
logger := logger.File("/var/log/app.log")
logger.Info("Application started")

// Use with HTTP middleware
handler := logger.WithLogger(logger)(http.HandlerFunc(myHandler))

// Get logger from HTTP request context
func myHandler(w http.ResponseWriter, r *http.Request) {
    log := logger.FromRequest(r)
    log.Info("Processing request")
}
```

### `manifest/` - Application Manifest
Application metadata and manifest management utilities.

### `paths/` - Path Manipulation
Enhanced path manipulation utilities for file system operations.

### `pushover/` - Push Notifications
Pushover notification service integration for sending push notifications to mobile devices.

## Testing

The repository includes comprehensive test suites for all modules. Run tests with:

```bash
# Run all tests
go test ./...

# Run tests for a specific module
go test ./webhook
go test ./exp/logger

# Run tests with verbose output
go test -v ./...
```

## Key Dependencies

- **Logging**: `go.uber.org/zap` with `lumberjack` for log rotation
- **HTTP**: `gorilla/handlers`, `rs/cors` for CORS handling
- **Cloud**: `cloud.google.com/go/storage` and `cloud.google.com/go/secretmanager` for GCP integration
- **Minification**: `tdewolff/minify` for HTML/CSS/JS minification
- **IoT**: `eclipse/paho.mqtt.golang` for MQTT communication
- **Notifications**: `gregdel/pushover` for push notifications

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Caveats

1. **Experimental modules**: Anything in `exp/` is to be considered experimental/work-in-progress
2. **Stable modules**: Everything else (not in `exp/`) can be relied upon for production use
3. **Breaking changes**: Experimental modules may have breaking changes without notice
4. **Pull requests**: PRs are always welcome!

## License
    
Copyright 2025 Nick Granado <ngranado@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
