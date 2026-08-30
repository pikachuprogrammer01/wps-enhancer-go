// Package uninstall 卸载清理项框架（对齐 core/uninstall.py）：
// 注册式清理项 + 逐项执行；单项失败不中断后续项。
// 所有路径通过 Paths 参数注入（不读全局），便于测试。
package uninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Paths 卸载涉及的目录/路径集合（由命令层解析后注入）。
type Paths struct {
	AppPath      string // 应用本体（macOS .app / Windows 安装目录；空 = 当前平台不支持自动删除）
	DataDir      string // 本地数据目录（settings.json / template / license.json）
	LogsDir      string // 日志目录
	DownloadsDir string // 更新包下载目录（只清本应用的 zip）
}

// Item 一个卸载清理项（前端展示用，不含路径）。
type Item struct {
	Key            string `json:"key"`             // 稳定标识（app / data / logs / downloads）
	Label          string `json:"label"`           // 勾选文案
	Risky          bool   `json:"risky"`           // 高风险项：默认不勾选 + 确认框标注（防误删用户数据）
	DefaultChecked bool   `json:"default_checked"` // 默认勾选状态
	Exists         bool   `json:"exists"`          // 目标是否存在（false = 无残留）
}

// Result 单项执行结果（Error 为空 = 成功或本就无残留）。
type Result struct {
	Key   string `json:"key"`
	Error string `json:"error,omitempty"`
}

// DefaultAppPath 按平台返回应用本体路径（macOS /Applications；Windows 安装目录；其他平台空）。
func DefaultAppPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Applications/WPS增强工具.app"
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}
			local = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(local, "WPSEnhancer")
	}
	return ""
}

// resolve 按 key 解析目标路径（内部使用，不外泄到前端）。
func resolve(p Paths, key string) string {
	switch key {
	case "app":
		return p.AppPath
	case "data":
		return p.DataDir
	case "logs":
		return p.LogsDir
	case "downloads":
		return p.DownloadsDir
	}
	return ""
}

// Items 返回全部清理项（顺序固定，UI 展示与执行共用同一注册表）。
func Items(p Paths) []Item {
	meta := []struct {
		key, label     string
		risky, checked bool
	}{
		{"app", "删除应用程序本体", false, true},
		{"data", "删除本地数据（设置/模板）", true, false}, // 用户数据默认不清理，防误删
		{"logs", "删除日志", false, true},
		{"downloads", "删除已下载的更新包（WPS增强工具_*.zip）", false, true},
	}
	items := make([]Item, 0, len(meta))
	for _, m := range meta {
		items = append(items, Item{
			Key:            m.key,
			Label:          m.label,
			Risky:          m.risky,
			DefaultChecked: m.checked,
			Exists:         exists(resolve(p, m.key)),
		})
	}
	return items
}

// Remove 按 key 顺序逐项执行卸载；单项失败只记录，不中断后续项。
// 返回与 keys 等长的结果切片，Error 为空表示该项成功（或本就无残留）。
func Remove(p Paths, keys []string) []Result {
	results := make([]Result, 0, len(keys))
	for _, key := range keys {
		label := ""
		for _, m := range []struct{ key, label string }{
			{"app", "删除应用程序本体"},
			{"data", "删除本地数据（设置/模板）"},
			{"logs", "删除日志"},
			{"downloads", "删除已下载的更新包（WPS增强工具_*.zip）"},
		} {
			if m.key == key {
				label = m.label
			}
		}
		if label == "" {
			results = append(results, Result{Key: key, Error: fmt.Sprintf("未知清理项：%s", key)})
			continue
		}
		results = append(results, Result{Key: key, Error: removeOne(p, key, label)})
	}
	return results
}

// removeOne 删除单个清理项；返回空串 = 成功（或本就无残留）。
func removeOne(p Paths, key, label string) string {
	target := strings.TrimSpace(resolve(p, key))
	if target == "" || !exists(target) {
		return "" // 无残留视为已清理
	}
	// downloads 只清本应用的更新包，不碰目录里的其他文件
	if key == "downloads" {
		entries, err := os.ReadDir(target)
		if err != nil {
			return fmt.Sprintf("%s 删除失败：%v", label, err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasPrefix(e.Name(), "WPS增强工具_") && strings.HasSuffix(e.Name(), ".zip") {
				if err := os.Remove(filepath.Join(target, e.Name())); err != nil {
					return fmt.Sprintf("%s 删除失败：%v", label, err)
				}
			}
		}
		return ""
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Sprintf("%s 删除失败：%v", label, err)
	}
	return ""
}

// exists 路径是否存在。
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
