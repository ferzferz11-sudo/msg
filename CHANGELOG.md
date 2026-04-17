# Changelog

## [0.9.0] - 2026-04-17
- **Current development version**
  - Server version: 0.9.0
  - Core server implementation
  - gRPC service setup
  - Database integration
  - **Files created:**
    - `main.go` - Entry point with server initialization and gRPC setup
    - `server.go` - Core server implementation with WebSocket hub and database handling
    - `.env` - Environment configuration file for server settings and database connection
    - `go.mod` - Go module definition with dependencies
    - `gen/` - Generated gRPC protocol buffer files
    - `client/` - Client applications directory
      - `client.go` - Command-line client implementation
      - `macos/` - macOS client application
        - `main.go` - macOS client entry point with UI and messaging logic
        - `config.yaml` - macOS client configuration file
