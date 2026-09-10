package log

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

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

var defaultLogger Logger

func init() {
	defaultLogger = newSugarLogger(NewOptions())
}

// Options holds the logger configuration.
type Options struct {
	LogName  string // logger name
	LogLevel string // log level
}

// Option configures Options.
type Option func(*Options)

// NewOptions builds an Options instance with defaults.
func NewOptions(opts ...Option) Options {
	options := Options{
		LogName:  "app",
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

// Levels maps the configured level name onto a zapcore level.
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
	writeSyncer := w.getLogWriter()
	core := zapcore.NewCore(encoder, writeSyncer, Levels[options.LogLevel])
	w.SugaredLogger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
	return w
}

func (w *zapLoggerWrapper) getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Use capital letters for log levels in the output.
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// NewConsoleEncoder produces a human friendly layout.
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// getLogWriter returns a WriteSyncer that writes to stdout only.
// Logs are not rotated; rely on the surrounding container/daemon to handle output.
func (w *zapLoggerWrapper) getLogWriter() zapcore.WriteSyncer {
	return zapcore.AddSync(os.Stdout)
}

// GetDefaultLogger returns the default logger implementation.
func GetDefaultLogger() Logger {
	return defaultLogger
}

// Debugf logs a Debug message.
func Debugf(format string, args ...any) {
	GetDefaultLogger().Debugf(format, args...)
}

// Infof logs an Info message.
func Infof(format string, args ...any) {
	GetDefaultLogger().Infof(format, args...)
}

// Warnf logs a Warn message.
func Warnf(format string, args ...any) {
	GetDefaultLogger().Warnf(format, args...)
}

// Errorf logs an Error message.
func Errorf(format string, args ...any) {
	GetDefaultLogger().Errorf(format, args...)
}

// DebugContext logs a Debug message.
func DebugContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Debug(args...)
}

// DebugContextf logs a Debug message.
func DebugContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Debugf(format, args...)
}

// InfoContext logs an Info message.
func InfoContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Info(args...)
}

// InfoContextf logs an Info message.
func InfoContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Infof(format, args...)
}

// WarnContext logs a Warn message.
func WarnContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Warn(args...)
}

// WarnContextf logs a Warn message.
func WarnContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Warnf(format, args...)
}

// ErrorContext logs an Error message.
func ErrorContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Error(args...)
}

func ErrorContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Errorf(format, args...)
}

func Fatalf(format string, args ...any) {
	Errorf(format, args...)
}
