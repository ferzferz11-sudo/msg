# Lavender Messenger — Client Integration Guide

**Server:** v1.3.0.31 | **Protocol:** gRPC + Protocol Buffers | **Date:** 2026-06-28

This document covers everything a client needs to integrate with the Lavender Messenger server. Platform-agnostic — applies to Android, iOS, Web, Desktop, or any gRPC-capable client.

---

## Server Endpoints

| Environment | gRPC | HTTP | Logs |
|-------------|------|------|------|
| **Prod** | `13.140.25.249:50051` | `13.140.25.249:8082` | — |
| **Dev** | `13.140.25.249:50052` | `13.140.25.249:8083` | `http://13.140.25.249/server-logs-dev` |

---

## Capability Negotiation

On startup, call `GET /info` to discover available services:

```
GET http://<host>:<port>/info
```

Response:
```json
{
  "version": "1.3.0.27",
  "time": "2026-06-23T22:00:00Z",
  "services": {
    "auth": "2.0",
    "chat": "2.0",
    "profile": "2.0",
    "ai": "2.0",
    "files": "1.0",
    "push": "1.0"
  }
}
```

Use these versions to decide which API paths to use:
- `auth >= "2.0"` → use `SignInV2` / `SignUpV2` (JWT tokens)
- `profile >= "2.0"` → use `ProfileService` v2 (separate gRPC service)
- `ai >= "2.0"` → use `ChatWithAIV2`

---

## Authentication (AuthService)

### Flow

```
1. SignInV2(username, password, device) → access_token + refresh_token
2. Every gRPC call: metadata["authorization"] = "Bearer <access_token>"
3. When access_token expires → RefreshToken(refresh_token) → new tokens
4. Refresh token rotation: each refresh invalidates the old refresh_token
```

### RPCs

#### SignInV2

```protobuf
message SignInRequestV2 {
  string username = 1;
  string password = 2;
  DeviceInfo device = 3;       // optional device info
  string client_version = 4;   // e.g. "1.3.0" from BuildConfig
}

message DeviceInfo {
  string device_id = 1;        // unique device identifier
  string device_name = 2;      // display name, e.g. "Samsung Galaxy S24"
  string client_version = 3;
  Timestamp last_seen_at = 4;
  string ip_address = 5;
}

message AuthResponseV2 {
  bool success = 1;
  string message = 2;
  string access_token = 3;     // JWT, valid 15 minutes
  string refresh_token = 4;    // JWT, valid 30 days
  int64 access_expires_at = 5; // unix timestamp (seconds)
  int64 refresh_expires_at = 6;
  User user = 7;               // { id, username, avatar_url }
}
```

#### SignUpV2

```protobuf
message SignUpRequestV2 {
  string username = 1;
  string password = 2;
  string email = 3;
  DeviceInfo device = 4;
  string client_version = 5;
}
// Returns: AuthResponseV2 (same as SignInV2)
```

#### RefreshToken

```protobuf
message RefreshTokenRequest {
  string refresh_token = 1;
}

message RefreshTokenResponse {
  string access_token = 1;
  string refresh_token = 2;    // new refresh token (old is invalidated)
  int64 access_expires_at = 3;
  int64 refresh_expires_at = 4;
}
```

#### SignOut

```protobuf
message SignOutRequest {
  string refresh_token = 1;
  bool all_devices = 2;        // true = invalidate all refresh tokens
}
// Returns: AuthResponse (success, message)
```

#### RevokeDevice

```protobuf
message RevokeDeviceRequest {
  string device_id = 1;
}
// Returns: AuthResponse
```

### Token Requirements

- Access token: JWT, HS256, expires in **15 minutes**
- Refresh token: JWT, HS256, expires in **30 days**
- `JWT_SECRET` must be ≥ 32 bytes (enforced at server startup)
- Refresh token rotation: every `RefreshToken` call returns a new refresh token and invalidates the old one

---

## Chat Stream (Bidirectional gRPC)

The Chat stream is the core real-time connection. It handles messaging, typing indicators, presence, and server commands.

### Connection

```protobuf
rpc Chat(stream Message) returns (stream Message);
```

### Auth on Connect

**First message** must contain your JWT access token:

```protobuf
message Message {
  string jwt_token = 26;      // JWT access token (first message only)
  string room_id = 10;        // chat ID to join
  string user_id = 23;        // your UUID (from AuthResponseV2.user.id)
  string client_version = 15; // your app version
  string device_id = 21;      // your device ID
  string device_name = 22;    // your device name
  // ... other fields empty in first message
}
```

The server validates the JWT, extracts `user_id` and `username` from it, and registers the stream.

### Receiving Messages

After auth, the server streams `Message` objects to you:

```protobuf
message Message {
  string id = 1;              // message UUID
  string user = 2;            // sender username
  string text = 3;            // message text (plaintext for non-E2EE)
  Timestamp created_at = 4;
  repeated Reaction reactions = 5;
  string replied_to_message_id = 7;
  string replied_to_user = 8;
  string replied_to_text = 9;
  string room_id = 10;
  bool is_read = 11;
  string avatar_url = 12;
  string image_url = 13;      // single image URL
  repeated string image_urls = 20; // multiple images
  bool edited = 14;
  string user_id = 23;        // sender UUID
  string voice_url = 17;      // voice message URL
  int32 duration = 18;        // voice message duration (seconds)
  bool is_e2ee = 24;          // true if E2EE message
  string e2ee_payload = 25;   // base64-encoded encrypted payload
}
```

