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

### `dotenv/` - Environment File Parser
Zero-dependency `.env` file parser with full feature support.

**Features:**
- Load, Overload, Read, Parse, Unmarshal, UnmarshalBytes for reading
- Marshal, Write for serialization
- Exec for running commands with loaded environment
- Single/double/backtick quoting with proper escape handling
- Variable expansion (`$VAR`, `${VAR}`)
- Export prefix, inline comments, CRLF normalization

**Example:**
```go
// Load .env file (does not override existing env vars)
err := dotenv.Load()

// Load and override existing env vars
err := dotenv.Overload(".env.local")

// Read into map without modifying os.Environ
envMap, err := dotenv.Read(".env")
```

### `gcs/` - Google Cloud Storage (deprecated)
Thin wrapper around `cloud.google.com/go/storage` for bucket and object operations.

**Deprecated** in favor of `exp/storage` with `gs://bucket/key` URIs. The new package pools the GCS client across calls and lets callers swap to local filesystem or in-memory backends without changing call sites. See the Deprecations section below.

### `gravatar/` - Gravatar Integration
Full Gravatar URL API client with functional options pattern. Supports avatar images, profile URLs (JSON/XML/VCF), and QR codes.

**Features:**
- `AvatarURL` with configurable size (1-2048px), default image, rating, and force default
- `ProfileURL` for JSON, XML, and vCard response formats
- `QRCodeURL` for the v3 QR code endpoint
- All 8 default image types (`404`, `mp`, `identicon`, `monsterid`, `wavatar`, `retro`, `robohash`, `blank`)
- SHA256 email hashing per current Gravatar spec

**Example:**
```go
url := gravatar.AvatarURL("user@example.com",
    gravatar.WithSize(200),
    gravatar.WithDefault(gravatar.DefaultIdenticon),
    gravatar.WithRating(gravatar.RatingPG),
)

profileJSON := gravatar.ProfileURL("user@example.com", gravatar.FormatJSON)
qrCode := gravatar.QRCodeURL("user@example.com")
```

### `progressbar/` - Progress Bar
Minimal, zero-dependency terminal progress bar implementing `io.Writer`.

**Features:**
- `DefaultBytes(total, description)` constructor
- Implements `io.Writer` for use with `io.TeeReader`, `io.Copy`, etc.
- Throttled redraws (100ms) to avoid terminal spam
- Human-readable byte formatting (B, KB, MB, GB)

**Example:**
```go
bar := progressbar.DefaultBytes(fileSize, "Uploading")
teeReader := io.TeeReader(file, bar)
io.Copy(dst, teeReader)
bar.Close()
```

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

### `xdg/` - XDG/Platform Path Resolution
Zero-dependency application path resolver following platform conventions.

