// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements AES-256 encryption/decryption for secure message storage.
// It provides functions to encrypt and decrypt messages using GCM mode.

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// getSecretKey подгружает ключ из переменной окружения
func getSecretKey() ([]byte, error) {
	key := os.Getenv("CHAT_SECRET_KEY")
	if len(key) != 32 {
		// Вместо паники возвращаем понятную ошибку
		return nil, fmt.Errorf("CHAT_SECRET_KEY должен быть длиной 32 символа, получено: %d", len(key))
	}
	return []byte(key), nil
}

func encrypt(text string) ([]byte, error) {
	// 1. Получаем ключ и проверяем, нет ли ошибки его длины
	key, err := getSecretKey()
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Шифруем данные
	return gcm.Seal(nonce, nonce, []byte(text), nil), nil
}

func decrypt(ciphertext []byte) (string, error) {
	// 1. Быстрая проверка на сервисные маркеры
	if len(ciphertext) > 0 {
		cipherStr := string(ciphertext)
		switch cipherStr {
		case "SERVICE_VOICE_MSG":
			return "Voice message", nil
		case "SERVICE_MEDIA_MSG":
			return "Image", nil
		case "FIXED_BY_MAINTENANCE", "CORRUPTED_FIX", "EMPTY_FIX":
			return "Message", nil
		}
	}

	// 2. Получаем ключ и проверяем ошибку
	key, err := getSecretKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("неверная длина шифротекста")
	}

	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// HashPassword хеширует пароль с использованием bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword проверяет пароль против хеша
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