### Sending Messages

Send a `Message` with `room_id` and content:

```protobuf
// Text message
Message {
  room_id: "chat-uuid",
  text: "Hello!",
  user_id: "your-uuid"
}

// Image message
Message {
  room_id: "chat-uuid",
  text: "",
  image_url: "http://host:8082/images/<hash>.jpg",
  user_id: "your-uuid"
}

// Voice message
Message {
  room_id: "chat-uuid",
  voice_url: "http://host:8082/audio/<hash>.m4a",
  duration: 12,
  user_id: "your-uuid"
}

// Reply
Message {
  room_id: "chat-uuid",
  text: "Reply text",
  replied_to_message_id: "original-msg-id",
  replied_to_user: "original-sender",
  replied_to_text: "Original message preview",
  user_id: "your-uuid"
}
```

### System Messages (ChatV2)

ChatV2 stream receives system messages via `ChatV2System`:

| Type | Message | Meaning |
|------|---------|---------|
| `AUTH_FAILED` | `"invalid token"` | Authentication failed |
| `AUTH_REQUIRED` | `"send jwt_token first"` | No auth token provided |
| `SERVER_SHUTTINGDOWN` | `""` | Server is about to restart |
| `ONLINE_USERS_UPDATE` | `["uuid1","uuid2",...]` | JSON array of online user IDs |
| `REACTION_V2` | `"messageId\|reactionsJSON"` | Reaction update (pipe-separated) |

**REACTION_V2 parsing:**
```kotlin
val parts = systemMessage.split("|", limit = 2)
val messageId = parts[0]
val reactions = JSON.parseObject(parts[1]) // {"userId":"emoji",...}
```

### Typing Indicator

```protobuf
rpc Typing(stream TypingRequest) returns (stream TypingSignal);
```

Separate bidirectional stream for typing indicators.

### Call Session (WebRTC Signaling)

```protobuf
rpc CallSession(stream CallMessage) returns (stream CallMessage);
```

Separate bidirectional stream for WebRTC signaling (SDP, ICE, accept, reject, hangup).

---

## Chat Management

### GetChatsV2 (Primary Chat List)

```protobuf
message GetChatsRequest {
  string user_id = 2;         // your UUID
  int32 limit = 3;            // max chats per page (0 = default 100)
  string filter = 5;          // "all" | "pinned" | "archived" | "muted"
  string cursor = 6;          // cursor from previous response (omit for first page)
}

message GetChatsResponse {
  repeated ChatInfo chats = 1;
  string next_cursor = 2;     // pass as `cursor` in next request (empty = no more)
  bool has_more = 3;
}

message ChatInfo {
  string id = 1;
  string name = 2;
  string type = 3;            // "direct" | "group" | "secret"
  string participants = 4;    // JSON array of usernames
  Timestamp created_at = 5;
  int32 unread_count = 6;
  Timestamp last_message_time = 7;
  string last_message_text = 9;
  string avatar_url = 10;
  string full_avatar_url = 11;
  string last_message_username = 12;
  bool last_message_has_image = 13;
  bool is_secret = 15;
}
```

**Pagination:** Use cursor-based pagination. First request: `cursor = ""`. Response includes `next_cursor` for the next page. Stop when `has_more = false`.

**Filtering:** Use `filter` to get only pinned, archived, or muted chats.

### CreateDirectChat

```protobuf
message CreateDirectChatRequest {
  string user1_id = 3;        // your UUID
  string user2_id = 4;        // other user's UUID
}
// Returns: CreateDirectChatResponse { chat_id, success }
```

### CreateGroupChat

```protobuf
message CreateGroupChatRequest {
  string name = 1;
  repeated string participant_ids = 5;  // UUIDs of participants
  string creator_id = 4;               // your UUID
}
// Returns: CreateGroupChatResponse { chat_id, success }
```

### DeleteChat

```protobuf
message DeleteChatRequest {
  string chat_id = 1;
  string requester_user_id = 2;  // your UUID
}
// Returns: DeleteChatResponse { success }
```

### Chat Participants

```protobuf
// Add participant
message AddParticipantRequest {
  string chat_id = 1;
  string user_id = 2;              // your UUID (must be chat creator/admin)
  string new_participant_id = 3;
}

// Remove participant
message RemoveParticipantRequest {
  string chat_id = 1;
  string user_id = 2;
  string participant_id = 3;
}
```

### Pin / Mute / Archive

```protobuf
rpc PinChat(PinChatRequest) returns (PinChatResponse);
rpc UnPinChat(UnPinChatRequest) returns (UnPinChatResponse);
rpc ArchiveChat(ArchiveChatRequest) returns (ArchiveChatResponse);
rpc UnarchiveChat(UnarchiveChatRequest) returns (UnarchiveChatResponse);
rpc SetMutedChat(SetMutedChatRequest) returns (SetMutedChatResponse);
rpc GetMutedChats(GetMutedChatsRequest) returns (GetMutedChatsResponse);
```

### ChatList Version (Cache Invalidation)

