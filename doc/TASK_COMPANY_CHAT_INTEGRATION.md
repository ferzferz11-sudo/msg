# TASK: Company Chat Integration with Chat System

**Priority:** High
**Date:** 2026-08-11

---

## Problem

`CreateCompanyChat` RPC creates a record in the company_chats table but does NOT create a corresponding chat in the regular `chats` table. As a result:

1. Company chats do NOT appear in `GetChatsV2` response → user can't see them in the chat list
2. User can't send messages to company chats (no chat stream exists)
3. Company chats are essentially dead records with no messaging functionality

## Expected Behavior

Company chats should work like regular group chats with additional company metadata:
- Appear in `GetChatsV2` response (so they show in the chat list)
- Have a working `ChatV2` bidi stream for real-time messaging
- Have `company_id` field set in `ChatInfoProto` so the client can identify them as company chats
- Respect access control (`access_level`, `min_position_level`) — only visible to eligible members

## Required Changes

### 1. `CreateCompanyChat` RPC — Create Regular Chat

When `CreateCompanyChat` is called:
1. Create a record in `company_chats` table (existing behavior)
2. **ALSO** create a regular chat in the `chats` table:
   - `type = "group"`
   - `name = <chat name from request>`
   - `creator = <current user ID>`
   - `participants = [<current user ID>]` (JSON array)
   - `company_id = <companyId from request>`
   - `company_chat_access = <accessLevel from request>`
   - `company_min_position_level = <minPositionLevel from request>`
3. Return the `chatId` of the newly created regular chat (not the company_chats record ID)

### 2. `GetChatsV2` — Include Company Chats

`GetChatsV2` should already return company chats since they're in the `chats` table. Verify that:
- The `company_id`, `company_chat_access`, `company_min_position_level` fields are populated in `ChatInfoProto`
- Company chats are included in the response for users who have access (based on their position level in the company)

### 3. Access Control in `GetChatsV2`

For each company chat in the response:
- Look up the user's position level in the chat's company
- If `company_min_position_level > 0`: user's level must be >= that value
- If `company_chat_access == "management"`: user's level must be >= 1
- If `company_chat_access == "owner_only"`: user's level must be >= 3
- If `company_chat_access == "member"`: all company employees can see it
- If the user doesn't have access, exclude the chat from the response

### 4. `ChatV2` Stream — Company Chat Messages

Company chat messages should flow through the regular `ChatV2` bidi stream:
- Messages sent to a company chat `roomId` are broadcast to all participants
- The `company_id` field in `ChatInfoProto` allows the client to identify company chats

### 5. `AddMember` / `RemoveMember` — Update Chat Participants

When a member is added/removed from a company:
- If they should have access to company chats (based on their position), add them as participants
- If they lose access, remove them from the chat participants

## Testing

After implementation, verify:
1. Create a company chat → it appears in `GetChatsV2` for the creator
2. Send a message to the company chat → it appears in the chat
3. Another company member can see the chat (if they have access)
4. A member without sufficient position level cannot see the chat
5. The `company_id` field is populated in `ChatInfoProto`

## Client-Side Notes

The Android client already has:
- Company tab in chat list (filters by `companyId.isNotEmpty()`)
- Company badge on chat items
- Access control filtering in `ChatListViewModel.buildSections()`
- Per-company position level caching via `companyPositionCache`

The client just needs the server to return company chats in `GetChatsV2` with proper `company_id` fields.
