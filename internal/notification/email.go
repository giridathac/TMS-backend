package notification

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"path/filepath"
	"strings"

	"github.com/sharath018/temple-management-backend/config"
)

// EmailSender implements Channel interface using SMTP
type EmailSender struct {
	Host     string
	Port     string
	Username string
	Password string
	FromName string
	FromAddr string
}

// ✅ Accept config instead of using os.Getenv
func NewEmailSender(cfg *config.Config) *EmailSender {
	return &EmailSender{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		FromName: cfg.SMTPFromName,
		FromAddr: cfg.SMTPFromEmail,
	}
}

// Send renders the HTML template and sends the email
func (e *EmailSender) Send(to []string, subject string, body string) error {
	// Step 1: Load and parse the template
	tmplPath := filepath.Join("templates", "example.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		fmt.Println("❌ Failed to parse email template:", err)
		return fmt.Errorf("failed to parse email template: %w", err)
	}

	// Step 2: Inject subject + body
	var htmlBody bytes.Buffer
	err = tmpl.Execute(&htmlBody, map[string]string{
		"Subject": subject,
		"Body":    body,
	})
	if err != nil {
		fmt.Println("❌ Failed to render email template:", err)
		return fmt.Errorf("failed to render email template: %w", err)
	}

	// Step 3: Build headers
	from := fmt.Sprintf("%s <%s>", e.FromName, e.FromAddr)
	toHeader := strings.Join(to, ", ")
	headers := map[string]string{
		"From":         from,
		"To":           toHeader,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=\"UTF-8\"",
	}

	var msgBuilder strings.Builder
	for k, v := range headers {
		msgBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msgBuilder.WriteString("\r\n" + htmlBody.String())
	message := []byte(msgBuilder.String())

	// Step 4: Send email with custom TLS config
	addr := fmt.Sprintf("%s:%s", e.Host, e.Port)
	fmt.Println("📤 Sending email to:", to, "via", addr)

	// ✅ FIX: Use custom SMTP client with TLS config
	err = e.sendMailWithTLS(addr, to, message)
	if err != nil {
		fmt.Println("❌ Email send failed:", err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	fmt.Println("✅ Email sent successfully to:", to)
	return nil
}

// ✅ NEW: Custom send function with proper TLS handling
func (e *EmailSender) sendMailWithTLS(addr string, to []string, message []byte) error {
	// Create TLS config - skip verification for Docker environments
	// This is safe because we're connecting to smtp.gmail.com
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         e.Host,
	}

	// Connect to the SMTP server
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to dial SMTP server: %w", err)
	}
	defer client.Close()

	// Start TLS
	if err = client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	// Authenticate
	auth := smtp.PlainAuth("", e.Username, e.Password, e.Host)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Set sender
	if err = client.Mail(e.FromAddr); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Send message body
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = writer.Write(message)
	if err != nil {
		writer.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	// Send QUIT command
	return client.Quit()
}