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

// saveThemeToEnv сохраняет или обновляет тему в .env
func saveThemeToEnv(isDark bool) {
	envFile := "macos_client/.env" // Указываем путь относительно корня проекта

	// Читаем текущее содержимое
	content, err := os.ReadFile(envFile)
	if err != nil {
		// Если файла нет, просто создадим его с темой
		if os.IsNotExist(err) {
			themeValue := "dark"
			if !isDark {
				themeValue = "light"
			}
			os.WriteFile(envFile, []byte(fmt.Sprintf("THEME=%s\n", themeValue)), 0644)
		}
		return
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "THEME=") {
			themeValue := "dark"
			if !isDark {
				themeValue = "light"
			}
			lines[i] = "THEME=" + themeValue
			found = true
			break
		}
	}

	if !found {
		themeValue := "dark"
		if !isDark {
			themeValue = "light"
		}
		lines = append(lines, "THEME="+themeValue)
	}

	os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0644)
}

// Кастомные имена цветов для пользователей
type userColorName string

const (
	UserColor1 userColorName = "UserColor1"
	UserColor2 userColorName = "UserColor2"
	UserColor3 userColorName = "UserColor3"
	UserColor4 userColorName = "UserColor4"
	UserColor5 userColorName = "UserColor5"
	UserColor6 userColorName = "UserColor6"
	UserColor7 userColorName = "UserColor7"
	UserColor8 userColorName = "UserColor8"
	UserColor9 userColorName = "UserColor9"
	UserColor10 userColorName = "UserColor10"
)

