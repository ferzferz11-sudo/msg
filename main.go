// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This is the main entry point for the Lavender Messenger server.
// It handles gRPC connections, message routing, and database operations.

package main

import (
	"context"
	"fmt"     // Standard formatting package for console output
	"net"     // Network functionality for TCP listener
	"os"      // Operating system interface for environment variables
	"os/signal"
	"strings" // String manipulation functions
	"syscall"

	"LavenderMessenger/gen" // Generated gRPC code package
	hermesagent "LavenderMessenger/gen/hermes_agent"

	"github.com/joho/godotenv" // Environment variable loading from .env files
	"google.golang.org/grpc"   // gRPC framework for RPC communication
	"google.golang.org/grpc/keepalive"
	"time"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

var firebaseApp *firebase.App

// OWL AI assistant globals
var (
	owlRateLimiter   *rateLimiter
	owlSessions      *owlSessionManager
	hermesSettings   *hermesSettingsManager
)

// main is the entry point of the Lavender messaging server application
// It initializes all necessary components: environment variables, database connection,
// gRPC server, and starts listening for client connections
func main() {
	// Print version at startup for visibility
	fmt.Printf("Lavender server version: %s\n", ServerVersion)

	// Load environment variables from .env file for local development
	// If .env file doesn't exist, fall back to system environment variables
	appEnv := os.Getenv("APP_ENV")
	if appEnv != "" {
		if err := godotenv.Load(".env." + appEnv); err != nil {
			logger.Warnf("No .env.%s file found, trying .env", appEnv)
			godotenv.Load()
		}
	} else {
		godotenv.Load()
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
		logger.Warnf("Warning: Failed to initialize Firebase: %v (Push notifications will not work)", err)
	} else {
		logger.Info("Firebase Admin SDK initialized successfully")
	}

	// Read server address from environment variables
	// Falls back to default port 50051 if not specified
	serverAddress := os.Getenv("SERVER_ADDRESS")
	logger.Infof("SERVER_ADDRESS from env: '%s'", serverAddress)
	if serverAddress == "" {
		serverAddress = ":50051" // Default gRPC port
		logger.Infof("Using default server address: %s", serverAddress)
	}

	// Establish database connection for message persistence
	db, err := ConnectDB()
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
	}
	// Ensure database connection is closed when the application shuts down
	defer func() {
		if db != nil {
			if err := db.Close(); err != nil {
				logger.Warnf("Warning: failed to close database connection: %v", err)
			}
		}
	}()

	// Cleanup empty/corrupted messages from database
	// DISABLED: This function was deleting ALL messages instead of just empty ones
	// deleted, err := db.CleanupEmptyMessages()
	// if err != nil {
	// 	logger.Warnf("Warning: failed to cleanup empty messages: %v", err)
	// } else if deleted > 0 {
	// 	logger.Infof("Cleaned up %d empty/corrupted messages", deleted)
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
			logger.Errorf("failed to listen: %v\n\nHint: Port %s is already in use. To fix:\n  lsof -ti:%s | xargs kill -9 2>/dev/null; go run .", err, port, port)
		}
		logger.Errorf("failed to listen: %v", err)
		return
	}

	// Initialize a new gRPC server instance with auth interceptors
	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second, // Minimum time between client pings
			PermitWithoutStream: true,            // Allow pings even without active streams
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  20 * time.Second, // Ping clients every 20s
			Timeout:               20 * time.Second, // Allow 20s for response (lenient for emulators/mobile)
		}),
		grpc.UnaryInterceptor(AuthInterceptor),
		grpc.StreamInterceptor(AuthStreamInterceptor),
	)

	// Create our chat service instance with Hub for connection management
	// and database for message persistence
	srv := &server{
		db:          db,          // Database connection for storing messages
		firebaseApp: firebaseApp, // Firebase Admin SDK instance
		owlModel:    os.Getenv("OPENROUTER_MODEL"),
		owlApiKey:   os.Getenv("OPENROUTER_API_KEY"),
	}
	srv.hub = NewHub(srv.broadcastOnlineUsers) // Hub manages all active client connections

	// Initialize OWL (AI assistant)
	owlRateLimiter = newRateLimiter(10, time.Minute)
	owlSessions = newOwlSessionManager(db.DB, 50)
	hermesSettings = newHermesSettingsManager(db.DB)
	logger.Info("OWL AI assistant initialized (rate limit: 10 req/min, history: 50 msgs, DB-backed)")

	// Initialize AI Chat Manager (unified for OWL + Hermes)
	aiChatManager := NewAIChatManager(db.DB)
	srv.aiChatManager = aiChatManager
	logger.Info("AI Chat Manager initialized (unified sessions, messages, settings)")

	// Initialize Hermes Multi-Agent Orchestrator
	hermesRegistry := NewAgentRegistry(db.DB)
	srv.hermesDB = NewHermesDB(db.DB)
	orchestrator := NewOrchestrator(hermesRegistry, db.DB, os.Getenv("OPENROUTER_API_KEY"), os.Getenv("OPENROUTER_MODEL"))
	srv.hermesOrchestrator = orchestrator
	logger.Infof("Hermes Orchestrator initialized with %d agents", len(hermesRegistry.GetAll()))

	// Run Hermes DB migrations
	runHermesMigrations(db.DB)

	// Create/migrate auth v2 tables (user_devices, device_auth_log)
	if err := db.MigrateDeviceTables(); err != nil {
		logger.Warnf("Warning: failed to migrate auth v2 tables: %v", err)
	} else {
		logger.Info("Auth v2 tables (user_devices, device_auth_log) ready")
	}

	// Register our chat service with the gRPC server
	gen.RegisterChatServiceServer(s, srv)

	// Register Auth Service (v1 legacy + v2 JWT)
	authServer := newAuthServerV2(db)
	gen.RegisterAuthServiceServer(s, authServer)

	// Register Hermes Agent Service (for hermes-agent daemon connections)
	hermesAgentServer := newHermesAgentServer(srv, orchestrator)
	hermesagent.RegisterHermesAgentServiceServer(s, hermesAgentServer)

	// Register ProfileService v2 (JWT-only, dev server only)
	if appEnv == "dev" {
		profileServer := newProfileServerV2(db)
		gen.RegisterProfileServiceServer(s, profileServer)
		logger.Info("ProfileService v2 registered (dev)")
	}

	// Register server management service (only dev)
	if appEnv == "dev" {
		srvMgmt := &serverServiceServer{db: db}
		gen.RegisterServerServiceServer(s, srvMgmt)
		logger.Info("ServerService registered (dev)")
	}

	// Log server startup information
	logger.Infof("Listening clients at %v", lis.Addr())

	// Periodic online users broadcast (every 60 seconds as a heartbeat)
	go func() {
		for {
			time.Sleep(60 * time.Second)
			srv.broadcastOnlineUsers()
		}
	}()

	// Start HTTP server for avatar uploads in a goroutine
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8082"
	}
	go StartHTTPServer(httpPort)

	// Graceful shutdown on SIGINT/SIGTERM
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Infof("Received signal %v, shutting down gracefully...", sig)
		s.GracefulStop()
	}()

	// Start the gRPC server and begin serving client requests
	// This is a blocking call that runs until the application is terminated
	if err := s.Serve(lis); err != nil {
		logger.Errorf("failed to serve: %v", err)
	}
}