```protobuf
rpc GetChatListVersion(GetChatListVersionRequest) returns (GetChatListVersionResponse);

message GetChatListVersionRequest {
  string user_id = 1;
}
message GetChatListVersionResponse {
  int64 version = 1;  // increment on any chat list change
}
```

Use this to check if the client's cached chat list is stale.

### Search Chats

```protobuf
message SearchChatsRequest {
  string user_id = 1;
  string query = 2;
  int32 limit = 3;
}
// Returns: SearchChatsResponse { chats: ChatInfo[] }
```

---

## Messages

### GetHistory

```protobuf
message GetHistoryRequest {
  int32 limit = 1;       // max messages (default 50)
  string room = 2;       // chat ID
}
// Returns: GetHistoryResponse { messages: Message[] }
```

### Edit / Delete

```protobuf
rpc EditMessage(EditMessageRequest) returns (EditMessageResponse);
rpc DeleteMessages(DeleteMessagesRequest) returns (DeleteMessagesResponse);
```

### Reactions

```protobuf
rpc SetReaction(ReactionRequest) returns (ReactionResponse);

message ReactionRequest {
  string message_id = 1;
  Reaction reaction = 2;    // { user, emoji }
}
```

### Pin Messages

```protobuf
rpc PinMessage(PinMessageRequest) returns (PinMessageResponse);
rpc UnPinMessage(UnPinMessageRequest) returns (UnPinMessageResponse);
rpc GetPinnedMessages(GetPinnedMessagesRequest) returns (GetPinnedMessagesResponse);
```

### Mark Read

```protobuf
rpc MarkRead(MarkReadRequest) returns (MarkReadResponse);

message MarkReadRequest {
  string room_id = 1;
  string message_id = 2;    // last read message ID
}
```

---

## Messages v2

Messages v2 provides a lightweight, type-safe message system with cursor-based pagination, JSONB reactions, and a ChatV2 bidirectional stream.

### Key differences from v1

| Feature | v1 Message | v2 MessageV2 |
|---------|-----------|-------------|
| Fields | 26 (God Object) | ~12 (lean) |
| Sender | `user` (username) | `sender_id` (UUID) |
| Content | flat fields | `oneof content` (text/media/reply) |
| Reactions | N+1 queries | JSONB in message |
| Pagination | OFFSET | Cursor-based |
| Auth | in-message (jwt_token, device_id) | interceptor-only |
| Avatar | denormalized | resolved at read time |

### ChatV2 Stream (bidirectional)

```protobuf
rpc ChatV2(stream ChatV2Message) returns (stream ChatV2Message);

message ChatV2Message {
  string jwt_token = 1;     // first message only (auth)
  string room_id = 2;       // room to join

  oneof payload {
    MessageV2 message = 10;
    ChatV2Typing typing = 11;
    ChatV2System system = 12;
  }
}

message ChatV2Typing {
  bool is_typing = 1;
}

message ChatV2System {
  string type = 1;    // "AUTH_FAILED", "AUTH_REQUIRED", "SERVER_SHUTTINGDOWN", "ONLINE_USERS_UPDATE", "REACTION_V2"
  string message = 2;
}
```

**Auth flow:**
1. Open ChatV2 stream
2. First message: `{ jwt_token: "Bearer ...", room_id: "chat-uuid" }`
3. Server validates JWT, registers stream in room
4. Send/receive `MessageV2` payloads

### MessageV2

```protobuf
message MessageV2 {
  string id = 1;           // UUID
  string room_id = 2;
  string sender_id = 3;    // UUID (not username)

  oneof content {
    string text = 10;
    MessageMedia media = 11;
    MessageReply reply = 12;
  }

  bool edited = 20;
  bool is_read = 21;
  google.protobuf.Timestamp created_at = 22;
  bytes reactions = 23;     // JSON: {"uuid":"emoji",...}

  bool is_e2ee = 30;
  string e2ee_payload = 31;
}

message MessageMedia {
  string type = 1;       // "image" | "voice" | "file"
  string url = 2;
  repeated string urls = 3; // gallery
  int32 duration = 4;    // voice duration
}

message MessageReply {
  string message_id = 1;
  string preview = 2;    // text preview of original
}
```

### GetHistoryV2 (cursor pagination)

```protobuf
rpc GetHistoryV2(GetHistoryV2Request) returns (GetHistoryV2Response);

message GetHistoryV2Request {
  string room_id = 1;
  int32 limit = 2;       // default 50, max 200
  string cursor = 3;     // from previous response
}

message GetHistoryV2Response {
  repeated MessageV2 messages = 1;
  string next_cursor = 2;  // empty = no more
  bool has_more = 3;
}
```

**Pagination:**
1. First request: `cursor = ""`
2. Response includes `next_cursor`
3. Pass `next_cursor` as `cursor` in next request
4. Stop when `has_more = false`

### SendMessageV2

```protobuf
rpc SendMessageV2(SendMessageV2Request) returns (SendMessageV2Response);

message SendMessageV2Request {
  string room_id = 1;
  oneof content {
    string text = 2;
    MessageMedia media = 3;
  }
  string reply_to_id = 4;
  bool is_e2ee = 5;
  string e2ee_payload = 6;
}

message SendMessageV2Response {
  MessageV2 message = 1;
  bool success = 2;
  string error = 3;
}
```

