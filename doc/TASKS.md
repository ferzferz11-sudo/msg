# Lavender Messenger — Задачи

**Версия:** v1.1.2.0
**Ветка:** feat/1.1.2.x
**Обновлено:** 2026-06-09
**Статус:** ✅ v1.1.2.0 — Prod релиз, начало новой итерации

---

## ✅ v1.1.2.0 — Prod Релиз

### Сервер
- Prod обновлён с v1.1.0.15 до v1.1.1.15
- Бэкап: lavender-server-backup-20260609
- Порт 50051, systemd сервис lavender-server

### Клиент
- APK v1.1.1.16 доступен для скачивания
- compileDebugKotlin ✅

---

## ✅ v1.1.1.16 — Багфикс + полировка (клиент)

### Android
- SplashActivity: логотип 🦞 → ic_notification_logo, надпись "Лава"/"Lava"
- AI навигация: return из AI активити → AI шторка открывается снова
- AIBottomSheet: после удаления чата шторка перестраивается
- ThemeApplier: aiFab добавлен в список FAB для кастомных тем
- Save button: style="@style/PrimaryButton"
- compileDebugKotlin ✅

### Сервер
- Без изменений (v1.1.1.15)

---

## ✅ v1.1.1.15 — Бесплатные модели + своя модель

### Server
- free_openrouter_models table
- GetFreeModels RPC, SetFreeModel/RemoveFreeModel RPC (admin)
- GetOwlSettings возвращает free_models
- Dev deployed

### Android
- Бесплатные модели с сервера
- Своя модель — текстовый ввод ID
- Favorites flickering fix

---

## 📋 Актуальный бэклог

### Средний приоритет
- Модульные тесты для OWL streaming

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- NewChatActivity → ChatWidget миграция

---

## Известные проблемы (не критично)
- Server migration warnings: `role "lavender" does not exist` — сервер работает
