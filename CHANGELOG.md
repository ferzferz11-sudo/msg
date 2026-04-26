# Lavender Messenger - Changelog

**Author:** Pavel Davydov (ferz)

## [1.0.2.5] - 2026-04-26
- **Consolidated User Identity & Enhanced Chat List**
  - **Android: Navigation Overhaul**
    - Replaced generic "Exit" icon with User Avatar in the main toolbar.
    - Added a modern Bottom Sheet User Menu for all profile actions, themes, notifications, and logout.
    - Integrated user's "Bio" into the new User Menu.
    - Updated Chat List to display the last message text instead of the chat type.
  - **Server: Data Enrichment**
    - Added `last_message_text` to the `ChatInfo` response for real-time list previews.
  - Server version: 1.0.2.5

## [1.0.2.4] - 2026-04-26
- **Final UI Cleanup & Performance Optimization**
  - **Android: Interface Refinement**
    - Removed redundant Contacts icon from the toolbar (all features unified in the FAB menu).
    - Simplified theme previews to reflect the cleaner toolbar state.
    - Verified push navigation and search stability.
  - Server version: 1.0.2.4

## [1.0.2.3] - 2026-04-26
- **Voice Messages & Stability Update**
  - **Android: Voice & UI Implementation**
    - Added full support for recording and uploading voice messages.
    - Implemented real-time playback synchronization with live waveform visualization.
    - Improved chat list sorting based on last message activity.
    - Fixed gRPC marshalling for voice fields ensuring history persistence.
    - Optimized message display to handle legacy voice messages with empty text.
    - Introduced **SplashActivity** as the new entry point for instant login redirect.
    - Redesigned the FAB menu into a modern **Bottom Sheet** with three actions: Start Chat, Add Contact, Add Group.
    - Fixed push notification navigation to correctly open specific chat rooms.
    - Improved background stability with "silent" gRPC reconnections.
    - Enhanced chat toolbar UI with adaptive background for status indicators.
  - **Server: Maintenance & Log Optimization**
    - Created `db_maintenance.sh` for automated remote database cleanup.
    - Fixed server decryption warnings for voice messages using placeholder text.
    - Optimized database with vacuuming and orphan record removal.
    - Enhanced server-side diagnostics with detailed chat creation logs.
  - Server version: 1.0.2.3

## [1.0.2.2] - 2026-04-26
- **UI/UX Polish & Theme Engine**
  - Perfected theme edit icon sizing and color accuracy across all UI components.
  - Advanced notification styles and support for dual (chat/list) backgrounds.
  - Refined theme engine with robust state management and live previews.
  - Added "About" dialog with version information and developer credits.
  - Server version: 1.0.2.2

## [1.0.2.1] - 2026-04-26
- **Advanced Notifications & Robust Themes**
  - Choice between multiple system notification styles (Standard, Messaging, Expanded).
  - Real-time notification style preview with localized examples.
  - Separated notification settings and logs into dedicated screens.
  - Real-time FCM server log viewer for Super Admins.
  - Personalized theme previews using the user's real name and avatar.
  - Fixed "white circle" avatar bug and perfected toolbar icon tinting.
  - Server version: 1.0.2.1

## [1.0.2.0] - 2026-04-26
- **Major Feature Update: Theme Editor & Notification Center**
  - Implemented a comprehensive Theme Editor with real-time preview and server-side storage.
  - Added a Notification Center for tracking system alerts and message history.
  - Redesigned profile and group settings with Material 3 cards.
  - Server version: 1.0.2.0

## [1.0.1.60] - 2026-04-26
- **Super Admin & Group Management**
  - Implemented Super Admin permissions system, fully controlled by the server.
  - Added a dedicated Super Admin menu for user management (locks/unlocks).
  - Enhanced group management: group name editing and bulk participant selection.
  - Fixed infinite FAB rotation and loading spinner bugs in ChatListActivity.
  - Server version: 1.0.1.60

## [1.0.1.59] - 2026-04-26
- **Custom Themes & Swipe Gestures**
  - Integrated custom theme support with live application across all activities.
  - Added swipe-to-reply and swipe-to-refresh gestures for better UX.
  - Fixed toolbar title overlap and optimized chat list update checks.
  - Server version: 1.0.1.59

