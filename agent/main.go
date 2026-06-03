package main

// hermes-agent — демон удалённого агента для Hermes Orchestrator
// Запускается на удалённом сервере, подключается к оркестратору через gRPC
//
// Использование:
//   hermes-agent --orchestrator=host:port --agent-id=agent-001 --token=secret
//
// Переменные окружения:
//   HERMES_ORCHESTRATOR — адрес оркестратора (host:port)
//   HERMES_AGENT_ID — ID агента
//   HERMES_AUTH_TOKEN — токен аутентификации
//   HERMES_CAPABILITIES — список возможностей (shell,git,build,deploy,docker)

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	pb "LavenderMessenger/gen/hermes_agent"
)

var (
	orchestratorAddr = flag.String("orchestrator", getEnv("HERMES_ORCHESTRATOR", "localhost:50051"), "Orchestrator address (host:port)")
	agentID          = flag.String("agent-id", getEnv("HERMES_AGENT_ID", ""), "Agent ID")
	authToken        = flag.String("token", getEnv("HERMES_AUTH_TOKEN", ""), "Auth token")
	capabilitiesStr  = flag.String("capabilities", getEnv("HERMES_CAPABILITIES", "shell,file"), "Comma-separated capabilities")
	agentName        = flag.String("name", getEnv("HERMES_AGENT_NAME", ""), "Agent name (default: hostname)")
)

// Agent — структура агента
type Agent struct {
	id            string
	name          string
	version       string
	host          string
	ipAddress     string
	os            string
	capabilities  []string
	authToken     string
	status        string
	activeTasks   int
	maxConcurrent int

	conn       *grpc.ClientConn
	client     pb.HermesAgentServiceClient
	stream     pb.HermesAgentService_ConnectClient
	streamMu   sync.Mutex
	tasks      map[string]*AgentTask
	tasksMu    sync.RWMutex
}

type AgentTask struct {
	ID         string
	Type       string
	Cmd        *exec.Cmd
	Cancel     context.CancelFunc
}

func main() {
	flag.Parse()

	if *agentID == "" {
		log.Fatal("agent-id is required")
	}

	hostname, _ := os.Hostname()
	name := *agentName
	if name == "" {
		name = hostname
	}

	agent := &Agent{
		id:            *agentID,
		name:          name,
		version:       "1.0.0",
		host:          hostname,
		ipAddress:     getLocalIP(),
		os:            runtime.GOOS,
		capabilities:  strings.Split(*capabilitiesStr, ","),
		authToken:     *authToken,
		status:        "starting",
		maxConcurrent: 3,
		tasks:         make(map[string]*AgentTask),
	}

	log.Printf("[Agent] starting: id=%s name=%s host=%s caps=%v",
		agent.id, agent.name, agent.host, agent.capabilities)

	if err := agent.connect(); err != nil {
		log.Fatalf("[Agent] connection failed: %v", err)
	}
	defer agent.conn.Close()

	agent.status = "connected"
	log.Printf("[Agent] connected to orchestrator at %s", *orchestratorAddr)

	if err := agent.register(); err != nil {
		log.Fatalf("[Agent] registration failed: %v", err)
	}

	go agent.heartbeatLoop()
	agent.receiveLoop()
}

func (a *Agent) connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, *orchestratorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	a.conn = conn
	a.client = pb.NewHermesAgentServiceClient(conn)

	stream, err := a.client.Connect(context.Background())
	if err != nil {
		conn.Close()
		return fmt.Errorf("connect stream failed: %w", err)
	}
	a.stream = stream
	return nil
}

func (a *Agent) register() error {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	info := &pb.RegistrationInfo{
		AgentId:      a.id,
		AgentName:    a.name,
		Version:      a.version,
		Host:         a.host,
		IpAddress:    a.ipAddress,
		Os:           a.os,
		Capabilities: a.capabilities,
		AuthToken:    a.authToken,
	}

	payload, _ := proto.Marshal(info)

	return a.stream.Send(&pb.AgentMessage{
		AgentId:     a.id,
		Type:        pb.AgentMessageType_AGENT_REGISTER,
		Payload:     payload,
		TimestampMs: time.Now().UnixMilli(),
	})
}

func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		a.streamMu.Lock()
		err := a.stream.Send(&pb.AgentMessage{
			AgentId:     a.id,
			Type:        pb.AgentMessageType_AGENT_HEARTBEAT,
			TimestampMs: time.Now().UnixMilli(),
		})
		a.streamMu.Unlock()

		if err != nil {
			log.Printf("[Agent] heartbeat error: %v", err)
			a.reconnect()
			return
		}
	}
}

