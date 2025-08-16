package logger

import (
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	dcLogger *slog.Logger
	once     sync.Once
)

// SetupFromConfig configures the global logger from config (call this once in your main client)
func SetupFromConfig(debug bool, customLogger *slog.Logger) {
	if customLogger != nil {
		setGlobalLogger(customLogger)
		return
	}

	// Determine log level
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	} else if envDebug := os.Getenv("DATACRUNCH_DEBUG"); envDebug != "" {
		if strings.ToLower(envDebug) == "true" || envDebug == "1" {
			level = slog.LevelDebug
		}
	}

	var handler slog.Handler
	if level == slog.LevelDebug {
		// Debug mode: use text format for better readability
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		// Production mode: use JSON format
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	}

	setGlobalLogger(slog.New(handler))
}

// setGlobalLogger sets the global logger (internal)
func setGlobalLogger(logger *slog.Logger) {
	if logger == nil {
		logger = getDefaultLogger()
	}
	dcLogger = logger
	slog.SetDefault(dcLogger) // Also set as Go's default logger
}

// getGlobalLogger returns the current global logger
func getGlobalLogger() *slog.Logger {
	if dcLogger == nil {
		once.Do(func() {
			setGlobalLogger(getDefaultLogger())
		})
	}
	return dcLogger
}

// getDefaultLogger creates a basic default logger
func getDefaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Convenience functions - use these anywhere in your codebase

func Debug(msg string, args ...any) {
	getGlobalLogger().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	getGlobalLogger().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	getGlobalLogger().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	getGlobalLogger().Error(msg, args...)
}

func With(args ...any) *slog.Logger {
	return getGlobalLogger().With(args...)
}

// Utility functions for security

// SanitizeToken masks sensitive token data for logging
func SanitizeToken(token string) string {
	if token == "" {
		return "<empty>"
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	// Show first 4 and last 4 characters
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

// SanitizeBody truncates and sanitizes request/response body for logging
func SanitizeBody(body []byte, maxLen int) string {
	if len(body) == 0 {
		return "<empty>"
	}

	bodyStr := string(body)

	// Truncate if too long
	if maxLen > 0 && len(bodyStr) > maxLen {
		bodyStr = bodyStr[:maxLen] + "... (truncated)"
	}

	return bodyStr
}

// IsDebugEnabled checks if debug logging is enabled
func IsDebugEnabled() bool {
	return getGlobalLogger().Enabled(nil, slog.LevelDebug)
}
