# Lavender Messenger - macOS Client Changelog

**Author:** Pavel Davydov (ferz)

## [0.1.5] - 2026-04-20
- **GetAllUsers RPC Implementation**
  - Updated showAllUsersList to use GetAllUsers RPC instead of GetClients
  - "Все" button now shows all registered users from database (not just online)
  - Build version: 0.1.5

## [0.1.4] - 2026-04-20
- **Toolbar UI Improvements**
  - Removed server address display from toolbar (shown in status after connection)
  - Added third button "Все" for showing all registered users
  - Renamed users button to "Онлайн" for clarity
  - Implemented showAllUsersList function (currently uses GetClients, needs GetAllUsers RPC)
  - Build version: 0.1.4

## [0.1.3] - 2026-04-20
- **Chat List and Users List Features**
  - Added "Chats" button in toolbar to show chat list
  - Added "Users" button in toolbar to show online users
  - Implemented showUsersList function to display online users
  - Implemented createDirectChat function to create direct chats with users
  - Implemented switchToChat function to switch between chats
  - Added global variables for chatBox and connectToServer
  - Build version: 0.1.3

## [0.1.2] - 2026-04-20
- **Message History and Room Support**
  - Added loadHistory function to retrieve message history from server
  - Implemented room_id support in messages
  - Fixed config loading order to check main.go directory first
  - Added server address display in toolbar (italic)
  - Updated toolbar layout with status indicator, status, and server address
  - Build version: 0.1.2

## [1.0.0] - 2026-04-20
- **Authentication and Chat List Support**
  - Added password field to login dialog
  - Added password persistence in config
  - Implemented GetChats RPC call for retrieving user's chats
  - Added chat list dialog for selecting chat room
  - Smart navigation: auto-open general chat if no chats exist
  - Password sent with auth/join message
  - Updated client version to 1.0.0 (build 0.1.1)
  - Synced version with Android client and server

## [0.9.1] - 2026-04-17
- **Current development version**
  - Updated project structure (moved to client/macos/)
  - Added emoji support with popup selector
  - Added server status monitoring with visual indicators
  - Enhanced theme management (light/dark themes)
  - User color customization
  - Configuration persistence

## [0.9.0] - 2026-04-16
- **Initial macOS Client release**
  - Basic messaging functionality
  - Configuration support
  - Theme management
  - Fyne-based GUI implementation
