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
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"LavenderMessenger/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerAddress string `yaml:"server_address"`
	LastUsername  string `yaml:"last_username"`
}

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

func getConfigPaths() []string {
	var paths []string

	// Current working directory
	paths = append(paths, "config.yaml")

	// Relative to executable location
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(filename)
		paths = append(paths, filepath.Join(dir, "config.yaml"))
	}

	// Common project-relative paths
	paths = append(paths, "client/console/config.yaml")
	paths = append(paths, "../console/config.yaml")

	return paths
}

func loadConfig(paths []string) (*Config, error) {
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				continue
			}
			return &cfg, nil
		}
	}
	return nil, fmt.Errorf("config not found in any of the searched paths")
}

func main() {
	// Try multiple paths for config.yaml
	configPaths := getConfigPaths()

	cfg, err := loadConfig(configPaths)
	if err != nil {
		log.Fatalf("Config load error: %v", err)
	}

	serverAddress := cfg.ServerAddress
	if serverAddress == "" {
		serverAddress = "localhost:50051"
	}

	if strings.HasPrefix(serverAddress, ":") {
		serverAddress = "localhost" + serverAddress
	}

	log.Printf("Connecting to server: %s", serverAddress)

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

	// Goroutine for receiving messages
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
			// \r clears current input line ("> "), prints message, and returns "> "
			fmt.Printf("\r[%s] %s: %s\n> ", timeStr, in.User, in.Text)
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)

	// Use last_username from config as default, allow user to change it
	defaultName := cfg.LastUsername
	if defaultName == "" {
		defaultName = "Anonymous"
	}
	fmt.Printf("Enter your name [%s]: ", defaultName)
	if !scanner.Scan() {
		return
	}
	username := fixUtf8(strings.TrimSpace(scanner.Text()))
	if username == "" {
		username = defaultName
	}

	fmt.Printf("Welcome, %s! Type your message and press Enter.\n> ", username)

	for scanner.Scan() {
		text := fixUtf8(strings.TrimSpace(scanner.Text()))
		if text == "" {
			// Generate test message with random number
			text = "test message " + strconv.Itoa(rand.Intn(10000))
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
