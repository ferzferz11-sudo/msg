package main

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"

	"msg/gen"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Вспомогательная функция для парсинга HEX в color.RGBA
func parseHexColor(s string) color.Color {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.Transparent
	}
	var r, g, b uint8
	fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// saveUsernameToEnv сохраняет или обновляет имя пользователя в .env
func saveUsernameToEnv(username string) {
	envFile := "macos_client/.env" // Указываем путь относительно корня проекта
	
	// Читаем текущее содержимое
	content, err := os.ReadFile(envFile)
	if err != nil {
		// Если файла нет, просто создадим его с именем
		if os.IsNotExist(err) {
			os.WriteFile(envFile, []byte(fmt.Sprintf("LAST_USERNAME=%s\n", username)), 0644)
		}
		return
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "LAST_USERNAME=") {
			lines[i] = "LAST_USERNAME=" + username
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, "LAST_USERNAME="+username)
	}

	os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0644)
}

// customTheme для поддержки своих цветов
type customTheme struct {
	isDark          bool
	lightBg, darkBg color.Color
	lightFg, darkFg color.Color
	lightPrimary, darkPrimary color.Color
}

func (c *customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Подменяем основные цвета
	if name == theme.ColorNameBackground {
		if c.isDark {
			return c.darkBg
		}
		return c.lightBg
	}
	// Используем Foreground для общего текста и текста в полях ввода
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

func main() {
	// Загружаем основной .env и клиентский
	godotenv.Load("../.env")
	godotenv.Load("macos_client/.env") // Локальный клиентский файл

	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = "localhost:50051"
	}
	if strings.HasPrefix(serverAddress, ":") {
		serverAddress = "localhost" + serverAddress
	}

	// Читаем настройки темы
	myTheme := &customTheme{
		isDark:       false, // По умолчанию светлая тема
		lightBg:      parseHexColor(os.Getenv("LIGHT_BG_COLOR")),
		darkBg:       parseHexColor(os.Getenv("DARK_BG_COLOR")),
		lightFg:      parseHexColor(os.Getenv("LIGHT_TEXT_COLOR")),
		darkFg:       parseHexColor(os.Getenv("DARK_TEXT_COLOR")),
		lightPrimary: parseHexColor(os.Getenv("LIGHT_NAME_COLOR")),
		darkPrimary:  parseHexColor(os.Getenv("DARK_NAME_COLOR")),
	}

	myApp := app.New()
	myApp.Settings().SetTheme(myTheme)

	myWindow := myApp.NewWindow(fmt.Sprintf("Go gRPC Chat (%s)", serverAddress))
	myWindow.Resize(fyne.NewSize(600, 400))

	var username string
	var stream gen.ChatService_ChatClient
	var conn *grpc.ClientConn

	// Чат (используем RichText для раскраски)
	chatBox := widget.NewRichText()
	chatBox.Scroll = container.ScrollBoth
	scrollContainer := container.NewVScroll(chatBox)
	
	// Ввод
	inputBox := widget.NewEntry()
	inputBox.SetPlaceHolder("Введите сообщение...")

	// Кнопка смены темы
	var themeBtn *widget.Button
	themeBtn = widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), func() {
		myTheme.isDark = !myTheme.isDark
		if myTheme.isDark {
			themeBtn.SetIcon(theme.VisibilityOffIcon()) // условная луна
		} else {
			themeBtn.SetIcon(theme.ColorPaletteIcon()) // условное солнце
		}
		myApp.Settings().SetTheme(myTheme)
		chatBox.Refresh()
	})
	// Устанавливаем начальную иконку в зависимости от isDark
	if myTheme.isDark {
		themeBtn.SetIcon(theme.VisibilityOffIcon())
	} else {
		themeBtn.SetIcon(theme.ColorPaletteIcon())
	}

	// Панель сверху для иконки
	topBar := container.NewHBox(layout.NewSpacer(), themeBtn)

	// Добавление сообщения
	appendMessage := func(timeStr, user, text string) {
		// Используем TextSegment для всех частей сообщения
		timeSeg := &widget.TextSegment{
			Text:  fmt.Sprintf("[%s] ", timeStr),
			Style: widget.RichTextStyle{ColorName: theme.ColorNameDisabled},
		}
		
		// Стиль для имени пользователя, используем Primary цвет из темы
		userSeg := &widget.TextSegment{
			Text: user + ": ",
			Style: widget.RichTextStyle{
				ColorName: theme.ColorNamePrimary, // Будет использовать lightPrimary или darkPrimary
				TextStyle: fyne.TextStyle{Bold: true},
			},
		}

		textSeg := &widget.TextSegment{
			Text:  text + "\n",
			Style: widget.RichTextStyleInline,
		}

		chatBox.Segments = append(chatBox.Segments, timeSeg, userSeg, textSeg)
		chatBox.Refresh()
		scrollContainer.ScrollToBottom()
	}

	safeAppendSystemMessage := func(msg string) {
		fyne.Do(func() {
			seg := &widget.TextSegment{
				Text:  msg + "\n",
				Style: widget.RichTextStyle{ColorName: theme.ColorNameError},
			}
			chatBox.Segments = append(chatBox.Segments, seg)
			chatBox.Refresh()
			scrollContainer.ScrollToBottom()
		})
	}

	// Функция отправки сообщения
	sendMsg := func() {
		text := inputBox.Text
		if text == "" || stream == nil {
			return
		}

		err := stream.Send(&gen.Message{
			User: username,
			Text: text,
		})
		if err != nil {
			safeAppendSystemMessage(fmt.Sprintf("[Ошибка отправки]: %v", err))
		}
		inputBox.SetText("")
	}

	sendBtn := widget.NewButton("Отправить", sendMsg)

	// Подключение к серверу
	connectToServer := func() {
		var err error
		conn, err = grpc.Dial(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

		safeAppendSystemMessage(fmt.Sprintf("Подключено к %s! Добро пожаловать, %s", serverAddress, username))

		// Чтение сообщений из потока
		go func() {
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					myApp.SendNotification(&fyne.Notification{Title: "Chat Error", Content: err.Error()})
					break
				}

				t := in.CreatedAt.AsTime().Local()
				timeStr := t.Format("15:04:05")

				fyne.Do(func() {
					appendMessage(timeStr, in.User, in.Text)
				})
			}
		}()
	}

	// Форма входа
	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Ваше имя")
	
	// Предзаполнение из .env
	lastUser := os.Getenv("LAST_USERNAME")
	if lastUser != "" {
		usernameEntry.SetText(lastUser)
	}

	loginForm := dialog.NewCustomConfirm("Вход в чат", "Войти", "Отмена", usernameEntry, func(b bool) {
		if b && usernameEntry.Text != "" {
			username = usernameEntry.Text
			saveUsernameToEnv(username) // Сохраняем имя
			connectToServer()
		} else {
			myApp.Quit() // Выход, если имя не введено
		}
	}, myWindow)

	// Сборка UI чата
	inputContainer := container.NewBorder(nil, nil, nil, sendBtn, inputBox)
	
	// Оборачиваем чат, чтобы иконка была сверху
	centerContent := container.NewBorder(topBar, nil, nil, nil, scrollContainer)
	mainLayout := container.NewBorder(nil, inputContainer, nil, nil, centerContent)

	myWindow.SetContent(mainLayout)

	// Обработка Enter для отправки
	inputBox.OnSubmitted = func(s string) {
		sendMsg()
	}

	// Показываем диалог входа после запуска
	myWindow.Show()
	loginForm.Show()

	myApp.Run()

	if conn != nil {
		conn.Close()
	}
}
