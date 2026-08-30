package logger

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// withDebugLogger 临时替换 slog 默认 logger，避免污染其他测试。
func withDebugLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	prev := slog.Default()
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestLogCall(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		buf := withDebugLogger(t)
		err := LogCall("ExportWithTemplate", func() error { return nil })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "ExportWithTemplate") {
			t.Errorf("log should mention fn name: %q", out)
		}
		if !strings.Contains(out, "→") || !strings.Contains(out, "←") {
			t.Errorf("log should contain enter/exit markers: %q", out)
		}
		if strings.Contains(out, "err=") {
			t.Errorf("success path should not log err: %q", out)
		}
	})

	t.Run("error", func(t *testing.T) {
		buf := withDebugLogger(t)
		errSentinel := errors.New("fail")
		err := LogCall("TestFn", func() error { return errSentinel })
		if err != errSentinel {
			t.Fatalf("expected sentinel error, got %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "TestFn") {
			t.Errorf("log should mention fn name: %q", out)
		}
		if !strings.Contains(out, "fail") {
			t.Errorf("error path should log error message: %q", out)
		}
	})
}

func TestLogCall1(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		buf := withDebugLogger(t)
		v, err := LogCall1("ReadSheet", func() (int, error) { return 42, nil })
		if err != nil || v != 42 {
			t.Fatalf("got v=%d err=%v", v, err)
		}
		if !strings.Contains(buf.String(), "ReadSheet") {
			t.Errorf("log should mention fn name")
		}
	})

	t.Run("error", func(t *testing.T) {
		buf := withDebugLogger(t)
		errSentinel := errors.New("read failed")
		v, err := LogCall1("ReadSheet", func() (int, error) { return 0, errSentinel })
		if err != errSentinel {
			t.Fatalf("expected sentinel error, got %v", err)
		}
		if v != 0 {
			t.Fatalf("expected zero value on error, got %d", v)
		}
		if !strings.Contains(buf.String(), "read failed") {
			t.Errorf("error path should log error message: %q", buf.String())
		}
	})
}
