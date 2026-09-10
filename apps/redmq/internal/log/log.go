// Package log provides the default structured logger used by redmq.
// All logs are written to standard output through zap.
package log

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the logging interface used by redmq.
type Logger interface {
	Error(v ...any)
	Warn(v ...any)
	Info(v ...any)
	Debug(v ...any)
	Errorf(format string, v ...any)
	Warnf(format string, v ...any)
	Infof(format string, v ...any)
	Debugf(format string, v ...any)
}

var (
	defaultLogger Logger
)

func init() {
	defaultLogger = newSugarLogger(NewOptions())
}

// Options holds the logger configuration.
type Options struct {
	LogName  string // logger name attached to every entry
	LogLevel string // log level: debug, info, warn, error or fatal
}

// Option customises the logger options.
type Option func(*Options)

// NewOptions builds logger options with defaults applied.
func NewOptions(opts ...Option) Options {
	options := Options{
		LogName:  "redmq",
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

// Levels maps level names onto zapcore levels.
var Levels = map[string]zapcore.Level{
	"":      zapcore.DebugLevel,
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
	"fatal": zapcore.FatalLevel,
}

type zapLoggerWrapper struct {
	*zap.SugaredLogger
	options Options
}

func newSugarLogger(options Options) *zapLoggerWrapper {
	w := &zapLoggerWrapper{options: options}
	encoder := w.getEncoder()
	writeSyncer := w.getWriteSyncer()
	core := zapcore.NewCore(encoder, writeSyncer, Levels[options.LogLevel])
	w.SugaredLogger = zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.Fields(zap.String("logger", options.LogName)),
	).Sugar()
	return w
}

func (w *zapLoggerWrapper) getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Use capital letters for log levels.
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// NewConsoleEncoder produces human readable output.
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// getWriteSyncer returns the destination for log entries, standard output.
func (w *zapLoggerWrapper) getWriteSyncer() zapcore.WriteSyncer {
	return zapcore.AddSync(os.Stdout)
}

// GetDefaultLogger returns the default logger implementation.
func GetDefaultLogger() Logger {
	return defaultLogger
}

// Debugf logs a message at debug level.
func Debugf(format string, args ...any) {
	GetDefaultLogger().Debugf(format, args...)
}

// Infof logs a message at info level.
func Infof(format string, args ...any) {
	GetDefaultLogger().Infof(format, args...)
}

// Warnf logs a message at warn level.
func Warnf(format string, args ...any) {
	GetDefaultLogger().Warnf(format, args...)
}

// Errorf logs a message at error level.
func Errorf(format string, args ...any) {
	GetDefaultLogger().Errorf(format, args...)
}

// DebugContext logs a message at debug level.
func DebugContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Debug(args...)
}

// DebugContextf logs a formatted message at debug level.
func DebugContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Debugf(format, args...)
}

// InfoContext logs a message at info level.
func InfoContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Info(args...)
}

// InfoContextf logs a formatted message at info level.
func InfoContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Infof(format, args...)
}

// WarnContext logs a message at warn level.
func WarnContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Warn(args...)
}

// WarnContextf logs a formatted message at warn level.
func WarnContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Warnf(format, args...)
}

// ErrorContext logs a message at error level.
func ErrorContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Error(args...)
}

// ErrorContextf logs a formatted message at error level.
func ErrorContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Errorf(format, args...)
}

// Fatalf logs a formatted message at error level.
func Fatalf(format string, args ...any) {
	Errorf(format, args...)
}
