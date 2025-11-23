package notification

import "time"

// NotificationMessage defines Kafka payload structure
type NotificationMessage struct {
	TemplateID *uint     `json:"template_id,omitempty"`
	Channel    string    `json:"channel"`       // email, sms, whatsapp
	Subject    string    `json:"subject"`       // email only
	Body       string    `json:"body"`          // rendered message
	Recipients []string  `json:"recipients"`    // email or phone list
	EntityID   uint      `json:"entity_id"`     // temple
	SenderID   uint      `json:"sender_id"`     // templeadmin
	SentAt     time.Time `json:"sent_at"`       // time sent
	IPAddress  string    `json:"ip_address"`    // ✅ NEW: For audit logging
}