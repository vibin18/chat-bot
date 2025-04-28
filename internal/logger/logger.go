package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Logger interface defines the methods required for logging
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	WithField(key string, value any) Logger
	WithFields(fields map[string]any) Logger
	WithContext(ctx context.Context) Logger
}

// SlogLogger implements the Logger interface using slog
type SlogLogger struct {
	logger *slog.Logger
	ctx    context.Context
}

// New creates a new SlogLogger with the given level and output
func New(level slog.Level, output io.Writer) *SlogLogger {
	if output == nil {
		output = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewJSONHandler(output, opts)
	logger := slog.New(handler)

	return &SlogLogger{
		logger: logger,
		ctx:    context.Background(),
	}
}

// Default returns a new SlogLogger with default settings
func Default() *SlogLogger {
	return New(slog.LevelInfo, os.Stdout)
}

// Debug logs a debug message
func (l *SlogLogger) Debug(msg string, args ...any) {
	l.logger.LogAttrs(l.ctx, slog.LevelDebug, msg, slog.Any("args", args))
}

// Info logs an info message
func (l *SlogLogger) Info(msg string, args ...any) {
	l.logger.LogAttrs(l.ctx, slog.LevelInfo, msg, slog.Any("args", args))
}

// Warn logs a warning message
func (l *SlogLogger) Warn(msg string, args ...any) {
	l.logger.LogAttrs(l.ctx, slog.LevelWarn, msg, slog.Any("args", args))
}

// Error logs an error message
func (l *SlogLogger) Error(msg string, args ...any) {
	l.logger.LogAttrs(l.ctx, slog.LevelError, msg, slog.Any("args", args))
}

// WithField returns a new logger with the given field
func (l *SlogLogger) WithField(key string, value any) Logger {
	newLogger := l.logger.With(key, value)
	return &SlogLogger{
		logger: newLogger,
		ctx:    l.ctx,
	}
}

// WithFields returns a new logger with the given fields
func (l *SlogLogger) WithFields(fields map[string]any) Logger {
	var attrs []any
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	newLogger := l.logger.With(attrs...)
	return &SlogLogger{
		logger: newLogger,
		ctx:    l.ctx,
	}
}

// WithContext returns a new logger with the given context
func (l *SlogLogger) WithContext(ctx context.Context) Logger {
	return &SlogLogger{
		logger: l.logger,
		ctx:    ctx,
	}
}
