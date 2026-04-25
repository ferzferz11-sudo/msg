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

	http.HandleFunc("/upload-avatar", uploadAvatarHandler)
	http.HandleFunc("/upload-image", uploadImageHandler)
	http.HandleFunc("/upload-file", uploadFileHandler)

	http.HandleFunc("/avatars/", func(w http.ResponseWriter, r *http.Request) {
		serveFileHandler(w, r, "/avatars/", avatarsPath)
	})
	http.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		serveFileHandler(w, r, "/images/", imagesPath)
	})
	http.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		serveFileHandler(w, r, "/files/", filesPath)
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

func handleUpload(w http.ResponseWriter, r *http.Request, formKey, saveDir, urlPrefix string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile(formKey)
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer closeFile(file)

	fileBytes, err := io.ReadAll(file)
	if err != nil {
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
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	publicIP := "159.195.38.145"
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}

	fileURL := fmt.Sprintf("http://%s:%s%s%s", publicIP, httpPort, urlPrefix, filename)
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

// DeleteImageFile deletes an image or file from the server
func DeleteImageFile(imageURL string) error {
	if imageURL == "" {
		return nil
	}

	// URL format: http://159.195.38.145:8082/[prefix]/filename
	parts := strings.Split(imageURL, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid file URL format")
	}

	filename := parts[len(parts)-1]
	prefix := parts[len(parts)-2]

	var saveDir string
	switch prefix {
	case "avatars":
		saveDir = avatarsPath
	case "images":
		saveDir = imagesPath
	case "files":
		saveDir = filesPath
	default:
		return fmt.Errorf("unknown file prefix: %s", prefix)
	}

	filePath := filepath.Join(saveDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // Already deleted
	}

	return os.Remove(filePath)
}
