package notification

import (
	"context"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/sharath018/temple-management-backend/config"
	"google.golang.org/api/option"
)

// FCMChannel implements the Channel interface for Firebase Cloud Messaging
type FCMChannel struct {
	client *messaging.Client
	ctx    context.Context
}

// NewFCMChannel initializes FCM with service account credentials
func NewFCMChannel(cfg *config.Config) Channel {
	ctx := context.Background()

	// Check if FCM is configured
	if cfg.FCMCredentialsPath == "" {
		log.Println("⚠️  FCM not configured (FCM_CREDENTIALS_PATH missing)")
		return &FCMChannel{client: nil, ctx: ctx}
	}

	// Initialize Firebase app with service account
	opt := option.WithCredentialsFile(cfg.FCMCredentialsPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Printf("❌ Error initializing Firebase app: %v\n", err)
		return &FCMChannel{client: nil, ctx: ctx}
	}

	// ✅ Added debug log to confirm Firebase project is set correctly
	log.Println("✅ Firebase app initialized successfully for project:", cfg.FCMProjectID)

	// Get messaging client
	client, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("❌ Error getting FCM client: %v\n", err)
		return &FCMChannel{client: nil, ctx: ctx}
	}

	log.Println("✅ FCM initialized successfully")
	return &FCMChannel{
		client: client,
		ctx:    ctx,
	}
}

// Send implements Channel interface for FCM
// recipients should be FCM device tokens
// subject is used as notification title
// body is the notification body
func (f *FCMChannel) Send(recipients []string, subject, body string) error {
	if f.client == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	if len(recipients) == 0 {
		return fmt.Errorf("no FCM tokens provided")
	}

	// If only one recipient, send single message
	if len(recipients) == 1 {
		return f.sendSingle(recipients[0], subject, body)
	}

	// For multiple recipients, use multicast
	return f.sendMulticast(recipients, subject, body)
}

// sendSingle sends notification to a single device token
func (f *FCMChannel) sendSingle(token, title, body string) error {
	message := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound:        "default",
				ChannelID:    "temple_notifications",
				Priority:     messaging.PriorityHigh,
				DefaultSound: true,
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
					Badge: intPtr(1),
				},
			},
		},
		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: title,
				Body:  body,
				Icon:  "/icon-192x192.png",
			},
		},
	}

	response, err := f.client.Send(f.ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send FCM message: %v", err)
	}

	log.Printf("✅ FCM message sent successfully: %s\n", response)
	return nil
}

// sendMulticast sends notification to multiple device tokens
func (f *FCMChannel) sendMulticast(tokens []string, title, body string) error {
	// FCM allows max 500 tokens per multicast
	batchSize := 500
	var failedTokens []string
	successCount := 0

	for i := 0; i < len(tokens); i += batchSize {
		end := i + batchSize
		if end > len(tokens) {
			end = len(tokens)
		}

		batch := tokens[i:end]
		message := &messaging.MulticastMessage{
			Tokens: batch,
			Notification: &messaging.Notification{
				Title: title,
				Body:  body,
			},
			Android: &messaging.AndroidConfig{
				Priority: "high",
				Notification: &messaging.AndroidNotification{
					Sound:        "default",
					ChannelID:    "temple_notifications",
					Priority:     messaging.PriorityHigh,
					DefaultSound: true,
				},
			},
			APNS: &messaging.APNSConfig{
				Payload: &messaging.APNSPayload{
					Aps: &messaging.Aps{
						Sound: "default",
						Badge: intPtr(1),
					},
				},
			},
			Webpush: &messaging.WebpushConfig{
				Notification: &messaging.WebpushNotification{
					Title: title,
					Body:  body,
					Icon:  "/icon-192x192.png",
				},
			},
		}

		response, err := f.client.SendMulticast(f.ctx, message)
		if err != nil {
			log.Printf("❌ Error sending FCM multicast batch: %v\n", err)
			failedTokens = append(failedTokens, batch...)
			continue
		}

		successCount += response.SuccessCount
		log.Printf("✅ FCM multicast: %d/%d messages sent successfully\n",
			response.SuccessCount, len(batch))

		// Collect failed tokens for logging
		if response.FailureCount > 0 {
			for idx, resp := range response.Responses {
				if !resp.Success {
					failedTokens = append(failedTokens, batch[idx])
					log.Printf("❌ Failed to send to token %s: %v\n",
						batch[idx][:20]+"...", resp.Error)
				}
			}
		}
	}

	if len(failedTokens) > 0 {
		return fmt.Errorf("failed to send to %d/%d tokens", len(failedTokens), len(tokens))
	}

	log.Printf("✅ All FCM messages sent: %d tokens\n", successCount)
	return nil
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

// SendToTopic sends notification to an FCM topic
func (f *FCMChannel) SendToTopic(topic, title, body string) error {
	if f.client == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	message := &messaging.Message{
		Topic: topic,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
	}

	response, err := f.client.Send(f.ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send topic message: %v", err)
	}

	log.Printf("✅ FCM topic message sent: %s\n", response)
	return nil
}

// SubscribeToTopic subscribes tokens to a topic
func (f *FCMChannel) SubscribeToTopic(tokens []string, topic string) error {
	if f.client == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	response, err := f.client.SubscribeToTopic(f.ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %v", err)
	}

	log.Printf("✅ Subscribed %d tokens to topic '%s' (failures: %d)\n",
		response.SuccessCount, topic, response.FailureCount)
	return nil
}

// UnsubscribeFromTopic unsubscribes tokens from a topic
func (f *FCMChannel) UnsubscribeFromTopic(tokens []string, topic string) error {
	if f.client == nil {
		return fmt.Errorf("FCM client not initialized")
	}

	response, err := f.client.UnsubscribeFromTopic(f.ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %v", err)
	}

	log.Printf("✅ Unsubscribed %d tokens from topic '%s' (failures: %d)\n",
		response.SuccessCount, topic, response.FailureCount)
	return nil
}
