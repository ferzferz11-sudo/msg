# Лава — Задачи

**Ветка:** feat/1.3.0.x
**Обновлено:** 2026-08-13

---

## ✅ Серверная часть завершена (52/52 задач + 3 test suites + Hermes ACP + Company System)

Прогресс: **100%**

### Тесты (crypto + AI v2 + company)

| Файл | Что тестирует | Тестов |
|------|--------------|--------|
| `crypto_test.go` | encrypt/decrypt (roundtrip, unicode, markers, tamper), HashPassword/CheckPassword, GenerateResetToken, getSecretKey | 22 |
| `ai_v2_test.go` | ProviderRegistry, ToolRegistry (whitelist, cache), HybridRouter (keywords, binding), AgentExecutor (mock provider, tool calls, model override), resolveAPIKey, toolCache (TTL, LRU, concurrent), isURLSafe SSRF, OpenRouter SSE streaming, query_database security, all tool interfaces | 76 |
| `company_test.go` | removeParticipant, access level thresholds, position hierarchy, chat type validation, default positions, builtin position protection, owner constraints, participants JSON | 9 |
| `self_destruct_test.go` | allowedTimerValues validation, ChatV2Row self_destruct_timer proto, rowToProtoV2 forwarded_from/mentions, SetSelfDestructTimerResponse proto | 6 |

---

## История релизов

| Версия | Описание |
|--------|----------|
| v1.4.0.0 | Self-Destruct Timer (auto-delete messages per chat: 30s/1m/5m/1h/24h), Delete Message Persistence (deleted_messages table prevents reappearance in history), fixed isChatParticipant UUID resolution |
| v1.3.4.7 | Conference improvements: IDENTITY signal type (replaces ICE_CANDIDATE hack), verified LEAVE/END_CONFERENCE + state cleanup |
| v1.3.4.2 | OIDC session validation (HMAC-SHA256 signed cookies), login flow fix |
| v1.3.4.0 | OIDC SSO Provider + Security Fixes (JWT issuer/audience, Agent Revoke, Rate Limiter, CORS, IP Extraction) |
| v1.3.2.1 | Bug fixes: UpdateChatParticipants type mismatch, DeleteCompany FK violation, CreateGroupChat type field |
| v1.3.2.0 | Company System: companies, positions, members, company chats, access control, multi-company support, primary company, GetUserInfo, GetUserCompanies, SetPrimaryCompany, max_upload_size in /info, READ_ALL broadcast to ChatV2 |
| v1.3.1.21 | Company System v1: CRUD, positions, members, company chats, access levels |
| v1.3.1.20 | Read receipts fix (messages_v2), max_upload_size in /info, READ_ALL broadcast |
| v1.3.0.31 | BroadcastGlobalV2 for online status, BroadcastShutdown for v2, reaction broadcast to ChatV2 stream, GetAdminUserList RPC, DeleteMessageV2 physical delete |
| v1.3.0.29 | Hermes ACP provider (JSON-RPC 2.0, persistent sessions), crypto + AI v2 tests (98), last_seen_at tracking, 11 preset agents |
| v1.3.0.27 | v1 messages tables dropped, v1 DB functions removed |
| v1.3.0.26 | v1 RPCs rewritten to v2 internally (backward compat), marked deprecated |
| v1.3.0.25 | Messages v1→v2 migration complete, SearchMessages RPC, dual-write removed |
| v1.3.0.19 | Messages v2: MessageV2 proto, ChatV2 stream, cursor pagination, JSONB reactions, dual-write |
| v1.3.0.18 | Reve Image Generation, image_url in ChatWithAIV2Response |
| v1.3.0.16 | RAG message indexing, JWT rotation, upload validation, DeleteProfile cascade, dead code removal |
| v1.3.0.15 | Security audit: message logging, LIKE injection, bcrypt cost |
| v1.3.0.14 | Security audit: Firebase key, HTTP auth, SSRF, SQL hardening, context timeouts |
| v1.3.0.13 | Performance optimizations: cursor pagination, DB pool tuning, AI dedup, tool caching, indexes |
| v1.3.0.12 | Unread count optimization (read_at based) |
| v1.3.0.11 | ProfileService v2 on prod + unread count fix + docs consolidation |
| v1.3.0.10 | Production RAG (Qdrant + OpenAI embeddings) |
| v1.3.0.9 | Redis rate limiter wired in + cleanup |
| v1.3.0.8 | v1 compat removal + AI gateway bug fix + logging |
| v1.3.0.7 | Redis rate limiter + billing guide |
| v1.3.0.4 | Graceful shutdown (SERVER_SHUTTINGDOWN broadcast) |
| v1.3.0.3 | Bug fixes (MarkRead, ghost AI chats) |
| v1.3.0.2 | AI v2 Usage Stats + Marketplace (7 new RPCs) |
| v1.3.0.0 | AI Services v2 (7 providers, 6 tools, 8 presets) |
| v1.2.0.11 | DB index optimization |
| v1.2.0.10 | E2EE fixes + P1/P2 optimizations |
| v1.2.0.9 | Performance optimizations (P0) |
| v1.2.0.8 | ChatList v2 last message optimization |
| v1.2.0.7 | UserInfo UUID + deploy scripts |
| v1.2.0.6 | ChatList v2 last message columns |
| v1.2.0.5 | GetChatsV2 (pagination, filters, v2 fields) |
| v1.2.0.4 | Security fixes (auth bypass, path traversal) |
| v1.2.0.3 | Security fixes (JWT, TURN, admin, user impersonation) |
| v1.2.0.0 | AuthService v2 (JWT) |
| v1.2.1.0 | ProfileService v2 (dev only) |