// customTheme для поддержки своих цветов
type customTheme struct {
	isDark                    bool
	lightBg, darkBg           color.Color
	lightFg, darkFg           color.Color
	lightPrimary, darkPrimary color.Color
	lightTime, darkTime       color.Color
	userColors               map[userColorName]color.Color
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
	// Добавляем новый цвет для времени
	if name == theme.ColorNameDisabled {
		if c.isDark {
			return c.darkTime
		}
		return c.lightTime
	}
	
	// Проверяем кастомные цвета пользователей
	if userColor, exists := c.userColors[userColorName(name)]; exists {
		return userColor
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
	themeValue := os.Getenv("THEME")
	isDarkTheme := (themeValue == "dark")
	
	// Инициализируем пользовательские цвета
	themeUserColors := make(map[userColorName]color.Color)
	
	myTheme := &customTheme{
		isDark:       isDarkTheme,
		lightBg:      parseHexColor(os.Getenv("LIGHT_BG_COLOR")),
		darkBg:       parseHexColor(os.Getenv("DARK_BG_COLOR")),
		lightFg:      parseHexColor(os.Getenv("LIGHT_TEXT_COLOR")),
		darkFg:       parseHexColor(os.Getenv("DARK_TEXT_COLOR")),
		lightPrimary: parseHexColor(os.Getenv("LIGHT_NAME_COLOR")),
		darkPrimary:  parseHexColor(os.Getenv("DARK_NAME_COLOR")),
		lightTime:    parseHexColor(os.Getenv("LIGHT_TIME_COLOR")),
		darkTime:     parseHexColor(os.Getenv("DARK_TIME_COLOR")),
		userColors:    themeUserColors,
	}

	myApp := app.New()
	myApp.Settings().SetTheme(myTheme)

	myWindow := myApp.NewWindow(fmt.Sprintf("Go gRPC Chat (%s)", serverAddress))
	myWindow.Resize(fyne.NewSize(600, 400))

	var username string
	var stream gen.ChatService_ChatClient
	var conn *grpc.ClientConn

	// Статус и приветствие в одной строке
	statusLabel := widget.NewLabel("Отключено")
	statusLabel.Alignment = fyne.TextAlignLeading
	
	// Переменные для отслеживания сообщений
	var lastUser string
	var userColors = make(map[string]fyne.ThemeColorName)
	var userColorIndex int
	
	// Загружаем цвета для пользователей
	lightUserColorsStr := os.Getenv("LIGHT_USER_COLORS")
	darkUserColorsStr := os.Getenv("DARK_USER_COLORS")
	
	// Функция для получения цвета пользователя
	getUserColor := func(user string) fyne.ThemeColorName {
		// Сначала проверяем локальный кэш
		if colorName, exists := userColors[user]; exists {
			return colorName
		}
		
		// Генерируем новый цвет для пользователя
		var colorsStr string
		if myTheme.isDark {
			colorsStr = darkUserColorsStr
		} else {
			colorsStr = lightUserColorsStr
		}
		
		var colorName fyne.ThemeColorName
		
		if colorsStr == "" {
			// Если цвета не заданы, используем стандартный
			colorName = theme.ColorNamePrimary
		} else {
			// Разбираем строку с цветами
			colors := strings.Split(colorsStr, ",")
			if len(colors) == 0 {
				colorName = theme.ColorNamePrimary
			} else {
				// Выбираем цвет по индексу
				colorHex := strings.TrimSpace(colors[userColorIndex%len(colors)])
				currentIndex := userColorIndex
				userColorIndex++
				
				// Добавляем цвет в тему
				var userColor userColorName
				switch currentIndex % 10 {
				case 0:
					userColor = UserColor1
				case 1:
					userColor = UserColor2
				case 2:
					userColor = UserColor3
				case 3:
					userColor = UserColor4
				case 4:
					userColor = UserColor5
				case 5:
					userColor = UserColor6
				case 6:
					userColor = UserColor7
				case 7:
					userColor = UserColor8
				case 8:
					userColor = UserColor9
				case 9:
					userColor = UserColor10
				}
				
				myTheme.userColors[userColor] = parseHexColor(colorHex)
				colorName = fyne.ThemeColorName(userColor)
			}
		}
		
		// Сохраняем в локальный кэш
		userColors[user] = colorName
		return colorName
	}

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
		saveThemeToEnv(myTheme.isDark) // Сохраняем тему в .env
	})
	// Устанавливаем начальную иконку в зависимости от isDark
	if myTheme.isDark {
		themeBtn.SetIcon(theme.VisibilityOffIcon())
	} else {
		themeBtn.SetIcon(theme.ColorPaletteIcon())
	}

	// Панель сверху для статуса и иконки темы
	topBar := container.NewBorder(nil, nil, statusLabel, themeBtn)

	// Добавление сообщения
	appendMessage := func(timeStr, user, text string) {
		// Проверяем, тот ли это же пользователь, что и в прошлый раз
		isSameUser := (lastUser == user)
		lastUser = user
		
		if !isSameUser {
			// Добавляем отступ перед новым пользователем
			spacingSeg := &widget.TextSegment{
				Text:  "\n",
				Style: widget.RichTextStyleInline,
			}
			
			// Время и имя в одной строке (цвет пользователя)
			headerSeg := &widget.TextSegment{
				Text: fmt.Sprintf("%s %s: ", timeStr, user),
				Style: widget.RichTextStyle{
					ColorName: getUserColor(user),
					TextStyle: fyne.TextStyle{Bold: true},
				},
			}
			
			// Текст сообщения
			textSeg := &widget.TextSegment{
				Text:  text + "\n",
				Style: widget.RichTextStyleInline,
			}
			
			chatBox.Segments = append(chatBox.Segments, spacingSeg, headerSeg, textSeg)
		} else {
			// Тот же пользователь - только текст
			textSeg := &widget.TextSegment{
				Text:  text + "\n",
				Style: widget.RichTextStyleInline,
			}
			
			chatBox.Segments = append(chatBox.Segments, textSeg)
		}
		
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
			statusLabel.SetText("Ошибка подключения")
			dialog.ShowError(err, myWindow)
			return
		}

		client := gen.NewChatServiceClient(conn)
		stream, err = client.Chat(context.Background())
		if err != nil {
			statusLabel.SetText("Ошибка подключения")
			dialog.ShowError(err, myWindow)
			return
		}

		statusLabel.SetText(fmt.Sprintf("Подключено к %s | Добро пожаловать, %s!", serverAddress, username))

		// Чтение сообщений из потока
		go func() {
			for {
				in, err := stream.Recv()
				if err == io.EOF {
					statusLabel.SetText("Отключено от сервера")
					break
				}
				if err != nil {
					statusLabel.SetText("Ошибка подключения")
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
	lastUsername := os.Getenv("LAST_USERNAME")
	if lastUsername != "" {
		usernameEntry.SetText(lastUsername)
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
