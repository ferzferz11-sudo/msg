// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements a console client for the Lavender Messenger.
// It provides a simple command-line interface for chatting.

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

	"LavenderMessenger/gen"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	// Загружаем .env
	if err := godotenv.Load("../../.env"); err != nil {
		// Try loading from project root if relative path fails
		if err := godotenv.Load(".env"); err != nil {
			log.Printf("Warning: Could not load .env file: %v", err)
		}
	}

	// Читаем настройки из .env
	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = "localhost:50051"
	}
	useTLS := os.Getenv("USE_TLS") == "true"

	log.Printf("Подключение к серверу: %s (TLS: %v)", serverAddress, useTLS)

	var opts []grpc.DialOption
	if useTLS {
		// Используем системные сертификаты для безопасного подключения к Render (порт 443)
		creds := credentials.NewClientTLSFromCert(nil, "")
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		// Обычное подключение для локальной разработки
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(serverAddress, opts...)
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

			// Обрабатываем время (сервер присылает google.protobuf.Timestamp)
			var timeStr string
			if in.CreatedAt != nil {
				timeStr = in.CreatedAt.AsTime().Local().Format("15:04:05")
			} else {
				timeStr = "--:--:--"
			}

			fmt.Printf("\r[%s] %s: %s\n> ", timeStr, in.User, in.Text)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Введите ваше имя: ")
	if !scanner.Scan() {
		return
	}
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
		})
		if err != nil {
			log.Printf("Ошибка отправки: %v", err)
			return
		}
	}
}
