package utils

import (
    "context"
    "fmt"
    "log"
    "os"
    "sync"

    firebase "firebase.google.com/go/v4"
    "firebase.google.com/go/v4/messaging"
    "google.golang.org/api/option"
)

var (
    FirebaseApp    *firebase.App
    FirebaseClient *messaging.Client
    once           sync.Once
    initErr        error
    isInitialized  bool
)

// InitFirebase initializes Firebase Admin SDK and FCM client (singleton pattern)
func InitFirebase() error {
    // Check if already initialized
    if isInitialized {
        log.Println("ℹ️  Firebase already initialized, skipping...")
        return initErr
    }

    once.Do(func() {
        ctx := context.Background()
        log.Println("🔄 Initializing Firebase...")

        // Get credentials path - prioritize GOOGLE_APPLICATION_CREDENTIALS
        credentialsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
        if credentialsPath == "" {
            credentialsPath = os.Getenv("FCM_CREDENTIALS_PATH")
        }
        if credentialsPath == "" {
            credentialsPath = "./serviceAccountKey.json"
        }

        // Get project ID from environment
        projectID := os.Getenv("FIREBASE_PROJECT_ID")
        if projectID == "" {
            projectID = os.Getenv("FCM_PROJECT_ID")
        }

        log.Printf("📂 Looking for Firebase credentials at: %s - FIREBASE_PROJECT_ID=%s", 
            credentialsPath, projectID)

        // Check if file exists
        if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
            log.Printf("⚠️  Firebase credentials file not found at: %s - FIREBASE_PROJECT_ID=%s", 
                credentialsPath, projectID)
            log.Println("ℹ️  Continuing without Firebase (push notifications will be disabled)")
            isInitialized = true
            initErr = fmt.Errorf("firebase credentials file not found: %s", credentialsPath)
            return
        }

        log.Println("✅ Firebase credentials file found")

        // Validate project ID
        if projectID == "" {
            log.Println("⚠️  FIREBASE_PROJECT_ID not set - FCM will not work properly")
            log.Println("ℹ️  Set FIREBASE_PROJECT_ID environment variable to enable FCM")
            isInitialized = true
            initErr = fmt.Errorf("FIREBASE_PROJECT_ID is required for FCM")
            return
        }

        log.Printf("✅ Using FIREBASE_PROJECT_ID: %s", projectID)

        // Create Firebase config with explicit project ID
        config := &firebase.Config{
            ProjectID: projectID,
        }

        // Initialize Firebase app
        opt := option.WithCredentialsFile(credentialsPath)
        app, err := firebase.NewApp(ctx, config, opt)
        if err != nil {
            log.Printf("❌ Error initializing Firebase app: %v", err)
            isInitialized = true
            initErr = fmt.Errorf("firebase app initialization failed: %v", err)
            return
        }

        log.Printf("✅ Firebase app initialized successfully for project: %s", projectID)

        // Try to get FCM client
        fcmClient, err := app.Messaging(ctx)
        if err != nil {
            log.Printf("❌ Error getting FCM client: %v", err)
            log.Println("ℹ️  Continuing without FCM (push notifications will be disabled)")
            
            // Store app but continue without FCM
            FirebaseApp = app
            FirebaseClient = nil
            isInitialized = true
            initErr = fmt.Errorf("FCM client initialization failed: %v", err)
            return
        }

        log.Println("✅ FCM client initialized successfully")

        // Store globally
        FirebaseApp = app
        FirebaseClient = fcmClient
        isInitialized = true
        initErr = nil
    })

    return initErr
}

// GetFCMClient returns the FCM client instance
func GetFCMClient() *messaging.Client {
    return FirebaseClient
}

// IsFCMEnabled checks if FCM is available
func IsFCMEnabled() bool {
    return FirebaseClient != nil
}

// GetFirebaseApp returns the Firebase app instance
func GetFirebaseApp() *firebase.App {
    return FirebaseApp
}

// GetInitError returns the initialization error if any
func GetInitError() error {
    return initErr
}

// ResetFirebase resets Firebase (for testing only - DO NOT use in production)
func ResetFirebase() {
    FirebaseApp = nil
    FirebaseClient = nil
    once = sync.Once{}
    initErr = nil
    isInitialized = false
}