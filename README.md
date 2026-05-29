# Lavender Messenger — Server

**Author:** Pavel Davydov (ferz)  
**Version:** 1.0.7.1  
**Language:** Go 1.26  

gRPC server with AES-256 encryption, PostgreSQL storage, Firebase push notifications.

## Repository

This is the **server** repository.  
Android client lives in a separate repo: `ferzferz11-sudo/msg.client.android`

## Running

```bash
go run .
# or build:
go build -o lavender-server .
```

**If port 50051 is in use:**
```bash
lsof -ti:50051 | xargs kill -9 2>/dev/null; go run .
```

## Project Structure

```
msg/                              # Server repo root
├── main.go                       # Entry point: gRPC + HTTP servers, env loading
├── server.go                     # Core gRPC handlers (chat, users, admin)
├── secret_chat.go                # E2EE handlers (ECDH + AES-256-GCM)
├── server_management.go          # Server management RPC methods
├── db.go                         # PostgreSQL: connection, migrations, queries
├── hub.go                        # Client connection hub (broadcast, register)
├── http_server.go                # HTTP servers (port 8081 APK, 8082 uploads)
├── crypto.go                     # AES-256-GCM + bcrypt
├── email.go                      # Email notification support
├── messenger.proto               # gRPC protocol definition
├── server.proto                  # Server management protocol
├── gen/                          # Generated protobuf Go code
│   ├── messenger.pb.go
│   ├── messenger_grpc.pb.go
│   ├── server.pb.go
│   └── server_grpc.pb.go
├── config.yaml                   # gRPC config (address, TLS)
├── .env                          # Runtime config (DB, secret key) — NOT committed
├── .env.example                  # Config template
├── go.mod / go.sum               # Go module
├── deploy.sh                     # Deploy to remote server
├── start.sh                      # Local startup
├── monitor.sh                    # Health check cron script
├── db_maintenance.sh             # DB cleanup utilities
├── check_message.sh              # Message debugging
├── get_last_messages.sh          # Fetch recent messages
├── CHANGELOG.md                  # Version history
├── README.md                     # This file
├── SKILLS_AND_COMMANDS.md        # Dev commands reference
├── PROJECT_MEMORY.md             # Project architecture notes
└── uploads/                      # File upload storage
    ├── audio/
    ├── avatars/
    ├── background/
    ├── files/
    └── images/
```

## Architecture

- **Protocol:** gRPC bidirectional streaming (Chat, Typing, CallSession) + unary RPCs
- **Database:** PostgreSQL with connection pooling
- **Encryption:** AES-256-GCM for messages, bcrypt for passwords, ECDH for E2EE
- **Push:** Firebase Cloud Messaging
- **HTTP Servers:** Port 8081 (APK distribution), 8082 (file uploads)
- **Keepalive:** 15s interval, 10s timeout

## Proto Regeneration

After editing `.proto` files — regenerate both:

```bash
export PATH=$PATH:/root/go/bin
protoc --go_out=gen --go_opt=paths=source_relative \
       --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
       messenger.proto server.proto
```

**Important:** Run both `messenger.proto` AND `server.proto` together.

## Environment Variables (.env)

- `DATABASE_URL` — PostgreSQL connection string (Supabase pooler port 6543 or localhost:5432)
- `CHAT_SECRET_KEY` — 32-byte AES key
- `FIREBASE_CREDENTIALS_PATH` — Firebase admin SDK JSON path
- `GRPC_ADDRESS` — gRPC listen address (default `:50051`)

## Deployment

User deploys manually via `./deploy.sh`.  
Servers:

| Server | IP | Role |
|--------|-----|------|
| Development | 13.140.25.249:50051 | Dev/testing |
| Production | 159.195.38.145:50051 | Production (old) |
| Local | 192.168.1.135:50051 | Dev laptop |

## Branches

- `main` — production (159.195.38.145)
- `feat/remove-username-compat` — new server (13.140.25.249)
