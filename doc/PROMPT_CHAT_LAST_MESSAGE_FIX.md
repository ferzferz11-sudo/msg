# Prompt: Fix Chat Last Message Not Updating After Message Deletion

**Проблема:**
Пользователь удаляет сообщения в чате. Последнее оставшееся сообщение — картинка. Однако в списке чатов всё ещё отображается текст удалённого сообщения как последнее. Pull-to-refresh не помогает — сервер отдаёт устаревший `lastMessageText`.

**Диагностика:**
1. Клиент вызывает `DeleteMessageV2` → сервер удаляет сообщение из БД
2. Клиент обновляет `GetChats` → сервер возвращает `lastMessageText` = текст удалённого сообщения
3. Клиент обновляет UI → список чатов показывает удалённый текст

**Корень проблемы:**
Сервер хранит `lastMessageText` в таблице `chats` как denormalized поле. При удалении сообщения `lastMessageText` НЕ обновляется на предыдущее сообщение.

---

## Исправление

### 1. `DeleteMessageV2` handler

После удаления сообщения из БД, обновить `lastMessageText` в `chats`:

```go
// После успешного удаления сообщения:
func (s *ChatServiceServer) deleteMessageAndUpdateLastMessage(ctx context.Context, chatId string, messageId string) {
    // 1. Удалить сообщение
    // 2. Найти последнее оставшееся сообщение в этом чате
    // 3. Обновить chats.lastMessageText, chats.lastMessageTime, chats.lastMessageUsername
    
    var lastMsg struct {
        Content     string
        SenderName  string
        CreatedAt   time.Time
        ContentType string
    }
    
    err := db.QueryRow(`
        SELECT content, sender_name, created_at, content_type 
        FROM messages 
        WHERE room_id = $1 AND content_type != 'deleted'
        ORDER BY created_at DESC 
        LIMIT 1
    `, chatId).Scan(&lastMsg.Content, &lastMsg.SenderName, &lastMsg.CreatedAt, &lastMsg.ContentType)
    
    if err == sql.ErrNoRows {
        // Нет сообщений — очистить lastMessageText
        db.Exec(`UPDATE chats SET last_message_text = '', last_message_time = NULL, last_message_username = '' WHERE id = $1`, chatId)
    } else if err == nil {
        // Есть сообщение — обновить lastMessageText
        preview := lastMsg.Content
        if len(preview) > 100 {
            preview = preview[:100]
        }
        db.Exec(`UPDATE chats SET last_message_text = $1, last_message_time = $2, last_message_username = $3 WHERE id = $4`,
            preview, lastMsg.CreatedAt, lastMsg.SenderName, chatId)
    }
}
```

### 2. Место вызова

Найти обработчик `DeleteMessageV2` RPC и вызвать `deleteMessageAndUpdateLastMessage` после успешного удаления.

Типичное место: `server_chat.go` или `server_messages.go`, где обрабатывается `DeleteMessageV2`.

### 3. Тест-кейс

1. Отправить 3 сообщения в чат (текст, картинка, текст)
2. Удалить последнее текстовое сообщение
3. **Ожидаемый результат:** в списке чатов отображается `[image]` (предпоследнее сообщение)
4. Удалить все сообщения
5. **Ожидаемый результат:** `lastMessageText` пустой

---

## Дополнительно: обновление при редактировании

Аналогичная проблема может быть при редактировании сообщения — `lastMessageText` тоже может устареть. Рекомендуется обновлять `lastMessageText` и при `EditMessageV2` если редактируется последнее сообщение.

```go
func (s *ChatServiceServer) editMessageAndUpdateLastMessage(ctx context.Context, chatId string, messageId string, newContent string) {
    // Проверить, является ли редактируемое сообщение последним
    var lastMsgId string
    err := db.QueryRow(`
        SELECT id FROM messages 
        WHERE room_id = $1 AND content_type != 'deleted'
        ORDER BY created_at DESC LIMIT 1
    `, chatId).Scan(&lastMsgId)
    
    if err == nil && lastMsgId == messageId {
        // Редактируется последнее сообщение — обновить lastMessageText
        preview := newContent
        if len(preview) > 100 {
            preview = preview[:100]
        }
        db.Exec(`UPDATE chats SET last_message_text = $1 WHERE id = $2`, preview, chatId)
    }
}
```
