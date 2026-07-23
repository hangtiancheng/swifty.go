package log

import (
	"fmt"
	"io"
	"log"
	"os"
)

type Logger interface {
	Errorf(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Debugf(format string, v ...interface{})
}

var defaultLogger Logger

func init() {
	defaultLogger = NewLogger(NewOptions())
}

// Options holds logger configuration.
type Options struct {
	LogName  string
	LogLevel string
	FileName string
	Writer   io.Writer
}

// Option mutates Options.
type Option func(*Options)

// NewOptions builds default Options and applies the given Option list.
func NewOptions(opts ...Option) Options {
	options := Options{
		LogName:  "app",
		LogLevel: "info",
		FileName: "",
		Writer:   os.Stdout,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// WithLogLevel sets the log level (debug, info, warn, error, fatal).
func WithLogLevel(level string) Option {
	return func(o *Options) {
		o.LogLevel = level
	}
}

// WithFileName redirects log output to the given file.
func WithFileName(filename string) Option {
	return func(o *Options) {
		o.FileName = filename
	}
}

// Level represents log severity.
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// Levels maps string level names to Level values.
var Levels = map[string]Level{
	"debug": DebugLevel,
	"info":  InfoLevel,
	"warn":  WarnLevel,
	"error": ErrorLevel,
	"fatal": FatalLevel,
}

type stdLogger struct {
	logger *log.Logger
	level  Level
}

// NewLogger constructs a Logger from Options.
func NewLogger(options Options) Logger {
	writer := options.Writer
	if writer == nil {
		if options.FileName != "" {
			f, err := os.OpenFile(options.FileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				panic(err)
			}
			writer = f
		} else {
			writer = os.Stdout
		}
	}

	return &stdLogger{
		logger: log.New(writer, "", log.LstdFlags|log.Lshortfile|log.Lmsgprefix),
		level:  Levels[options.LogLevel],
	}
}

func (l *stdLogger) output(calldepth int, level, msg string) {
	l.logger.Output(calldepth+1, fmt.Sprintf("[%s] %s", level, msg))
}

func (l *stdLogger) Debugf(format string, v ...interface{}) {
	if l.level <= DebugLevel {
		l.output(2, "DEBUG", fmt.Sprintf(format, v...))
	}
}

func (l *stdLogger) Infof(format string, v ...interface{}) {
	if l.level <= InfoLevel {
		l.output(2, "INFO", fmt.Sprintf(format, v...))
	}
}

func (l *stdLogger) Warnf(format string, v ...interface{}) {
	if l.level <= WarnLevel {
		l.output(2, "WARN", fmt.Sprintf(format, v...))
	}
}

func (l *stdLogger) Errorf(format string, v ...interface{}) {
	if l.level <= ErrorLevel {
		l.output(2, "ERROR", fmt.Sprintf(format, v...))
	}
}

// GetDefaultLogger returns the package-level default Logger.
func GetDefaultLogger() Logger {
	return defaultLogger
}
