// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This is the main entry point for the Lavender Messenger server.
// It handles gRPC connections, message routing, and database operations.

package main

import (
	"context"
	"fmt" // Standard formatting package for console output
	"net" // Network functionality for TCP listener
	"os"  // Operating system interface for environment variables
	"os/signal"
	"strings" // String manipulation functions
	"syscall"

	"LavenderMessenger/auth" // Agent JWT token management
	"LavenderMessenger/gen"  // Generated gRPC code package
	hermesagent "LavenderMessenger/gen/hermes_agent"

	"time"

	"github.com/joho/godotenv" // Environment variable loading from .env files
	"google.golang.org/grpc"   // gRPC framework for RPC communication
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

var firebaseApp *firebase.App

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

	// Validate critical secrets at startup
	if secret := os.Getenv("JWT_SECRET"); len(secret) < 32 {
		logger.Fatal("FATAL: JWT_SECRET is missing or too short (must be >= 32 bytes). Set JWT_SECRET in .env or environment.")
	}
	if key := os.Getenv("CHAT_SECRET_KEY"); len(key) < 32 {
		logger.Fatal("FATAL: CHAT_SECRET_KEY is missing or too short (must be >= 32 bytes). Set CHAT_SECRET_KEY in .env or environment.")
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
		if ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second); cancel != nil {
			defer cancel()
			if client, err := firebaseApp.Messaging(ctx); err != nil {
				logger.Errorf("CRITICAL: Firebase credentials invalid: %v — push notifications will NOT work!", err)
				logger.Errorf("Fix: replace %s with a valid service account key from Firebase Console", firebaseCredentials)
			} else {
				_ = client
				logger.Info("Firebase credentials validated successfully")
			}
		}
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
		return
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
			MaxConnectionIdle:     30 * time.Minute,
			MaxConnectionAge:      2 * time.Hour,
			MaxConnectionAgeGrace: 30 * time.Second,
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
	srv.pushDebouncer = NewPushDebouncer(srv.sendBatchPushNotifications)

	// Initialize Hermes DB
	srv.hermesDB = NewHermesDB(db.DB)
	srv.remoteAgentManager = &RemoteAgentManager{agents: make(map[string]*RemoteAgent)}

	// Inject DB into auth package for token revocation
	auth.SetDB(db.DB)

	// Set agent revocation check hook (queries agent_tokens.revoked)
	if srv.hermesDB != nil {
		auth.SetAgentRevocationCheck(srv.hermesDB.IsAgentRevoked)
	}

	// Initialize AI Gateway v2
	srv.aiGateway = NewAIGateway(db.DB)
	logger.Info("AI Gateway v2 initialized")

	// Run Hermes DB migrations
	runHermesMigrations(db.DB)

	// Create v2 AI tables
	if err := MigrateAIV2(db.DB); err != nil {
		logger.Errorf("Failed to migrate AI v2 tables: %v", err)
	} else {
		logger.Info("AI v2 tables ready")
	}

	// Create/migrate auth v2 tables (user_devices, device_auth_log)
	if err := db.MigrateDeviceTables(); err != nil {
		logger.Warnf("Warning: failed to migrate auth v2 tables: %v", err)
	} else {
		logger.Info("Auth v2 tables (user_devices, device_auth_log) ready")
	}

	// OIDC tables
	if os.Getenv("OIDC_ENABLED") != "false" {
		if err := runOIDCMigrations(db.DB); err != nil {
			logger.Warnf("Warning: failed to migrate OIDC tables: %v", err)
		}
		// Init OIDC keys
		if err := initOIDCKeys(); err != nil {
			logger.Warnf("Warning: failed to init OIDC keys: %v", err)
		}
	}

	// Register our chat service with the gRPC server
	gen.RegisterChatServiceServer(s, srv)

	// Register Auth Service (v2 JWT)
	authServer := newAuthServerV2(db)
	gen.RegisterAuthServiceServer(s, authServer)

	// Register Hermes Agent Service (for hermes-agent daemon connections)
	hermesAgentServer := newHermesAgentServer(srv, &Orchestrator{remoteManager: srv.remoteAgentManager})
	hermesagent.RegisterHermesAgentServiceServer(s, hermesAgentServer)

	// Register ProfileService v2 (JWT-only)
	profileServer := newProfileServerV2(db)
	gen.RegisterProfileServiceServer(s, profileServer)
	ProfileServiceVersion = "2.0"
	logger.Info("ProfileService v2 registered")

	// Register CompanyService
	companyServer := newCompanyServer(db)
	gen.RegisterCompanyServiceServer(s, companyServer)
	logger.Info("CompanyService registered")

	// Register StickerService
	stickerServer := newStickerServer(db)
	gen.RegisterStickerServiceServer(s, stickerServer)
	logger.Info("StickerService registered")

	// Register server management service (only dev)
	if appEnv == "dev" {
		srvMgmt := &serverServiceServer{db: db}
		gen.RegisterServerServiceServer(s, srvMgmt)
		logger.Info("ServerService registered (dev)")
	}

	// Enable gRPC server reflection (for grpcurl, debugging) — dev only
	if appEnv == "dev" {
		reflection.Register(s)
		logger.Info("gRPC reflection enabled (dev)")
	}

	// Log server startup information
	logger.Infof("Listening clients at %v", lis.Addr())

	// Background goroutines with cancellation on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Periodic online users broadcast (every 60 seconds as a heartbeat)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.broadcastOnlineUsers()
				srv.cleanupRecentMsgs()
			}
		}
	}()

	// Periodic device auth log cleanup (every 24 hours)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.db.CleanupDeviceAuthLog()
			}
		}
	}()

	// Rate limiter cleanup (every 5 minutes)
	go authLimiter.cleanup(ctx)

	// Self-destruct message cleanup (every 30 seconds)
	srv.startSelfDestructCleanup(ctx)

	// Deleted messages cleanup (every hour)
	srv.startDeletedMessagesCleanup(ctx)

	// Revoked token cleanup (every 1 hour)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				db.Exec(`DELETE FROM revoked_tokens WHERE expires_at < NOW()`)
			}
		}
	}()

	// Start HTTP server for avatar uploads
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8082"
	}
	SetHTTPDependencies(db, srv.hub)
	httpSrv := StartHTTPServerAndReturn(httpPort)

	// Graceful shutdown on SIGINT/SIGTERM
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Infof("Received signal %v, shutting down gracefully...", sig)

		// Set shutting down flag
		srv.isShuttingDown.Store(true)
		httpShuttingDown.Store(true)

		// Notify all connected clients before stopping — send twice with 500ms gap
		// to increase delivery probability on slow/unstable connections
		srv.hub.BroadcastShutdown()
		logger.Info("Sent SERVER_SHUTTINGDOWN to all clients (attempt 1)")
		time.Sleep(500 * time.Millisecond)
		srv.hub.BroadcastShutdown()
		logger.Info("Sent SERVER_SHUTTINGDOWN to all clients (attempt 2)")
		time.Sleep(2500 * time.Millisecond) // Total grace period: 3s

		// Cancel background goroutines
		cancel()

		// Shutdown HTTP server first (drain in-flight requests)
		if httpSrv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := httpSrv.Shutdown(ctx); err != nil {
				logger.Warnf("HTTP server shutdown error: %v", err)
			} else {
				logger.Info("HTTP server stopped")
			}
		}

		// Shutdown gRPC server with timeout
		go func() {
			time.Sleep(30 * time.Second)
			logger.Warn("Forcing gRPC shutdown after 30s timeout")
			s.Stop()
		}()
		s.GracefulStop()
	}()

	// Start the gRPC server and begin serving client requests
	// This is a blocking call that runs until the application is terminated
	if err := s.Serve(lis); err != nil {
		logger.Errorf("failed to serve: %v", err)
	}
}
