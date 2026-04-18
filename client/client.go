// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements a command-line client for the Lavender Messenger.
// It handles gRPC communication with the server and user input/output.

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
	"google.golang.org/grpc/credentials/insecure"
)

func fixUtf8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.Map(func(r rune) rune {
		if r == utf8.RuneError {
			return -1
		}
		return r
	}, s)
}

func main() {
	_ = godotenv.Load("../.env")

	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = "localhost:50051"
	}

	if strings.HasPrefix(serverAddress, ":") {
		serverAddress = "localhost" + serverAddress
	}

	log.Printf("Connecting to server: %s", serverAddress)

	// Обновлено: grpc.NewClient вместо устаревшего Dial
	conn, err := grpc.NewClient(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("error closing connection: %v", err)
		}
	}()

	client := gen.NewChatServiceClient(conn)
	stream, err := client.Chat(context.Background())
	if err != nil {
		log.Fatalf("Error creating stream: %v", err)
	}

	// Горутина для получения сообщений
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				fmt.Println("\n[System] Server closed connection.")
				break
			}
			if err != nil {
				log.Printf("\n[System] Connection error: %v", err)
				break
			}

			timeStr := in.CreatedAt.AsTime().Local().Format("15:04:05")
			// \r очищает текущую строку ввода ("> "), печатает сообщение и возвращает "> "
			fmt.Printf("\r[%s] %s: %s\n> ", timeStr, in.User, in.Text)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Enter your name: ")
	if !scanner.Scan() {
		return
	}
	username := fixUtf8(strings.TrimSpace(scanner.Text()))
	if username == "" {
		username = "Anonymous"
	}

	fmt.Printf("Welcome, %s! Type your message and press Enter.\n> ", username)

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
			log.Printf("Send error: %v", err)
			return
		}
	}
}
