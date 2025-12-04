package config

import (
	"log"
	"strings"
)

// ConfigureLogging configures the logging based on the configuration
func ConfigureLogging(config LoggingConfig) {
	// Set log level (simplified - in a real implementation you'd use a proper logging library)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if strings.ToLower(config.Level) == "debug" {
		log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	}
}
