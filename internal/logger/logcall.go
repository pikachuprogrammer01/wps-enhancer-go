package logger

import (
	"log/slog"
	"time"
)

// LogCall 记录无返回值命令的进入/退出（debug 级别，需 log_debug 开启）。
func LogCall(name string, fn func() error) error {
	start := time.Now()
	slog.Debug("→", "fn", name)
	err := fn()
	logReturn(name, start, err)
	return err
}

// LogCall1 记录单返回值命令的进入/退出（debug 级别，需 log_debug 开启）。
func LogCall1[T any](name string, fn func() (T, error)) (T, error) {
	start := time.Now()
	slog.Debug("→", "fn", name)
	v, err := fn()
	logReturn(name, start, err)
	return v, err
}

// logReturn 写入命令退出日志。
func logReturn(name string, start time.Time, err error) {
	ms := time.Since(start).Milliseconds()
	if err != nil {
		slog.Debug("←", "fn", name, "ms", ms, "err", err)
		return
	}
	slog.Debug("←", "fn", name, "ms", ms)
}
