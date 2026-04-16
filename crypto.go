package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// getSecretKey подгружает ключ из переменной окружения
func getSecretKey() []byte {
	key := os.Getenv("CHAT_SECRET_KEY")
	if len(key) != 32 {
		// Для AES-256 ключ должен быть строго 32 байта
		panic("Критическая ошибка: CHAT_SECRET_KEY должен быть длиной 32 символа!")
	}
	return []byte(key)
}

func encrypt(text string) ([]byte, error) {
	key := getSecretKey()
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
	key := getSecretKey()
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
