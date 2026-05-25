# Lavender Messenger - A secure messaging application

**Author:** Pavel Davydov (ferz)

**Version:** 1.0.6.19

A real-time secure messaging application with gRPC server and multiple client implementations.

## Running Commands

### Server

**Default (recommended):**
```bash
go run .
```

**If port 50051 is already in use:**
```bash
lsof -ti:50051 | xargs kill -9 2>/dev/null; go run .
```

### Console Client

```bash
go run ./client/console/main.go
```

**Hints:**
- Press **Enter** to use the default username from config
- Press **Enter** (without typing a message) to send an auto-generated test message

### macOS Client

```bash
go run ./client/macos/main.go
```

## Project Structure

```
LavenderMessenger/
|
|--- main.go                    # Server entry point with gRPC setup
|--- server.go                  # Core server implementation
|--- db.go                      # Database operations and PostgreSQL integration
|--- hub.go                     # Client connection management hub
|--- crypto.go                  # AES-256 encryption/decryption for messages
|--- .env.example               # Environment configuration template (server only)
|--- .env                       # Environment configuration (server runtime)
|--- go.mod                     # Go module definition
|--- go.sum                     # Go module dependencies checksum
|--- CHANGELOG.md               # Server changelog
|--- README.md                  # This documentation
|--- messenger.proto            # Protocol buffer definition for gRPC
|
|--- gen/                       # Generated gRPC files
|    |
|    |--- messenger.pb.go       # Protocol buffer Go code
|    |--- messenger_grpc.pb.go  # gRPC service code
|
|--- client/                    # All client applications
|    |
|    |--- android/              # Android client application
|    |    |
|    |    |--- app/             # Android app source
|    |    |    |--- src/        # Source code
|    |    |    |    |--- main/  # Main source set
|    |    |    |    |--- java/  # Kotlin code
|    |    |    |    |--- res/   # Resources
|    |    |    |--- build.gradle
|    |    |    |--- CHANGELOG.md
|    |    |--- build.gradle
|    |    |--- README.md
|    |    |--- README.ru.md
|    |
|    |--- console/              # Console client application
|    |    |
|    |    |--- main.go          # Console client entry point
|    |    |--- config.yaml      # Console client configuration
|    |
|    |--- macos/                # macOS client application
|    |    |
|    |    |--- main.go          # macOS client entry point with GUI
|    |    |--- config.yaml      # macOS client configuration
|    |    |--- CHANGELOG.md     # macOS client changelog
|
|--- .github/                   # GitHub workflows
|    |
|    |--- workflows/
|    |    |
|    |    |--- go.yml           # Go CI/CD workflow
```

## File Descriptions

### Server Files

- **`main.go`** - Server entry point and initialization
  - Loads environment variables from `.env`
  - Sets up gRPC server with version "1.0.6.19"
  - Initializes database connection
  - Starts TCP listener on configured address
  - Registers chat service

- **`server.go`** - Core server implementation
  - gRPC service implementation for real-time messaging
  - Client connection management
  - Message broadcasting and encryption
  - Database integration for message persistence
  - Administrative actions (Admin Password Reset)

- **`db.go`** - Database operations and PostgreSQL integration
  - PostgreSQL connection management
  - Messages table creation and management
  - Secure message storage with encrypted content
  - Connection pooling and error handling

- **`hub.go`** - Client connection management hub
  - Real-time client connection tracking
  - Message broadcasting to all connected clients
  - Thread-safe client registration/unregistration
  - Connection state management

- **`crypto.go`** - AES-256 encryption/decryption for messages
  - Secure message encryption using AES-256 GCM mode
  - Key management from environment variables
  - Message confidentiality and integrity

- **`messenger.proto`** - Protocol buffer definition for gRPC
  - Message structure definitions
  - gRPC service interface
  - Message serialization format

- **`.env.example`** - Environment configuration template
  - Server address and port settings
  - Database connection string
  - Security keys for encryption
  - Configuration examples and documentation

- **`go.mod`** - Go module definition
  - Project dependencies and versions
  - Go version requirements
  - Module path configuration

- **`go.sum`** - Go module dependencies checksum
  - Dependency integrity verification
  - Version locking for reproducible builds

### Generated Files (`gen/`)

- **`messenger.pb.go`** - Protocol buffer generated Go code
  - Message struct definitions
  - Serialization/deserialization methods
  - Protocol buffer implementation

- **`messenger_grpc.pb.go`** - gRPC service generated code
  - gRPC client and server interfaces
  - Service method implementations
  - gRPC communication handlers

### Client Applications (`client/`)

- **`android/`** - Android client application
  - Native Android application with modern UI
  - Real-time messaging with gRPC
  - Private messaging system (direct chats)
  - Chat list UI with room management
  - Theme support (light/dark)
  - Russian localization
  - Message reactions support
  - Custom lavender logo and branding
  - Avatar display in chat list and user list
  - **Background APK downloading and update system**
  - **Super Admin tools: User/Chat management and Password Reset**
  - **Group Video Conferences with Presence and Admin controls**
  - **Call Duration Tracking in chat history**
  - Version: 1.0.6.26

