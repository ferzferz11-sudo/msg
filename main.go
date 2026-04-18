// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This is the main entry point for the Lavender Messenger server.
// It handles gRPC connections, message routing, and database operations.

package main

import (
	"log" // Standard logging package
	"net" // Network functionality for TCP listener
	"os"  // Operating system interface for environment variables

	"LavenderMessenger/gen" // Generated gRPC code package

	"github.com/joho/godotenv" // Environment variable loading from .env files
	"google.golang.org/grpc"   // gRPC framework for RPC communication
)

const (
	// serverVersion indicates the current version of the Lavender messaging server
	serverVersion = "0.9.1"
)

// main is the entry point of the Lavender messaging server application
// It initializes all necessary components: environment variables, database connection,
// gRPC server, and starts listening for client connections
func main() {
	// Load environment variables from .env file for local development
	// If .env file doesn't exist, fall back to system environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it, using system environment variables")
	}

	// Read server address from environment variables
	// Falls back to default port 50051 if not specified
	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = ":50051" // Default gRPC port
	}

	// Establish database connection for message persistence
	db, err := ConnectDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	// Ensure database connection is closed when the application shuts down
	defer func() {
		if db != nil {
			db.Close()
		}
	}()

	// Create TCP listener on the specified address for incoming connections
	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Initialize a new gRPC server instance
	s := grpc.NewServer()

	// Create our chat service instance with Hub for connection management
	// and database for message persistence
	srv := &server{
		hub: NewHub(), // Hub manages all active client connections
		db:  db,       // Database connection for storing messages
	}

	// Register our chat service with the gRPC server
	// This makes the service available to clients
	gen.RegisterChatServiceServer(s, srv)

	// Log server startup information
	log.Printf("Lavender server version: %s", serverVersion)
	log.Printf("Listening clients at %v", lis.Addr())

	// Start the gRPC server and begin serving client requests
	// This is a blocking call that runs until the application is terminated
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
