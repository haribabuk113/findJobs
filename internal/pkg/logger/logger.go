package logger

import (
	"os"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func init() {
	if os.Getenv("ENVIRONMENT") == "prod" {
		// Production: JSON output for machine readability and log aggregation
		log = zerolog.New(os.Stdout).Level(zerolog.InfoLevel).With().Timestamp().Logger()
	} else {
		// Development: Human-readable console output with colors
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	}
}

// Get returns the global logger instance
func Get() zerolog.Logger {
	return log
}

// Info logs a message at Info level
func Info() zerolog.Event {
	return *log.Info()
}

// Warn logs a message at Warn level
func Warn() zerolog.Event {
	return *log.Warn()
}

// Error logs a message at Error level
func Error() zerolog.Event {
	return *log.Error()
}

// Fatal logs a message at Fatal level and exits
func Fatal() zerolog.Event {
	return *log.Fatal()
}

// Debug logs a message at Debug level
func Debug() zerolog.Event {
	return *log.Debug()
}

// SetLevel sets the global log level
func SetLevel(level zerolog.Level) {
	log = log.Level(level)
}

// GetLevel returns the current global log level
func GetLevel() zerolog.Level {
	return log.GetLevel()
}