### EditMessageV2

```protobuf
rpc EditMessageV2(EditMessageV2Request) returns (EditMessageV2Response);

message EditMessageV2Request {
  string message_id = 1;
  string text = 2;
}
```

### DeleteMessageV2

```protobuf
rpc DeleteMessageV2(DeleteMessageV2Request) returns (DeleteMessageV2Response);

message DeleteMessageV2Request {
  repeated string message_ids = 1;
  string requester_user_id = 2;
}
```

### SetReactionV2

```protobuf
rpc SetReactionV2(SetReactionV2Request) returns (SetReactionV2Response);

message SetReactionV2Request {
  string message_id = 1;
  string emoji = 2;       // empty = remove reaction
}

message SetReactionV2Response {
  bool success = 1;
  bytes reactions = 2;    // updated reactions JSON
}
```

### SearchMessages

Search messages inside a specific chat or across all user's chats.

```protobuf
rpc SearchMessages(SearchMessagesRequest) returns (SearchMessagesResponse);

message SearchMessagesRequest {
  string room_id = 1;       // optional: limit to specific chat (empty = all user's chats)
  string query = 2;         // search keyword (required, non-empty)
  int32 limit = 3;          // max results (default 20, max 100)
}

message SearchMessagesResponse {
  repeated SearchResult messages = 1;
}

message SearchResult {
  string message_id = 1;
  string room_id = 2;
  string username = 3;      // sender
  string preview = 4;       // text snippet (first 200 chars)
  string created_at = 5;    // ISO 8601
}
```

**Usage:**
- Single chat search: set `room_id` to the chat ID
- Cross-chat search: leave `room_id` empty (searches all chats where user is participant)
- Results are ordered by `created_at` DESC (newest first)

---

## Profile (ProfileService v2)

**Important:** ProfileService is a separate gRPC service (not part of ChatService). Connect to the same gRPC endpoint.

### RPCs

```protobuf
service ProfileService {
  rpc GetProfile(GetProfileRequest) returns (GetProfileResponse);
  rpc UpdateProfile(UpdateProfileV2Request) returns (UpdateProfileV2Response);
  rpc UpdateAvatar(UpdateAvatarV2Request) returns (UpdateAvatarV2Response);
  rpc DeleteProfile(DeleteProfileV2Request) returns (DeleteProfileV2Response);
  rpc GetUserSettings(GetUserSettingsRequest) returns (GetUserSettingsResponse);
  rpc UpdateUserSettings(UpdateUserSettingsRequest) returns (UpdateUserSettingsResponse);
}
```

### GetProfile

Returns the authenticated user's profile (user_id from JWT, no request fields needed):

```protobuf
message GetProfileResponse {
  string user_id = 1;
  string username = 2;
  string email = 3;
  string avatar_url = 4;
  string full_avatar_url = 5;
  string bio = 6;
  string status = 7;
  string locale = 8;
  bool is_super_admin = 9;
  string created_at = 10;
}
```

### UpdateProfile

```protobuf
message UpdateProfileV2Request {
  string username = 1;    // optional: change username
  string bio = 2;
  string status = 3;
  string locale = 4;      // "en", "ru", etc.
}
// Returns: UpdateProfileV2Response { success, message, profile: GetProfileResponse }
```

### UpdateAvatar

```protobuf
message UpdateAvatarV2Request {
  string avatar_url = 1;
  string full_avatar_url = 2;
}
// Returns: UpdateAvatarV2Response { success, avatar_url, full_avatar_url }
```

### DeleteProfile

```protobuf
message DeleteProfileV2Request {
  string password = 1;    // required: confirm with password
}
// Returns: DeleteProfileV2Response { success, message }
```

**Warning:** Deleting a profile permanently removes all user data including AI chats, themes, contacts, favorites, and pins. Messages in group chats are preserved (shared history).

### User Settings

```protobuf
message GetUserSettingsRequest {}  // empty — user_id from JWT

message GetUserSettingsResponse {
  string locale = 1;
  string theme_id = 2;
  bool push_enabled = 3;
  map<string, string> custom = 4;  // extensible key-value store
}

message UpdateUserSettingsRequest {
  string locale = 1;
  string theme_id = 2;
  bool push_enabled = 3;
  map<string, string> custom = 4;
}
```

---

## Users

### GetAllUsers

```protobuf
rpc GetAllUsers(GetAllUsersRequest) returns (GetAllUsersResponse);

message GetAllUsersResponse {
  repeated UserInfo users = 1;
  Timestamp server_time = 2;
}

message UserInfo {
  string username = 1;
  string avatar_url = 2;
  string last_client_version = 3;   // e.g. "1.3.0"
  Timestamp last_seen_at = 4;
  string email = 5;
  string user_id = 6;               // UUID
  bool is_super_admin = 7;
}
```

### GetAdminUserList (Admin Panel)

Extended user list with last message, chat count, and real-time online status. Admin-only (requires `is_super_admin`).

