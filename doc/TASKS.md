# Lavender Messenger — Задачи

**Версия:** v1.1.1.16
**Обновлено:** 2026-06-09
**Статус:** ✅ v1.1.1.16 — Багфикс + полировка

---

## ✅ v1.1.1.16 — Багфикс + полировка

### Android
- **SplashActivity**: логотип 🦞 → ic_notification_logo (как в шторке логина)
- **SplashActivity**: надпись "Lavender" → "Лава" (ru) / "Lava" (en) по языку
- **AIBottomSheet**: rebuildContent() + updateChats() для перестройки без закрытия
- **AIBottomSheet**: popup menu delete/settings больше не закрывает шторку
- **ChatListActivity**: shouldShowAiSheetOnResume флаг для возврата из AI активити
- **ChatListActivity**: return из OwlChat/HermesChat/Settings/Notifications → AI шторка открывается снова
- **ThemeApplier**: aiFab добавлен в список FAB для кастомных тем
- **activity_owl_settings.xml**: Save button использует style="@style/PrimaryButton"
- compileDebugKotlin ✅
- Тег v1.1.1.16

Сервер: без изменений (v1.1.1.15)

---

## ✅ v1.1.1.15 — Бесплатные модели + своя модель

### Server
- free_openrouter_models table
- GetFreeModels RPC, SetFreeModel/RemoveFreeModel RPC (admin)
- GetOwlSettings возвращает free_models
- Dev deployed

### Android
- Бесплатные модели с сервера (GetFreeModels RPC)
- Своя модель — текстовый ввод ID (только с ключом)
- Favorites flickering fix
- compileDebugKotlin ✅

---

## ✅ v1.1.1.14 — Дизайн + полировка
- Анимации сообщений (fade-in + slide), typing indicator (ValueAnimator)
- Bottom sheets: MaterialCardView, hover-эффекты, per-command иконки
- Splash screen анимация, statusBarColor = bgColor
- compileDebugKotlin ✅, dev deployed ✅

---

## 📋 Актуальный бэклог

### Высокий приоритет (v1.1.2.0 — деплой на prod)
1. **Деплой клиента на prod** — собрать APK, залить на сервер для клиентов
2. **Деплой сервера на prod** — после клиента
3. **Тестирование на prod**

### Средний приоритет
- Модульные тесты для OWL streaming

### Низкий приоритет
- Auth токены для удалённых агентов (JWT)
- Qdrant + CLIP (production RAG)
- NewChatActivity → ChatWidget миграция

---

## 🟡 Известные баги

### Favorites — мерцание при обновлении списка чатов
- **Статус:** исправлено в c873fbc (v1.1.1.15)

---

## Известные проблемы (не критично)
- Server migration warnings: `role "lavender" does not exist` — сервер работает
