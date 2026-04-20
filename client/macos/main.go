// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements a macOS GUI client for the Lavender Messenger.
// It provides a graphical interface using Fyne framework with themes and emojis.

package main

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"LavenderMessenger/gen"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

const clientVersion = "1.0.1.27"

var myWindow fyne.Window
var chatBox *widget.RichText
var connectToServer func(string)
var avatarCache = make(map[string]string) // Cache for avatar URLs
var avatarCacheMutex sync.Mutex           // Mutex for thread-safe avatarCache access

func getConfigPaths() []string {
	var paths []string
	// Сначала проверяем директорию, где находится main.go
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(filename)
		paths = append(paths, filepath.Join(dir, "config.yaml"))
	}
	// Затем проверяем текущую директорию
	paths = append(paths, "config.yaml")
	// И другие возможные пути
	paths = append(paths, "client/macos/config.yaml")
	paths = append(paths, "../macos/config.yaml")
	return paths
}

type Config struct {
	ServerAddress string `yaml:"server_address"`
	Themes        struct {
		Light ThemeConfig `yaml:"light"`
		Dark  ThemeConfig `yaml:"dark"`
	} `yaml:"themes"`
	CurrentTheme string `yaml:"current_theme"`
	LastUsername string `yaml:"last_username"`
	LastPassword string `yaml:"last_password"`
}

type ThemeConfig struct {
	BgColor    string   `yaml:"bg_color"`
	TextColor  string   `yaml:"text_color"`
	NameColor  string   `yaml:"name_color"`
	TimeColor  string   `yaml:"time_color"`
	UserColors []string `yaml:"user_colors"`
}

// loadConfig загружает конфигурацию из YAML файла
func loadConfig() (*Config, error) {
	paths := getConfigPaths()
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
	return nil, fmt.Errorf("config not found")
}

// saveConfig сохраняет конфигурацию в YAML файл
func saveConfig(cfg *Config) error {
	// Try to save to first writable path
	paths := getConfigPaths()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.WriteFile(path, data, 0644); err == nil {
			return nil
		}
	}
	return fmt.Errorf("could not write config to any path")
}

// Вспомогательная функция для парсинга HEX в color.RGBA
func parseHexColor(s string) color.Color {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.Transparent
	}
	var r, g, b uint8
	_, _ = fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// Кастомные имена цветов для пользователей
const (
	UserColor1  fyne.ThemeColorName = "UserColor1"
	UserColor2  fyne.ThemeColorName = "UserColor2"
	UserColor3  fyne.ThemeColorName = "UserColor3"
	UserColor4  fyne.ThemeColorName = "UserColor4"
	UserColor5  fyne.ThemeColorName = "UserColor5"
	UserColor6  fyne.ThemeColorName = "UserColor6"
	UserColor7  fyne.ThemeColorName = "UserColor7"
	UserColor8  fyne.ThemeColorName = "UserColor8"
	UserColor9  fyne.ThemeColorName = "UserColor9"
	UserColor10 fyne.ThemeColorName = "UserColor10"
)

// customTheme для поддержки своих цветов
type customTheme struct {
	isDark                    bool
	lightBg, darkBg           color.Color
	lightFg, darkFg           color.Color
	lightPrimary, darkPrimary color.Color
	lightTime, darkTime       color.Color
	userColors                []color.Color
}

