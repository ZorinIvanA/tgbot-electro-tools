// Package main Telegram Bot Electro Tools API
//
// @title Telegram Bot Electro Tools API
// @version 1.0
// @description API for managing Telegram Bot Electro Tools settings and metrics
// @host localhost:54321
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter the token in the format: Bearer {token}
package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	_ "github.com/ZorinIvanA/tgbot-electro-tools/docs"
	"github.com/ZorinIvanA/tgbot-electro-tools/internal/api"
	"github.com/ZorinIvanA/tgbot-electro-tools/internal/bot"
	"github.com/ZorinIvanA/tgbot-electro-tools/internal/metrics"
	"github.com/ZorinIvanA/tgbot-electro-tools/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Get configuration from environment
	config := loadConfig()

	// Validate required configuration
	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize database
	log.Println("Connecting to database...")
	db, err := storage.NewPostgresStorage(
		config.DBHost,
		config.DBPort,
		config.DBUser,
		config.DBPassword,
		config.DBName,
		config.DBSSLMode,
	)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected successfully")

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector(db)

	// Initialize bot
	log.Println("Initializing Telegram bot...")
	telegramBot, err := bot.NewBot(config.TelegramBotToken, db, config.RateLimitPerMinute, config.OpenAIEnabled, config.OpenAIAPIURL, config.OpenAIAPIKey, config.OpenAIModel)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}
	log.Printf("Bot initialized: @%s", telegramBot.GetUsername())

	// Initialize HTTP API server
	apiServer := api.NewServer(db, metricsCollector, config.AdminAPIToken, config.HTTPPort, config.DebugMode)

	// Start HTTP API server in a separate goroutine
	go func() {
		log.Println("Starting HTTP API server...")
		if err := apiServer.Start(); err != nil {
			log.Fatalf("HTTP API server error: %v", err)
		}
	}()

	// Start bot in a separate goroutine
	go func() {
		log.Println("Starting Telegram bot...")
		if err := telegramBot.Start(); err != nil {
			log.Fatalf("Bot error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")
	telegramBot.Stop()
	log.Println("Bot stopped")
}

// Config holds application configuration
type Config struct {
	TelegramBotToken   string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBSSLMode          string
	HTTPPort           string
	AdminAPIToken      string
	RateLimitPerMinute int
	OpenAIEnabled      bool
	OpenAIAPIURL       string
	OpenAIAPIKey       string
	OpenAIModel        string
	DebugMode          bool
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	rateLimitStr := getEnv("RATE_LIMIT_PER_MINUTE", "10")
	rateLimit, err := strconv.Atoi(rateLimitStr)
	if err != nil {
		log.Printf("Warning: invalid RATE_LIMIT_PER_MINUTE value, using default: 10")
		rateLimit = 10
	}

	openAIEnabledStr := getEnv("OPENAI_API_ENABLED", "false")
	openAIEnabled := openAIEnabledStr == "true"

	debugModeStr := getEnv("DEBUG_MODE", "false")
	debugMode := debugModeStr == "true"

	return &Config{
		TelegramBotToken:   getEnv("TELEGRAM_BOT_TOKEN", ""),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPassword:         getEnv("DB_PASSWORD", "postgres"),
		DBName:             getEnv("DB_NAME", "electro_tools_bot"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		HTTPPort:           getEnv("HTTP_PORT", "8080"),
		AdminAPIToken:      getEnv("ADMIN_API_TOKEN", ""),
		RateLimitPerMinute: rateLimit,
		OpenAIEnabled:      openAIEnabled,
		OpenAIAPIURL:       getEnv("OPENAI_API_URL", "https://bothub.ru/v1"),
		OpenAIAPIKey:       getEnv("OPENAI_API_KEY", ""),
		OpenAIModel:        getEnv("OPENAI_MODEL", "gpt-3.5-turbo"),
		DebugMode:          debugMode,
	}
}

// validateConfig validates required configuration
func validateConfig(config *Config) error {
	if config.TelegramBotToken == "" {
		return &ConfigError{Field: "TELEGRAM_BOT_TOKEN", Message: "is required"}
	}
	if config.AdminAPIToken == "" {
		return &ConfigError{Field: "ADMIN_API_TOKEN", Message: "is required"}
	}
	return nil
}

// getEnv gets environment variable with fallback to default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + " " + e.Message
}
