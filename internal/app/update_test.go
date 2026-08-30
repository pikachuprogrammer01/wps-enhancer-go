package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestReplaceGuide_含退出与平台提示 指引文案对齐 Python update_flow。
func TestReplaceGuide_含退出与平台提示(t *testing.T) {
	guide := replaceGuide("/Applications")
	if !strings.Contains(guide, "完全退出") {
		t.Errorf("应提示完全退出: %q", guide)
	}
	if !strings.Contains(guide, "/Applications") {
		t.Errorf("应含安装目录: %q", guide)
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(guide, "无法验证开发者") {
			t.Errorf("macOS 应含 Gatekeeper 提示: %q", guide)
		}
	default:
		if !strings.Contains(guide, "SmartScreen") {
			t.Errorf("非 macOS 应含 SmartScreen 提示: %q", guide)
		}
	}
	fallback := replaceGuide("")
	if fallback == "" || !strings.Contains(fallback, "完全退出") {
		t.Errorf("空 install_dir 应有兜底指引: %q", fallback)
	}
}

// TestRevealCommand_文件与目录 校验选中文件用 -R /select 分参，目录直接打开。
func TestRevealCommand_文件与目录(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "pack.zip")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	fileCmd := revealCommand(file)
	dirCmd := revealCommand(dir)
	missing := filepath.Join(dir, "gone", "missing.zip")
	missCmd := revealCommand(missing)

	switch runtime.GOOS {
	case "darwin":
		if len(fileCmd.Args) < 3 || fileCmd.Args[1] != "-R" {
			t.Errorf("文件应 open -R: %v", fileCmd.Args)
		}
		if len(dirCmd.Args) < 2 || dirCmd.Args[1] != dir {
			t.Errorf("目录应 open path: %v", dirCmd.Args)
		}
		if len(missCmd.Args) < 2 || missCmd.Args[1] != filepath.Dir(missing) {
			t.Errorf("缺失文件应 open 父目录: %v", missCmd.Args)
		}
	case "windows":
		if len(fileCmd.Args) < 3 || fileCmd.Args[1] != "/select," {
			t.Errorf("文件应 explorer /select, path 分参: %v", fileCmd.Args)
		}
	}

	a := testApp(t)
	if err := a.OpenPath(""); err == nil {
		t.Error("空路径应报错")
	}
}
