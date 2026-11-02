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

// Get returns the global logger: l := logger.Get(); l.Info().Msg("...").
func Get() zerolog.Logger {
	return log
}