- **`console/main.go`** - Console client application (primary CLI client)
  - YAML-based configuration (no .env required)
  - gRPC communication with server using `grpc.NewClient`
  - Interactive message input/output
  - Connection status monitoring
  - Auto-generated test messages on empty input (`test message NNNN`)
  - Default username from config with override support

- **`macos/main.go`** - macOS client application
  - Fyne-based GUI application for macOS
  - Real-time messaging interface with rich text
  - Server connection handling and status monitoring
  - Theme management (light/dark themes)
  - Emoji support with popup selector
  - User color customization
  - Configuration persistence
  - Server availability checking

- **`console/config.yaml`** - Console client configuration
  - `server_address`: gRPC server connection endpoint
  - `last_username`: Default username (user can override on start)

- **`macos/config.yaml`** - macOS client configuration
  - Server connection settings
  - Theme definitions (light/dark)
  - User preferences and last username
  - Custom color schemes

- **`macos/CHANGELOG.md`** - macOS client version history
  - Client-specific updates and features
  - UI improvements and bug fixes
  - Version tracking for macOS client

### GitHub Workflows (`.github/`)

- **`.github/workflows/go.yml`** - Go CI/CD workflow
  - Automated testing and building
  - Go version matrix testing
  - Continuous integration setup

### Deployment & Monitoring

- **`deploy.sh`** - Automated deployment script (local Mac → remote Linux server)
  - Cross-compilation for Linux (GOOS=linux GOARCH=amd64)
  - Safe deployment with health checks and automatic rollback
  - Log rotation: keeps only last 100 lines of logs.txt
  - Automatic cron setup for server monitoring
  - rsync file synchronization with proper excludes

- **`monitor.sh`** - Server health monitoring script
  - Runs every 30 minutes via cron job to check server status
  - Verifies server is listening on port 50051
  - Automatically restarts server if it's down
  - All checks logged to `logs.txt` with `[MONITOR]` prefix
  - Integrated into `deploy.sh` for automatic setup

- **`start.sh`** - Server startup script (generated during deployment)
  - Health check before declaring deployment successful
  - Graceful process termination and startup
  - Automatic rollback to previous version on failure

## Getting Started

### Server Setup

1. Copy `.env.example` to `.env` and configure settings
2. Install dependencies: `go mod tidy`
3. Run server: `go run main.go server.go`

### Client Setup

#### Console Client (Recommended for CLI)
1. Configure `client/console/config.yaml`:
   ```yaml
   server_address: 192.168.1.135:50051
   last_username: YourName
   ```
2. Run from project root: `go run ./client/console/main.go`
3. Press Enter to accept default username, or type new one
4. Press Enter without message to send auto-generated test message

#### macOS Client
1. Configure `client/macos/config.yaml` with server address
2. Run client from project root: `go run ./client/macos/main.go`

#### Android Client
1. Open project in Android Studio: `client/android/`
2. Build APK: `./gradlew assembleDebug`
3. Install on device or emulator
4. Configure server address in app settings

## Architecture

- **Server**: gRPC-based with WebSocket hub for real-time communication
- **Database**: PostgreSQL with connection pooling
- **Clients**:
  - Android native application with modern UI (Kotlin)
  - Console client with YAML config (primary CLI client)
  - Native macOS application with real-time messaging
- **Protocol**: Protocol Buffers for message serialization

## Recent Changes

### Bug Fixes
- **Reaction foreign key constraint**: Fixed foreign key constraint violation in `SetReaction` when message doesn't exist. Added message existence check before inserting reaction.
- **SQL NULL handling**: Fixed scanning error in `GetUserChats` when `last_message_time` is NULL for empty chats. Now uses `sql.NullTime` to properly handle NULL values from the MAX() subquery.

### Features
- **Admin Password Reset**: Super Admins can now securely reset any user's password directly from the admin panel.
- **Background Android Updates**: Implemented `WorkManager` based background downloading for Android APKs with progress notifications and auto-navigation to install.
- **Group Video Conferences**: Comprehensive support for group video meetings with real-time presence tracking, system notifications, and organizer administrative controls.
- **Modernized Call Experience**: Redesigned call UI with large avatars, blurred backgrounds, and avatar-first startup for reliable performance on slow networks.
- **Improved Stability (Universal UUID Migration)**: Full migration to stable User IDs (UUIDs) for all account operations, ensuring session and data continuity even after username changes.
- **Improved gRPC Reliability**: Fixed reconnection race conditions on Android to ensure messages sent while offline are delivered immediately upon reconnection.
- **Push notification improvements**: Added `room_id` to push notification data payload for proper chat navigation on notification click.
- **Auto-navigation**: Server now sends room_id with push notifications to enable direct chat opening from notifications.

### Security Fixes
- **CVE-2026-33809**: Updated `golang.org/x/image` to v0.38.0+ (was v0.24.0)

### Structural Changes
- Module path renamed from `msg` to `LavenderMessenger`
- Console client now uses YAML config instead of `.env` (moved from `client/client.go`)
- Added auto-generated test messages feature for console client
- Removed deprecated `client/client.go`

## Version History

See `CHANGELOG.md` for server changes, `client/android/CHANGELOG.md` for Android client changes, and `client/macos/CHANGELOG.md` for macOS client changes.
