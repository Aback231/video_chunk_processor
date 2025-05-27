package logger

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	cases := []struct {
		in  string
		out zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"info", zapcore.InfoLevel},
		{"somethingelse", zapcore.InfoLevel},
		{"", zapcore.InfoLevel},
	}
	for _, c := range cases {
		if got := parseLevel(c.in); got != c.out {
			t.Errorf("parseLevel(%q) = %v, want %v", c.in, got, c.out)
		}
	}
}

func TestNewLogger(t *testing.T) {
	l := New("debug")
	if l == nil {
		t.Error("New should not return nil")
	}
	l.Info("test info log")
	l.Debug("test debug log")
	l.Warn("test warn log")
	l.Error("test error log")
}