```protobuf
rpc GetAdminUserList(GetAdminUserListRequest) returns (GetAdminUserListResponse);

message GetAdminUserListRequest {
  string query = 1;       // search by username/email (empty = all)
  string cursor = 2;      // cursor-based pagination (from previous response)
  int32 limit = 3;        // max results (default 50, max 200)
  string sort_by = 4;     // "last_message" (default), "last_seen", "username"
}

message GetAdminUserListResponse {
  repeated AdminUserInfo users = 1;
  string next_cursor = 2;  // pass as cursor in next request
  bool has_more = 3;
  Timestamp server_time = 4;
}

message AdminUserInfo {
  string user_id = 1;
  string username = 2;
  string avatar_url = 3;
  string full_avatar_url = 4;
  string email = 5;
  bool is_super_admin = 6;
  string last_client_version = 7;
  Timestamp last_seen_at = 8;
  bool is_online = 9;               // real-time from hub
  string last_message_text = 10;    // truncated to 100 chars
  Timestamp last_message_time = 11;
  string last_message_username = 12;
  int32 chat_count = 13;
}
```

**Pagination:** Cursor-based. First request: `cursor = ""`. Response includes `next_cursor`. Stop when `has_more = false`.

**Online status:** `is_online` is real-time from the hub (WebSocket/gRPC connection status), not from `last_seen_at` (which may be stale).

### Other User RPCs

```protobuf
rpc GetUserProfile(GetUserProfileRequest) returns (GetUserProfileResponse);
rpc GetUserAvatar(GetUserAvatarRequest) returns (GetUserAvatarResponse);
rpc GetUserId(GetUserIdRequest) returns (GetUserIdResponse);  // username → UUID
```

---

## Contacts

```protobuf
rpc AddContact(AddContactRequest) returns (AddContactResponse);
rpc RemoveContact(RemoveContactRequest) returns (RemoveContactResponse);
rpc GetContacts(GetContactsRequest) returns (GetContactsResponse);
```

---

## Favorites

```protobuf
rpc AddFavorite(AddFavoriteRequest) returns (AddFavoriteResponse);
rpc RemoveFavorite(RemoveFavoriteRequest) returns (RemoveFavoriteResponse);
rpc GetFavorites(GetFavoritesRequest) returns (GetFavoritesResponse);
```

---

## Drafts

```protobuf
rpc SaveDraft(SaveDraftRequest) returns (SaveDraftResponse);
rpc GetDraft(GetDraftRequest) returns (GetDraftResponse);
rpc DeleteDraft(DeleteDraftRequest) returns (DeleteDraftResponse);
```

---

## Themes

```protobuf
rpc GetThemes(GetThemesRequest) returns (GetThemesResponse);
rpc SaveTheme(SaveThemeRequest) returns (SaveThemeResponse);
rpc SetCurrentTheme(SetCurrentThemeRequest) returns (SetCurrentThemeResponse);
rpc DeleteTheme(DeleteThemeRequest) returns (DeleteThemeResponse);
```

---

## Push Notifications (FCM)

```protobuf
rpc RegisterToken(TokenRequest) returns (TokenResponse);

message TokenRequest {
  string user_id = 1;
  string token = 2;        // FCM registration token
  string platform = 3;     // "android", "ios", "web"
  string device_id = 4;
}
```

---

## Secret Chats (E2EE)

```protobuf
rpc CreateSecretChat(CreateSecretChatRequest) returns (CreateSecretChatResponse);
rpc ExchangeSecretKey(ExchangeSecretKeyRequest) returns (ExchangeSecretKeyResponse);
rpc GetSecretChatKey(GetSecretChatKeyRequest) returns (GetSecretChatKeyResponse);
```

E2EE messages are encrypted client-side. The server stores only the encrypted payload. When sending, set `is_e2ee = true` and `e2ee_payload` with the base64-encoded ciphertext.

---

## Password Reset

```protobuf
rpc RequestPasswordReset(RequestPasswordResetRequest) returns (RequestPasswordResetResponse);
rpc ResetPassword(ResetPasswordRequest) returns (ResetPasswordResponse);
```

---

## AI Services v2

### ChatWithAIV2 (Streaming)

```protobuf
service ChatService {
  rpc ChatWithAIV2(ChatWithAIV2Request) returns (stream ChatWithAIV2Response);
}
```

```protobuf
message ChatWithAIV2Request {
  string session_id = 1;     // empty = create new chat
  string message = 2;
  repeated bytes images = 3; // base64 images for multimodal
  string agent_id = 4;       // force specific agent (optional)
  repeated ToolCallV2 tool_calls = 5;  // tool execution results (for agentic loop)
}

message ChatWithAIV2Response {
  string token = 1;          // streaming token
  bool finished = 2;         // true = response complete
  string error = 3;          // error message (if any)
  string agent_id = 4;       // which agent answered (now populated in every token)
  string agent_name = 5;     // display name (now populated in every token)
  repeated ToolCallRequestV2 tool_calls = 6;  // tool execution requests
  bool has_rag_context = 7;  // RAG was used
  string model_used = 8;     // model name
  int32 token_count = 9;     // token count
  string image_url = 10;     // image URL (for Reve image generation)
}
```

### Tool Calling Flow

When the agent needs to call a tool, the server sends `tool_calls` in the response. The client must:

1. Receive `ChatWithAIV2Response` with `tool_calls` (and `finished = false`)
2. Execute each tool (locally or via API)
3. Send back results via `ChatWithAIV2Request` with `tool_calls` containing results
4. Continue receiving tokens

