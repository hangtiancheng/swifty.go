// Package log provides the default zap-based logger used across gotcc.
// All output is written to stdout; log rotation is intentionally left to
// external tooling (e.g. journald, logrotate or a container log driver).
package log

import (
	"context"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the logging interface consumed by gotcc and its users.
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
	defaultLogger = NewSugarLogger(NewOptions())
}

// Options holds the logger configuration.
type Options struct {
	// LogLevel is one of "debug", "info", "warn", "error" or "fatal"
	// (case-insensitive). An empty value means "debug".
	LogLevel string
}

// Option mutates Options.
type Option func(*Options)

// NewOptions builds an Options value, applying the given options on top of
// the defaults.
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

// Levels maps the textual level names understood by Options.LogLevel to
// zapcore levels. Unknown names fall back to InfoLevel.
var Levels = map[string]zapcore.Level{
	"":      zapcore.InfoLevel,
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
	"fatal": zapcore.FatalLevel,
}

// level resolves the configured level, falling back to info for unknown
// names so that a typo cannot silently disable logging.
func level(name string) zapcore.Level {
	if lv, ok := Levels[strings.ToLower(name)]; ok {
		return lv
	}
	return zapcore.InfoLevel
}

type zapLoggerWrapper struct {
	*zap.SugaredLogger
	options Options
}

// NewSugarLogger builds a console-encoded sugared zap logger writing to
// stdout.
func NewSugarLogger(options Options) *zapLoggerWrapper {
	w := &zapLoggerWrapper{options: options}
	encoder := w.getEncoder()
	writeSyncer := w.getLogWriter()
	core := zapcore.NewCore(encoder, writeSyncer, level(options.LogLevel))
	w.SugaredLogger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
	return w
}

func (w *zapLoggerWrapper) getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// Print the level in capital letters, e.g. INFO, ERROR.
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// The console encoder keeps single-line, human readable output.
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func (w *zapLoggerWrapper) getLogWriter() zapcore.WriteSyncer {
	return zapcore.AddSync(os.Stdout)
}

// GetDefaultLogger returns the process-wide default logger.
func GetDefaultLogger() Logger {
	return defaultLogger
}

// Debugf logs at debug level.
func Debugf(format string, args ...any) {
	GetDefaultLogger().Debugf(format, args...)
}

// Infof logs at info level.
func Infof(format string, args ...any) {
	GetDefaultLogger().Infof(format, args...)
}

// Warnf logs at warn level.
func Warnf(format string, args ...any) {
	GetDefaultLogger().Warnf(format, args...)
}

// Errorf logs at error level.
func Errorf(format string, args ...any) {
	GetDefaultLogger().Errorf(format, args...)
}

// DebugContext logs at debug level.
func DebugContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Debug(args...)
}

// DebugContextf logs at debug level.
func DebugContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Debugf(format, args...)
}

// InfoContext logs at info level.
func InfoContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Info(args...)
}

// InfoContextf logs at info level.
func InfoContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Infof(format, args...)
}

// WarnContext logs at warn level.
func WarnContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Warn(args...)
}

// WarnContextf logs at warn level.
func WarnContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Warnf(format, args...)
}

// ErrorContext logs at error level.
func ErrorContext(ctx context.Context, args ...any) {
	GetDefaultLogger().Error(args...)
}

// ErrorContextf logs at error level.
func ErrorContextf(ctx context.Context, format string, args ...any) {
	GetDefaultLogger().Errorf(format, args...)
}

// Fatalf logs at error level. It deliberately does not exit the process; a
// library must not terminate the host application.
func Fatalf(format string, args ...any) {
	Errorf(format, args...)
}
