package logging

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds logging configuration
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, pretty
}

// Setup initializes the global logger
func Setup(cfg Config) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set time format
	zerolog.TimeFieldFormat = time.RFC3339

	// Set output format
	var output io.Writer = os.Stdout
	if cfg.Format == "pretty" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05",
			NoColor:    false,
		}
	}

	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

// NewLogger creates a component-specific logger
func NewLogger(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

// WithRequestID creates a logger with request ID
func WithRequestID(requestID string) zerolog.Logger {
	return log.With().Str("request_id", requestID).Logger()
}

// ComponentLogger returns a logger for a specific component with request ID
func ComponentLogger(component, requestID string) zerolog.Logger {
	return log.With().
		Str("component", component).
		Str("request_id", requestID).
		Logger()
}
