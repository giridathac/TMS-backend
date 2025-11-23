package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// ✅ Global constants (accessible from other packages)
var UploadPath = "./uploads"
var BaseURL = "http://localhost:8080"

type Config struct {
	Port string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	JWTAccessSecret    string
	JWTRefreshSecret   string
	JWTAccessTTLHours  int
	JWTRefreshTTLHours int

	// ✅ Redis Config
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// ✅ Razorpay Keys
	RazorpayKey    string
	RazorpaySecret string

	// ✅ SMTP Config
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromName  string
	SMTPFromEmail string

	// ✅ FCM Config
	FCMCredentialsPath string // Path to Firebase service account JSON
	FCMProjectID       string // Firebase Project ID (optional, can be in JSON)
}

// Load reads environment variables and returns a Config object
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file, using environment variables")
	}

	accessTTL, _ := strconv.Atoi(os.Getenv("JWT_ACCESS_TTL_HOURS"))
	refreshTTL, _ := strconv.Atoi(os.Getenv("JWT_REFRESH_TTL_HOURS"))
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

	return &Config{
		Port: os.Getenv("PORT"),

		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),

		JWTAccessSecret:    os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:   os.Getenv("JWT_REFRESH_SECRET"),
		JWTAccessTTLHours:  accessTTL,
		JWTRefreshTTLHours: refreshTTL,

		RedisAddr:     os.Getenv("REDIS_ADDR"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       redisDB,

		RazorpayKey:    os.Getenv("RAZORPAY_KEY_ID"),
		RazorpaySecret: os.Getenv("RAZORPAY_KEY_SECRET"),

		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      os.Getenv("SMTP_PORT"),
		SMTPUsername:  os.Getenv("SMTP_USERNAME"),
		SMTPPassword:  os.Getenv("SMTP_PASSWORD"),
		SMTPFromName:  os.Getenv("SMTP_FROM_NAME"),
		SMTPFromEmail: os.Getenv("SMTP_FROM_EMAIL"),

		FCMCredentialsPath: os.Getenv("FCM_CREDENTIALS_PATH"),
		FCMProjectID:       os.Getenv("FCM_PROJECT_ID"),
	}
}