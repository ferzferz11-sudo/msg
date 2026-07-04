// Lavender Messenger - HTTP Server for Uploads
// Author: Pavel Davydov (ferz)

package main

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const (
	maxUploadSize   = 30 * 1024 * 1024 // 30MB
	avatarsPath     = "./uploads/avatars"
	imagesPath      = "./uploads/images"
	filesPath       = "./uploads/files"
	backgroundsPath = "./uploads/background"
	audioPath       = "./uploads/audio"
	defaultHTTPPort = "8082"
)

var (
	// TURN server credentials (from env, not hardcoded)
	turnServerHost   = getEnvOrDefault("TURN_SERVER_HOST", "13.140.25.249:3478")
	turnSharedSecret = os.Getenv("TURN_SHARED_SECRET")
	turnTTL          = 86400 // 24 hours

	// Shutdown state — set to true during graceful shutdown
	httpShuttingDown atomic.Bool
)

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func closeFile(file io.ReadCloser) {
	if err := file.Close(); err != nil {
		logger.Errorf("Error closing file: %v", err)
	}
}

// requireAuth is HTTP middleware that validates a JWT Bearer token from the Authorization header.
// On success, it sets X-User-ID and X-Username headers for downstream handlers.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := ValidateToken(tokenString)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}
		r.Header.Set("X-User-ID", claims.UserID)
		r.Header.Set("X-Username", claims.Username)
		next(w, r)
	}
}

// sanitizeFileExtension validates and normalizes a file extension, returning empty string if disallowed
func sanitizeFileExtension(filename string, allowed []string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return ""
	}
	for _, a := range allowed {
		if ext == a {
			return ext
		}
	}
	return ""
}

