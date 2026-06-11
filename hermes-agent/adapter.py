"""
Lavender Messenger — Platform Adapter for Hermes Agent

Подключает Lavender gRPC-мессенджер к экосистеме Hermes Agent
через bidirectional streaming (ChatService.Chat).

Протокол: messenger.proto → ChatService.Chat(stream Message) returns (stream Message)
Ключевые поля Message: room_id (chat_id), user_id (sender_id), text, id
"""

import asyncio
import logging
import os
import signal
import sys
import time
from pathlib import Path
from typing import Optional

import grpc

# ── proto imports ──
# messenger_pb2 и messenger_pb2_grpc генерируются из messenger.proto
# Путь: либо тот же каталог, либо LAVENDER_PROTO_DIR
_SCRIPT_DIR = Path(__file__).resolve().parent

# Добавляем LAVENDER_PROTO_DIR в path (если задан)
_proto_dir = os.environ.get("LAVENDER_PROTO_DIR", str(_SCRIPT_DIR))
if _proto_dir not in sys.path:
    sys.path.insert(0, _proto_dir)

try:
    import messenger_pb2 as pb
    import messenger_pb2_grpc as pb_grpc
except ImportError as exc:
    raise ImportError(
        f"messenger_pb2 / messenger_pb2_grpc not found. "
        f"LAVENDER_PROTO_DIR={_proto_dir}. "
        f"Run: python3 -m grpc_tools.protoc --proto_path=. --python_out=. --grpc_python_out=. messenger.proto"
    ) from exc


