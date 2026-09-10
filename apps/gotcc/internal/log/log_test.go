package log

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// captureLevel builds a logger whose entries are written to the returned
// buffer instead of stdout, so tests can assert on the produced output.
func captureLevel(t *testing.T, lv zapcore.Level) (*zapLoggerWrapper, *bytes.Buffer) {
	t.Helper()
	w := &zapLoggerWrapper{options: NewOptions()}
	logger := NewSugarLogger(NewOptions(WithLogLevel(lv.String())))
	// Redirect the wrapper's output to a buffer for assertions.
	buf := &bytes.Buffer{}
	logger.SugaredLogger = zap.New(
		zapcore.NewCore(w.getEncoder(), zapcore.AddSync(&testWriter{buf: buf}), level(logger.options.LogLevel)),
	).Sugar()
	return logger, buf
}

type testWriter struct{ buf *bytes.Buffer }

func (w *testWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }

func TestNewSugarLoggerWritesToConfiguredWriter(t *testing.T) {
	logger, buf := captureLevel(t, zapcore.InfoLevel)

	logger.Infof("hello %s", "world")
	logger.Errorf("boom")

	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Fatalf("output %q does not contain formatted info message", out)
	}
	if !strings.Contains(out, "boom") {
		t.Fatalf("output %q does not contain error message", out)
	}
	if !strings.Contains(out, "INFO") {
		t.Fatalf("output %q does not contain capitalized level", out)
	}
}

func TestLevelFiltering(t *testing.T) {
	logger, buf := captureLevel(t, zapcore.InfoLevel)

	logger.Debug("should be filtered")
	if buf.Len() != 0 {
		t.Fatalf("debug entry must be filtered at info level, got %q", buf.String())
	}

	logger.Info("kept")
	if !strings.Contains(buf.String(), "kept") {
		t.Fatalf("info entry missing from %q", buf.String())
	}
}

func TestLevelUnknownNameFallsBackToInfo(t *testing.T) {
	if got := level("not-a-level"); got != zapcore.InfoLevel {
		t.Fatalf("level(unknown) = %v, want info", got)
	}
	if got := level("DEBUG"); got != zapcore.DebugLevel {
		t.Fatalf("level(DEBUG) = %v, want debug", got)
	}
}

func TestDefaultLoggerAndPackageHelpers(t *testing.T) {
	if GetDefaultLogger() == nil {
		t.Fatal("GetDefaultLogger() must never return nil")
	}

	ctx := context.Background()
	now := "now"
	Infof("info... %v", now)
	Warnf("warn... %v", now)
	Errorf("error... %v", now)
	Fatalf("fatal... %v", now)
	DebugContext(ctx, "debug...")
	DebugContextf(ctx, "debug... %v", now)
	InfoContext(ctx, "info...")
	InfoContextf(ctx, "info... %v", now)
	WarnContext(ctx, "warn...")
	WarnContextf(ctx, "warn... %v", now)
	ErrorContext(ctx, "error...")
	ErrorContextf(ctx, "error... %v", now)
}
