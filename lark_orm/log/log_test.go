package log

import (
	"os"
	"testing"
)

func TestSetLevel(t *testing.T) {
	SetLevel(ErrorLevel)
	if infoLogger.Writer() == os.Stdout || errorLogger.Writer() != os.Stdout {
		t.Fatal("failed to set log level")
	}
	SetLevel(Disabled)
	if infoLogger.Writer() == os.Stdout || errorLogger.Writer() == os.Stdout {
		t.Fatal("failed to set log level")
	}
}
