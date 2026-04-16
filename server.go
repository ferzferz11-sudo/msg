package main

import (
	"io"
	"log"
	"msg/gen"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type server struct {
	gen.UnimplementedChatServiceServer
	hub *Hub
	db  *DB
}

func (s *server) Chat(stream gen.ChatService_ChatServer) error {
	s.hub.Register(stream)
	defer func() {
		s.hub.Unregister(stream)
		log.Println("Клиент отключился")
	}()

	log.Println("Новый клиент подключен")

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Устанавливаем время получения сообщения сервером
		msg.CreatedAt = timestamppb.Now()

		log.Printf("[%s]: %s", msg.User, msg.Text)

		// Шифруем текст сообщения перед сохранением
		encryptedText, err := encrypt(msg.Text)
		if err != nil {
			log.Printf("Failed to encrypt message: %v", err)
			continue
		}

		// Сохраняем зашифрованное сообщение в БД
		err = s.db.SaveMessage(msg.User, encryptedText, msg.CreatedAt.AsTime())
		if err != nil {
			log.Printf("Failed to save message to DB: %v", err)
		}

		s.hub.Broadcast(msg)
	}
}