class LavenderPlatformAdapter:
    """
    Platform Adapter для Lavender Messenger.

    Подключается к серверу через gRPC bidirectional streaming (ChatService.Chat),
    слушает входящие сообщения и маршрутизирует их в Hermes Agent
    через handle_inbound_message(). Ответы агента отправляются обратно
    через send_message().

    Атрибуты:
        server_addr: адрес gRPC-сервера (например "localhost:50052")
        user_id: UUID пользователя для авторизации в стриме
        username: имя пользователя (для отображения)
        reconnect_base_delay: базовая задержка переподключения (сек)
        reconnect_max_delay: максимальная задержка переподключения (сек)
    """

    def __init__(
        self,
        server_addr: str = "localhost:50052",
        user_id: str = "",
        username: str = "",
        reconnect_base_delay: float = 3.0,
        reconnect_max_delay: float = 60.0,
        logger: Optional[logging.Logger] = None,
    ):
        self.server_addr = server_addr
        self.user_id = user_id
        self.username = username
        self.reconnect_base_delay = reconnect_base_delay
        self.reconnect_max_delay = reconnect_max_delay
        self.logger = logger or logging.getLogger(self.__class__.__name__)

        self._channel: Optional[grpc.aio.Channel] = None
        self._running = False
        self._send_queue: asyncio.Queue[pb.Message] = asyncio.Queue()
        self._inbound_task: Optional[asyncio.Task] = None
        self._outbound_task: Optional[asyncio.Task] = None

    # ── public API ──────────────────────────────────────────

    async def start(self) -> None:
        """Запускает адаптер: подключается и начинает слушать."""
        self._running = True
        await self._connect_loop()

    async def stop(self) -> None:
        """Останавливает адаптер и закрывает gRPC-канал."""
        self.logger.info("LavenderAdapter: stopping...")
        self._running = False

        # Отменяем задачи
        for task in (self._inbound_task, self._outbound_task):
            if task and not task.done():
                task.cancel()
                try:
                    await task
                except asyncio.CancelledError:
                    pass

        # Закрываем канал
        if self._channel:
            try:
                await self._channel.close(grace=2.0)
            except Exception:
                pass
            self._channel = None

        self.logger.info("LavenderAdapter: stopped")

    async def send_message(
        self,
        session_id: str,
        text: str,
        **kwargs,
    ) -> bool:
        """
        Отправляет исходящее сообщение в Lavender-чат.

        Args:
            session_id: room_id / chat_id куда отправить
            text: текст сообщения
            **kwargs: дополнительные поля Message (опционально)

        Returns:
            True если сообщение поставлено в очередь отправки
        """
        if not self._running or not self._channel:
            self.logger.warning(
                "LavenderAdapter: send_message called but adapter not running"
            )
            return False

        msg = pb.Message(
            room_id=session_id,
            text=text,
            user=self.username,
            user_id=self.user_id,
        )

        # Применяем дополнительные поля из kwargs
        for field_name in ("id", "device_id", "image_url"):
            if field_name in kwargs:
                setattr(msg, field_name, kwargs[field_name])

        await self._send_queue.put(msg)
        self.logger.debug(
            "LavenderAdapter: queued outbound → room=%s text=%r",
            session_id,
            text[:120],
        )
        return True

    async def handle_inbound_message(
        self,
        session_id: str,
        text: str,
        user_id: str,
    ) -> Optional[str]:
        """
        Вызывается при получении входящего сообщения от Lavender-сервера.

        Переопределите этот метод или зарегистрируйте callback.

        Args:
            session_id: room_id / chat_id откуда пришло
            text: текст сообщения
            user_id: UUID отправителя

        Returns:
            Текст ответа (опционально). Если возвращает строку —
            ответ будет отправлен обратно через send_message().
        """
        self.logger.info(
            "LavenderAdapter: inbound ← room=%s user=%s text=%r",
            session_id,
            user_id,
            text[:200],
        )
        # По умолчанию — заглушка. Подклассы/Hermes переопределяют.
        return None

    # ── connection management ───────────────────────────────

    async def _connect_loop(self) -> None:
        """Цикл подключения с экспоненциальным backoff."""
        delay = self.reconnect_base_delay
        attempt = 0

        while self._running:
            try:
                self.logger.info(
                    "LavenderAdapter: connecting to %s (attempt %d)...",
                    self.server_addr,
                    attempt + 1,
                )
                await self._connect_and_stream()
                # Если стрим завершился нормально — сбрасываем delay
                delay = self.reconnect_base_delay
                attempt = 0

            except asyncio.CancelledError:
                self.logger.info("LavenderAdapter: connection loop cancelled")
                return

            except Exception as exc:
                attempt += 1
                self.logger.warning(
                    "LavenderAdapter: connection error (attempt %d): %s",
                    attempt,
                    exc,
                )

            if not self._running:
                return

            # Exponential backoff с jitter
            import random
            jitter = delay * 0.25
            actual_delay = delay + random.uniform(-jitter, jitter)
            actual_delay = max(self.reconnect_base_delay, min(actual_delay, self.reconnect_max_delay))

            self.logger.info(
                "LavenderAdapter: reconnecting in %.1fs...", actual_delay
            )
            await asyncio.sleep(actual_delay)
            delay = min(delay * 2, self.reconnect_max_delay)

    async def _connect_and_stream(self) -> None:
        """Устанавливает соединение и запускает bidirectional stream."""
        self._channel = grpc.aio.insecure_channel(self.server_addr)
        stub = pb_grpc.ChatServiceStub(self._channel)

        # Проверяем доступность канала
        try:
            await asyncio.wait_for(
                self._channel.channel_ready(),
                timeout=10.0,
            )
        except asyncio.TimeoutError:
            raise ConnectionError(
                f"Cannot connect to {self.server_addr} within 10s"
            )

        self.logger.info("LavenderAdapter: channel ready ✓")

        # Запускаем bidirectional Chat stream
        await self._chat_stream(stub)

    async def _chat_stream(self, stub: pb_grpc.ChatServiceStub) -> None:
        """
        Управляет bidirectional streaming-соединением ChatService.Chat.

        Два конкурирующих процесса:
        1. _inbound_reader — читает сообщения из серверного стрима
        2._outbound_writer — отправляет сообщения из очереди
        """
        self.logger.info("LavenderAdapter: starting Chat bidirectional stream...")

        # Создаём стрим
        call = stub.Chat(
            self._outbound_generator(),
            metadata=self._auth_metadata(),
        )

        self.logger.info("LavenderAdapter: Chat stream established ✓")

        # Читаем входящие
        try:
            async for msg in call:
                await self._process_inbound(msg)
        except grpc.aio.AioRpcError as exc:
            code = exc.code()
            if code == grpc.StatusCode.CANCELLED:
                self.logger.info("LavenderAdapter: stream cancelled by server")
            elif code == grpc.StatusCode.UNAVAILABLE:
                self.logger.warning("LavenderAdapter: server unavailable")
            else:
                self.logger.warning(
                    "LavenderAdapter: stream RPC error: %s — %s",
                    code,
                    exc.details(),
                )
            raise
        except asyncio.CancelledError:
            raise
        except Exception as exc:
            self.logger.error("LavenderAdapter: stream error: %s", exc)
            raise
        finally:
            # Помечаем очередь как «завершённую»
            await self._send_queue.put(None)

    async def _inbound_reader(self, call) -> None:
        """Читает сообщения из серверного стрима."""
        try:
            async for msg in call:
                await self._process_inbound(msg)
        except grpc.aio.AioRpcError as exc:
            self.logger.warning("LavenderAdapter: inbound RPC error: %s", exc.code())
        except asyncio.CancelledError:
            pass

    async def _process_inbound(self, msg: pb.Message) -> None:
        """Обрабатывает одно входящее сообщение."""
        room_id = msg.room_id or msg.id
        user_id = msg.user_id or msg.user
        text = msg.text

        if not text:
            self.logger.debug("LavenderAdapter: skipping empty message")
            return

        # Игнорируем собственные сообщения (эхо)
        if user_id == self.user_id:
            self.logger.debug("LavenderAdapter: ignoring own echo")
            return

        self.logger.info(
            "LavenderAdapter: ← room=%s user=%s: %r",
            room_id,
            user_id,
            text[:120],
        )

        try:
            response_text = await self.handle_inbound_message(
                session_id=room_id,
                text=text,
                user_id=user_id,
            )
            if response_text:
                await self.send_message(
                    session_id=room_id,
                    text=response_text,
                )
        except Exception as exc:
            self.logger.error(
                "LavenderAdapter: error processing inbound: %s", exc, exc_info=True
            )

    async def _outbound_generator(self):
        """
        Генератор для outbound-части bidirectional stream.

        Первое сообщение — регистрационное (register=true).
        Далее — из очереди _send_queue.
        """
        # Отправляем регистрационное сообщение
        if self.user_id:
            reg_msg = pb.Message(
                user_id=self.user_id,
                user=self.username,
                register=True,
            )
            self.logger.debug("LavenderAdapter: sending registration")
            yield reg_msg

        # Читаем из очереди
        while self._running:
            try:
                msg = await asyncio.wait_for(
                    self._send_queue.get(),
                    timeout=30.0,
                )
            except asyncio.TimeoutError:
                continue

            if msg is None:  # poison pill
                break

            self.logger.debug(
                "LavenderAdapter: → room=%s: %r",
                msg.room_id,
                msg.text[:120],
            )
            yield msg

    def _auth_metadata(self) -> tuple:
        """Метаданные для авторизации в gRPC-стриме."""
        metadata = []
        if self.user_id:
            metadata.append(("x-user-id", self.user_id))
        return tuple(metadata) if metadata else ()


