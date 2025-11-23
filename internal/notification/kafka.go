package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

var kafkaWriter *kafka.Writer

// 🔁 StartKafkaConsumer launches the background worker to process notifications
func StartKafkaConsumer(svc Service) {
	go func() {
		brokerURL := "kafka:9092" // must match docker-compose
		topic := "notifications"

		r := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  []string{brokerURL},
			Topic:    topic,
			GroupID:  "notification-consumers",
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
		})

		fmt.Println("🔁 Kafka Notification Worker Started...")

		for {
			m, err := r.ReadMessage(context.Background())
			if err != nil {
				log.Printf("❌ Kafka read error: %v", err)
				continue
			}

			var msg NotificationMessage
			if err := json.Unmarshal(m.Value, &msg); err != nil {
				log.Printf("❌ Failed to unmarshal Kafka message: %v", err)
				continue
			}

			// Process the notification
			// ✅ FIXED: Use IP from message or default to "kafka-worker"
			ip := msg.IPAddress
			if ip == "" {
				ip = "kafka-worker" // Default for Kafka workers
			}
			
			err = svc.SendNotification(
				context.Background(),
				msg.SenderID,
				msg.EntityID,
				msg.TemplateID,
				msg.Channel,
				msg.Subject,
				msg.Body,
				msg.Recipients,
				ip, // ✅ FIXED: Added IP parameter for audit logging
			)

			if err != nil {
				log.Printf("❌ Failed to send notification: %v", err)
			} else {
				log.Printf("✅ Notification sent via %s to %v", msg.Channel, msg.Recipients)
			}
		}
	}()
}

// 🔼 PublishNotification sends a message to the Kafka topic
func PublishNotification(msg NotificationMessage) error {
	initKafkaWriter()

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return kafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(time.Now().Format(time.RFC3339Nano)),
		Value: payload,
	})
}

// 🔧 Initialize the Kafka writer (used by PublishNotification)
func initKafkaWriter() {
	if kafkaWriter != nil {
		return
	}

	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP("kafka:9092"),
		Topic:    "notifications",
		Balancer: &kafka.LeastBytes{},
	}
}