**Features:**
- XDG Base Directory spec compliance on Linux (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`, etc.)
- macOS `~/Library/` conventions (Preferences, Application Support, Caches, Logs)
- Windows `%LOCALAPPDATA%` / `%PROGRAMDATA%` support
- User, System, and CustomHome scope types
- Vendor prefix support
- Config, Data, State, Cache, and Log path resolution
- File lookup across priority-ordered directories

**Example:**
```go
scope := xdg.NewScope(xdg.User, "myapp")

configPath, err := scope.ConfigPath("config.yaml")
logPath, err := scope.LogPath("app.log")
statePath, err := scope.StatePath("state.db")
cacheDir, err := scope.CacheDir()

// With vendor prefix
scope := xdg.NewVendorScope(xdg.User, "mycompany", "myapp")
```

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

### `epub/` - EPUB Metadata
EPUB metadata parsing, cover image extraction, and word counting. Parses OPF metadata directly instead of relying on third-party libraries that panic on optional-element gaps. Supports EPUB 2, EPUB 3, and Calibre custom metadata.

**Features:**
- `Metadata`: title, authors, ISBN, publisher, subjects, description, language, series, edition, publish date
- Cover image extraction
- Word-count estimation across spine items

### `storage/` - URI-Addressable Blob Storage
URI-dispatched `Store` abstraction over three backends: Google Cloud Storage, local filesystem, and an in-memory store for tests. Callers switch backends by changing a URI; the surface never exposes backend-specific types.

**Schemes:**
- `gs://bucket/key` - Google Cloud Storage, with a lazily-initialized `*storage.Client` reused across calls via `sync.Once` and package-level memoization.
- `file:///abs/path` - Local filesystem. Preserves `ContentType` via a `<path>.meta.json` sidecar. Rejects path traversal (`..` / `.` segments, non-empty host, non-absolute paths). POSIX-only; Windows file URIs are not handled in this version.
- `mem://namespace/key` - Process-global in-memory backend keyed by full URI. Intended for tests; callers isolate with a per-test namespace (e.g., `mem://<t.Name()>/...`).

**Features:**
- `Store` interface with `Get`, `PutFile`, `PutBytes`, `Delete`, `List`.
- `For(uri)` resolves and memoizes one `Store` per scheme so the GCS HTTP/gRPC pool is reused across calls within a process.
- Package-level helpers (`storage.Get`, `storage.PutBytes`, etc.) dispatch by URI scheme.
- `List` returns a backend-neutral `[]Object` with `URI`, `Size`, `ContentType`, `Updated`, `Generation`, `Metageneration`.
- Sentinels: `ErrUnsupportedScheme`, `ErrInvalidURI`, `ErrNotExist` (aliases `io/fs.ErrNotExist`).
- Integration tests against real GCS run via `mage integration` with `STORAGE_TEST_BUCKET` and ADC.

**Example:**
```go
// Same call, different backend — only the URI changes.
data, err := storage.Get(ctx, "gs://my-bucket/configs/app.yaml")
data, err := storage.Get(ctx, "file:///var/lib/myapp/configs/app.yaml")
data, err := storage.Get(ctx, "mem://test/configs/app.yaml")

// Or bind once:
s, _ := storage.For(os.Getenv("STORAGE_URI"))
data, err := s.Get(ctx, uri)

// Detect missing objects uniformly across backends:
if errors.Is(err, storage.ErrNotExist) { /* ... */ }
```

### `http/` - HTTP Server Utilities
Comprehensive HTTP server middleware and utilities.

#### `http/clients/` - HTTP Client Configurations
Pre-configured HTTP clients with sensible defaults, timeouts, and proxy support.

#### `http/handlers/` - HTTP Middleware
Production-ready HTTP middleware including:
- **CORS**: Cross-origin resource sharing with configurable policies
- **Recovery**: Panic recovery with structured logging
- **AccessLog**: Structured HTTP access logging (method, path, status, bytes, client IP, user-agent, duration)
- **Compression**: Gzip compression with configurable levels
- **Minification**: HTML, CSS, and JavaScript minification
- **Dump**: Full request dump logging for debugging
- **Rate Limiting**: Request throttling and rate limiting

**Example:**
```go
mux := http.NewServeMux()
mux.HandleFunc("/api", apiHandler)

// Patch applies: Recover -> AccessLog -> Compress -> Minify -> CORS
handler := handlers.Patch(mux,
    []string{"https://example.com"}, // allowed origins
    handlers.DefaultAllowedMethods,   // allowed methods
    handlers.DefaultAllowedHeaders,   // allowed headers
)

log.Fatal(http.ListenAndServe(":8080", handler))

// Or use AccessLog standalone
handler := handlers.AccessLog(myHandler)
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

### `loom/` - Remote Service Deployment
Automated deployment and management utilities for remote Linux services over SSH.

**Features:**
- SSH-based remote command execution
- systemd service management
- File upload and directory setup
- Service file generation
- Support for SSH agent and password authentication
- Environment-based configuration via `.env` files

**Environment Variables:**
- `LOOM_SSH_LOGIN`: SSH username
- `LOOM_SSH_PASSWORD`: SSH password (when not using agent)
- `LOOM_SSH_HOSTNAME`: Target hostname
- `LOOM_SSH_PORT`: SSH port (defaults to 22)
- `LOOM_SSH_DESTINATION`: Remote upload destination

**Example:**
```go
// Create a new loom instance
loom, err := loom.New("my-service", true) // use SSH agent
if err != nil {
    log.Fatal(err)
}

// Generate and setup systemd service
serviceFile, err := loom.ServiceFile("/opt/my-service/bin/my-service")
if err != nil {
    log.Fatal(err)
}

// Deploy the service
err = loom.Setup(serviceFile)
if err != nil {
    log.Fatal(err)
}

// Control the service
err = loom.Service("start")
if err != nil {
    log.Fatal(err)
}
```

### `manifest/` - Application Manifest
Versioned manifest storage for static-asset deploys, built on `exp/storage`. Tracks published `Item` entries (timestamp, `major.minor.point` version, content prefix) and prunes stale content prefixes.

**Features:**
- URI-based storage root — works against any `exp/storage` backend (`gs://`, `file://`, `mem://`)
- `Save` / `Load` round-trip to `manifest.json` under the root URI
- `Init` returns the next `Item` plus a rolling window of prior versions (returns `storage.ErrNotExist` when the manifest is missing or empty)
- `Clean` prunes objects whose URIs don't match any current-version prefix or caller-provided allow-list; matching is exact-or-trailing-slash, not a raw `HasPrefix`

**Example:**
```go
m := manifest.New("gs://my-bucket", "2024-01-01")
item, history, err := m.Init(ctx)
// ... upload assets under gs://my-bucket/<item.Prefix>/ ...
_ = m.Save(ctx, history)
_ = m.Clean(ctx, history, []string{"manifest.json"})
```

### `paths/` - Path Manipulation
Enhanced path manipulation utilities using the `xdg` module for platform-appropriate config, log, and data paths.

### `pushover/` - Push Notifications
Pushover notification service integration for sending push notifications to mobile devices.

## Deprecations

The following APIs are deprecated. Each continues to work; callers should migrate to the replacement.

| Deprecated | Replacement | Why |
|---|---|---|
| `gcs` package (entire package: `Get`, `PutFile`, `PutBytes`, `Delete`, `List`) | `exp/storage` with `gs://bucket/key` URIs | URI-based dispatch, GCS client reuse, backend-neutral `List` (no `cloud.google.com/go/storage` types leak through the API) |
| `dotenv.Exec` | `dotenv.ExecContext` | Lets the caller cancel or set a deadline on the spawned process |
| `shell.Execute` | `shell.ExecuteContext` | Context-aware execution |
| `shell.ExecuteWith` | `shell.ExecuteWithContext` | Context-aware execution |
| `ssh.NewWithAgent` | `ssh.NewWithAgentContext` | Lets the caller bound the agent-socket dial |
| `term.PasswordPrompt` | `term.PasswordPromptContext` | Returns errors instead of terminating the process; caller wires signal handling |

`staticcheck` / `golangci-lint` flag calls to any of these with `SA1019`.

## Testing

The repository includes test suites for all modules. Run tests with:

```bash
# Run all tests
go test ./...

# Run tests for a specific module
go test ./webhook
go test ./exp/logger

# Run tests with verbose output
go test -v ./...
```

### Mage targets

A `magefile.go` at the repo root exposes convenience targets (requires `mage` — `go install github.com/magefile/mage@latest`):

```bash
mage test         # go test -race -count=1 ./...
mage lint         # golangci-lint run --timeout=5m ./...
mage sec          # gosec ./...
mage integration  # go test -tags=integration -run=Integration ./exp/storage/...
                  # requires STORAGE_TEST_BUCKET and Application Default Credentials
```

## Key Dependencies

- **Logging**: `go.uber.org/zap` with `lumberjack` for log rotation
- **HTTP**: `gorilla/handlers`, `rs/cors` for CORS handling
- **Cloud**: `cloud.google.com/go/storage` and `cloud.google.com/go/secretmanager` for GCP integration
- **Minification**: `tdewolff/minify` for HTML/CSS/JS minification
- **IoT**: `eclipse/paho.mqtt.golang` for MQTT communication
- **Notifications**: `gregdel/pushover` for push notifications
- **YAML**: `gopkg.in/yaml.v3` for configuration file parsing

Notable: `dotenv`, `gravatar`, `progressbar`, and `xdg` are zero-dependency, stdlib-only implementations.

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

Copyright 2026 Nick Granado <ngranado@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
