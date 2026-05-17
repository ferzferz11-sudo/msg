// Lavender Messenger - A secure messaging application
// Author: Pavel Davydov (ferz)
//
// This file implements email sending functionality for password reset.

package main

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

// SendPasswordResetEmail отправляет email с токеном для сброса пароля
func SendPasswordResetEmail(email, token string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	fromEmail := os.Getenv("SMTP_FROM")

	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPassword == "" || fromEmail == "" {
		return fmt.Errorf("SMTP_NOT_CONFIGURED")
	}

	// Формируем сообщение
	subject := "Password Reset - Lavender Messenger"
	body := fmt.Sprintf(`
You have requested a password reset for your Lavender Messenger account.

Your reset token is: %s

This token will expire in 1 hour.

If you did not request this password reset, please ignore this email.
`, token)

	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		fromEmail, email, subject, body)

	// Отправляем email
	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	err := smtp.SendMail(addr, auth, fromEmail, []string{email}, []byte(message))
	if err != nil {
		log.Printf("Failed to send email to %s: %v", email, err)
		return err
	}

	log.Printf("Password reset email sent to %s", email)
	return nil
}
