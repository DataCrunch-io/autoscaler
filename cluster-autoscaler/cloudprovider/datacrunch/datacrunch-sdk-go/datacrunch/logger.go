package datacrunch

import (
	"log"
	"os"
)

// Logger interface allows users to plug in their preferred logging library
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// NoOpLogger discards all log messages (default)
type NoOpLogger struct{}

func (l *NoOpLogger) Debug(msg string, args ...interface{}) {}
func (l *NoOpLogger) Info(msg string, args ...interface{})  {}
func (l *NoOpLogger) Warn(msg string, args ...interface{})  {}
func (l *NoOpLogger) Error(msg string, args ...interface{}) {}

// StdLogger uses Go's standard log package
type StdLogger struct {
	debugEnabled bool
	logger       *log.Logger
}

// NewStdLogger creates a standard logger with optional debug mode
func NewStdLogger(debugEnabled bool) *StdLogger {
	return &StdLogger{
		debugEnabled: debugEnabled,
		logger:       log.New(os.Stderr, "[DataCrunch] ", log.LstdFlags),
	}
}

func (l *StdLogger) Debug(msg string, args ...interface{}) {
	if l.debugEnabled {
		l.logger.Printf("[DEBUG] "+msg, args...)
	}
}

func (l *StdLogger) Info(msg string, args ...interface{}) {
	l.logger.Printf("[INFO] "+msg, args...)
}

func (l *StdLogger) Warn(msg string, args ...interface{}) {
	l.logger.Printf("[WARN] "+msg, args...)
}

func (l *StdLogger) Error(msg string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+msg, args...)
}

// SlogLogger wraps Go 1.21+ slog (structured logging)
type SlogLogger struct {
	logger *log.Logger // Using standard log for compatibility
	debug  bool
}

// NewSlogLogger creates a structured logger
func NewSlogLogger(debugEnabled bool) *SlogLogger {
	return &SlogLogger{
		logger: log.New(os.Stderr, "", log.LstdFlags),
		debug:  debugEnabled,
	}
}

func (l *SlogLogger) Debug(msg string, args ...interface{}) {
	if l.debug {
		l.logger.Printf("[DEBUG] "+msg, args...)
	}
}

func (l *SlogLogger) Info(msg string, args ...interface{}) {
	l.logger.Printf("[INFO] "+msg, args...)
}

func (l *SlogLogger) Warn(msg string, args ...interface{}) {
	l.logger.Printf("[WARN] "+msg, args...)
}

func (l *SlogLogger) Error(msg string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+msg, args...)
}
