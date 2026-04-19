# Lavender Messenger - Changelog

**Author:** Pavel Davydov (ferz)

## [0.9.6] - 2026-04-20
- **Full-stack Message Reactions and UUID Support**
  - **Android Client:**
    - Implemented UI for message reactions (long-press to select emoji).
    - Added UUID generation for unique message tracking.
    - Updated `Message` model to include `reactions` and `id`.
    - Improved UI stability and message ordering.
  - **Go Server:**
    - Updated `messenger.proto` with `id`, `reactions`, and `SetReaction` RPC.
    - Added `reactions` table and `message_id` column to PostgreSQL schema.
    - Implemented `SetReaction` gRPC endpoint and database logic.
    - Enhanced `GetHistory` to include message reactions.
    - Added robust message deletion by both UUID and content fallback.
  - **Database:**
    - Automated schema migration for `messages` and `reactions` tables.

## [0.9.2] - 2026-04-19
- **Server version update**
  - Updated server version to 0.9.2
- **New features: Message history and online users**
  - Added `GetMessages(limit int)` in `db.go` - retrieve recent messages from database
  - Added `GetOnlineUsers()` in `hub.go` - get list of unique connected usernames
  - Added `UpdateName(stream, name)` in `hub.go` - track usernames per connection
  - Added `GetClients()` RPC in `server.go` - list active users endpoint
  - Added `GetHistory()` RPC in `server.go` - message history endpoint with decryption
  - Hub now tracks usernames per stream for accurate online user listing

## [0.9.1] - 2026-04-18
- **Code quality and documentation improvements**
  - Added comprehensive English comments to all Go source files
  - Added Lavender Messenger branding headers to all files with author attribution
  - Fixed error handling consistency in db.go (line 57)
  - Updated project documentation with proper branding
  - **Files updated:**
    - `main.go` - Added comprehensive English comments and Lavender Messenger header
    - `db.go` - Added comprehensive English comments, fixed error handling, and Lavender Messenger header
    - `server.go` - Added comprehensive English comments and Lavender Messenger header
    - `hub.go` - Added comprehensive English comments and Lavender Messenger header
    - `crypto.go` - Added comprehensive English comments and Lavender Messenger header
    - `client/client.go` - Added comprehensive English comments and Lavender Messenger header
    - `client/console/client.go` - Added comprehensive English comments and Lavender Messenger header
    - `client/macos/main.go` - Added comprehensive English comments and Lavender Messenger header
    - `README.md` - Updated with Lavender Messenger branding and author information
    - `CHANGELOG.md` - Updated with Lavender Messenger branding and author information
    - `client/macos/CHANGELOG.md` - Updated with Lavender Messenger branding and author information
    - `.env.example` - Added Lavender Messenger header

## [0.9.0] - 2026-04-17
- **Initial release**
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
