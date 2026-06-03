package main

// hermes — консольный клиент для управления Hermes Orchestrator
//
// Использование:
//   hermes chat                    — интерактивный чат с оркестратором
//   hermes chat "Привет"           — одноразовое сообщение
//   hermes agents                  — список локальных агентов
//   hermes agents remote           — список удалённых агентов
//   hermes agent create --preset developer --name "My Dev"  — создать агента
//   hermes agent delete <id>       — удалить агента
//   hermes remote task <agent> shell "ls -la"  — задача на удалённый агент
//   hermes remote status <agent>   — статус удалённого агента

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "LavenderMessenger/gen"
)

var (
	serverAddr = flag.String("server", getEnv("HERMES_SERVER", "localhost:50052"), "Orchestrator address")
	userID     = flag.String("user", getEnv("HERMES_USER", "cli-user"), "User ID")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if len(flag.Args()) == 0 {
		usage()
		os.Exit(1)
	}

	ctx := context.Background()

	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	cmd := flag.Arg(0)
	switch cmd {
	case "chat":
		handleChat(ctx, client)
	case "agents":
		handleAgents(ctx, client)
	case "agent":
		handleAgent(ctx, client)
	case "remote":
		handleRemote(ctx, client)
	case "presets":
		handlePresets(ctx, client)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

// ===== CHAT =====

func handleChat(ctx context.Context, client pb.ChatServiceClient) {
	args := flag.Args()[1:]

	if len(args) > 0 {
		// Одноразовое сообщение
		msg := strings.Join(args, " ")
		printStream(client, ctx, *userID, msg)
		return
	}

	// Интерактивный режим
	fmt.Println("Hermes Orchestrator chat. Type 'quit' to exit.")
	fmt.Println("─" + strings.Repeat("─", 50))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("You> ")
		if !scanner.Scan() {
			break
		}
		msg := scanner.Text()
		if strings.ToLower(msg) == "quit" || strings.ToLower(msg) == "exit" {
			fmt.Println("Bye!")
			break
		}
		if msg == "" {
			continue
		}
		printStream(client, ctx, *userID, msg)
		fmt.Println()
	}
}

func printStream(client pb.ChatServiceClient, ctx context.Context, userID, msg string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	stream, err := client.ChatWithOrchestrator(ctx, &pb.OrchestratorRequest{
		UserId:  userID,
		Message: msg,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Print("Hermes> ")
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("\n[stream error: %v]\n", err)
			break
		}
		fmt.Print(resp.Token)
		if resp.Finished {
			break
		}
	}
	fmt.Println()
}

// ===== AGENTS =====

func handleAgents(ctx context.Context, client pb.ChatServiceClient) {
	args := flag.Args()[1:]

	mode := "local"
	if len(args) > 0 {
		mode = args[0]
	}

	switch mode {
	case "remote", "r":
		listRemoteAgents(ctx, client)
	default:
		listLocalAgents(ctx, client)
	}
}

func listLocalAgents(ctx context.Context, client pb.ChatServiceClient) {
	resp, err := client.ListUserAgents(ctx, &pb.ListUserAgentsRequest{UserId: *userID})
	if err != nil {
		log.Fatalf("List agents error: %v", err)
	}

	if len(resp.Agents) == 0 {
		fmt.Println("No agents configured.")
		return
	}

	fmt.Println("Local agents:")
	fmt.Println("─" + strings.Repeat("─", 50))
	for _, a := range resp.Agents {
		fmt.Printf("  %-20s %s\n", a.Id, a.Name)
	}
}

func listRemoteAgents(ctx context.Context, client pb.ChatServiceClient) {
	resp, err := client.ListRemoteAgents(ctx, &pb.ListRemoteAgentsRequest{})
	if err != nil {
		log.Fatalf("List remote agents error: %v", err)
	}

	if len(resp.Agents) == 0 {
		fmt.Println("No remote agents connected.")
		return
	}

	fmt.Println("Remote agents:")
	fmt.Println("─" + strings.Repeat("─", 70))
	fmt.Printf("  %-15s %-15s %-10s %-12s %s\n", "ID", "NAME", "HOST", "STATUS", "CAPS")
	fmt.Println("─" + strings.Repeat("─", 70))
	for _, a := range resp.Agents {
		caps := strings.Join(a.Capabilities, ",")
		if len(caps) > 20 {
			caps = caps[:17] + "..."
		}
		fmt.Printf("  %-15s %-15s %-10s %-12s %s\n", a.Id, a.Name, a.Host, a.Status, caps)
	}
}

// ===== AGENT CRUD =====

func handleAgent(ctx context.Context, client pb.ChatServiceClient) {
	args := flag.Args()[1:]
	if len(args) == 0 {
		fmt.Println("Usage: hermes agent <create|delete|info> [options]")
		return
	}

	switch args[0] {
	case "create", "c":
		agentCreate(ctx, client, args[1:])
	case "delete", "d":
		agentDelete(ctx, client, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown agent command: %s\n", args[0])
	}
}

func agentCreate(ctx context.Context, client pb.ChatServiceClient, args []string) {
	fs := flag.NewFlagSet("agent create", flag.ContinueOnError)
	preset := fs.String("preset", "", "Preset ID (e.g. preset-developer)")
	name := fs.String("name", "", "Custom name")
	prompt := fs.String("prompt", "", "Custom system prompt")
	model := fs.String("model", "", "Model override")
	maxTokens := fs.Int("tokens", 0, "Max tokens (0 = from preset)")

	if err := fs.Parse(args); err != nil {
		return
	}

	if *preset == "" {
		fmt.Println("Usage: hermes agent create --preset <preset-id> [--name <name>] [--prompt <prompt>]")
		fmt.Println("\nAvailable presets:")
		handlePresets(ctx, client)
		return
	}

	resp, err := client.CreateAgent(ctx, &pb.CreateAgentRequest{
		UserId:      *userID,
		PresetId:    *preset,
		CustomName:  *name,
		CustomPrompt: *prompt,
		Model:       *model,
		MaxTokens:   int32(*maxTokens),
	})
	if err != nil {
		log.Fatalf("Create agent error: %v", err)
	}

	if resp.Success {
		fmt.Printf("Agent created: %s\n", resp.AgentId)
		if resp.Agent != nil {
			fmt.Printf("  Name: %s\n", resp.Agent.Name)
			fmt.Printf("  Description: %s\n", resp.Agent.Description)
		}
	} else {
		fmt.Printf("Error: %s\n", resp.Message)
	}
}

func agentDelete(ctx context.Context, client pb.ChatServiceClient, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: hermes agent delete <agent-id>")
		return
	}

	resp, err := client.DeleteAgent(ctx, &pb.DeleteAgentRequest{
		AgentId: args[0],
		UserId:  *userID,
	})
	if err != nil {
		log.Fatalf("Delete agent error: %v", err)
	}

	if resp.Success {
		fmt.Printf("Agent %s deleted.\n", args[0])
	} else {
		fmt.Printf("Error: %s\n", resp.Message)
	}
}

// ===== REMOTE =====

func handleRemote(ctx context.Context, client pb.ChatServiceClient) {
	args := flag.Args()[1:]
	if len(args) == 0 {
		fmt.Println("Usage: hermes remote <task|status> [options]")
		return
	}

	switch args[0] {
	case "task", "t":
		remoteTask(ctx, client, args[1:])
	case "status", "s":
		remoteStatus(ctx, client, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown remote command: %s\n", args[0])
	}
}

func remoteTask(ctx context.Context, client pb.ChatServiceClient, args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: hermes remote task <agent-id> <task-type> [params...]")
		fmt.Println("  task-types: shell, file_read, file_write, git, build, deploy, docker")
		fmt.Println("  params: key=value ...")
		return
	}

	agentID := args[0]
	taskType := args[1]
	params := make(map[string]string)

	for _, arg := range args[2:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		}
	}

	// Специальная обработка shell — объединить всё после task_type в command
	if taskType == "shell" && len(args) > 2 {
		params["command"] = strings.Join(args[2:], " ")
	}

	resp, err := client.DeployAgentTask(ctx, &pb.DeployAgentTaskRequest{
		AgentId:  agentID,
		TaskType: taskType,
		Params:   params,
	})
	if err != nil {
		log.Fatalf("Deploy task error: %v", err)
	}

	if resp.Success {
		fmt.Printf("Task sent: %s\n", resp.TaskId)
	} else {
		fmt.Printf("Error: %s\n", resp.Message)
	}
}