```
Client → ChatWithAIV2Request { message: "What's the weather?" }
Server → ChatWithAIV2Response { tool_calls: [{id, name: "web_search", args}], finished: false }
Client → ChatWithAIV2Request { tool_calls: [{id, name: "web_search", result: "Sunny, 22°C"}] }
Server → ChatWithAIV2Response { token: "The weather is sunny...", finished: false }
Server → ChatWithAIV2Response { finished: true }
```

### Built-in Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `search_messages` | Search chat messages | `query`, `chat_id?`, `limit?` |
| `search_users` | Search users | `query`, `limit?` |
| `web_search` | Web search (DuckDuckGo) | `query` |
| `web_fetch` | Fetch URL content | `url`, `max_chars?` |
| `get_chat_info` | Chat metadata | `chat_id` |
| `query_database` | SQL SELECT (admin only) | `query` |

### Agent Management

```protobuf
rpc CreateAIAgent(CreateAIAgentRequest) returns (CreateAIAgentResponse);
rpc UpdateAIAgent(UpdateAIAgentRequest) returns (UpdateAIAgentResponse);
rpc DeleteAIAgent(DeleteAIAgentRequest) returns (DeleteAIAgentResponse);
rpc GetAIAgent(GetAIAgentRequest) returns (GetAIAgentResponse);
rpc ListAIAgents(ListAIAgentsRequest) returns (ListAIAgentsResponse);
rpc CloneAIAgent(CloneAIAgentRequest) returns (CloneAIAgentResponse);
rpc ListAITools(ListAIToolsRequest) returns (ListAIToolsResponse);
```

### AI Marketplace

```protobuf
rpc RateAIAgent(RateAIAgentRequest) returns (RateAIAgentResponse);
rpc GetAIAgentReviews(GetAIAgentReviewsRequest) returns (GetAIAgentReviewsResponse);
rpc ListMarketplaceAgents(ListMarketplaceAgentsRequest) returns (ListMarketplaceAgentsResponse);
rpc GetAIAgentStats(GetAIAgentStatsRequest) returns (GetAIAgentStatsResponse);
rpc ShareAIAgent(ShareAIAgentRequest) returns (ShareAIAgentResponse);
rpc InstallAIAgent(InstallAIAgentRequest) returns (InstallAIAgentResponse);
rpc GetAIUsageStats(GetAIUsageStatsRequest) returns (GetAIUsageStatsResponse);
```

### AI Chat v2 History & List

#### GetAIV2ChatHistory

Load chat history with agent metadata per message.

```protobuf
rpc GetAIV2ChatHistory(GetAIV2ChatHistoryRequest) returns (GetAIV2ChatHistoryResponse);

message GetAIV2ChatHistoryRequest {
  string session_id = 1;  // AI chat session ID
  int32 limit = 2;        // max messages (default 50)
}

message AIV2ChatMessage {
  int64 id = 1;
  string chat_id = 2;
  string role = 3;         // "user" or "assistant"
  string content = 4;
  string agent_id = 5;     // which agent produced this message
  int32 token_count = 6;
  string model_used = 7;
  string created_at = 8;   // ISO 8601
}

message GetAIV2ChatHistoryResponse {
  repeated AIV2ChatMessage messages = 1;
}
```

#### ListAIV2Chats

List all AI v2 chats for the current user.

```protobuf
rpc ListAIV2Chats(ListAIV2ChatsRequest) returns (ListAIV2ChatsResponse);

message ListAIV2ChatsRequest {}

message AIV2ChatInfo {
  string id = 1;
  string name = 2;
  string chat_type = 3;   // "simple", "agent", "pipeline", "multi_agent"
  string agent_id = 4;
  string created_at = 5;  // ISO 8601
  string updated_at = 6;  // ISO 8601
}

message ListAIV2ChatsResponse {
  repeated AIV2ChatInfo chats = 1;
}
```

### Multi-Agent Chat (Client-Side Routing)

For group AI chats with multiple agents, the client sends separate `ChatWithAIV2` requests for each agent and aggregates responses:

```kotlin
// Client-side multi-agent routing
for (agentId in selectedAgentIds) {
    scope.launch {
        val stream = chatStub.chatWithAIV2(ChatWithAIV2Request {
            sessionId = sessionId
            message = userMessage
            this.agentId = agentId
            images = userImages
        })
        stream.collect { response ->
            // response.agent_id and response.agent_name identify the agent
            // UI renders responses from different agents with agent names/colors
        }
    }
}
```

**Each response token now includes `agent_id` and `agent_name`**, so the client can attribute tokens to the correct agent in the UI.

### AI Limits

| Parameter | Value |
|-----------|-------|
| Max tool calling iterations | 10 |
| Rate limit (default) | 10 req/min |
| Rate limit (free tier) | 20 req/hr |
| Max images per request | 5 |

### Preset Agents (Free via Server Key)

By default, all clients use the **server's OpenRouter API key** — no payment required. Preset agents use free `:free` models:

