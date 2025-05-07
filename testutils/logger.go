package testutils

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
)

// NewTestLogger creates a TestLogger.
func NewTestLogger(tb testing.TB) *TestLogger {
	tb.Helper()

	return &TestLogger{tb}
}

// TestLogger logs to a t.Log function.
type TestLogger struct {
	tb testing.TB
}

// Logger returns a zerolog Logger.
func (tl *TestLogger) Logger() zerolog.Logger {
	level := zerolog.DebugLevel

	if os.Getenv("TEST_TRACE") != "" {
		level = zerolog.TraceLevel
	}

	if os.Getenv("TEST_QUIET") != "" {
		level = zerolog.InfoLevel
	}

	return zerolog.New(zerolog.ConsoleWriter{Out: tl}).
		With().Timestamp().Logger().
		Level(level)
}

// Write writes the given string to t.Logf.
func (tl *TestLogger) Write(m []byte) (int, error) {
	tl.tb.Log(string(m))

	return len(m), nil
}

// SetTB changes the current TB, and returns a function to get back to the
// previous one.
func (tl *TestLogger) SetTB(tb testing.TB) func() {
	tb.Helper()
	otb := tl.tb
	tl.tb = tb

	return func() {
		tl.tb = otb
	}
}

// GetLogger returns a test Logger.
func GetLogger(tb testing.TB) zerolog.Logger {
	tb.Helper()

	return NewTestLogger(tb).Logger()
}