func StartHTTPServerAndReturn(port string) *http.Server {
	// Ensure directories exist
	os.MkdirAll(avatarsPath, 0755)
	os.MkdirAll(imagesPath, 0755)
	os.MkdirAll(filesPath, 0755)
	os.MkdirAll(backgroundsPath, 0755)
	os.MkdirAll(audioPath, 0755)

	http.HandleFunc("/upload-avatar", requireAuth(uploadAvatarHandler))
	http.HandleFunc("/upload-image", requireAuth(uploadImageHandler))
	http.HandleFunc("/upload-file", requireAuth(uploadFileHandler))
	http.HandleFunc("/upload-background", requireAuth(uploadBackgroundHandler))
	http.HandleFunc("/upload-audio", requireAuth(uploadAudioHandler))

	// TURN credentials endpoint
	http.HandleFunc("/turn-credentials", requireAuth(turnCredentialsHandler))

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if httpShuttingDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"shutting_down","version":"%s","time":"%s"}`, ServerVersion, time.Now().Format(time.RFC3339))
			return
		}
		fmt.Fprintf(w, `{"status":"ok","version":"%s","time":"%s"}`, ServerVersion, time.Now().Format(time.RFC3339))
	})

	// Server info endpoint — returns service versions for client capability negotiation
	http.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		info := map[string]interface{}{
			"version":          ServerVersion,
			"time":             time.Now().Format(time.RFC3339),
			"max_upload_size":  maxUploadSize,
			"services": map[string]string{
				"auth":    AuthServiceVersion,
				"chat":    ChatServiceVersion,
				"profile": ProfileServiceVersion,
				"ai":      AIServiceVersion,
				"files":   FileServiceVersion,
				"push":    PushServiceVersion,
			},
		}
		json.NewEncoder(w).Encode(info)
	})

	http.HandleFunc("/avatars/", func(w http.ResponseWriter, r *http.Request) {
		serveFileHandler(w, r, "/avatars/", avatarsPath)
	})
	http.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		serveFileHandler(w, r, "/images/", imagesPath)
	})
	http.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		serveFileHandler(w, r, "/files/", filesPath)
	})
	http.HandleFunc("/background/", func(w http.ResponseWriter, r *http.Request) {
		serveFileHandler(w, r, "/background/", backgroundsPath)
	})
	http.HandleFunc("/audio/", func(w http.ResponseWriter, r *http.Request) {
		serveFileHandler(w, r, "/audio/", audioPath)
	})

	srv := &http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: corsMiddleware(http.DefaultServeMux),
	}

	logger.Infof("HTTP server started on port %s", port)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("HTTP server error: %v", err)
		}
	}()

	return srv
}

func uploadAvatarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		logger.Errorf("Upload error: file too large: %v", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	// Process thumbnail (required)
	thumbFile, thumbHandler, err := r.FormFile("avatar")
	if err != nil {
		logger.Errorf("Upload error: retrieving thumbnail file: %v", err)
		http.Error(w, "Error retrieving thumbnail file", http.StatusBadRequest)
		return
	}
	defer closeFile(thumbFile)

	thumbBytes, err := io.ReadAll(thumbFile)
	if err != nil {
		logger.Errorf("Upload error: reading thumbnail file: %v", err)
		http.Error(w, "Error reading thumbnail file", http.StatusInternalServerError)
		return
	}

	// Validate thumbnail extension
	hash := md5.Sum(thumbBytes)
	allowedImageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	ext := sanitizeFileExtension(thumbHandler.Filename, allowedImageExts)
	if ext == "" {
		http.Error(w, "File extension not allowed", http.StatusBadRequest)
		return
	}
	thumbFilename := hex.EncodeToString(hash[:]) + ext

	thumbPath := filepath.Join(avatarsPath, thumbFilename)
	if err := os.WriteFile(thumbPath, thumbBytes, 0644); err != nil {
		logger.Errorf("Upload error: saving thumbnail to %s: %v", thumbPath, err)
		http.Error(w, "Error saving thumbnail file", http.StatusInternalServerError)
		return
	}

	publicIP := os.Getenv("PUBLIC_IP")
	if publicIP == "" {
		publicIP = "localhost"
	}
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}

	thumbURL := fmt.Sprintf("http://%s:%s/avatars/%s", publicIP, httpPort, thumbFilename)

	// Process full image (optional)
	var fullURL string
	fullFile, fullHandler, err := r.FormFile("avatar_full")
	if err == nil {
		defer closeFile(fullFile)

		fullBytes, err := io.ReadAll(fullFile)
		if err == nil {
			// Validate full image extension
			fullExt := sanitizeFileExtension(fullHandler.Filename, allowedImageExts)
			if fullExt == "" {
				fullExt = ext // fallback to thumbnail's validated extension
			}
			fullHash := md5.Sum(fullBytes)
			fullFilename := hex.EncodeToString(fullHash[:]) + "_full" + fullExt

			fullPath := filepath.Join(avatarsPath, fullFilename)
			if err := os.WriteFile(fullPath, fullBytes, 0644); err == nil {
				fullURL = fmt.Sprintf("http://%s:%s/avatars/%s", publicIP, httpPort, fullFilename)
			}
		}
	}

	if fullURL != "" {
		logger.Infof("Avatar uploaded: thumb=%s, full=%s", thumbFilename, filepath.Base(fullURL))
		fmt.Fprintf(w, `{"url": "%s", "full_url": "%s"}`, thumbURL, fullURL)
	} else {
		logger.Infof("Avatar uploaded: %s", thumbFilename)
		fmt.Fprintf(w, `{"url": "%s"}`, thumbURL)
	}
}

func uploadImageHandler(w http.ResponseWriter, r *http.Request) {
	handleUpload(w, r, "image", imagesPath, "/images/")
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	handleUpload(w, r, "file", filesPath, "/files/")
}

func uploadBackgroundHandler(w http.ResponseWriter, r *http.Request) {
	handleUpload(w, r, "background", backgroundsPath, "/background/")
}

func uploadAudioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("Received audio upload request")

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		logger.Errorf("Upload error: file too large: %v", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	// Get duration from form
	durationStr := r.FormValue("duration")
	duration := 0
	if durationStr != "" {
		_, err := fmt.Sscanf(durationStr, "%d", &duration)
		if err != nil {
			logger.Errorf("Upload error: invalid duration format: %v", err)
			http.Error(w, "Invalid duration format", http.StatusBadRequest)
			return
		}
	}

	file, handler, err := r.FormFile("audio")
	if err != nil {
		logger.Errorf("Upload error: retrieving audio file: %v", err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer closeFile(file)

	logger.Infof("Uploading audio file: %s (size: %d bytes, duration: %d seconds)", handler.Filename, handler.Size, duration)

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		logger.Errorf("Upload error: reading file: %v", err)
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	// Validate audio file extension
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	validExts := map[string]bool{".m4a": true, ".aac": true, ".ogg": true, ".mp3": true, ".wav": true}
	if !validExts[ext] {
		logger.Errorf("Upload error: invalid audio format: %s", ext)
		http.Error(w, "Invalid audio format. Supported: m4a, aac, ogg, mp3, wav", http.StatusBadRequest)
		return
	}

	// Generate unique filename
	hash := md5.Sum(fileBytes)
	filename := hex.EncodeToString(hash[:]) + ext

	filePath := filepath.Join(audioPath, filename)
	if err := os.WriteFile(filePath, fileBytes, 0644); err != nil {
		logger.Errorf("Upload error: saving file to %s: %v", filePath, err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	publicIP := os.Getenv("PUBLIC_IP")
	if publicIP == "" {
		publicIP = "localhost"
	}
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}

	fileURL := fmt.Sprintf("http://%s:%s/audio/%s", publicIP, httpPort, filename)
	logger.Infof("Audio file uploaded successfully! URL: %s, Duration: %d seconds", fileURL, duration)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"url": "%s", "duration": %d}`, fileURL, duration)
}

