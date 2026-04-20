// Lavender Messenger - HTTP Server for Avatar Uploads
// Author: Pavel Davydov (ferz)
//
// This file handles HTTP requests for avatar uploads and serving

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
	uploadPath      = "./uploads/avatars"
	defaultHTTPPort = "8082"
)

// closeFile safely closes a file and logs any error
func closeFile(file io.ReadCloser) {
	if err := file.Close(); err != nil {
		log.Printf("Error closing file: %v", err)
	}
}

// StartHTTPServer starts the HTTP server for avatar uploads
func StartHTTPServer(port string) {
	// Ensure upload directory exists
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		log.Printf("Failed to create upload directory: %v", err)
		return
	}

	http.HandleFunc("/upload-avatar", uploadAvatarHandler)
	http.HandleFunc("/upload-image", uploadImageHandler)
	http.HandleFunc("/avatars/", serveAvatarHandler)
	http.HandleFunc("/images/", serveImageHandler)

	log.Printf("HTTP server started on port %s", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		log.Printf("HTTP server error: %v", err)
	}
}

// uploadAvatarHandler handles avatar upload requests
func uploadAvatarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Printf("Failed to parse multipart form: %v", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("avatar")
	if err != nil {
		log.Printf("Failed to retrieve file from form: %v", err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer closeFile(file)

	// Validate file type
	contentType := handler.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		log.Printf("Invalid file type: %s", contentType)
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	// Read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	// Generate unique filename based on content hash
	hash := md5.Sum(fileBytes)
	// Determine file extension based on Content-Type
	var ext string
	switch contentType {
	case "image/gif":
		ext = ".gif"
	case "image/png":
		ext = ".png"
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	default:
		ext = ".jpg" // Default to jpg
	}
	filename := hex.EncodeToString(hash[:]) + ext
	filePath := filepath.Join(uploadPath, filename)

	// Save file
	if err := os.WriteFile(filePath, fileBytes, 0644); err != nil {
		log.Printf("Failed to save file: %v", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	log.Printf("Avatar uploaded successfully: %s", filename)

	// Return the URL
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}
	avatarURL := fmt.Sprintf("http://159.195.38.145:%s/avatars/%s", httpPort, filename)
	w.Header().Set("Content-Type", "application/json")
	if _, err := fmt.Fprintf(w, `{"url": "%s"}`, avatarURL); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// uploadImageHandler handles message image upload requests
func uploadImageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Printf("Failed to parse multipart form: %v", err)
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("image")
	if err != nil {
		log.Printf("Failed to retrieve file from form: %v", err)
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer closeFile(file)

	// Validate file type
	contentType := handler.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		log.Printf("Invalid file type: %s", contentType)
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	// Read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	// Generate unique filename based on content hash
	hash := md5.Sum(fileBytes)
	// Determine file extension based on Content-Type
	var ext string
	switch contentType {
	case "image/gif":
		ext = ".gif"
	case "image/png":
		ext = ".png"
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	default:
		ext = ".jpg" // Default to jpg
	}
	filename := hex.EncodeToString(hash[:]) + ext
	filePath := filepath.Join(uploadPath, filename)

	// Save file
	if err := os.WriteFile(filePath, fileBytes, 0644); err != nil {
		log.Printf("Failed to save file: %v", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	log.Printf("Image uploaded successfully: %s", filename)

	// Return the URL
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = defaultHTTPPort
	}
	imageURL := fmt.Sprintf("http://159.195.38.145:%s/images/%s", httpPort, filename)
	w.Header().Set("Content-Type", "application/json")
	if _, err := fmt.Fprintf(w, `{"url": "%s"}`, imageURL); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// serveAvatarHandler serves uploaded avatar images
func serveAvatarHandler(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/avatars/")
	filePath := filepath.Join(uploadPath, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Serve file
	http.ServeFile(w, r, filePath)
}

// serveImageHandler serves uploaded message images
func serveImageHandler(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/images/")
	filePath := filepath.Join(uploadPath, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Serve file
	http.ServeFile(w, r, filePath)
}

// DeleteImageFile deletes an image file given its URL
func DeleteImageFile(imageURL string) error {
	if imageURL == "" {
		return nil // Nothing to delete
	}

	// Extract filename from URL
	// URL format: http://159.195.38.145:8082/images/filename.jpg
	parts := strings.Split(imageURL, "/")
	if len(parts) < 1 {
		return fmt.Errorf("invalid image URL format")
	}

	filename := parts[len(parts)-1]
	filePath := filepath.Join(uploadPath, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("Image file not found, skipping deletion: %s", filename)
		return nil // File doesn't exist, nothing to delete
	}

	// Delete file
	if err := os.Remove(filePath); err != nil {
		log.Printf("Failed to delete image file %s: %v", filename, err)
		return err
	}

	log.Printf("Successfully deleted image file: %s", filename)
	return nil
}
