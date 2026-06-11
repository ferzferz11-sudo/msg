# Lavender Messenger — Промпт для Android-сессий

## Текущий статус

**Версия:** v1.1.2.7 (prod)
**Ветка:** feat/1.1.2.x
**APK:** /var/www/lavender/lavender.apk
**GitHub:** https://github.com/ferzferz11-sudo/msg.client.android/releases/tag/v1.1.2.7

---

## Контекст

- Сервер: /root/msg, dev порт 50052, prod порт 50051
- Android: /root/msg/client.android
- Оба репозитория на ветке feat/1.1.2.x
- v1.1.2.7 — prod версия (таг выпущен)

---

## Что сделано в v1.1.2.7

- SplashActivity: увеличено расстояние логотип→текст (60px → 90dp)
- SplashLoadingActivity: новый оверлей загрузки для логина/регистрации
- Login/Register: показывается SplashLoadingActivity во время авторизации
- Онбординг полностью удалён (welcomeContainer, onboardingProfileBubble, onboardingFabBubble)
- Чекбокс "Сразу создать личный чат" при добавлении контакта (включён по умолчанию)
- Исправления: crash при выборе чатов, getSelectedChats offset, loadingContainer удалён, statusBarColor deprecation
- compileDebugKotlin passes

---

## Известная проблема (не исправлено)

### Favorites при пустом списке чатов
- **Симптом:** при входе после очистки памяти Favorites не отображается если нет созданных чатов
- **Причина:** `chatAdapter.setChats()` вызывается с `[Favorites]`, но `displayedChats` остаётся пустым
- **Попытки:** `selectedPositions.clear()`, `post { notifyDataSetChanged() }`, убран `loadChatsFromCache`
- **Нужно:** разобраться почему `getItemCount()` возвращает 1 но RecyclerView не отображает элемент

---

## Бэклог (приоритет)

1. **Favorites при пустом списке** — высокий приоритет
2. **Модульные тесты для OWL streaming** — средний
3. **Qdrant + CLIP (production RAG)** — низкий, ночная задача

---

## Архитектура

```
ANDROID:
├── OwlGrpc.kt          — OWL: chatWithOwl, processBotCommand, getBotCommands, getOWLStatus
├── HermesGrpc.kt       — Hermes: chatWithOrchestrator, agent management
├── GrpcClient.kt       — единая точка доступа
├── OwlChatActivity.kt  — UI чата с OWL
├── OwlChatViewModel.kt — ViewModel (отдельные owlTyping/owlResponses flows)
└── HermesChatActivity.kt — UI чата с Hermes
```

---

## Правила

- Коммитить после каждого значимого изменения, пушить в feat/1.1.2.x
- При каждом релизе: git tag, CHANGELOG.md, bundled, version.txt
- assembleRelease НЕ запускать на сервере (OOM kill)
- Дизайн — минималистичный, чистый
- userId (UUID) — всегда как ключ, НЕ username
- creator_id (UUID) — для проверки владельца
- participants ВСЕГДА через json.Marshal, никогда вручную
- Для кастомных тем: новые FAB кнопки добавлять в ThemeApplier.kt в список FABs
- Proto поля: всегда сверять номера полей с messenger.proto!

---

## Важно (changelog)

- changelog.txt УДАЛЁН из проекта и из деплоя (v1.1.2.6)
- Вместо него: app/src/main/assets/changelog_bundled.txt — встроен в APK
- При каждом релизе: обновлять assets/changelog_bundled.txt вместе с CHANGELOG.md
- Формат: emoji-заголовки, буллеты —, секции по версиям

---

## Важно (темы)

- НЕ использовать ?attr/colorOnSurface в XML для текста на кастомных тёмных темах
- Всегда программно через ThemeStore.currentTheme() + ThemeUtils.parseSafeColor()
- Устанавливать и background (backgroundColor) и text (textPrimaryColor) явно
- ThemeApplier.apply вызывать до setContentView

---

## Команды

```bash
cd /root/msg/client.android
./gradlew compileDebugKotlin    # проверка компиляции
# assembleRelease НЕ запускать на сервере — OOM
```

---

## Документация (читать в начале каждой сессии)

- Индекс: /root/msg/doc/INDEX.md
- Сервер: /root/msg/doc/INTEGRATION_SESSION.md, /root/msg/doc/TASKS.md
- Android: /root/msg/client.android/doc/TASKS.md
- AI сервисы: /root/msg/doc/AI_SERVICES.md
- Подводные камни: /root/msg/doc/PITFALLS.md
- Changelog: /root/msg/doc/CHANGELOG.md
- Memory pad: /root/.hermes/memory/pad.md
