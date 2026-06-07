package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"LavenderMessenger/gen"
)

// ======= Bot Command Rate Limimiter =======

type botRateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func newBotRateLimiter(limit int, window time.Duration) *botRateLimiter {
	return &botRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *botRateLimiter) allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var valid []time.Time
	for _, t := range rl.requests[userID] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[userID] = valid
		return false
	}

	rl.requests[userID] = append(valid, now)
	return true
}

var botCmdRateLimiter = newBotRateLimiter(30, time.Minute)

// ======= Bot Command Registry =======

var botCommandList = []*gen.BotCommandInfo{
	{
		Command:     "/status",
		Description: "Статус сервера (CPU, RAM, uptime)",
		Usage:       "/status",
		Category:    "server",
	},
	{
		Command:     "/deploy",
		Description: "Деплой на dev или prod сервер",
		Usage:       "/deploy [dev|prod]",
		Category:    "server",
	},
	{
		Command:     "/logs",
		Description: "Последние N строк логов сервера",
		Usage:       "/logs [N]",
		Category:    "server",
	},
	{
		Command:     "/restart",
		Description: "Перезапуск dev сервера",
		Usage:       "/restart",
		Category:    "server",
	},
	{
		Command:     "/ai",
		Description: "Прямой запрос к OWL AI",
		Usage:       "/ai <сообщение>",
		Category:    "ai",
	},
	{
		Command:     "/help",
		Description: "Список всех доступных команд",
		Usage:       "/help",
		Category:    "system",
	},
	{
		Command:     "/version",
		Description: "Версия сервера и информация о сборке",
		Usage:       "/version",
		Category:    "system",
	},
}

// ======= Utility Functions =======

var serverStartTime = time.Now()

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dд %dч %dм", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dч %dм", hours, mins)
	}
	return fmt.Sprintf("%dм", mins)
}

func getLoadAverage() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "N/A"
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		return fmt.Sprintf("%s / %s / %s (1/5/15 min)", fields[0], fields[1], fields[2])
	}
	return "N/A"
}

func readLastLines(filename string, n int) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	// Remove last empty line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, "\n"), nil
}

// ======= Bot Command Handlers =======

func handleBotStatus(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	uptime := time.Since(serverStartTime)
	loadAvg := getLoadAverage()

	text := fmt.Sprintf("🦞 Статус сервера Lavender\n\n"+
		"📊 CPU Load: %s\n"+
		"💾 RAM: %d MB allocated / %d MB total\n"+
		"⏱ Uptime: %s\n"+
		"🔢 Goroutines: %d\n"+
		"📡 Version: %s\n",
		loadAvg,
		memStats.Alloc/1024/1024,
		memStats.Sys/1024/1024,
		formatDuration(uptime),
		runtime.NumGoroutine(),
		ServerVersion,
	)

	return &gen.BotCommandResponse{
		Success:      true,
		ResponseText: text,
	}
}

