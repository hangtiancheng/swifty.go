package log

import (
	"context"
	"fmt"
	"log"
	"os"
)

type Logger interface {
	Error(v ...interface{})
	Warn(v ...interface{})
	Info(v ...interface{})
	Debug(v ...interface{})
	Errorf(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Debugf(format string, v ...interface{})
}

var (
	defaultLogger Logger
)

func init() {
	defaultLogger = NewSugarLogger(NewOptions())
}

// Options 选项配置
type Options struct {
	LogName    string // 日志名称
	LogLevel   string // 日志级别
	FileName   string // 文件名称
	MaxAge     int    // 日志保留时间，以天为单位
	MaxSize    int    // 日志保留大小，以 M 为单位
	MaxBackups int    // 保留文件个数
	Compress   bool   // 是否压缩
}

// Option 选项方法
type Option func(*Options)

// NewOptions 初始化
func NewOptions(opts ...Option) Options {

	options := Options{
		LogName:    "app",
		LogLevel:   "info",
		FileName:   "app.log",
		MaxAge:     10,
		MaxSize:    100,
		MaxBackups: 3,
		Compress:   true,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// WithLogLevel 日志级别
func WithLogLevel(level string) Option {
	return func(o *Options) {
		o.LogLevel = level
	}
}

// WithFileName 日志文件
func WithFileName(filename string) Option {
	return func(o *Options) {
		o.FileName = filename
	}
}

// Levels log level
var Levels = map[string]int{
	"":      0,
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
	"fatal": 4,
}

type stdLoggerWrapper struct {
	logger *log.Logger
	level  int
}

func NewSugarLogger(options Options) *stdLoggerWrapper {
	file, err := os.OpenFile(options.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		file = os.Stderr
	}
	return &stdLoggerWrapper{
		logger: log.New(file, "", log.LstdFlags|log.Lshortfile),
		level:  Levels[options.LogLevel],
	}
}

func (w *stdLoggerWrapper) Debug(v ...interface{}) {
	if w.level <= 0 {
		w.logger.Output(2, fmt.Sprint(v...))
	}
}

func (w *stdLoggerWrapper) Info(v ...interface{}) {
	if w.level <= 1 {
		w.logger.Output(2, fmt.Sprint(v...))
	}
}

func (w *stdLoggerWrapper) Warn(v ...interface{}) {
	if w.level <= 2 {
		w.logger.Output(2, fmt.Sprint(v...))
	}
}

func (w *stdLoggerWrapper) Error(v ...interface{}) {
	if w.level <= 3 {
		w.logger.Output(2, fmt.Sprint(v...))
	}
}

func (w *stdLoggerWrapper) Debugf(format string, v ...interface{}) {
	if w.level <= 0 {
		w.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (w *stdLoggerWrapper) Infof(format string, v ...interface{}) {
	if w.level <= 1 {
		w.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (w *stdLoggerWrapper) Warnf(format string, v ...interface{}) {
	if w.level <= 2 {
		w.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (w *stdLoggerWrapper) Errorf(format string, v ...interface{}) {
	if w.level <= 3 {
		w.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

// GetDefaultLogger 获取默认日志实现
func GetDefaultLogger() Logger {
	return defaultLogger
}

// Debugf 打印 Debug 日志
func Debugf(format string, args ...interface{}) {
	GetDefaultLogger().Debugf(format, args...)
}

// Infof 打印 Info 日志
func Infof(format string, args ...interface{}) {
	GetDefaultLogger().Infof(format, args...)
}

// Warnf 打印 Warn 日志
func Warnf(format string, args ...interface{}) {
	GetDefaultLogger().Warnf(format, args...)
}

// Errorf 打印 Error 日志
func Errorf(format string, args ...interface{}) {
	GetDefaultLogger().Errorf(format, args...)
}

// DebugContext 打印 Debug 日志
func DebugContext(ctx context.Context, args ...interface{}) {
	GetDefaultLogger().Debug(args...)
}

// DebugContextf 打印 Debug 日志
func DebugContextf(ctx context.Context, format string, args ...interface{}) {
	GetDefaultLogger().Debugf(format, args...)
}

// InfoContext 打印 Info 日志
func InfoContext(ctx context.Context, args ...interface{}) {
	GetDefaultLogger().Info(args...)
}

// InfoContextf 打印 Info 日志
func InfoContextf(ctx context.Context, format string, args ...interface{}) {
	GetDefaultLogger().Infof(format, args...)
}

// WarnContext 打印 Warn 日志
func WarnContext(ctx context.Context, args ...interface{}) {
	GetDefaultLogger().Warn(args...)
}

// WarnContextf 打印 Warn 日志
func WarnContextf(ctx context.Context, format string, args ...interface{}) {
	GetDefaultLogger().Warnf(format, args...)
}

// ErrorContext 打印 Error 日志
func ErrorContext(ctx context.Context, args ...interface{}) {
	GetDefaultLogger().Error(args...)
}

func ErrorContextf(ctx context.Context, format string, args ...interface{}) {
	GetDefaultLogger().Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	Errorf(format, args...)
}
