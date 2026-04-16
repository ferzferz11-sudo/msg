package main

import (
	"log"
	"net"
	"os"

	"msg/gen"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

func main() {
	// Загружаем переменные окружения из файла .env
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it, using system environment variables")
	}

	// Читаем переменные окружения для сервера
	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = ":50051" // Значение по умолчанию
	}
	serverVersion := os.Getenv("SERVER_VERSION")
	if serverVersion == "" {
		serverVersion = "unknown"
	}

	// Подключаемся к базе данных
	db := ConnectDB()
	defer db.Close()

	// Создаем TCP слушателя по адресу из .env
	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Создаем новый gRPC сервер
	s := grpc.NewServer()

	// Инициализируем наш сервис с Hub и БД
	srv := &server{
		hub: NewHub(),
		db:  db,
	}

	// Регистрируем сервис
	gen.RegisterChatServiceServer(s, srv)

	log.Printf("Server Version: %s", serverVersion)
	log.Printf("Server listening at %v", lis.Addr())

	// Запускаем сервер
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
