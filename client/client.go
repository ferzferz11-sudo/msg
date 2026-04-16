package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"msg/gen"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func fixUtf8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var v []rune
	for _, r := range s {
		if r != utf8.RuneError {
			v = append(v, r)
		}
	}
	return string(v)
}

func main() {
	// Пытаемся загрузить .env, если он доступен
	godotenv.Load("../.env")

	// Получаем адрес сервера из переменной окружения или используем значение по умолчанию
	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = "localhost:50051"
	}

	// Если в .env адрес начинается с ':', добавляем localhost (полезно для локального запуска)
	if strings.HasPrefix(serverAddress, ":") {
		serverAddress = "localhost" + serverAddress
	}

	log.Printf("Подключение к серверу: %s", serverAddress)

	conn, err := grpc.Dial(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться: %v", err)
	}
	defer conn.Close()

	client := gen.NewChatServiceClient(conn)
	stream, err := client.Chat(context.Background())
	if err != nil {
		log.Fatalf("Ошибка создания стрима: %v", err)
	}

	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("\n[Система] Ошибка связи: %v", err)
				break
			}

			// Форматируем время в понятный вид (HH:MM:SS)
			t := in.CreatedAt.AsTime().Local()
			timeStr := t.Format("15:04:05")

			fmt.Printf("\r[%s] %s: %s\n> ", timeStr, in.User, in.Text)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Введите ваше имя: ")
	scanner.Scan()
	username := fixUtf8(strings.TrimSpace(scanner.Text()))

	fmt.Printf("Добро пожаловать, %s!\n> ", username)

	for scanner.Scan() {
		text := fixUtf8(strings.TrimSpace(scanner.Text()))
		if text == "" {
			fmt.Print("> ")
			continue
		}

		err := stream.Send(&gen.Message{
			User: username,
			Text: text,
			// CreatedAt заполнит сервер
		})
		if err != nil {
			log.Printf("Ошибка отправки: %v", err)
			return
		}
	}
}
