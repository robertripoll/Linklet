package main

import (
	"log/slog"
	"os"
	"sync"
)

// Logger wraps slog.Logger to provide a centralized logging interface.
type Logger struct {
	*slog.Logger
}

var (
	loggerInstance *Logger
	loggerOnce     sync.Once
)

// GetLogger returns the singleton Logger instance.
func GetLogger() *Logger {
	loggerOnce.Do(func() {
		handler := slog.NewTextHandler(os.Stdout, nil)
		loggerInstance = &Logger{
			Logger: slog.New(handler),
		}
	})
	return loggerInstance
}

// Info logs an informational message.
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}
