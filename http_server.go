// Lavender Messenger - HTTP Server for Uploads
// Author: Pavel Davydov (ferz)

package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxUploadSize   = 10 * 1024 * 1024 // 10MB
	avatarsPath     = "./uploads/avatars"
	imagesPath      = "./uploads/images"
	filesPath       = "./uploads/files"
	backgroundsPath = "./uploads/background"
	audioPath       = "./uploads/audio"
	defaultHTTPPort = "8082"
)

func closeFile(file io.ReadCloser) {
	if err := file.Close(); err != nil {
		log.Printf("Error closing file: %v", err)
	}
}

func StartHTTPServer(port string) {
	// Ensure directories exist
	os.MkdirAll(avatarsPath, 0755)
	os.MkdirAll(imagesPath, 0755)
	os.MkdirAll(filesPath, 0755)
	os.MkdirAll(backgroundsPath, 0755)
	os.MkdirAll(audioPath, 0755)

	http.HandleFunc("/upload-avatar", uploadAvatarHandler)
	http.HandleFunc("/upload-image", uploadImageHandler)
	http.HandleFunc("/upload-file", uploadFileHandler)
	http.HandleFunc("/upload-background", uploadBackgroundHandler)
	http.HandleFunc("/upload-audio", uploadAudioHandler)

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

	log.Printf("HTTP server started on port %s", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		log.Printf("HTTP server error: %v", err)
	}
}

func StartAPKServer(port string) {
	apkDir := os.Getenv("APK_DIR")
	if apkDir == "" {
		apkDir = "/home/ferz/LavenderMessengerAndroid"
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir(apkDir))
	mux.Handle("/", fileServer)

	log.Printf("APK server started on port %s serving %s", port, apkDir)
	if err := http.ListenAndServe("0.0.0.0:"+port, mux); err != nil {
		log.Printf("APK server error: %v", err)
	}
}

func uploadAvatarHandler(w http.ResponseWriter, r *http.Request) {
	handleUpload(w, r, "avatar", avatarsPath, "/avatars/")
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

	log.Printf("Received audio upload request")

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Printf("Upload error: file too large: %v", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	// Get duration from form
	durationStr := r.FormValue("duration")
	duration := 0
	if durationStr != "" {
		_, err := fmt.Sscanf(durationStr, "%d", &duration)
		if err != nil {
			log.Printf("Upload error: invalid duration format: %v", err)
			http.Error(w, "Invalid duration format", http.StatusBadRequest)
			return
		}
	}

	file, handler, err := r.FormFile("audio")
	if err != nil {
		log.Printf("Upload error: retrieving audio file: %v", err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer closeFile(file)

	log.Printf("Uploading audio file: %s (size: %d bytes, duration: %d seconds)", handler.Filename, handler.Size, duration)

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Upload error: reading file: %v", err)
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	// Validate audio file extension
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	validExts := map[string]bool{".m4a": true, ".aac": true, ".ogg": true, ".mp3": true, ".wav": true}
	if !validExts[ext] {
		log.Printf("Upload error: invalid audio format: %s", ext)
		http.Error(w, "Invalid audio format. Supported: m4a, aac, ogg, mp3, wav", http.StatusBadRequest)
		return
	}

	// Generate unique filename
	hash := md5.Sum(fileBytes)
	filename := hex.EncodeToString(hash[:]) + ext

	filePath := filepath.Join(audioPath, filename)
	if err := os.WriteFile(filePath, fileBytes, 0644); err != nil {
		log.Printf("Upload error: saving file to %s: %v", filePath, err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	publicIP := "159.195.38.145"
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}

	fileURL := fmt.Sprintf("http://%s:%s/audio/%s", publicIP, httpPort, filename)
	log.Printf("Audio file uploaded successfully! URL: %s, Duration: %d seconds", fileURL, duration)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"url": "%s", "duration": %d}`, fileURL, duration)
}

func handleUpload(w http.ResponseWriter, r *http.Request, formKey, saveDir, urlPrefix string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Received upload request for key: %s", formKey)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Printf("Upload error: file too large: %v", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile(formKey)
	if err != nil {
		log.Printf("Upload error: retrieving file for key %s: %v", formKey, err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer closeFile(file)

	log.Printf("Uploading file: %s (size: %d bytes)", handler.Filename, handler.Size)

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Upload error: reading file: %v", err)
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	// For files, we might want to keep the original name or hash it
	var filename string
	if formKey == "file" {
		filename = handler.Filename
	} else {
		hash := md5.Sum(fileBytes)
		ext := filepath.Ext(handler.Filename)
		if ext == "" {
			ext = ".jpg"
		}
		filename = hex.EncodeToString(hash[:]) + ext
	}

	filePath := filepath.Join(saveDir, filename)
	if err := os.WriteFile(filePath, fileBytes, 0644); err != nil {
		log.Printf("Upload error: saving file to %s: %v", filePath, err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	publicIP := "159.195.38.145"
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}

	fileURL := fmt.Sprintf("http://%s:%s%s%s", publicIP, httpPort, urlPrefix, filename)
	log.Printf("File uploaded successfully! URL: %s", fileURL)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"url": "%s"}`, fileURL)
}

func serveFileHandler(w http.ResponseWriter, r *http.Request, prefix, dir string) {
	filename := strings.TrimPrefix(r.URL.Path, prefix)
	filePath := filepath.Join(dir, filename)
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

	filePath := filepath.Join(saveDir, filename)

	// Безопасность: Проверяем, что итоговый путь действительно лежит внутри целевой папки
	if !strings.HasPrefix(filePath, saveDir) {
		return fmt.Errorf("security alert: attempt to delete file outside of the allowed directory")
	}

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // Файл уже удален
	}

	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to remove file from disk: %w", err)
	}

	log.Printf("🗑️ Successfully deleted file from disk: %s", filePath)
	return nil
}