func handleBotVersion(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse {
	text := fmt.Sprintf("🦞 Lavender Messenger v%s\nGo: %s\nOS: %s/%s",
		ServerVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
	return &gen.BotCommandResponse{
		Success:      true,
		ResponseText: text,
	}
}

func handleBotHelp(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse {
	var result strings.Builder
	result.WriteString("📋 Доступные команды:\n\n")

	categories := map[string][]*gen.BotCommandInfo{}
	for _, cmd := range botCommandList {
		categories[cmd.Category] = append(categories[cmd.Category], cmd)
	}

	catNames := []struct {
		key  string
		name string
	}{
		{"server", "🖥 Сервер"},
		{"ai", "🤖 AI"},
		{"system", "⚙️ Система"},
	}

	for _, cat := range catNames {
		result.WriteString(cat.name + ":\n")
		for _, cmd := range categories[cat.key] {
			result.WriteString(fmt.Sprintf("  %s — %s\n", cmd.Command, cmd.Description))
		}
		result.WriteString("\n")
	}

	return &gen.BotCommandResponse{
		Success:      true,
		ResponseText: result.String(),
	}
}

func handleBotDeploy(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse {
	if !s.db.IsSuperAdmin(req.UserId) && !s.db.IsSuperAdmin(req.Username) {
		return &gen.BotCommandResponse{
			Success:      false,
			IsError:      true,
			ErrorMessage: "Доступ запрещён: только для супер-админа",
		}
	}

	target := "dev"
	if len(req.Args) > 0 {
		target = strings.ToLower(req.Args[0])
	}
	if target != "dev" && target != "prod" {
		return &gen.BotCommandResponse{
			Success:      false,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("Неизвестный таргет: %s (используйте dev или prod)", target),
		}
	}

	log.Printf("[BotCommand] Deploy to %s requested by %s", target, req.Username)

	go func() {
		script := "/root/scripts/deploy-dev.sh"
		if target == "prod" {
			script = "/root/scripts/deploy-prod.sh"
		}
		out, err := exec.Command("bash", script).CombinedOutput()
		if err != nil {
			log.Printf("[BotCommand] Deploy to %s failed: %v\nOutput: %s", target, err, string(out))
		} else {
			log.Printf("[BotCommand] Deploy to %s completed", target)
		}
	}()

	return &gen.BotCommandResponse{
		Success:      true,
		ResponseText: fmt.Sprintf("🚀 Деплой на %s сервер запущен...\nСтатус: выполняется", target),
	}
}

func handleBotRestart(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse {
	if !s.db.IsSuperAdmin(req.UserId) && !s.db.IsSuperAdmin(req.Username) {
		return &gen.BotCommandResponse{
			Success:      false,
			IsError:      true,
			ErrorMessage: "Доступ запрещён: только для супер-админа",
		}
	}

	log.Printf("[BotCommand] Restart requested by %s", req.Username)

	go func() {
		time.Sleep(2 * time.Second)
		log.Printf("[BotCommand] Restarting server via systemd...")
		exec.Command("systemctl", "restart", "lavender-server-dev").Run()
	}()

	return &gen.BotCommandResponse{
		Success:      true,
		ResponseText: "🔄 Сервер перезапускается через 2 секунды...",
	}
}

func handleBotLogs(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse {
	lines := 20
	if len(req.Args) > 0 {
		n, _ := fmt.Sscanf(req.Args[0], "%d", &lines)
		if n != 1 || lines <= 0 || lines > 100 {
			lines = 20
		}
	}

	logFile := "/root/LavenderMessenger/run/server.log"
	result, err := readLastLines(logFile, lines)
	if err != nil || result == "" {
		out, jerr := exec.Command("journalctl", "-u", "lavender-server-dev",
			"-n", fmt.Sprintf("%d"), "--no-pager", "-q").CombinedOutput()
		if jerr != nil || len(out) == 0 {
			return &gen.BotCommandResponse{
				Success:      true,
				ResponseText: fmt.Sprintf("⚠️ Не удалось прочитать логи: %v", err),
			}
		}
		result = string(out)
	}

	if result == "" {
		return &gen.BotCommandResponse{
			Success:      true,
			ResponseText: "📋 Логи пусты",
		}
	}

	return &gen.BotCommandResponse{
		Success:      true,
		ResponseText: fmt.Sprintf("📋 Последние %d строк логов:\n\n%s", lines, result),
	}
}

func handleBotAI(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse {
	message := strings.Join(req.Args, " ")
	if message == "" {
		return &gen.BotCommandResponse{
			Success:      false,
			IsError:      true,
			ErrorMessage: "Использование: /ai <сообщение>",
		}
	}

	// Use OWL rate limiter
	if !owlRateLimiter.allow(req.UserId) {
		return &gen.BotCommandResponse{
			Success:      false,
			IsError:      true,
			ErrorMessage: "Rate limit exceeded: max 10 AI requests per minute",
		}
	}

	systemPrompt := `Вы — AI-ассистент OWL в мессенджере Lavender. Отвечайте кратко и по делу на русском языке.
Вы можете помочь с программированием, настройкой серверов, ответами на вопросы.
Будьте дружелюбны, но лаконичны.`

	messages := []map[string]string{{"role": "user", "content": message}}

	log.Printf("[BotCommand] /ai from %s: %q", req.Username, message)

	response, err := callOpenRouterContext(context.Background(), s.owlApiKey, s.owlModel, systemPrompt, messages)
	if err != nil {
		log.Printf("[BotCommand] /ai error for %s: %v", req.Username, err)
		return &gen.BotCommandResponse{
			Success:      false,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("AI сервис недоступен: %v", err),
		}
	}

	return &gen.BotCommandResponse{
		Success:      true,
		ResponseText: response,
	}
}

// ======= Bot Command Dispatcher =======

var botCommandHandlers = map[string]func(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse{
	"/status":  handleBotStatus,
	"/version": handleBotVersion,
	"/help":    handleBotHelp,
	"/deploy":  handleBotDeploy,
	"/restart": handleBotRestart,
	"/logs":    handleBotLogs,
	"/ai":      handleBotAI,
}

func dispatchBotCommand(s *server, req *gen.BotCommandRequest) *gen.BotCommandResponse {
	// Rate limit check
	if !botCmdRateLimiter.allow(req.UserId) {
		return &gen.BotCommandResponse{
			Success:      false,
			IsError:      true,
			ErrorMessage: "Rate limit exceeded: max 30 commands per minute. Подождите немного.",
		}
	}

	cmd := strings.TrimSpace(strings.ToLower(req.Command))

	handler, ok := botCommandHandlers[cmd]
	if !ok {
		return &gen.BotCommandResponse{
			Success:      false,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("Неизвестная команда: %s. Введите /help для списка команд.", cmd),
		}
	}

	return handler(s, req)
}

// ======= Server-side RPC implementations =======

func (s *server) ProcessBotCommand(ctx context.Context, req *gen.BotCommandRequest) (*gen.BotCommandResponse, error) {
	return dispatchBotCommand(s, req), nil
}

func (s *server) GetBotCommands(ctx context.Context, req *gen.GetBotCommandsRequest) (*gen.GetBotCommandsResponse, error) {
	return &gen.GetBotCommandsResponse{
		Commands: botCommandList,
	}, nil
}

func (s *server) GetOWLStatus(ctx context.Context, req *gen.OWLStatusRequest) (*gen.OWLStatusResponse, error) {
	status := "ready"
	if s.owlApiKey == "" {
		status = "offline"
	}
	return &gen.OWLStatusResponse{
		Available: s.owlApiKey != "",
		Model:     s.owlModel,
		Status:    status,
	}, nil
}

// ======= Notification Service =======

type notificationService struct {
	mu          sync.Mutex
	subscribers map[string]map[chan *gen.ServerNotification]bool
	history     []*gen.ServerNotification
	maxHistory  int
}

var notifications = &notificationService{
	subscribers: make(map[string]map[chan *gen.ServerNotification]bool),
	maxHistory:  100,
}

func (ns *notificationService) subscribe(userID string, ch chan *gen.ServerNotification) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if ns.subscribers[userID] == nil {
		ns.subscribers[userID] = make(map[chan *gen.ServerNotification]bool)
	}
	ns.subscribers[userID][ch] = true
}

func (ns *notificationService) unsubscribe(userID string, ch chan *gen.ServerNotification) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	if subs, ok := ns.subscribers[userID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(ns.subscribers, userID)
		}
	}
	close(ch)
}

func (ns *notificationService) broadcast(notif *gen.ServerNotification) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	ns.history = append(ns.history, notif)
	if len(ns.history) > ns.maxHistory {
		ns.history = ns.history[len(ns.history)-ns.maxHistory:]
	}

	for _, subs := range ns.subscribers {
		for ch := range subs {
			select {
			case ch <- notif:
			default:
			}
		}
	}
}

func (ns *notificationService) getHistory(userID string, limit int32) []*gen.ServerNotification {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	start := len(ns.history) - int(limit)
	if start < 0 {
		start = 0
	}
	result := make([]*gen.ServerNotification, len(ns.history)-start)
	copy(result, ns.history[start:])
	return result
}

// SendServerNotification — helper to send a notification from anywhere
func SendServerNotification(notifType, title, message string, metadata map[string]string) {
	notif := &gen.ServerNotification{
		Id:        fmt.Sprintf("notif_%d", time.Now().UnixNano()),
		Type:      notifType,
		Title:     title,
		Message:   message,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Metadata:  metadata,
	}
	notifications.broadcast(notif)
	log.Printf("[Notification] %s: %s - %s", notifType, title, message)
}

// ======= Notification RPC implementations =======

func (s *server) SubscribeNotifications(req *gen.SubscribeNotificationsRequest, stream gen.ChatService_SubscribeNotificationsServer) error {
	ch := make(chan *gen.ServerNotification, 10)
	notifications.subscribe(req.UserId, ch)
	defer notifications.unsubscribe(req.UserId, ch)

	for {
		select {
		case notif := <-ch:
			if err := stream.Send(notif); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func (s *server) GetNotificationHistory(ctx context.Context, req *gen.GetNotificationHistoryRequest) (*gen.GetNotificationHistoryResponse, error) {
	history := notifications.getHistory(req.UserId, req.Limit)
	return &gen.GetNotificationHistoryResponse{
		Notifications: history,
	}, nil
}

func (s *server) MarkNotificationsRead(ctx context.Context, req *gen.MarkNotificationReadRequest) (*gen.MarkNotificationReadResponse, error) {
	return &gen.MarkNotificationReadResponse{Success: true}, nil
}
