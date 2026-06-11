"""
Lavender Messenger — Hermes Agent Platform Adapter Plugin

Регистрирует адаптер как плагин платформы для Hermes Agent.
Фабрика создаёт экземпляр LavenderPlatformAdapter при обнаружении
конфигурации lavender.* в .env / config.
"""

import logging
import os
from pathlib import Path
from typing import Optional

from adapter import LavenderPlatformAdapter

logger = logging.getLogger(__name__)

# ── Plugin metadata ────────────────────────────────────────
PLUGIN_NAME = "lavender"
PLUGIN_VERSION = "1.0.0"
PLUGIN_DESCRIPTION = "Lavender Messenger Platform Adapter — bidirectional gRPC streaming"


def create_adapter(
    server_addr: Optional[str] = None,
    user_id: Optional[str] = None,
    username: Optional[str] = None,
    **kwargs,
) -> LavenderPlatformAdapter:
    """
    Фабричный метод — создаёт экземпляр адаптера из конфигурации.

    Вызывается Hermes Agent при инициализации плагина.
    Параметры берутся из .env / config с префиксом LAVENDER_.

    Args:
        server_addr: адрес gRPC-сервера (env: LAVENDER_SERVER)
        user_id: UUID пользователя (env: LAVENDER_USER_ID)
        username: имя пользователя (env: LAVENDER_USERNAME)
        **kwargs: дополнительные параметры

    Returns:
        Настроенный экземпляр LavenderPlatformAdapter

    Raises:
        ValueError: если не заданы обязательные параметры
    """
    _server = server_addr or os.environ.get("LAVENDER_SERVER", "localhost:50052")
    _user_id = user_id or os.environ.get("LAVENDER_USER_ID", "")
    _username = username or os.environ.get("LAVENDER_USERNAME", "hermes-agent")

    if not _user_id:
        raise ValueError(
            "LAVENDER_USER_ID is required. "
            "Set it in .env or pass user_id to create_adapter()."
        )

    adapter = LavenderPlatformAdapter(
        server_addr=_server,
        user_id=_user_id,
        username=_username,
        reconnect_base_delay=float(
            kwargs.get("reconnect_base_delay", os.environ.get("LAVENDER_RECONNECT_DELAY", "3.0"))
        ),
        reconnect_max_delay=float(
            kwargs.get("reconnect_max_delay", "60.0")
        ),
    )

    logger.info(
        "Lavender plugin v%s: adapter created (server=%s, user=%s)",
        PLUGIN_VERSION,
        _server,
        _user_id[:8] + "...",
    )
    return adapter


def register(hooks: dict) -> None:
    """
    Регистрирует плагин в Hermes Agent.

    Вызывается при загрузке плагина. Добавляет хуки для:
    - platform.start — запуск адаптера
    - platform.stop — остановка адаптера
    - platform.send — отправка сообщения

    Args:
        hooks: словарь хуков Hermes Agent
    """
    logger.info("Lavender plugin v%s: registering hooks...", PLUGIN_VERSION)

    _adapter: Optional[LavenderPlatformAdapter] = None

    async def on_start(config: dict):
        nonlocal _adapter
        _adapter = create_adapter(**config)
        await _adapter.start()

    async def on_stop():
        nonlocal _adapter
        if _adapter:
            await _adapter.stop()
            _adapter = None

    async def on_send(session_id: str, text: str, **kwargs):
        if _adapter:
            return await _adapter.send_message(session_id, text, **kwargs)
        return False

    hooks.setdefault("platform.start", []).append(on_start)
    hooks.setdefault("platform.stop", []).append(on_stop)
    hooks.setdefault("platform.send", []).append(on_send)

    logger.info("Lavender plugin v%s: hooks registered ✓", PLUGIN_VERSION)


# ── Hermes Agent plugin entry point ───────────────────────
# При импорте плагина Hermes Agent вызывает get_plugin()

def get_plugin():
    """
    Точка входа плагина для Hermes Agent.

    Returns:
        dict с метаданными и функцией регистрации
    """
    return {
        "name": PLUGIN_NAME,
        "version": PLUGIN_VERSION,
        "description": PLUGIN_DESCRIPTION,
        "register": register,
        "create_adapter": create_adapter,
    }
