package uninstall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testPaths 构造临时目录环境：app 本体（假 .app 目录）/ 数据 / 日志 / 下载（含本应用 zip 与无关文件）。
func testPaths(t *testing.T) (p Paths, appDir, dataDir, logsDir, dlDir string) {
	t.Helper()
	root := t.TempDir()
	appDir = filepath.Join(root, "WPS增强工具.app")
	dataDir = filepath.Join(root, "data")
	logsDir = filepath.Join(root, "logs")
	dlDir = filepath.Join(root, "downloads")
	for _, d := range []string{
		appDir, filepath.Join(appDir, "Contents"),
		dataDir, filepath.Join(dataDir, "template"),
		logsDir, dlDir,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(appDir, "Contents", "app"), "binary")
	write(filepath.Join(dataDir, "settings.json"), "{}")
	write(filepath.Join(dataDir, "template", "t.json"), "{}")
	write(filepath.Join(logsDir, "app-2026-08-25.log"), "log")
	write(filepath.Join(dlDir, "WPS增强工具_v1.1.0.zip"), "zip")
	write(filepath.Join(dlDir, "无关文件.txt"), "keep me")
	return Paths{AppPath: appDir, DataDir: dataDir, LogsDir: logsDir, DownloadsDir: dlDir}, appDir, dataDir, logsDir, dlDir
}

// TestItemsDefaults 清理项注册表：顺序、风险标记与默认勾选对齐 Python 版。
func TestItemsDefaults(t *testing.T) {
	p, _, dataDir, _, _ := testPaths(t)
	items := Items(p)
	if len(items) != 4 {
		t.Fatalf("应有 4 个清理项，实际 %d", len(items))
	}
	want := []struct {
		key                    string
		risky, checked, exists bool
	}{
		{"app", false, true, true},
		{"data", true, false, true},
		{"logs", false, true, true},
		{"downloads", false, true, true},
	}
	for i, w := range want {
		got := items[i]
		if got.Key != w.key || got.Risky != w.risky || got.DefaultChecked != w.checked || got.Exists != w.exists {
			t.Errorf("item[%d] = %+v, want key=%s risky=%v checked=%v exists=%v", i, got, w.key, w.risky, w.checked, w.exists)
		}
	}
	// Item 序列化不应泄漏路径（前端只见 key/label/标记位）
	for _, it := range items {
		b, _ := json.Marshal(it)
		if strings.Contains(string(b), dataDir) || strings.Contains(string(b), "testdata") {
			t.Errorf("Item %s 序列化泄漏了路径: %s", it.Key, b)
		}
	}
}

// TestRemoveAll 全部清理：目录与更新包删除、无关文件保留。
func TestRemoveAll(t *testing.T) {
	p, appDir, dataDir, logsDir, dlDir := testPaths(t)
	results := Remove(p, []string{"app", "data", "logs", "downloads"})
	for _, res := range results {
		if res.Error != "" {
			t.Errorf("清理项 %s 失败: %s", res.Key, res.Error)
		}
	}
	if exists(appDir) || exists(dataDir) || exists(logsDir) {
		t.Error("app/data/logs 应已删除")
	}
	if _, err := os.Stat(filepath.Join(dlDir, "WPS增强工具_v1.1.0.zip")); !os.IsNotExist(err) {
		t.Error("本应用更新包应已删除")
	}
	if _, err := os.Stat(filepath.Join(dlDir, "无关文件.txt")); err != nil {
		t.Error("下载目录的无关文件不应被删除")
	}
}

// TestRemoveMissing 无残留 = 视为已清理（Error 为空）。
func TestRemoveMissing(t *testing.T) {
	p := Paths{AppPath: "/nonexistent/app", DataDir: "", LogsDir: "", DownloadsDir: "/nonexistent/dl"}
	results := Remove(p, []string{"app", "data", "logs", "downloads"})
	for _, res := range results {
		if res.Error != "" {
			t.Errorf("无残留项 %s 不应报错: %s", res.Key, res.Error)
		}
	}
}

// TestRemoveUnknownKey 未知 key 报错不 panic。
func TestRemoveUnknownKey(t *testing.T) {
	p, _, _, _, _ := testPaths(t)
	results := Remove(p, []string{"nope"})
	if results[0].Error == "" {
		t.Error("未知清理项应返回错误")
	}
}

// TestRemoveDataPreservesOthers 默认勾选策略验证：只清 logs+downloads 时 data/app 不受影响。
func TestRemoveDataPreservesOthers(t *testing.T) {
	p, appDir, dataDir, _, _ := testPaths(t)
	results := Remove(p, []string{"logs", "downloads"})
	for _, res := range results {
		if res.Error != "" {
			t.Fatalf("清理失败: %s", res.Error)
		}
	}
	if !exists(appDir) || !exists(dataDir) {
		t.Error("未勾选的 app/data 不应被删除")
	}
}
