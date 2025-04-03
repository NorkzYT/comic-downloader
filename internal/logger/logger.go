// Package logger provides simple logging functions.
// Debug messages are printed only if the DEBUG environment variable is set to "true" or "1".
package logger

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

var debugEnabled bool

func init() {
	// Load environment variables from .env file if available.
	if err := godotenv.Load(); err != nil {
		log.Printf("logger: no .env file found or error loading .env: %v", err)
	}

	// Check if DEBUG environment variable is set to "true" (case-insensitive) or "1"
	debugValue := os.Getenv("DEBUG")
	if strings.EqualFold(debugValue, "true") || debugValue == "1" {
		debugEnabled = true
	}

	// Set log flags to include date and time.
	log.SetFlags(log.LstdFlags)
}

// Debug prints debug-level logs if debugEnabled is true.
// Use it for detailed information useful for troubleshooting.
func Debug(format string, args ...interface{}) {
	if debugEnabled {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Info prints informational messages.
func Info(format string, args ...interface{}) {
	log.Printf("[INFO] "+format, args...)
}

// Error prints error messages.
func Error(format string, args ...interface{}) {
	log.Printf("[ERROR] "+format, args...)
}