# ── standalone runner ──────────────────────────────────────

async def main():
    """Запуск адаптера как standalone-процесса."""
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(name)s] %(levelname)s: %(message)s",
    )
    log = logging.getLogger("lavender-adapter")

    server_addr = os.environ.get("LAVENDER_SERVER", "localhost:50052")
    user_id = os.environ.get("LAVENDER_USER_ID", "")
    username = os.environ.get("LAVENDER_USERNAME", "hermes-agent")

    adapter = LavenderPlatformAdapter(
        server_addr=server_addr,
        user_id=user_id,
        username=username,
    )

    # Пример: переопределяем handle_inbound_message для демо
    async def on_message(session_id: str, text: str, user_id: str) -> Optional[str]:
        log.info("Demo handler: room=%s from=%s: %r", session_id, user_id, text)
        if text.lower().startswith("/ping"):
            return "pong 🏓"
        return None

    adapter.handle_inbound_message = on_message

    loop = asyncio.get_event_loop()

    def _shutdown(sig, _frame):
        log.info("Received signal %s, shutting down...", sig)
        asyncio.ensure_future(adapter.stop())

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)

    try:
        await adapter.start()
    except KeyboardInterrupt:
        pass
    finally:
        await adapter.stop()


if __name__ == "__main__":
    asyncio.run(main())