| ID | Name | Model | Tools | RAG | Use Case |
|----|------|-------|-------|-----|----------|
| `mimo` | MiMo | `mimo-auto` | ✅ | ✅ | AI assistant (own server) |
| `assistant` | Assistant | `meta-llama/llama-3.3-70b-instruct:free` | ✅ | ✅ | Universal assistant |
| `developer` | Developer | `qwen/qwen3-coder:free` | ✅ | ❌ | Code writing & debugging |
| `devops` | DevOps | `meta-llama/llama-3.3-70b-instruct:free` | ✅ | ❌ | Server & infra |
| `architect` | Architect | `nvidia/nemotron-3-super-120b-a12b:free` | ❌ | ❌ | System design |
| `writer` | Writer | `meta-llama/llama-3.3-70b-instruct:free` | ❌ | ❌ | Creative writing |
| `analyst` | Analyst | `qwen/qwen3-next-80b-a3b-instruct:free` | ✅ | ✅ | Data analysis |
| `translator` | Translator | `meta-llama/llama-3.3-70b-instruct:free` | ❌ | ❌ | Translation |
| `vision` | Vision | `google/gemma-4-26b-a4b-it:free` | ✅ | ❌ | Image analysis |
| `reve` | Reve Image | `reve-2.0` | ❌ | ❌ | AI image generation |

**Key resolution priority:**
1. User's own API key (set via `UpdateAIChatSettings`) → **unlocks paid models**
2. Agent's `provider_config.api_key` → agent-specific key
3. Server's `OPENROUTER_API_KEY` env var → **free tier (default)**

### AgentInfoV2 — provider_config

All agent RPCs (`CreateAIAgent`, `GetAIAgent`, `ListAIAgents`, `CloneAIAgent`, `ListMarketplaceAgents`) return `AgentInfoV2` which now includes `provider_config`:

```protobuf
message AgentInfoV2 {
  // ... fields 1-21 ...
  string share_code = 21;
  string provider_config = 22;  // JSON string
}
```

**`provider_config` examples:**

Preset agent:
```json
{"api_key_source": "server", "default_model": "reve-2.0"}
```

User agent with custom key:
```json
{"api_key": "sk-or-v1-...", "default_model": "anthropic/claude-sonnet-4"}
```

**Client usage:** Parse `provider_config` as JSON to read API keys, model overrides, and other provider-specific settings. Mask API keys in UI (`sk-...xxxx`).

### AI Chat Settings (Per-Session)

Each AI chat session can have its own API key and model override. This allows users to use their own OpenRouter key for paid models while defaulting to free models.

#### GetAIChatSettings

```protobuf
rpc GetAIChatSettings(GetAIChatSettingsRequest) returns (AIChatSettings);

message GetAIChatSettingsRequest {
  string session_id = 1;  // AI chat session ID
}

message AIChatSettings {
  string session_id = 1;
  string user_api_key = 2;        // user's OpenRouter key (hidden from response if empty)
  string model = 3;               // model override (empty = use agent default)
  bool is_using_custom_key = 4;   // true if user has set their own key
  int32 remaining = 5;            // rate limit remaining (current window)
  int32 limit = 6;                // rate limit max
  int32 window_seconds = 7;       // rate limit window size
}
```

#### UpdateAIChatSettings

```protobuf
rpc UpdateAIChatSettings(UpdateAIChatSettingsRequest) returns (UpdateAIChatSettingsResponse);

message UpdateAIChatSettingsRequest {
  string session_id = 1;  // AI chat session ID
  string api_key = 2;     // OpenRouter API key (empty = remove, use server key)
  string model = 3;       // Model override (empty = remove, use agent default)
}

message UpdateAIChatSettingsResponse {
  bool success = 1;
  string message = 2;
}
```

**Usage example:**
```
// Client wants to use their own key for paid models
UpdateAIChatSettings {
  session_id: "ai-chat-abc123"
  api_key: "sk-or-v1-..."
  model: "anthropic/claude-sonnet-4"
}
→ success: true

// Client wants to revert to free server key
UpdateAIChatSettings {
  session_id: "ai-chat-abc123"
  api_key: ""        // empty = remove user key
  model: ""          // empty = remove override
}
→ success: true
```

### Reve Image Generation

The `reve` preset agent generates images via the Reve API. Unlike text agents, it returns an `image_url` instead of streaming tokens.

**How it works:**
1. Client sends a message to agent `reve` via `ChatWithAIV2`
2. Server calls Reve API (`POST /v1/image/create`) with the prompt
3. Server decodes base64 image, uploads to `/upload-image`, gets a URL
4. Server sends final `ChatWithAIV2Response` with `image_url` set

**Client handling:**
```protobuf
// ChatWithAIV2Response with image
ChatWithAIV2Response {
  token: "Image generated",  // text description
  finished: true,
  image_url: "http://host:8082/images/abc123.png",
  agent_id: "reve",
  agent_name: "Reve Image"
}
```

**Important:** When `image_url` is non-empty, the client should display the image instead of (or in addition to) the text token.

**Reve API limitations:**
- Max image: 40MB, 33,554,432px (8192×4096)
- Input formats: WEBP, JPEG, PNG, GIF, TIFF, AVIF
- `test_time_scaling`: 1–15 (higher = better quality, more credits)
- Post-processing: upscale (1–4×), remove_background, fit_image

---

## HTTP Endpoints

All HTTP endpoints are on port 8082 (prod) or 8083 (dev).

### Public (No Auth)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check (returns version + status) |
| `GET` | `/info` | Service versions for capability negotiation |

