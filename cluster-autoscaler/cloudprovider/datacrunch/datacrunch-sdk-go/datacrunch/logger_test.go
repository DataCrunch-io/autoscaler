package datacrunch

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestNoOpLogger(t *testing.T) {
	logger := &NoOpLogger{}

	// These should not panic and should do nothing
	logger.Debug("test debug")
	logger.Info("test info")
	logger.Warn("test warn")
	logger.Error("test error")
}

func TestStdLogger(t *testing.T) {
	var buf bytes.Buffer

	// Create logger with custom output
	stdLogger := &StdLogger{
		debugEnabled: true,
		logger:       log.New(&buf, "[Test] ", 0),
	}

	stdLogger.Debug("debug message")
	stdLogger.Info("info message")
	stdLogger.Warn("warn message")
	stdLogger.Error("error message")

	output := buf.String()

	// Check that all messages were logged
	if !strings.Contains(output, "[DEBUG] debug message") {
		t.Error("Debug message not found in output")
	}
	if !strings.Contains(output, "[INFO] info message") {
		t.Error("Info message not found in output")
	}
	if !strings.Contains(output, "[WARN] warn message") {
		t.Error("Warn message not found in output")
	}
	if !strings.Contains(output, "[ERROR] error message") {
		t.Error("Error message not found in output")
	}
}

func TestStdLoggerDebugDisabled(t *testing.T) {
	var buf bytes.Buffer

	// Create logger with debug disabled
	stdLogger := &StdLogger{
		debugEnabled: false,
		logger:       log.New(&buf, "[Test] ", 0),
	}

	stdLogger.Debug("debug message")
	stdLogger.Info("info message")

	output := buf.String()

	// Debug should not be logged
	if strings.Contains(output, "[DEBUG] debug message") {
		t.Error("Debug message should not be logged when debug is disabled")
	}

	// Info should still be logged
	if !strings.Contains(output, "[INFO] info message") {
		t.Error("Info message not found in output")
	}
}

func TestSlogLogger(t *testing.T) {
	var buf bytes.Buffer

	slogLogger := &SlogLogger{
		logger: log.New(&buf, "", 0),
		debug:  true,
	}

	slogLogger.Debug("debug message")
	slogLogger.Info("info message")
	slogLogger.Warn("warn message")
	slogLogger.Error("error message")

	output := buf.String()

	// Check that all messages were logged
	if !strings.Contains(output, "[DEBUG] debug message") {
		t.Error("Debug message not found in output")
	}
	if !strings.Contains(output, "[INFO] info message") {
		t.Error("Info message not found in output")
	}
	if !strings.Contains(output, "[WARN] warn message") {
		t.Error("Warn message not found in output")
	}
	if !strings.Contains(output, "[ERROR] error message") {
		t.Error("Error message not found in output")
	}
}

func TestNewStdLogger(t *testing.T) {
	// Test with debug enabled
	logger1 := NewStdLogger(true)
	if !logger1.debugEnabled {
		t.Error("Debug should be enabled")
	}

	// Test with debug disabled
	logger2 := NewStdLogger(false)
	if logger2.debugEnabled {
		t.Error("Debug should be disabled")
	}
}

func TestNewSlogLogger(t *testing.T) {
	// Test with debug enabled
	logger1 := NewSlogLogger(true)
	if !logger1.debug {
		t.Error("Debug should be enabled")
	}

	// Test with debug disabled
	logger2 := NewSlogLogger(false)
	if logger2.debug {
		t.Error("Debug should be disabled")
	}
}

func TestClientWithLogger(t *testing.T) {
	var buf bytes.Buffer
	customLogger := &StdLogger{
		debugEnabled: true,
		logger:       log.New(&buf, "[Test] ", 0),
	}

	client, err := NewClient(
		WithClientID("test_id"),
		WithClientSecret("test_secret"),
		WithLogger(customLogger),
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the logger was set
	if client.Logger != customLogger {
		t.Error("Custom logger was not set on client")
	}
}

func TestClientWithDebugLogging(t *testing.T) {
	client, err := NewClient(
		WithClientID("test_id"),
		WithClientSecret("test_secret"),
		WithDebugLogging(true),
	)

	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Verify the logger is a StdLogger
	if _, ok := client.Logger.(*StdLogger); !ok {
		t.Errorf("Expected StdLogger, got %T", client.Logger)
	}
}
