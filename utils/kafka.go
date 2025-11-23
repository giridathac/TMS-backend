package utils

import (
	"log"
	"os"

	"github.com/segmentio/kafka-go"
)

var kafkaWriter *kafka.Writer

// InitializeKafka sets up the global Kafka writer
func InitializeKafka() {
	brokerURL := os.Getenv("KAFKA_BROKER_URL")
	if brokerURL == "" {
		log.Fatal("❌ KAFKA_BROKER_URL not set in .env")
	}

	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(brokerURL),
		Topic:    "notifications",
		Balancer: &kafka.LeastBytes{},
	}
}

// GetKafkaWriter returns the shared writer instance (used by notification/kafka.go)
func GetKafkaWriter() *kafka.Writer {
	if kafkaWriter == nil {
		InitializeKafka()
	}
	return kafkaWriter
}
