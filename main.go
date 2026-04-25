// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This is the main entry point for the Lavender Messenger server.
// It handles gRPC connections, message routing, and database operations.

package main

import (
	"context"
	"fmt"     // Standard formatting package for console output
	"log"     // Standard logging package
	"net"     // Network functionality for TCP listener
	"os"      // Operating system interface for environment variables
	"strings" // String manipulation functions

	"LavenderMessenger/gen" // Generated gRPC code package

	"github.com/joho/godotenv" // Environment variable loading from .env files
	"google.golang.org/grpc"   // gRPC framework for RPC communication
	"google.golang.org/grpc/keepalive"
	"time"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

const (
	// serverVersion indicates the current version of the Lavender messaging server
	serverVersion = "1.0.1.58"
)

var firebaseApp *firebase.App

// main is the entry point of the Lavender messaging server application
// It initializes all necessary components: environment variables, database connection,
// gRPC server, and starts listening for client connections
func main() {
	// Print version at startup for visibility
	fmt.Printf("Lavender server version: %s\n", serverVersion)

	// Load environment variables from .env file for local development
	// If .env file doesn't exist, fall back to system environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it, using system environment variables")
	}

	// Initialize Firebase Admin SDK
	firebaseCredentials := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if firebaseCredentials == "" {
		firebaseCredentials = "lavender-messenger-firebase-adminsdk-fbsvc-1b8ed485d7.json"
	}
	opt := option.WithCredentialsFile(firebaseCredentials)
	var err error
	firebaseApp, err = firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Printf("Warning: Failed to initialize Firebase: %v (Push notifications will not work)", err)
	} else {
		log.Println("Firebase Admin SDK initialized successfully")
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
			if err := db.Close(); err != nil {
				log.Printf("Warning: failed to close database connection: %v", err)
			}
		}
	}()

	// Cleanup empty/corrupted messages from database
	// DISABLED: This function was deleting ALL messages instead of just empty ones
	// deleted, err := db.CleanupEmptyMessages()
	// if err != nil {
	// 	log.Printf("Warning: failed to cleanup empty messages: %v", err)
	// } else if deleted > 0 {
	// 	log.Printf("Cleaned up %d empty/corrupted messages", deleted)
	// }

	// Extract just the port number from serverAddress for lsof command
	portParts := strings.Split(serverAddress, ":")
	port := portParts[len(portParts)-1]
	if port == "" {
		port = "50051"
	}

	// Create TCP listener on the specified address for incoming connections
	lis, err := net.Listen("tcp", serverAddress)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			log.Fatalf("failed to listen: %v\n\nHint: Port %s is already in use. To fix:\n  lsof -ti:%s | xargs kill -9 2>/dev/null; go run .", err, port, port)
		}
		log.Fatalf("failed to listen: %v", err)
	}

	// Initialize a new gRPC server instance
	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // Minimum time between client pings
			PermitWithoutStream: true,            // Allow pings even without active streams
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  10 * time.Second, // Ping clients every 10s to keep connection alive
			Timeout:               5 * time.Second,  // Wait 5s for ping response
		}),
	)

	// Create our chat service instance with Hub for connection management
	// and database for message persistence
	srv := &server{
		hub:         NewHub(),    // Hub manages all active client connections
		db:          db,          // Database connection for storing messages
		firebaseApp: firebaseApp, // Firebase Admin SDK instance
	}

	// Register our chat service with the gRPC server
	// This makes the service available to clients
	gen.RegisterChatServiceServer(s, srv)

	// Log server startup information
	log.Printf("Listening clients at %v", lis.Addr())

	// Periodic online users broadcast (every 10 seconds for high reliability)
	go func() {
		for {
			time.Sleep(10 * time.Second)
			srv.broadcastOnlineUsers()
		}
	}()

	// Start HTTP server for avatar uploads in a goroutine
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8082"
	}
	go StartHTTPServer(httpPort)

	// Start HTTP server for APK updates in a goroutine
	apkPort := os.Getenv("APK_PORT")
	if apkPort == "" {
		apkPort = "8081"
	}
	go StartAPKServer(apkPort)

	// Start the gRPC server and begin serving client requests
	// This is a blocking call that runs until the application is terminated
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