func (a *Agent) receiveLoop() {
	for {
		msg, err := a.stream.Recv()
		if err == io.EOF {
			log.Printf("[Agent] stream closed by orchestrator")
			a.reconnect()
			return
		}
		if err != nil {
			log.Printf("[Agent] receive error: %v", err)
			a.reconnect()
			return
		}

		switch msg.Type {
		case pb.OrchestratorMessageType_ORCHESTRATOR_TASK:
			var task pb.Task
			if err := proto.Unmarshal(msg.Payload, &task); err != nil {
				log.Printf("[Agent] task unmarshal error: %v", err)
				continue
			}
			go a.executeTask(&task)

		case pb.OrchestratorMessageType_ORCHESTRATOR_PING:
			a.streamMu.Lock()
			a.stream.Send(&pb.AgentMessage{
				AgentId:     a.id,
				Type:        pb.AgentMessageType_AGENT_HEARTBEAT,
				TimestampMs: time.Now().UnixMilli(),
			})
			a.streamMu.Unlock()

		case pb.OrchestratorMessageType_ORCHESTRATOR_DISCONNECT:
			log.Printf("[Agent] disconnect requested by orchestrator")
			a.status = "disconnected"
			return

		case pb.OrchestratorMessageType_ORCHESTRATOR_CONFIG_UPDATE:
			var config pb.AgentConfig
			if err := proto.Unmarshal(msg.Payload, &config); err != nil {
				log.Printf("[Agent] config unmarshal error: %v", err)
				continue
			}
			a.applyConfig(&config)
		}
	}
}

func (a *Agent) executeTask(task *pb.Task) {
	log.Printf("[Agent] executing task: id=%s type=%s", task.TaskId, task.TaskType)

	a.tasksMu.Lock()
	if a.activeTasks >= a.maxConcurrent {
		a.tasksMu.Unlock()
		a.sendTaskResult(task.TaskId, "error", "", "agent is busy", -1, 0)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.TimeoutSec)*time.Second)
	if task.TimeoutSec <= 0 {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	}

	agentTask := &AgentTask{
		ID:     task.TaskId,
		Type:   task.TaskType.String(),
		Cancel: cancel,
	}
	a.tasks[task.TaskId] = agentTask
	a.activeTasks++
	a.tasksMu.Unlock()
	var stdoutStr, stderrStr string
	var exitCode int
	var execErr error

	startTime := time.Now()

	switch task.TaskType {
	case pb.TaskType_TASK_SHELL:
		stdoutStr, stderrStr, exitCode, execErr = a.executeShell(ctx, task.Params, task.WorkingDir)
	case pb.TaskType_TASK_FILE_READ:
		stdoutStr, stderrStr, exitCode, execErr = a.executeFileRead(ctx, task.Params)
	case pb.TaskType_TASK_FILE_WRITE:
		stdoutStr, stderrStr, exitCode, execErr = a.executeFileWrite(ctx, task.Params)
	case pb.TaskType_TASK_GIT:
		stdoutStr, stderrStr, exitCode, execErr = a.executeGit(ctx, task.Params, task.WorkingDir)
	case pb.TaskType_TASK_BUILD:
		stdoutStr, stderrStr, exitCode, execErr = a.executeBuild(ctx, task.Params, task.WorkingDir)
	case pb.TaskType_TASK_DEPLOY:
		stdoutStr, stderrStr, exitCode, execErr = a.executeDeploy(ctx, task.Params, task.WorkingDir)
	case pb.TaskType_TASK_DOCKER:
		stdoutStr, stderrStr, exitCode, execErr = a.executeDocker(ctx, task.Params, task.WorkingDir)
	default:
		execErr = fmt.Errorf("unknown task type: %s", task.TaskType)
	}

	status := "success"
	if execErr != nil {
		status = "error"
	}
	if ctx.Err() == context.DeadlineExceeded {
		status = "timeout"
	}

	duration := time.Since(startTime)
	a.sendTaskResult(task.TaskId, status, stdoutStr, stderrStr, exitCode, duration.Milliseconds())

	a.tasksMu.Lock()
	delete(a.tasks, task.TaskId)
	a.activeTasks--
	a.tasksMu.Unlock()

	cancel()
}

