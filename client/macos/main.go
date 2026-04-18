// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements a macOS GUI client for the Lavender Messenger.
// It provides a graphical interface using Fyne framework with themes and emojis.

package main

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"LavenderMessenger/gen"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	clientVersion = "0.9.1"
	configFile    = "client/macos/config.yaml"
)

type Config struct {
	ServerAddress string `yaml:"server_address"`
	Themes        struct {
		Light ThemeConfig `yaml:"light"`
		Dark  ThemeConfig `yaml:"dark"`
	} `yaml:"themes"`
	CurrentTheme string `yaml:"current_theme"`
	LastUsername string `yaml:"last_username"`
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
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// saveConfig сохраняет конфигурацию в YAML файл
func saveConfig(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
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

	myWindow := myApp.NewWindow(fmt.Sprintf("Go gRPC Chat v%s (%s)", clientVersion, serverAddress))
	myWindow.Resize(fyne.NewSize(600, 400))

	var username string
	var stream gen.ChatService_ChatClient
	var conn *grpc.ClientConn

	// UI для статуса (индикатор и текст)
	statusIndicator := canvas.NewCircle(color.RGBA{R: 255, G: 255, B: 0, A: 255}) // Желтый
	statusIndicator.Resize(fyne.NewSize(12, 12))
	statusIndicator.Move(fyne.NewPos(4, 4))
	indicatorContainer := container.NewWithoutLayout(statusIndicator)
	indicatorContainer.Resize(fyne.NewSize(20, 20))

	statusLabel := widget.NewLabel("Подключение...")
	statusLabel.Alignment = fyne.TextAlignLeading

	statusBox := container.NewHBox(indicatorContainer, statusLabel)

	var lastUser string
	var userColorMap = make(map[string]fyne.ThemeColorName)
	var userColorIndex int

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

	chatBox := widget.NewRichText()
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

	topBar := container.NewBorder(nil, nil, statusBox, themeBtn)

	appendMessage := func(timeStr, user, text string) {
		isSameUser := lastUser == user
		lastUser = user
		if !isSameUser {
			chatBox.Segments = append(chatBox.Segments, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyleInline})
		}
		headerStyle := widget.RichTextStyle{ColorName: getUserColorName(user), TextStyle: fyne.TextStyle{Bold: true}}
		headerSeg := &widget.TextSegment{Text: fmt.Sprintf("%s %s: ", timeStr, user), Style: headerStyle}
		textSeg := &widget.TextSegment{Text: text + "\n", Style: widget.RichTextStyleInline}
		if isSameUser {
			chatBox.Segments = append(chatBox.Segments, textSeg)
		} else {
			chatBox.Segments = append(chatBox.Segments, headerSeg, textSeg)
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
		err := stream.Send(&gen.Message{User: username, Text: text})
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

	var loginForm *dialog.ConfirmDialog

	connectToServer := func() {
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
					appendMessage(timeStr, in.User, in.Text)
				})
			}
		}()
	}

	loginForm = dialog.NewCustomConfirm("Вход в чат", "Войти", "Отмена", usernameEntry, func(b bool) {
		if b && usernameEntry.Text != "" {
			username = usernameEntry.Text
			cfg.LastUsername = username
			_ = saveConfig(cfg)
			connectToServer()
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
