# Lavender Messenger - Changelog

**Author:** Pavel Davydov (ferz)

## [1.0.1.30] - 2026-04-22
- **Server: Group Chats and Deletion Features**
  - **New RPC Methods:**
    - `CreateGroupChat` - Create group chats with multiple participants
    - `DeleteChat` - Delete chats and all associated messages/images
    - `DeleteProfile` - Delete user profile and all their data
  - **gRPC Improvements:**
    - Added keepalive enforcement policy (5 second min ping interval)
    - Permit pings without active streams for better connection stability
  - **Proto Updates:**
    - Added `CreateGroupChatRequest/Response` messages
    - Added `DeleteChatRequest/Response` messages
    - Added `DeleteProfileRequest/Response` messages
  - **Go Server:**
    - Server version: 1.0.1.30

## [1.0.1.28] - 2026-04-20
- **Android Avatar Display Fix**
  - **ChatAdapter:**
    - Fixed avatar caching issue where avatar URLs were not being saved for users with avatars
    - Now correctly saves all avatar URLs (including empty strings) to cache
    - Removed fade-in animation for avatars in chat list for better performance
  - **ChatListActivity:**
    - Updated avatar loading logic to save all URLs to cache before displaying chats
    - Chats now display only after all participant avatars are loaded
    - Added 5-second fallback timeout for avatar loading
    - Applied same fixes to startPollingChats and onResume for consistency
  - **Result:**
    - Avatars now correctly display in chat list (small icons next to chat names)
    - Avatars correctly display in user list dialog
    - No more flickering or missing avatars
  - **Android Version:** 1.0.1.28

## [0.1.5] - 2026-04-20
- **GetAllUsers RPC Implementation**
  - **Proto Files:**
    - Added GetAllUsersRequest and GetAllUsersResponse messages
    - Added GetAllUsers RPC method to ChatService
    - Successfully generated Go code using protoc with --go_out=gen parameter
  - **Go Server:**
    - Implemented GetAllUsers RPC handler in server.go
    - Implemented GetAllUsers DB method in db.go to query users table
    - Build version: 0.1.5
  - **macOS Client:**
    - Updated showAllUsersList to use GetAllUsers RPC instead of GetClients
    - Client version: 1.0.0 (build 0.1.5)
  - **Android Client:**
    - Build version: 0.1.5
  - **gRPC Generated Files:**
    - Updated gen/messenger.pb.go with new message types
    - Updated gen/messenger_grpc.pb.go with new RPC method

## [0.1.4] - 2026-04-20
- **macOS Client Toolbar UI Improvements**
  - **macOS Client:**
    - Removed server address display from toolbar (shown in status after connection)
    - Added third button "Все" for showing all registered users
    - Renamed users button to "Онлайн" for clarity
    - Implemented showAllUsersList function (currently uses GetClients, needs GetAllUsers RPC)
    - Client version: 1.0.0 (build 0.1.4)
  - **Android Client:**
    - Build version: 0.1.4
  - **Go Server:**
    - Build version: 0.1.4

## [0.1.3] - 2026-04-20
- **macOS Client Chat List and Users List**
  - **macOS Client:**
    - Added "Chats" button in toolbar to show chat list
    - Added "Users" button in toolbar to show online users
    - Implemented showUsersList function to display online users
    - Implemented createDirectChat function to create direct chats with users
    - Implemented switchToChat function to switch between chats
    - Added global variables for chatBox and connectToServer
    - Client version: 1.0.0 (build 0.1.3)
  - **Android Client:**
    - Build version: 0.1.3
  - **Go Server:**
    - Build version: 0.1.3

## [0.1.2] - 2026-04-20
- **macOS Client Message History and Room Support**
  - **macOS Client:**
    - Added loadHistory function to retrieve message history from server
    - Implemented room_id support in messages for proper room filtering
    - Fixed config loading order to check main.go directory first
    - Added server address display in toolbar (italic)
    - Updated toolbar layout with status indicator, status, and server address
    - Client version: 1.0.0 (build 0.1.2)
  - **Android Client:**
    - Build version: 0.1.2
  - **Go Server:**
    - Build version: 0.1.2

## [0.1.1] - 2026-04-20
- **Server Address Configuration and macOS Client Update**
  - **Android Client:**
    - Moved server address to string resources (server_address, server_address_local)
    - Updated MainActivity server list: ["159.195.38.145:50051", "192.168.1.135:50051"]
    - Removed "10.0.2.2:50051" and "localhost:50051" from server list
    - Updated ChatActivity to use string resource for server address
    - Updated ChatListActivity to parse server address from string resource
    - Updated ServerConnectivityTest with new address list
    - Fixed username intent key from "USERNAME" to "username"
    - Added logging for loadHistory calls
    - Build version: 0.1.1
  - **macOS Client:**
    - Added password field to login dialog
    - Added password persistence in config (LastPassword)
    - Implemented GetChats RPC call for retrieving user's chats
    - Added chat list dialog for selecting chat room
    - Smart navigation: auto-open general chat if no chats exist
    - Password sent with auth/join message
    - Client version: 1.0.0 (build 0.1.1)
    - Synced version with Android client and server
  - **Go Server:**
    - Build version: 0.1.1 (synced with clients)

## [1.0.0] - 2026-04-20
- **Major Release: Private Messaging System**
  - **Android Client:**
    - Implemented private messaging system (direct chats between users)
    - Added chat list UI with room management
    - Long press on user to create direct chat
    - Smart navigation (auto-open general chat if no chats exist)
    - Russian localization (Общий чат, Личное сообщение, Чаты)
    - Custom lavender logo from IMG_9717.jpeg
    - Logo on splash screen above "Lavanda" text
    - Background image cropping (40% right removed)
    - Proper back button navigation (chat list ↔ chat, chat list → exit)
    - Fixed deprecated APIs (adapterPosition → bindingAdapterPosition, locale → locales[0])
    - Dark toolbar background (#070531) on chat list screen
    - Theme toggle icon consistency between screens
    - Message alignment fixes (outgoing messages right-aligned, incoming left-aligned)
    - Message persistence across app restarts
    - Empty message prevention on client and server
    - Server authentication temporarily disabled for debugging
    - Database cleanup of corrupted messages on startup
  - **Go Server:**
    - Added chats table with user_id, type, name fields
    - Implemented GetChats and CreateDirectChat RPC methods
    - Room-based message filtering via room_id map in hub
    - Added room_id to messages table (default 'general')
    - Updated proto files with ChatInfo, GetChatsRequest, GetChatsResponse
    - Server version: 1.0.0, build version: 0.1.0
  - **Database:**
    - Added chats table for private messaging
    - ALTER TABLE messages ADD COLUMN room_id VARCHAR(255) DEFAULT 'general'
  - **Build:**
    - Updated build version to 0.1.0 across client and server

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
