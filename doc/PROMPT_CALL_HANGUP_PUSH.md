# Prompt: Send FCM push on HANGUP when callee stream is offline

**Problem:**
When the caller hangs up, the HANGUP signal is broadcast via `BroadcastCall` to the callee. But if the callee's call session stream is not registered in `callStreams` (e.g., callee was woken up via FCM push, not via call stream), the signal is silently lost. The callee keeps seeing the ringing notification and can re-enter the ended call.

**Root cause:**
In `server_chat.go` line 669, after the HANGUP case, `BroadcastCall(msg)` is called. If `delivered == false`, only a warning is logged — no push notification is sent to the callee.

**Fix:**
After `BroadcastCall` at line 669, if `!delivered` AND the signal type is `HANGUP` or `REJECT`, send a FCM push notification to `msg.ReceiverId` with type `"CALL_ENDED"`.

1. **In `server_chat.go`**, after line 669 (`delivered := s.hub.BroadcastCall(msg)`), add:

```go
if !delivered && (msg.Type == gen.CallMessage_HANGUP || msg.Type == gen.CallMessage_REJECT) {
    s.sendCallEndedPushNotification(msg.ReceiverId, msg.SenderId, msg.CallId)
}
```

2. **In `server_push.go`**, add a new function `sendCallEndedPushNotification`:

```go
func (s *server) sendCallEndedPushNotification(receiverId, senderId, callId string) {
    if s.firebaseApp == nil {
        return
    }
    token, err := s.db.GetUserTokenByUserID(receiverId)
    if err != nil || token == "" || token == "DISABLED" {
        return
    }
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    client, err := s.firebaseApp.Messaging(ctx)
    if err != nil {
        return
    }
    message := &messaging.Message{
        Token: token,
        Data: map[string]string{
            "type":      "CALL_ENDED",
            "call_id":   callId,
            "sender_id": senderId,
        },
        Android: &messaging.AndroidConfig{
            Priority: "high",
        },
    }
    _, err = client.Send(ctx, message)
    if err != nil {
        s.logFCM("ERROR", "Call Ended Push to %s failed: %v", receiverId, err)
    } else {
        s.logFCM("SUCCESS", "Call Ended Push sent to %s", receiverId)
    }
}
```

**Files to change:**
- `server_chat.go` — after line 669
- `server_push.go` — new function

**Note:** The `handleAbruptDisconnect` function already handles this case correctly (it broadcasts HANGUP and logs when not delivered). The issue is only with the normal HANGUP flow at line 669.
