// internal/notification/channel.go
package notification

import (
	"fmt"
)

// Channel defines the contract for sending notifications
type Channel interface {
	Send(to []string, subject string, body string) error
}

// EmailChannel - mock implementation
type EmailChannel struct{}

func NewEmailChannel() *EmailChannel {
	return &EmailChannel{}
}

func (e *EmailChannel) Send(to []string, subject string, body string) error {
	// You can later plug this with real SMTP logic
	for _, recipient := range to {
		fmt.Printf("📧 Sending EMAIL to: %s\nSubject: %s\nBody: %s\n\n", recipient, subject, body)
	}
	return nil
}

// WhatsAppChannel - mock
type WhatsAppChannel struct{}

func NewWhatsAppChannel() *WhatsAppChannel {
	return &WhatsAppChannel{}
}

func (w *WhatsAppChannel) Send(to []string, subject string, body string) error {
	for _, recipient := range to {
		fmt.Printf("📱 Sending WhatsApp to: %s\nMessage: %s\n\n", recipient, body)
	}
	return nil
}

// SMSChannel - mock
type SMSChannel struct{}

func NewSMSChannel() *SMSChannel {
	return &SMSChannel{}
}

func (s *SMSChannel) Send(to []string, subject string, body string) error {
	for _, recipient := range to {
		fmt.Printf("📲 Sending SMS to: %s\nMessage: %s\n\n", recipient, body)
	}
	return nil
}
