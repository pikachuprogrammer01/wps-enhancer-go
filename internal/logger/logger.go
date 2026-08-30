// Package logger 统一日志入口（log/slog + 按天轮转文件 writer）。
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dailyWriter 按天轮转文件 writer：文件名为 app-YYYY-MM-DD.log，跨天自动切换。
type dailyWriter struct {
	mu      sync.Mutex
	dir     string
	current string // 当前日期 YYYY-MM-DD
	file    *os.File
}

// Write 写入日志（跨天时切换文件）。
func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	date := time.Now().Format("2006-01-02")
	if date != w.current {
		if w.file != nil {
			_ = w.file.Close()
			w.file = nil
		}
		w.current = date
	}
	if w.file == nil {
		name := filepath.Join(w.dir, fmt.Sprintf("app-%s.log", date))
		f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 0, err
		}
		w.file = f
	}
	return w.file.Write(p)
}

// Close 关闭当前文件。
func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// New 创建带文件输出的 slog.Logger（按天轮转 + 保留天数清理）。
func New(logDir string, debug bool) (*slog.Logger, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	w := &dailyWriter{dir: logDir}
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	handler := slog.NewTextHandler(io.MultiWriter(w, os.Stderr), &slog.HandlerOptions{Level: level})
	return slog.New(handler), nil
}

// Cleanup 清理 N 天前的日志文件（等价 Python cleanup_logs，静默失败）。
func Cleanup(logDir string, retainDays int) (deleted, failed int) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0, 0
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "app-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		// 从文件名解析日期 app-YYYY-MM-DD.log
		dateStr := strings.TrimSuffix(strings.TrimPrefix(name, "app-"), ".log")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if date.Before(cutoff) {
			if err := os.Remove(filepath.Join(logDir, name)); err != nil {
				failed++
			} else {
				deleted++
			}
		}
	}
	return deleted, failed
}
