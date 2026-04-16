package main

import (
	"sync"

	"msg/gen" // Используем ваш путь импорта
)

// Hub управляет активными gRPC стримами
type Hub struct {
	// mu защищает карту clients от одновременной записи из разных горутин
	mu      sync.RWMutex
	clients map[gen.ChatService_ChatServer]struct{}
}

// NewHub создает новый экземпляр концентратора
func NewHub() *Hub {
	return &Hub{
		clients: make(map[gen.ChatService_ChatServer]struct{}),
	}
}

// Register добавляет новый поток (клиента) в список рассылки
func (h *Hub) Register(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	h.clients[stream] = struct{}{}
	h.mu.Unlock()
}

// Unregister удаляет поток из списка рассылки
func (h *Hub) Unregister(stream gen.ChatService_ChatServer) {
	h.mu.Lock()
	delete(h.clients, stream)
	h.mu.Unlock()
}

// Broadcast отправляет сообщение всем подключенным клиентам
func (h *Hub) Broadcast(msg *gen.Message) {
	h.mu.RLock()
	// Освобождаем блокировку в конце функции
	defer h.mu.RUnlock()

	for stream := range h.clients {
		// Отправляем сообщение в gRPC stream
		err := stream.Send(msg)
		if err != nil {
			// Если отправить не удалось (клиент отключился),
			// логику удаления лучше оставить на defer в методе Chat сервера
			continue
		}
	}
}
