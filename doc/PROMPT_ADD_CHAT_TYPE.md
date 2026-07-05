# Prompt: Add `type` field to CreateGroupChatRequest

## Problem

When the Android client creates a conference chat, it sends `type = "conference"` in `CreateGroupChatRequest`, but the server ignores it because the proto doesn't have a `type` field. The server hardcodes `"group"` at `server_chats.go:103`.

**Evidence from server logs:**
```
CreateGroupChat: ttttt (Creator: ferz)
Group chat created: cfaaf788-4d2a-4081-952e-1f28d0d50bd7 (ttttt)
```
No conference type logged — always "group".

**Android client already sends `type`:**
- `MessengerProto.kt:294-301`: `CreateGroupChatRequestProto` has `type: String = "group"`
- `GrpcChatClient.kt:233`: `call.sendMessage(CreateGroupChatRequestProto(name, participants, creator, getUserId() ?: "", emptyList(), type))`
- `ChatListFABs.kt:403`: `GrpcClient.createGroupChat(topic, participants, username, "conference") { chatId ->`

## Required Changes

### 1. Proto: `messenger.proto`

Add `type` field to `CreateGroupChatRequest` (field 6):

```protobuf
message CreateGroupChatRequest {
  string name = 1;
  repeated string participants = 2;
  string creator = 3;
  string creator_id = 4;
  repeated string participant_ids = 5;
  string type = 6;  // "group" (default), "conference"
}
```

### 2. Regenerate Go code

```bash
cd /Users/paveld/LavenderMessenger-server
protoc --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative messenger.proto
```

### 3. Server handler: `server_chats.go:103`

Change:
```go
err = s.db.CreateChat(chatID, req.Name, "group", string(participantsJSON), creator, req.CreatorId)
```

To:
```go
chatType := req.Type
if chatType == "" {
    chatType = "group"
}
err = s.db.CreateChat(chatID, req.Name, chatType, string(participantsJSON), creator, req.CreatorId)
```

And update the log at line 108:
```go
logger.Infof("Group chat created: %s (%s) type=%s", chatID, req.Name, chatType)
```

### 4. Also check: `db_chats.go:219`

`CreateChat` function signature — verify the `t` parameter is used correctly for the `type` column in the INSERT.

## Testing

After the fix, creating a conference from the Android client should log:
```
CreateGroupChat: ttttt (Creator: ferz) type=conference
Group chat created: <uuid> (ttttt) type=conference
```

And the chat in the database should have `type = 'conference'` instead of `type = 'group'`.