func handleUpload(w http.ResponseWriter, r *http.Request, formKey, saveDir, urlPrefix string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Infof("Received upload request for key: %s", formKey)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		logger.Errorf("Upload error: file too large: %v", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile(formKey)
	if err != nil {
		logger.Errorf("Upload error: retrieving file for key %s: %v", formKey, err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer closeFile(file)

	logger.Infof("Uploading file: %s (size: %d bytes)", handler.Filename, handler.Size)

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		logger.Errorf("Upload error: reading file: %v", err)
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	// Validate file extension
	var allowedExts []string
	switch formKey {
	case "image", "background":
		allowedExts = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	case "file":
		allowedExts = []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
			".txt", ".csv", ".json", ".xml", ".zip", ".rar", ".7z",
			".mp3", ".mp4", ".avi", ".mov", ".mkv", ".webm", ".m4a", ".aac", ".ogg", ".wav"}
	default:
		allowedExts = []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	}
	ext := sanitizeFileExtension(handler.Filename, allowedExts)
	if ext == "" {
		http.Error(w, "File extension not allowed", http.StatusBadRequest)
		return
	}

	// Hash filename to prevent path traversal and naming conflicts
	hash := md5.Sum(fileBytes)
	filename := hex.EncodeToString(hash[:]) + ext

	filePath := filepath.Join(saveDir, filename)
	if err := os.WriteFile(filePath, fileBytes, 0644); err != nil {
		logger.Errorf("Upload error: saving file to %s: %v", filePath, err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	publicIP := os.Getenv("PUBLIC_IP")
	if publicIP == "" {
		publicIP = "localhost"
	}
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}

	fileURL := fmt.Sprintf("http://%s:%s%s%s", publicIP, httpPort, urlPrefix, filename)
	logger.Infof("File uploaded successfully! URL: %s", fileURL)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"url": "%s"}`, fileURL)
}

func serveFileHandler(w http.ResponseWriter, r *http.Request, prefix, dir string) {
	filename := strings.TrimPrefix(r.URL.Path, prefix)
	filename = filepath.Clean(filename)
	if filename == "." || filename == ".." || strings.Contains(filename, "..") || filepath.IsAbs(filename) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	filePath := filepath.Join(dir, filename)
	absDir, _ := filepath.Abs(dir)
	absFile, _ := filepath.Abs(filePath)
	if !strings.HasPrefix(absFile, absDir+string(os.PathSeparator)) && absFile != absDir {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, filePath)
}

// DeleteImageFile deletes an image, file, or audio from the server
func DeleteImageFile(imageURL string) error {
	if imageURL == "" {
		return nil
	}

	// Skip non-HTTP(S) URLs as they are client-side references (e.g. content:// or local paths)
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return nil
	}

	// URL format: [prefix]/filename
	parts := strings.Split(imageURL, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid file URL format")
	}

	// Безопасно забираем имя файла (избегаем инъекций путей)
	filename := filepath.Base(parts[len(parts)-1]) // filepath.Base гарантирует извлечение ТОЛЬКО имени файла
	prefix := parts[len(parts)-2]

	var saveDir string
	switch prefix {
	case "avatars":
		saveDir = avatarsPath
	case "images":
		saveDir = imagesPath
	case "files":
		saveDir = filesPath
	case "background":
		saveDir = backgroundsPath
	case "audio":
		saveDir = audioPath
	default:
		return fmt.Errorf("unknown file prefix: %s", prefix)
	}

	// Нормализуем пути для корректного сравнения
	cleanSaveDir := filepath.Clean(saveDir)
	filePath := filepath.Join(cleanSaveDir, filename)

	// Дополнительная проверка безопасности: путь должен оставаться внутри целевой директории
	// Используем Abs для абсолютной уверенности
	absSaveDir, _ := filepath.Abs(cleanSaveDir)
	absFilePath, _ := filepath.Abs(filePath)

	if !strings.HasPrefix(absFilePath, absSaveDir) {
		return fmt.Errorf("security alert: attempt to delete file outside of the allowed directory (path: %s, base: %s)", absFilePath, absSaveDir)
	}

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // Файл уже удален или не существует
	}

	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to remove file from disk: %w", err)
	}

	logger.Infof("🗑️ Successfully deleted file from disk: %s", filePath)
	return nil
}

// turnCredentialsHandler generates temporary TURN credentials using HMAC
// Client sends GET /turn-credentials and receives JSON with iceServers array
func turnCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate timestamp-based username (valid for TTL)
	timestamp := time.Now().Unix() + int64(turnTTL)
	username := fmt.Sprintf("%d", timestamp)

	// HMAC-SHA1 password using shared secret
	mac := hmac.New(sha1.New, []byte(turnSharedSecret))
	mac.Write([]byte(username))
	password := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Build ICE servers response
	servers := map[string]interface{}{
		"iceServers": []map[string]interface{}{
			{
				"urls": []string{
					"stun:stun.l.google.com:19302",
					fmt.Sprintf("turn:%s?transport=udp", turnServerHost),
					fmt.Sprintf("turn:%s?transport=tcp", turnServerHost),
				},
				"username":   username,
				"credential": password,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