func (c *customTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameBackground {
		if c.isDark {
			return c.darkBg
		}
		return c.lightBg
	}
	if name == theme.ColorNameForeground {
		if c.isDark {
			return c.darkFg
		}
		return c.lightFg
	}
	if name == theme.ColorNamePrimary {
		if c.isDark {
			return c.darkPrimary
		}
		return c.lightPrimary
	}
	if name == theme.ColorNameDisabled {
		if c.isDark {
			return c.darkTime
		}
		return c.lightTime
	}

	// Обработка кастомных цветов пользователей
	if len(c.userColors) > 0 {
		switch name {
		case UserColor1:
			return c.userColors[0%len(c.userColors)]
		case UserColor2:
			return c.userColors[1%len(c.userColors)]
		case UserColor3:
			return c.userColors[2%len(c.userColors)]
		case UserColor4:
			return c.userColors[3%len(c.userColors)]
		case UserColor5:
			return c.userColors[4%len(c.userColors)]
		case UserColor6:
			return c.userColors[5%len(c.userColors)]
		case UserColor7:
			return c.userColors[6%len(c.userColors)]
		case UserColor8:
			return c.userColors[7%len(c.userColors)]
		case UserColor9:
			return c.userColors[8%len(c.userColors)]
		case UserColor10:
			return c.userColors[9%len(c.userColors)]
		}
	}

	// Для остальных используем дефолтную тему
	if c.isDark {
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
	return theme.DefaultTheme().Color(name, theme.VariantLight)
}

func (c *customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (c *customTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (c *customTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// getChats получает список чатов пользователя
func getChats(client gen.ChatServiceClient, username string) ([]*gen.ChatInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetChats(ctx, &gen.GetChatsRequest{Username: username})
	if err != nil {
		return nil, err
	}
	return resp.Chats, nil
}

// getUserAvatar loads user avatar from server
func getUserAvatar(client gen.ChatServiceClient, username string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.GetUserAvatar(ctx, &gen.GetUserAvatarRequest{Username: username})
	if err != nil {
		return "", err
	}
	return resp.AvatarUrl, nil
}

// loadHistory загружает историю сообщений для комнаты
func loadHistory(client gen.ChatServiceClient, roomId string, appendMsg func(string, string, string, string, string, string, []*gen.Reaction), clearMsgIDs func()) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Clear message IDs before loading new history
	clearMsgIDs()

	resp, err := client.GetHistory(ctx, &gen.GetHistoryRequest{Room: roomId})
	if err != nil {
		return err
	}

	// Collect unique users from history
	usersToLoad := make(map[string]bool)
	for _, msg := range resp.Messages {
		if msg.User != "" {
			usersToLoad[msg.User] = true
		}
	}

	// Load avatars for all users in history
	for user := range usersToLoad {
		avatarCacheMutex.Lock()
		needsLoad := avatarCache[user] == ""
		avatarCacheMutex.Unlock()

		if needsLoad {
			go func(username string) {
				avatarURL, err := getUserAvatar(client, username)
				if err == nil && avatarURL != "" {
					avatarCacheMutex.Lock()
					avatarCache[username] = avatarURL
					avatarCacheMutex.Unlock()
				}
			}(user)
		}
	}

	for _, msg := range resp.Messages {
		// Skip join messages
		if strings.HasSuffix(msg.Text, " joined") || strings.HasSuffix(msg.Text, " присоединился") {
			continue
		}
		t := msg.CreatedAt.AsTime().Local()
		timeStr := t.Format("15:04:05")
		appendMsg(timeStr, msg.User, msg.Text, msg.Id, msg.RepliedToUser, msg.RepliedToText, msg.Reactions)
	}
	return nil
}

// showChatList показывает диалог со списком чатов
func showChatList(chats []*gen.ChatInfo, switchToChat func(string)) {
	chatList := container.NewVBox()
	var chatDialog *dialog.CustomDialog

	// Check if general chat is already in the list
	hasGeneral := false
	for _, chat := range chats {
		if chat.Id == "general" {
			hasGeneral = true
			break
		}
	}

	for _, chat := range chats {
		chatName := chat.Name
		if chatName == "" {
			chatName = chat.Id
		}
		chatType := chat.Type
		if chatType == "" {
			chatType = "general"
		}

		// Create unread count badge
		var unreadBadge fyne.CanvasObject
		if chat.UnreadCount > 0 {
			badgeLabel := widget.NewLabel(fmt.Sprintf("%d", chat.UnreadCount))
			badgeLabel.TextStyle = fyne.TextStyle{Bold: true}
			unreadBadge = badgeLabel
		} else {
			unreadBadge = widget.NewLabel("")
		}

		// Create participant avatars (simplified to avoid crashes)
		var avatarsText string
		if chat.Participants != "" {
			// Try to parse as JSON array first
			participants := []string{}
			if strings.HasPrefix(chat.Participants, "[") {
				// JSON format
				cleanJSON := strings.ReplaceAll(chat.Participants, "[", "")
				cleanJSON = strings.ReplaceAll(cleanJSON, "]", "")
				cleanJSON = strings.ReplaceAll(cleanJSON, "\"", "")
				if cleanJSON != "" {
					participants = strings.Split(cleanJSON, ",")
				}
			} else {
				// Comma-separated format
				participants = strings.Split(chat.Participants, ",")
			}

			maxAvatars := 3
			for i, participant := range participants {
				if i >= maxAvatars {
					break
				}
				participant = strings.TrimSpace(participant)
				if participant != "" {
					avatarsText += "👤"
				}
			}
			if len(participants) > maxAvatars {
				avatarsText += fmt.Sprintf("+%d", len(participants)-maxAvatars)
			}
		}

		// Create display text with unread count and avatars
		displayText := fmt.Sprintf("%s %s %s", unreadBadge.(*widget.Label).Text, chatName, avatarsText)

		btn := widget.NewButton(displayText, func() {
			// Switch to selected chat
			switchToChat(chat.Id)
			// Close dialog
			if chatDialog != nil {
				chatDialog.Hide()
			}
		})
		btn.Importance = widget.LowImportance
		chatList.Add(btn)
	}

	// Add button for general chat only if it's not already in the list
	if !hasGeneral {
		generalBtn := widget.NewButton("Общий чат", func() {
			switchToChat("general")
			// Close dialog
			if chatDialog != nil {
				chatDialog.Hide()
			}
		})
		generalBtn.Importance = widget.LowImportance
		chatList.Add(generalBtn)
	}

	scroll := container.NewVScroll(chatList)
	scroll.SetMinSize(fyne.NewSize(600, 500))
	chatDialog = dialog.NewCustom("Выберите чат", "Закрыть", scroll, myWindow)
	chatDialog.Resize(fyne.NewSize(800, 600))
	chatDialog.Show()
}

// showUsersList показывает диалог со списком всех пользователей с индикаторами онлайн статуса
func showUsersList(client gen.ChatServiceClient, currentUsername string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get all users
	allUsersResp, err := client.GetAllUsers(ctx, &gen.GetAllUsersRequest{})
	if err != nil {
		dialog.ShowError(err, myWindow)
		return
	}

	// Get online users
	onlineUsersResp, err := client.GetClients(ctx, &gen.ClientListRequest{})
	if err != nil {
		dialog.ShowError(err, myWindow)
		return
	}

	// Create a set of online users for quick lookup
	onlineUsersSet := make(map[string]bool)
	for _, user := range onlineUsersResp.Clients {
		onlineUsersSet[user] = true
	}

	usersList := container.NewVBox()
	userCount := 0

	for _, user := range allUsersResp.Users {
		if user == currentUsername {
			continue // Skip current user
		}

		userLabel := widget.NewLabel(user)
		userLabel.TextStyle = fyne.TextStyle{Bold: true}

		// Create green circle indicator for online users
		var statusIndicator fyne.CanvasObject
		if onlineUsersSet[user] {
			statusIndicator = canvas.NewCircle(color.RGBA{R: 0, G: 255, B: 0, A: 255}) // Green
			statusIndicator.Resize(fyne.NewSize(12, 12))
		} else {
			statusIndicator = canvas.NewCircle(color.RGBA{R: 128, G: 128, B: 128, A: 255}) // Gray
			statusIndicator.Resize(fyne.NewSize(12, 12))
		}

		btn := widget.NewButton("", func() {
			// Create direct chat with selected user
			createDirectChat(client, currentUsername, user)
		})
		btn.Importance = widget.LowImportance

		userRow := container.NewHBox(statusIndicator, userLabel, btn)
		usersList.Add(userRow)
		userCount++
	}

	if userCount == 0 {
		usersList.Add(widget.NewLabel("Нет зарегистрированных пользователей"))
	}

	scroll := container.NewVScroll(usersList)
	scroll.SetMinSize(fyne.NewSize(600, 500))
	d := dialog.NewCustom("Все пользователи (кликните для создания чата)", "Закрыть", scroll, myWindow)
	d.Resize(fyne.NewSize(800, 600))
	d.Show()
}

// createDirectChat создает прямую комнату с выбранным пользователем
func createDirectChat(client gen.ChatServiceClient, user1, user2 string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.CreateDirectChat(ctx, &gen.CreateDirectChatRequest{User1: user1, User2: user2})
	if err != nil {
		dialog.ShowError(err, myWindow)
		return
	}

	if resp.Success {
		dialog.ShowInformation("Успех", fmt.Sprintf("Чат с %s создан! ID: %s", user2, resp.ChatId), myWindow)
		// Switch to the newly created chat
		switchToChat(resp.ChatId)
	} else {
		dialog.ShowError(errors.New("Не удалось создать чат"), myWindow)
	}
}

// switchToChat переключается на другой чат
func switchToChat(roomId string) {
	// Clear chat
	chatBox.Segments = []widget.RichTextSegment{}
	chatBox.Refresh()
	// Reconnect to new room
	connectToServer(roomId)
}

// checkServerAvailability проверяет доступность gRPC сервера
func checkServerAvailability(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("не удалось инициализировать подключение к серверу %s: %v", addr, err)
	}
	defer func() { _ = conn.Close() }()

	client := gen.NewChatServiceClient(conn)
	_, err = client.Chat(ctx)
	if err != nil && !strings.Contains(err.Error(), "stream") && !strings.Contains(err.Error(), "EOF") {
		return fmt.Errorf("сервер %s недоступен: %v", addr, err)
	}

	return nil
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		cfg = &Config{}
	}

	// Читаем адрес сервера из конфига, с фоллбэком на localhost
	serverAddress := cfg.ServerAddress
	if serverAddress == "" {
		serverAddress = "localhost:50051"
	}

	isDarkTheme := cfg.CurrentTheme == "dark"
	activeThemeConfig := cfg.Themes.Light
	if isDarkTheme {
		activeThemeConfig = cfg.Themes.Dark
	}

	myTheme := &customTheme{
		isDark:       isDarkTheme,
		lightBg:      parseHexColor(cfg.Themes.Light.BgColor),
		darkBg:       parseHexColor(cfg.Themes.Dark.BgColor),
		lightFg:      parseHexColor(cfg.Themes.Light.TextColor),
		darkFg:       parseHexColor(cfg.Themes.Dark.TextColor),
		lightPrimary: parseHexColor(cfg.Themes.Light.NameColor),
		darkPrimary:  parseHexColor(cfg.Themes.Dark.NameColor),
		lightTime:    parseHexColor(cfg.Themes.Light.TimeColor),
		darkTime:     parseHexColor(cfg.Themes.Dark.TimeColor),
	}
	for _, c := range activeThemeConfig.UserColors {
		myTheme.userColors = append(myTheme.userColors, parseHexColor(c))
	}

	myApp := app.New()
	myApp.Settings().SetTheme(myTheme)

	myWindow = myApp.NewWindow(fmt.Sprintf("Lavender Messenger v%s", clientVersion))
	myWindow.Resize(fyne.NewSize(1200, 800))

	var username string
	var password string
	var currentRoomId string
	var stream gen.ChatService_ChatClient
	var conn *grpc.ClientConn

	// UI для статуса (индикатор и текст)
	statusIndicator := canvas.NewCircle(color.RGBA{R: 255, G: 255, B: 0, A: 255}) // Желтый
	statusIndicator.Resize(fyne.NewSize(12, 12))

	statusLabel := widget.NewLabel("Подключение...")
	statusLabel.Alignment = fyne.TextAlignLeading

	statusBox := container.NewHBox(statusIndicator, statusLabel)

	var lastUser string
	var userColorMap = make(map[string]fyne.ThemeColorName)
	var userColorIndex int
	var messageIDs = make(map[string]bool) // Track message IDs to avoid duplicates

	getUserColorName := func(user string) fyne.ThemeColorName {
		if colorName, exists := userColorMap[user]; exists {
			return colorName
		}
		if len(myTheme.userColors) == 0 {
			return theme.ColorNamePrimary
		}
		var colorName fyne.ThemeColorName
		switch userColorIndex % 10 {
		case 0:
			colorName = UserColor1
		case 1:
			colorName = UserColor2
		case 2:
			colorName = UserColor3
		case 3:
			colorName = UserColor4
		case 4:
			colorName = UserColor5
		case 5:
			colorName = UserColor6
		case 6:
			colorName = UserColor7
		case 7:
			colorName = UserColor8
		case 8:
			colorName = UserColor9
		case 9:
			colorName = UserColor10
		}
		userColorMap[user] = colorName
		userColorIndex++
		return colorName
	}

	chatBox = widget.NewRichText()
	scrollContainer := container.NewVScroll(chatBox)

	inputBox := widget.NewEntry()
	inputBox.SetPlaceHolder("Введите сообщение...")

	var themeBtn *widget.Button
	themeBtn = widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), func() {
		myTheme.isDark = !myTheme.isDark
		if myTheme.isDark {
			themeBtn.SetIcon(theme.VisibilityOffIcon())
			cfg.CurrentTheme = "dark"
		} else {
			themeBtn.SetIcon(theme.ColorPaletteIcon())
			cfg.CurrentTheme = "light"
		}
		myApp.Settings().SetTheme(myTheme)
		chatBox.Refresh()
		_ = saveConfig(cfg)
	})
	if myTheme.isDark {
		themeBtn.SetIcon(theme.VisibilityOffIcon())
	} else {
		themeBtn.SetIcon(theme.ColorPaletteIcon())
	}

	// Chat list button
	chatListBtn := widget.NewButtonWithIcon("Чаты", theme.ListIcon(), func() {
		// Show chat list
		if conn != nil {
			client := gen.NewChatServiceClient(conn)
			chats, err := getChats(client, username)
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			showChatList(chats, switchToChat)
		} else {
			dialog.ShowError(errors.New("Сначала подключитесь к серверу"), myWindow)
		}
	})

	// Users list button (all users with online status)
	usersBtn := widget.NewButtonWithIcon("Пользователи", theme.AccountIcon(), func() {
		// Show all users list with online status
		if conn != nil {
			client := gen.NewChatServiceClient(conn)
			showUsersList(client, username)
		} else {
			dialog.ShowError(errors.New("Сначала подключитесь к серверу"), myWindow)
		}
	})

	rightButtons := container.NewHBox(chatListBtn, usersBtn, themeBtn)
	topBar := container.NewBorder(nil, nil, statusBox, rightButtons)

	appendMessage := func(timeStr, user, text string, msgID string, repliedToUser, repliedToText string, reactions []*gen.Reaction) {
		// Skip all join messages (not just for current user)
		if strings.HasSuffix(text, " joined") || strings.HasSuffix(text, " присоединился") {
			return
		}

		// Skip duplicate messages by ID
		if msgID != "" {
			if messageIDs[msgID] {
				return
			}
			messageIDs[msgID] = true
		}

		isSameUser := lastUser == user
		lastUser = user
		if !isSameUser {
			chatBox.Segments = append(chatBox.Segments, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyleInline})
		}

		// Show reply quote if present
		if repliedToUser != "" && repliedToText != "" {
			replyStyle := widget.RichTextStyle{ColorName: theme.ColorNameDisabled, TextStyle: fyne.TextStyle{Italic: true}}
			replySeg := &widget.TextSegment{Text: fmt.Sprintf("↳ %s: %s\n", repliedToUser, repliedToText), Style: replyStyle}
			chatBox.Segments = append(chatBox.Segments, replySeg)
		}

		// Load user avatar if not in cache
		avatarCacheMutex.Lock()
		needsLoad := avatarCache[user] == ""
		avatarCacheMutex.Unlock()

		if needsLoad && conn != nil {
			go func(username string) {
				client := gen.NewChatServiceClient(conn)
				avatarURL, err := getUserAvatar(client, username)
				if err == nil && avatarURL != "" {
					avatarCacheMutex.Lock()
					avatarCache[username] = avatarURL
					avatarCacheMutex.Unlock()
				}
			}(user)
		}

		// Always show avatar emoji
		avatarEmoji := "👤"

		avatarCacheMutex.Lock()
		hasAvatar := avatarCache[user] != ""
		avatarCacheMutex.Unlock()

		if hasAvatar {
			avatarEmoji = "👤"
		}

		headerStyle := widget.RichTextStyle{ColorName: getUserColorName(user), TextStyle: fyne.TextStyle{Bold: true}}
		headerSeg := &widget.TextSegment{Text: fmt.Sprintf("%s %s%s: ", timeStr, avatarEmoji, user), Style: headerStyle}
		textSeg := &widget.TextSegment{Text: text + "\n", Style: widget.RichTextStyleInline}
		if isSameUser {
			chatBox.Segments = append(chatBox.Segments, textSeg)
		} else {
			chatBox.Segments = append(chatBox.Segments, headerSeg, textSeg)
		}

		// Show reactions if present
		if len(reactions) > 0 {
			reactionMap := make(map[string]int)
			for _, r := range reactions {
				reactionMap[r.Emoji]++
			}
			var reactionStrs []string
			for emoji, count := range reactionMap {
				reactionStrs = append(reactionStrs, fmt.Sprintf("%s %d", emoji, count))
			}
			reactionStyle := widget.RichTextStyle{ColorName: theme.ColorNamePrimary}
			reactionSeg := &widget.TextSegment{Text: "  " + strings.Join(reactionStrs, " ") + "\n", Style: reactionStyle}
			chatBox.Segments = append(chatBox.Segments, reactionSeg)
		}

		chatBox.Refresh()
		scrollContainer.ScrollToBottom()
	}

	safeAppendSystemMessage := func(msg string) {
		fyne.Do(func() {
			seg := &widget.TextSegment{Text: msg + "\n", Style: widget.RichTextStyle{ColorName: theme.ColorNameError}}
			chatBox.Segments = append(chatBox.Segments, seg)
			chatBox.Refresh()
			scrollContainer.ScrollToBottom()
		})
	}

	sendMsg := func() {
		text := inputBox.Text
		if text == "" || stream == nil {
			return
		}
		err := stream.Send(&gen.Message{User: username, Text: text, RoomId: currentRoomId})
		if err != nil {
			safeAppendSystemMessage(fmt.Sprintf("[Ошибка отправки]: %v", err))
		}
		inputBox.SetText("")
	}

	sendBtn := widget.NewButton("Отправить", sendMsg)

	// Emoji popup
	emojis := []string{"😀", "😃", "😄", "😁", "😅", "😂", "🤣", "😊", "😇", "🙂", "😉", "😌", "😍", "🥰", "😘", "😗", "😙", "😚", "😋", "😛", "😜", "🤪", "😝", "🤗", "🤭", "🤫", "🤔", "🤐", "🤨", "😐", "😑", "😶", "😏", "😒", "🙄", "😬", "🤥", "😌", "😔", "😪", "🤤", "😴", "😷", "🤒", "🤕", "🤢", "🤮", "🤧", "🥵", "🥶", "🥴", "😵", "🤯", "🤠", "🥳", "😎", "🤓", "🧐", "😕", "😟", "🙁", "☹️", "😮", "😯", "😲", "😳", "🥺", "😦", "😧", "😨", "😰", "😥", "😢", "😭", "😱", "😖", "😣", "😞", "😓", "😩", "😫", "🥱", "😤", "😡", "😠", "🤬", "😈", "👿", "💀", "☠️", "💩", "🤡", "👹", "👺", "👻", "👽", "👾", "🤖", "❤️", "🧡", "💛", "💚", "💙", "💜", "🖤", "🤍", "🤎", "💔", "❣️", "💕", "💞", "💓", "💗", "💖", "💘", "💝", "👍", "👎", "👌", "✌️", "🤞", "🤟", "🤘", "🤙", "👈", "👉", "👆", "👇", "☝️", "✋", "🤚", "🖐", "🖖", "👋", "🤙", "💪", "🙏"}
	var emojiPopup *widget.PopUp
	emojiGrid := container.NewGridWithColumns(8)
	for _, emoji := range emojis {
		e := emoji
		emojiGrid.Add(widget.NewButton(e, func() {
			inputBox.SetText(inputBox.Text + e)
			emojiPopup.Hide()
		}))
	}
	emojiPopup = widget.NewPopUp(container.NewVScroll(emojiGrid), myWindow.Canvas())
	emojiPopup.Hide()

	emojiBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		emojiPopup.Show()
	})

	inputContainer := container.NewBorder(nil, nil, emojiBtn, sendBtn, inputBox)
	centerContent := container.NewBorder(topBar, nil, nil, nil, scrollContainer)
	mainLayout := container.NewBorder(nil, inputContainer, nil, nil, centerContent)

	myWindow.SetContent(mainLayout)
	inputBox.OnSubmitted = func(s string) { sendMsg() }

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Ваше имя")
	if cfg.LastUsername != "" {
		usernameEntry.SetText(cfg.LastUsername)
	}

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Пароль")
	if cfg.LastPassword != "" {
		passwordEntry.SetText(cfg.LastPassword)
	}

	var loginForm *dialog.ConfirmDialog

	connectToServer = func(roomId string) {
		var err error
		conn, err = grpc.NewClient(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		client := gen.NewChatServiceClient(conn)
		stream, err = client.Chat(context.Background())
		if err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		currentRoomId = roomId

		// Clear message IDs before loading new room
		messageIDs = make(map[string]bool)

		// Load message history
		err = loadHistory(client, roomId, appendMessage, func() {})
		if err != nil {
			safeAppendSystemMessage(fmt.Sprintf("[Ошибка загрузки истории]: %v", err))
		}

		// Send auth/join message with password and roomId
		joinMessage := fmt.Sprintf("%s присоединился", username)
		err = stream.Send(&gen.Message{User: username, Text: joinMessage, Password: password, RoomId: roomId})
		if err != nil {
			safeAppendSystemMessage(fmt.Sprintf("[Ошибка отправки]: %v", err))
		}

		statusLabel.SetText(fmt.Sprintf("Подключено к %s | %s", serverAddress, username))
		statusIndicator.FillColor = color.RGBA{R: 0, G: 255, B: 0, A: 255} // Зеленый
		statusIndicator.Refresh()

		go func() {
			defer func() {
				fyne.Do(func() {
					statusLabel.SetText("Соединение потеряно. Перезапустите клиент.")
					statusIndicator.FillColor = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Красный
					statusIndicator.Refresh()
					inputBox.Disable()
					sendBtn.Disable()
					emojiBtn.Disable()
				})
			}()
			for {
				in, err := stream.Recv()
				if err != nil {
					return // Выход из горутины при любой ошибке
				}
				t := in.CreatedAt.AsTime().Local()
				timeStr := t.Format("15:04:05")
				fyne.Do(func() {
					appendMessage(timeStr, in.User, in.Text, in.Id, in.RepliedToUser, in.RepliedToText, in.Reactions)
				})
			}
		}()
	}

	loginForm = dialog.NewCustomConfirm("Вход в чат", "Войти", "Отмена",
		container.NewVBox(
			widget.NewLabel("Имя пользователя:"),
			usernameEntry,
			widget.NewLabel("Пароль:"),
			passwordEntry,
		),
		func(b bool) {
			if b && usernameEntry.Text != "" {
				username = usernameEntry.Text
				password = passwordEntry.Text
				cfg.LastUsername = username
				cfg.LastPassword = password
				_ = saveConfig(cfg)

				// Connect to server first
				var err error
				conn, err = grpc.NewClient(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					dialog.ShowError(err, myWindow)
					myApp.Quit()
					return
				}
				client := gen.NewChatServiceClient(conn)

				// Load current user avatar
				go func() {
					avatarURL, err := getUserAvatar(client, username)
					if err == nil && avatarURL != "" {
						avatarCacheMutex.Lock()
						avatarCache[username] = avatarURL
						avatarCacheMutex.Unlock()
					}
				}()

				// Get chats
				chats, err := getChats(client, username)
				if err != nil {
					dialog.ShowError(fmt.Errorf("Ошибка получения чатов: %v", err), myWindow)
					// If error, just connect to general
					connectToServer("general")
					return
				}

				if len(chats) == 0 {
					// No chats, connect to general
					connectToServer("general")
				} else {
					// Show chat list
					showChatList(chats, connectToServer)
				}
			} else {
				myApp.Quit()
			}
		}, myWindow)

	myWindow.Show()

	// Асинхронно проверяем доступность сервера после отображения окна
	go func() {
		err := checkServerAvailability(serverAddress)
		fyne.Do(func() {
			if err != nil {
				statusLabel.SetText("Сервер недоступен")
				statusIndicator.FillColor = color.RGBA{R: 255, G: 0, B: 0, A: 255} // Красный
				statusIndicator.Refresh()
				inputBox.Disable()
				sendBtn.Disable()
				emojiBtn.Disable()
				dialog.ShowError(fmt.Errorf("Не удалось подключиться к серверу.\nПожалуйста, убедитесь, что он запущен."), myWindow)
			} else {
				statusLabel.SetText("Ожидание входа...")
				statusIndicator.FillColor = color.RGBA{R: 255, G: 255, B: 0, A: 255} // Желтый
				statusIndicator.Refresh()
				loginForm.Show()
			}
		})
	}()

	myApp.Run()

	if conn != nil {
		_ = conn.Close()
	}
}
