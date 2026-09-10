// Package log provides the default zap-based logger used by consistent_cache.
// Log output is written to standard output only.
package log

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// defaultLogger is created at package initialization with default options.
var defaultLogger = NewLogger(NewOptions())

// GetLogger returns the default logger instance.
func GetLogger() *Logger {
	return defaultLogger
}

// Options holds the logger configuration.
type Options struct {
	LogLevel string // log level: debug, info, warn, error or fatal
}

// Option configures Options.
type Option func(*Options)

// NewOptions builds Options with default values and applies the given options.
func NewOptions(opts ...Option) Options {
	options := Options{
		LogLevel: "info",
	}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// WithLogLevel sets the log level.
func WithLogLevel(level string) Option {
	return func(o *Options) {
		o.LogLevel = level
	}
}

// Levels maps level names to zapcore levels.
var Levels = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
	"fatal": zapcore.FatalLevel,
}

// Logger wraps a zap sugared logger. It satisfies the consistent_cache.Logger
// interface and writes all output to standard output.
type Logger struct {
	*zap.SugaredLogger
}

// NewLogger builds a logger that writes to standard output with the given options.
func NewLogger(options Options) *Logger {
	level, ok := Levels[options.LogLevel]
	if !ok {
		level = zapcore.InfoLevel
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// Record the log level with capital letters.
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// NewConsoleEncoder produces human-friendly output.
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)
	return &Logger{SugaredLogger: zap.New(core).Sugar()}
}
