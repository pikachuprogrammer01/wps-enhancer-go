package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewAndCleanup 验证日志创建与过期清理。
func TestNewAndCleanup(t *testing.T) {
	dir := t.TempDir()
	log, err := New(dir, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	log.Info("测试日志")

	// 验证文件已创建
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("应生成 1 个日志文件，实际 %d", len(entries))
	}
	name := entries[0].Name()
	if name[:4] != "app-" || name[len(name)-4:] != ".log" {
		t.Errorf("日志文件名格式异常: %s", name)
	}
	_ = log

	// 构造过期日志文件（8 天前）
	oldDate := time.Now().AddDate(0, 0, -8).Format("2006-01-02")
	oldFile := filepath.Join(dir, "app-"+oldDate+".log")
	_ = os.WriteFile(oldFile, []byte("old"), 0o644)
	// 构造非日志文件（不应被清理）
	_ = os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0o644)

	deleted, failed := Cleanup(dir, 7)
	if deleted != 1 {
		t.Errorf("应清理 1 个过期日志，实际 %d", deleted)
	}
	if failed != 0 {
		t.Errorf("清理失败 %d 个", failed)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("过期日志应被删除")
	}
	if _, err := os.Stat(filepath.Join(dir, "other.txt")); err != nil {
		t.Error("非日志文件不应被删除")
	}
}
