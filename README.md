# MSG - Messaging Application

A real-time messaging application with gRPC server and macOS client.

## Project Structure

```
msg/
|
|--- main.go                    # Server entry point
|--- server.go                  # Core server implementation  
|--- .env                       # Environment configuration
|--- go.mod                     # Go module definition
|--- CHANGELOG.md               # Server changelog
|--- README.md                  # This documentation
|
|--- gen/                       # Generated gRPC files
|    |
|    |--- *.pb.go              # Protocol buffer Go code
|    |--- *_grpc.pb.go         # gRPC service code
|
|--- client/                    # All client applications
|    |
|    |--- client.go            # CLI client implementation
|    |
|    |--- macos/               # macOS client application
|    |    |
|    |    |--- main.go         # macOS client entry point with UI
|    |    |--- config.yaml     # macOS client configuration
|    |    |--- CHANGELOG.md    # macOS client changelog
```

## File Descriptions

### Server Files

- **`main.go`** - Server entry point and initialization
  - Loads environment variables from `.env`
  - Sets up gRPC server with version "1.0.0"
  - Initializes database connection
  - Starts TCP listener on configured address
  - Registers chat service

- **`server.go`** - Core server implementation
  - WebSocket hub for real-time messaging
  - Database operations and message persistence
  - Client connection management
  - gRPC service implementation

- **`.env`** - Environment configuration
  - Server address and port settings
  - Database connection string
  - Security keys for encryption
  - Client connection settings

- **`go.mod`** - Go module definition
  - Project dependencies
  - Go version requirements
  - Module path configuration

### Generated Files (`gen/`)

- **`*.pb.go`** - Protocol buffer generated Go code
- **`*_grpc.pb.go`** - gRPC service generated code

### Client Applications (`client/`)

- **`client.go`** - CLI client implementation
  - Command-line interface for testing and debugging
  - gRPC client with TLS/insecure connection support
  - Real-time messaging with interactive input
  - Environment configuration loading

- **`macos/main.go`** - macOS client application
  - Fyne-based GUI application for macOS
  - Real-time messaging interface with rich text
  - Server connection handling and status monitoring
  - Theme management (light/dark themes)
  - Emoji support with popup selector
  - User color customization
  - Configuration persistence

- **`macos/CHANGELOG.md`** - macOS client version history
  - Client-specific updates and features
  - UI improvements and bug fixes

- **`config.yaml`** - macOS client configuration
  - Server connection settings
  - Theme definitions (light/dark)
  - User preferences and last username

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