## [1.0.1.58] - 2026-04-25
- **Protocol Optimization & Version Tracking**
  - Added client version tracking in server logs for easier debugging.
  - Implemented faster room switching without full session resets.
  - Improved auto-reconnect stability when switching networks.
  - Server version: 1.0.1.58

## [1.0.1.56] - 2026-04-25
- **Advanced Search & Batch Management**
  - Added toolbar search toggle for both Chats and Contacts.
  - Implemented bulk participant selection for group management.
  - Optimized online status detection for near-instant updates.
  - Server version: 1.0.1.56

## [1.0.1.55] - 2026-04-25
- **UX Stability & Instant Presence**
  - Reduced gRPC keep-alive interval to 10s for more reliable online status.
  - Integrated search bar directly into the main chat list.
  - Server version: 1.0.1.55

## [1.0.1.53] - 2026-04-25
- **Real-time Status & UX Polish**
  - Added real-time Online/Offline indicators (green/gray dots) in all lists.
  - Major UX improvements for chat loading and navigation.
  - Full group administration features completed.
  - Server version: 1.0.1.53

## [1.0.1.51] - 2026-04-25
- **Full Custom Themes & Chat Backgrounds**
  - **Android: Enhanced Theme Management**
    - Fixed infinite loading spinner in `ChatListActivity` by clearing menu animations on state reset.
    - Implemented `ThemeManager` for dynamic UI coloring, allowing themes to be independent of system dark/light mode.
    - Added support for chat background images (URL-based and direct upload).
    - Integrated background image selection and upload directly into the Theme Editor.
    - Automatic transparency adjustment for message lists when a background image is active.
  - **Server: Theme & Upload Improvements**
    - Updated `messenger.proto` and database schema to support `background_image_url` in `CustomTheme`.
    - Added database migration to add `background_image_url` column to `user_themes` table.
    - Implemented detailed server-side logging for file uploads and theme persistence operations.
    - Fixed Go package name conflicts by cleaning up duplicate proto-generated files in the root directory.
  - Server version: 1.0.1.58

## [1.0.1.51] - 2026-04-25
- **Server: Session Stability & Persistent Streams**
  - Improved session handling: navigating between rooms no longer causes client disconnection.
  - Added support for lightweight "room switch" signals within a single gRPC stream.
  - Corrected versioning synchronization.
  - Server version: 1.0.1.51

## [1.0.1.50] - 2026-04-25
- **Server: Database Migration Fix**
  - Fixed SQL syntax error in chat creator migration: replaced invalid subscripting with `->>0` operator.
  - Resolved server startup crash caused by Postgres JSON array access.
  - Server version: 1.0.1.50

## [1.0.1.49] - 2026-04-25
- **Server: UX & Group Consistency Improvements**
  - Added full reload support for chat history.
  - Implemented automatic admin assignment for existing groups during migration.
  - Fixed admin visibility issues across re-connections.
  - Server version: 1.0.1.49

## [1.0.1.47] - 2026-04-25
- **Server: Deployment & Online Status Finalization**
  - Refactored deployment logic into local/remote parts for non-blocking execution.
  - Resolved race condition in online status broadcasting (fixed "Anonymous" bug).
  - Synchronized server version with major stability improvements.
  - Server version: 1.0.1.47

## [1.0.1.46] - 2026-04-25
- **Server: Online Status Reliability Fix**
  - Fixed a race condition where users would appear as "Anonymous" in the online list during connection.
  - Ensured `UpdateName` is called before the first `broadcastOnlineUsers` signal.
  - Users now correctly see each other online immediately after authentication.
  - Server version: 1.0.1.46

## [1.0.1.45] - 2026-04-25
- **Server: Real-time Presence and Admin Permissions**
  - Implemented real-time online status broadcasting to all connected clients.
  - Added group administration logic: creators are now stored and verified for moderation.
  - Restricted message deletion and participant management based on admin roles.
  - Added new gRPC signals for immediate UI synchronization.
  - Server version: 1.0.1.45

## [1.0.1.44] - 2026-04-25
- **Server: Group Admin and Permission Fixes**
  - Updated `CreateGroupChat` to properly store the creator as the group admin
  - Fixed compilation error in `server.go` due to missing arguments in `CreateChat` call
  - Server version: 1.0.1.44