### Upload (Requires JWT Bearer Token)

All upload endpoints require `Authorization: Bearer <access_token>` header.

| Method | Path | Content-Type | Description |
|--------|------|-------------|-------------|
| `POST` | `/upload-avatar` | `multipart/form-data` | Upload avatar (fields: `avatar` required, `avatar_full` optional) |
| `POST` | `/upload-image` | `multipart/form-data` | Upload image (field: `image`) |
| `POST` | `/upload-file` | `multipart/form-data` | Upload file (field: `file`) |
| `POST` | `/upload-background` | `multipart/form-data` | Upload background (field: `background`) |
| `POST` | `/upload-audio` | `multipart/form-data` | Upload audio (field: `audio`) |

**Allowed extensions:**
- Images (avatar, image, background): `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`
- Audio: `.m4a`, `.aac`, `.ogg`, `.mp3`, `.wav`
- Files: `.pdf`, `.doc`, `.docx`, `.xls`, `.xlsx`, `.ppt`, `.pptx`, `.txt`, `.csv`, `.json`, `.xml`, `.zip`, `.rar`, `.7z`, `.mp3`, `.mp4`, `.avi`, `.mov`, `.mkv`, `.webm`, `.m4a`, `.aac`, `.ogg`, `.wav`

**Response:** `{"url": "http://host:port/<prefix>/<hash>.<ext>"}`

### Static Files

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/avatars/<filename>` | View avatar |
| `GET` | `/images/<filename>` | View image |
| `GET` | `/files/<filename>` | View file |
| `GET` | `/background/<filename>` | View background |
| `GET` | `/audio/<filename>` | View audio |

### TURN Credentials (WebRTC)

```
GET /turn-credentials
Authorization: Bearer <access_token>
```

Returns TURN server credentials for WebRTC.

---

## Graceful Shutdown

The server supports graceful shutdown — clients receive a warning before disconnection.

### What Happens

1. Server receives SIGTERM (restart, deploy)
2. Sends `SERVER_SHUTTINGDOWN` to all connected Chat streams
3. Waits 2 seconds
4. Calls `GracefulStop()` — closes new connections, waits up to 30s for active RPCs

### Health During Shutdown

```
GET /health → 503 {"status":"shutting_down","version":"1.3.0.17"}
```

### Client Behavior

1. **Detect shutdown:** Check for `SERVER_SHUTTINGDOWN` system message in Chat stream
2. **Handle disconnection:** When you get `UNAVAILABLE` error, do NOT clear UI data
3. **Poll health:** Check `GET /health` before reconnecting
4. **Reconnect:** Exponential backoff (1s → 2s → 4s → ... → max 30s)
5. **Cache locally:** Store chat list in local database for offline display

```
Reconnection flow:
1. Receive SERVER_SHUTTINGDOWN → show "Reconnecting..."
2. Server closes connections → client gets UNAVAILABLE
3. Client shows cached data (NOT empty state)
4. Client polls /health → 503 → wait
5. New server starts → /health returns 200
6. Client reconnects → fetch fresh data
```

---

## Proto Files

| File | Service | Description |
|------|---------|-------------|
| `messenger.proto` | ChatService, AuthService, ProfileService | All main RPCs |
| `server.proto` | ServerService | Server management (admin) |
| `hermes_remote.proto` | HermesAgentService | Agent daemon communication |

### Proto Generation

```bash
PATH=$PATH:~/go/bin protoc \
  --go_out=./gen --go_opt=paths=source_relative \
  --go-grpc_out=./gen --go-grpc_opt=paths=source_relative \
  messenger.proto
```

---

## Error Handling

### gRPC Status Codes

| Code | Meaning | Client Action |
|------|---------|---------------|
| `OK` (0) | Success | Process response |
| `UNAVAILABLE` (14) | Server down / network issue | Retry with backoff |
| `UNAUTHENTICATED` (16) | Invalid/expired token | Refresh token, retry |
| `PERMISSION_DENIED` (7) | Not authorized for this action | Check permissions |
| `INVALID_ARGUMENT` (3) | Bad request parameters | Fix request |
| `NOT_FOUND` (5) | Resource not found | Show appropriate UI |

### Token Expiry Handling

```
1. Call fails with UNAUTHENTICATED
2. Call RefreshToken(refresh_token)
3. If RefreshToken succeeds → retry original call with new access_token
4. If RefreshToken fails → user must re-login (SignInV2)
```

---

## Integration Checklist

For a new client, implement in this order:

1. **Capability negotiation:** `GET /info` → determine which APIs to use
2. **Auth:** `SignInV2` → store tokens securely
3. **Token refresh:** Automatic refresh before expiry
4. **Chat stream:** Open bidirectional stream, send JWT as first message
5. **Chat list:** `GetChatsV2` with cursor pagination
6. **Messages:** `GetHistory`, send via Chat stream
7. **Profile:** `ProfileService.GetProfile`
8. **File uploads:** Upload with JWT auth, use returned URLs in messages
9. **Push notifications:** `RegisterToken` with FCM token
10. **AI chat:** `ChatWithAIV2` with streaming
11. **Graceful shutdown:** Handle `SERVER_SHUTTINGDOWN` and reconnect logic