func (a *Agent) executeShell(ctx context.Context, params map[string]string, workingDir string) (string, string, int, error) {
	cmdStr, ok := params["command"]
	if !ok {
		return "", "", -1, fmt.Errorf("command parameter required")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdout.String(), stderr.String(), exitCode, err
}

func (a *Agent) executeFileRead(ctx context.Context, params map[string]string) (string, string, int, error) {
	path, ok := params["path"]
	if !ok {
		return "", "", -1, fmt.Errorf("path parameter required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err.Error(), -1, err
	}

	return string(data), "", 0, nil
}

func (a *Agent) executeFileWrite(ctx context.Context, params map[string]string) (string, string, int, error) {
	path, ok := params["path"]
	if !ok {
		return "", "", -1, fmt.Errorf("path parameter required")
	}
	content, ok := params["content"]
	if !ok {
		return "", "", -1, fmt.Errorf("content parameter required")
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", err.Error(), -1, err
	}

	return "OK", "", 0, nil
}

func (a *Agent) executeGit(ctx context.Context, params map[string]string, workingDir string) (string, string, int, error) {
	operation, ok := params["operation"]
	if !ok {
		return "", "", -1, fmt.Errorf("operation parameter required")
	}

	var args []string
	switch operation {
	case "clone":
		args = []string{"clone", params["url"]}
	case "pull":
		args = []string{"pull"}
	case "commit":
		args = []string{"commit", "-m", params["message"]}
	case "push":
		args = []string{"push"}
	case "status":
		args = []string{"status"}
	case "log":
		args = []string{"log", "--oneline", "-20"}
	default:
		return "", "", -1, fmt.Errorf("unknown git operation: %s", operation)
	}

	return a.executeShell(ctx, map[string]string{"command": strings.Join(args, " ")}, workingDir)
}

func (a *Agent) executeBuild(ctx context.Context, params map[string]string, workingDir string) (string, string, int, error) {
	buildCmd := params["command"]
	if buildCmd == "" {
		buildCmd = "go build"
	}
	return a.executeShell(ctx, map[string]string{"command": buildCmd}, workingDir)
}

func (a *Agent) executeDeploy(ctx context.Context, params map[string]string, workingDir string) (string, string, int, error) {
	deployCmd, ok := params["command"]
	if !ok {
		return "", "", -1, fmt.Errorf("deploy command required")
	}
	return a.executeShell(ctx, map[string]string{"command": deployCmd}, workingDir)
}

func (a *Agent) executeDocker(ctx context.Context, params map[string]string, workingDir string) (string, string, int, error) {
	dockerCmd, ok := params["command"]
	if !ok {
		return "", "", -1, fmt.Errorf("docker command required")
	}
	return a.executeShell(ctx, map[string]string{"command": dockerCmd}, workingDir)
}

func (a *Agent) sendTaskResult(taskID, statusStr, stdout, stderr string, exitCode int, durationMs int64) {
	var taskStatus pb.TaskStatus
	switch statusStr {
	case "success":
		taskStatus = pb.TaskStatus_TASK_SUCCESS
	case "error":
		taskStatus = pb.TaskStatus_TASK_ERROR
	case "timeout":
		taskStatus = pb.TaskStatus_TASK_TIMEOUT
	default:
		taskStatus = pb.TaskStatus_TASK_STATUS_UNKNOWN
	}

	result := &pb.TaskResult{
		TaskId:     taskID,
		Status:     taskStatus,
		Stdout:     stdout,
		Stderr:     stderr,
		ExitCode:   int32(exitCode),
		DurationMs: durationMs,
	}

	payload, _ := proto.Marshal(result)

	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	err := a.stream.Send(&pb.AgentMessage{
		AgentId:     a.id,
		Type:        pb.AgentMessageType_AGENT_TASK_RESULT,
		Payload:     payload,
		TimestampMs: time.Now().UnixMilli(),
	})

	if err != nil {
		log.Printf("[Agent] send result error: %v", err)
	}
}

func (a *Agent) applyConfig(config *pb.AgentConfig) {
	if config.MaxConcurrentTasks > 0 {
		a.maxConcurrent = int(config.MaxConcurrentTasks)
	}
	log.Printf("[Agent] config updated: max_concurrent=%d", a.maxConcurrent)
}

func (a *Agent) reconnect() {
	a.status = "reconnecting"
	log.Printf("[Agent] reconnecting...")

	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		time.Sleep(backoff)

		if err := a.connect(); err != nil {
			log.Printf("[Agent] reconnect failed: %v", err)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		if err := a.register(); err != nil {
			log.Printf("[Agent] re-registration failed: %v", err)
			a.conn.Close()
			continue
		}

		a.status = "connected"
		log.Printf("[Agent] reconnected")
		go a.receiveLoop()
		return
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "unknown"
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