func remoteStatus(ctx context.Context, client pb.ChatServiceClient, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: hermes remote status <agent-id>")
		return
	}

	resp, err := client.GetRemoteAgentStatus(ctx, &pb.GetRemoteAgentStatusRequest{
		AgentId: args[0],
	})
	if err != nil {
		log.Fatalf("Get status error: %v", err)
	}

	fmt.Printf("Agent: %s (%s)\n", resp.Name, resp.Id)
	fmt.Printf("Host: %s\n", resp.Host)
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Active tasks: %d\n", resp.ActiveTasks)
	fmt.Printf("Capabilities: %s\n", strings.Join(resp.Capabilities, ", "))
	fmt.Printf("Last heartbeat: %s\n", resp.LastHeartbeat)
}

// ===== PRESETS =====

func handlePresets(ctx context.Context, client pb.ChatServiceClient) {
	resp, err := client.ListAgentPresets(ctx, &pb.ListAgentPresetsRequest{})
	if err != nil {
		log.Fatalf("List presets error: %v", err)
	}

	fmt.Println("Available presets:")
	fmt.Println("─" + strings.Repeat("─", 60))
	for _, p := range resp.Presets {
		fmt.Printf("  %-22s %-15s %s\n", p.Id, p.Name, p.Role)
	}
}

// ===== USAGE =====

func usage() {
	fmt.Println(`Hermes — CLI client for Hermes Orchestrator

Usage:
  hermes <command> [options]

Commands:
  chat                     Interactive chat with orchestrator
  chat "message"           Single message
  agents                   List local agents
  agents remote            List connected remote agents
  agent create             Create agent from preset
  agent delete <id>        Delete agent
  remote task <agent> <type> [params]   Send task to remote agent
  remote status <agent>    Show remote agent status
  presets                  List available agent presets

Options:
  -server <addr>    Orchestrator address (default: localhost:50052)
  -user <id>        User ID (default: cli-user)

Environment:
  HERMES_SERVER     Orchestrator address
  HERMES_USER       User ID

Examples:
  hermes chat
  hermes chat "Привет, как дела?"
  hermes agents remote
  hermes agent create --preset developer --name "Backend Dev"
  hermes remote task agent-001 shell "ls -la /root"
  hermes remote task agent-001 git operation=pull path=/root/msg`)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
