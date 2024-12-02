package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

type Logger struct {
	log     zerolog.Logger
	console zerolog.Logger
}

// NewLogger creates a new Logger instance.
//
// The logger has two outputs: a file logger that logs everything to a file
// in the given directory, and a console logger that logs everything to the
// console. The file logger is configured to log at the INFO level, and the
// console logger is configured to log at the DEBUG level.
//
// The method returns an error if it cannot create the log file or directory.
func NewLogger(logDir string) (*Logger, error) {
	// ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// create log file
	logFile := filepath.Join(logDir,
		fmt.Sprintf("api_test_%s.log", time.Now().Format("2006-01-02_15-04-05")))

	file, err := os.Create(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}
	fileLogger := zerolog.New(file).With().Timestamp().Str("component", "tmago").Logger()

	// Create console logger with colors
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05",
		NoColor:    false,
	}
	consoleLogger := zerolog.New(consoleWriter).
		Level(zerolog.DebugLevel).
		With().
		Timestamp().
		Logger()

	return &Logger{
		log:     fileLogger,
		console: consoleLogger,
	}, nil
}

// TestStarted logs a message when a test is started, including the name of the
// endpoint being tested, the HTTP method, and the URL.
func (l *Logger) TestStarted(endpoint string, method string, url string) {
	l.console.Info().
		Str("endpoint", endpoint).
		Str("method", method).
		Str("url", url).
		Msg("🚀 Starting test")

	l.log.Info().
		Str("endpoint", endpoint).
		Str("method", method).
		Str("url", url).
		Msg("Test started")
}

func (l *Logger) RequestStarted(id int, endpoint string) {
	l.log.Debug().
		Int("requestId", id).
		Str("endpoint", endpoint).
		Msg("Request started")
}

// RequestCompleted logs the completion of a request.
//
// The method logs the request ID, endpoint, duration and status code of the
// request to both the main logger and the console logger.
func (l *Logger) RequestCompleted(id int, endpoint string, duration time.Duration, statusCode int) {
	l.log.Info().
		Int("requestId", id).
		Str("endpoint", endpoint).
		Dur("duration", duration).
		Int("statusCode", statusCode).
		Msg("Request completed")

	l.console.Info().
		Int("requestId", id).
		Str("endpoint", endpoint).
		Dur("duration", duration).
		Int("statusCode", statusCode).
		Msg("✅ Request completed")
}

// RequestFailed logs a failed request to both the main logger and the console logger.
// The method logs the request ID, endpoint, and error.
func (l *Logger) RequestFailed(id int, endpoint string, err error) {
	l.log.Error().
		Int("requestId", id).
		Str("endpoint", endpoint).
		Err(err).
		Msg("Request failed")

	l.console.Error().
		Int("requestId", id).
		Str("endpoint", endpoint).
		Err(err).
		Msg("❌ Request failed")
}
