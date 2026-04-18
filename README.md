# Lavender Messenger - A secure messaging application

**Author:** Pavel Davydov (ferz)

A real-time secure messaging application with gRPC server and multiple client implementations.

## Project Structure

```
LavenderMessenger/
|
|--- main.go                    # Server entry point with gRPC setup
|--- server.go                  # Core server implementation
|--- db.go                      # Database operations and PostgreSQL integration
|--- hub.go                     # Client connection management hub
|--- crypto.go                  # AES-256 encryption/decryption for messages
|--- .env.example               # Environment configuration template
|--- .env                       # Environment configuration (runtime)
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
|    |--- client.go             # CLI client implementation
|    |
|    |--- console/              # Console client application
|    |    |
|    |    |--- client.go        # Console client entry point
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
  - Sets up gRPC server with version "0.9.1"
  - Initializes database connection
  - Starts TCP listener on configured address
  - Registers chat service

- **`server.go`** - Core server implementation
  - gRPC service implementation for real-time messaging
  - Client connection management
  - Message broadcasting and encryption
  - Database integration for message persistence

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

- **`client.go`** - CLI client implementation
  - Command-line interface for testing and debugging
  - gRPC client with insecure connection support
  - Real-time messaging with interactive input
  - Environment configuration loading
  - UTF-8 character handling

- **`console/client.go`** - Console client application
  - Simple console-based chat interface
  - gRPC communication with server
  - Interactive message input/output
  - Connection status monitoring

- **`macos/main.go`** - macOS client application
  - Fyne-based GUI application for macOS
  - Real-time messaging interface with rich text
  - Server connection handling and status monitoring
  - Theme management (light/dark themes)
  - Emoji support with popup selector
  - User color customization
  - Configuration persistence
  - Server availability checking

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

## Getting Started

### Server Setup

1. Copy `.env.example` to `.env` and configure settings
2. Install dependencies: `go mod tidy`
3. Run server: `go run main.go server.go`

### Client Setup

#### Command-line Client
1. Configure environment variables in `.env`
2. Run client from project root: `go run ./client/client.go`

#### macOS Client
1. Configure `client/macos/config.yaml` with server address
2. Run client from project root: `go run ./client/macos/main.go`

## Architecture

- **Server**: gRPC-based with WebSocket hub for real-time communication
- **Database**: PostgreSQL with connection pooling
- **Clients**: 
  - Command-line client for testing and debugging
  - Native macOS application with real-time messaging
- **Protocol**: Protocol Buffers for message serialization

## Version History

See `CHANGELOG.md` for server changes and `client/macos/CHANGELOG.md` for macOS client changes.
