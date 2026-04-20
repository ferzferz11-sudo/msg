# Lavender Messenger - macOS Client Changelog

**Author:** Pavel Davydov (ferz)

## [1.0.1.27] - 2026-04-20
- **Chat Message Avatar Display**
  - Added avatar emoji (👤) display in chat messages next to username
  - Automatic avatar loading for users when messages are received
  - Avatar cache integration for chat messages
  - Avatar loading for all users in message history
- **Chat List UI Enhancement**
  - Added participant avatars display on the right side of chat items
  - Added unread count badge on the left side of chat name with fixed indentation
  - Increased avatar size for better visibility
  - Added avatar cache for efficient loading
  - Added automatic current user avatar loading on login
  - Added default avatar (👤 emoji) for participants without custom avatars
  - Show all participant avatars in direct chats (not just current user)
  - Show remaining participant count when more than 3 avatars
  - Added GetUserAvatar RPC integration for avatar loading

## [1.0.1.26] - 2026-04-20
- **UI Improvements and Bug Fixes**
  - Increased main window size from 600x400 to 1200x800 for better chat list display
  - Increased chat list dialog size to 800x600
  - Added automatic dialog close when selecting a chat
  - Fixed duplicate general chat in list (added hasGeneral check)
  - Filtered system join messages (no longer displayed in chat)
  - Fixed message duplication on send (added message ID tracking)
  - Added visual display for message replies (quote with username and text)
  - Added visual display for message reactions (emoji + count)
  - Updated users button to show all users with online status indicators (green/gray)
  - Removed redundant "Все" button (functionality merged into "Пользователи")

## [1.0.0] - 2026-04-20
### Build 0.1.5
- **GetAllUsers RPC Implementation**
  - Updated showAllUsersList to use GetAllUsers RPC instead of GetClients
  - "Все" button now shows all registered users from database (not just online)

### Build 0.1.4
- **Toolbar UI Improvements**
  - Removed server address display from toolbar (shown in status after connection)
  - Added third button "Все" for showing all registered users
  - Renamed users button to "Онлайн" for clarity
  - Implemented showAllUsersList function (currently uses GetClients, needs GetAllUsers RPC)

### Build 0.1.3
- **Chat List and Users List Features**
  - Added "Chats" button in toolbar to show chat list
  - Added "Users" button in toolbar to show online users
  - Implemented showUsersList function to display online users
  - Implemented createDirectChat function to create direct chats with users
  - Implemented switchToChat function to switch between chats
  - Added global variables for chatBox and connectToServer

### Build 0.1.2
- **Message History and Room Support**
  - Added loadHistory function to retrieve message history from server
  - Implemented room_id support in messages
  - Fixed config loading order to check main.go directory first
  - Added server address display in toolbar (italic)
  - Updated toolbar layout with status indicator, status, and server address

### Build 0.1.1
- **Authentication and Chat List Support**
  - Added password field to login dialog
  - Added password persistence in config
  - Implemented GetChats RPC call for retrieving user's chats
  - Added chat list dialog for selecting chat room
  - Smart navigation: auto-open general chat if no chats exist
  - Password sent with auth/join message
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