## [1.0.1.43] - 2026-04-25
- **Server: Message Deduplication & UX Improvements**
  - Added message deduplication logic to prevent double-posting from source apps (e.g. Google Photos)
  - Full integration of Custom Themes management and storage
  - Dedicated APK distribution server on port 8081
  - Automated deployment script for Mac-to-Linux cross-compilation
  - Improved logging with username context for connections/disconnections
  - Server version: 1.0.1.43

## [1.0.1.42] - 2026-04-24
- **Server: Push Notification Room Navigation**
  - Added `room_id` to push notification data payload
  - Updated `sendPushNotification` to accept and include room_id parameter
  - Enables direct chat navigation when clicking on push notifications
  - Server version: 1.0.1.42

## [1.0.1.41] - 2026-04-24
- **Server: Reaction Foreign Key Constraint Fix**
  - Fixed foreign key constraint violation in `SetReaction` when message doesn't exist
  - Added message existence check before inserting reaction to prevent "reactions_message_id_fkey" error
  - Server version: 1.0.1.41

## [1.0.1.40] - 2026-04-24
- **Server: SQL NULL Handling Fix**
  - Fixed scanning error in `GetUserChats` when `last_message_time` is NULL for empty chats
  - Now uses `sql.NullTime` to properly handle NULL values from MAX() subquery
  - Resolves "unsupported Scan, storing driver.Value type <nil> into type *time.Time" error
  - Server version: 1.0.1.40

## [1.0.1.39] - 2026-04-24
- **Android: FCM Push Notifications Improvements**
  - **Token Registration Fixes:**
    - Added automatic token registration in `onNewToken` for token updates
    - Added delay in token registration to ensure username is loaded
    - Fixed token registration after app reinstallation
  - **Push Notification Logic:**
    - Changed to send push notifications to all users in room (not just offline)
    - This ensures background users receive notifications even with active gRPC connection
    - Combined notification + data payload for better compatibility
  - **Notification History:**
    - Added "Test Notification" button for local testing
    - Fixed notification channel creation for test notifications
    - FCM token display with copy functionality
  - **Server Updates:**
    - Updated Firebase credentials file path
    - Simplified push notification logic (send to all room participants)
    - Server version: 1.0.1.32
  - **Android Version:** 1.0.1.39
  - **Bug Fixes:**
    - Fixed "Requested entity was not found" error after app reinstallation
    - Fixed Context import in LavenderMessagingService

## [1.0.1.38] - 2026-04-23
- **Android: FCM Push Notifications**
  - **Firebase Cloud Messaging Integration:**
    - Added Firebase Admin SDK to server for push notification delivery
    - Implemented FCM token registration on Android client
    - Added POST_NOTIFICATIONS permission request for Android 13+
    - Real-time push notifications for offline users
  - **Notification History:**
    - Added NotificationHistory class to track received notifications
    - New "Notification History" menu item in ChatListActivity
    - View last 20 notifications with timestamp, title, body, and sender
    - Clear notification history option
  - **Server Updates:**
    - Firebase Admin SDK initialization with service account credentials
    - Real push notification sending via Firebase Messaging API
    - Automatic push notifications to offline users in chat rooms
    - Server version: 1.0.1.31
  - **Android Version:** 1.0.1.38
  - **Bug Fixes:**
    - Fixed inflate calls in ChatListActivity to prevent IllegalStateException
    - Fixed chat deletion dialog inflation crash

## [1.0.1.37] - 2026-04-23
- **Android: Profile Editing Redesign**
  - **New EditProfileActivity:**
    - Replaced profile editing dialog with dedicated activity
    - Added MaterialToolbar with "Редактировать профиль" title
    - Russian localization for all labels
    - Buttons without CAPS (textAllCaps="false")
  - **Profile Features:**
    - Edit bio (Кратко о себе) - multi-line text field with 4 line limit
    - Change username via dialog
    - Change password via dialog with old/new password fields
    - Change avatar with automatic ChatListActivity refresh
    - Delete profile with confirmation
  - **Database Updates:**
    - Added `bio` column to users table (TEXT)
    - Added `status` column to users table (VARCHAR(255))
    - Fixed `UpdateProfile` method to save bio and status
    - Fixed `GetUserProfile` method to return bio and status
  - **UI Improvements:**
    - ProfileActivity now reloads data on resume to show updated profile
    - ChatListActivity refreshes avatar when returning from EditProfileActivity
    - Avatar cache updated after profile changes
  - **Android Version:** 1.0.1.37
  - **Server Version:** 1.0.1.37

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
