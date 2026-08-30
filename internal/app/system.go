package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wps-enhancer-go/internal/errs"
	"wps-enhancer-go/internal/settings"
	"wps-enhancer-go/internal/updater"
	"wps-enhancer-go/internal/version"
)

// Version 返回应用版本号（关于页展示 + 更新比较基准）。
func (a *App) Version() string {
	return version.Version
}

// SettingsReset 恢复默认设置并持久化（设置页「恢复默认设置」）。
func (a *App) SettingsReset() error {
	a.settings = settings.New()
	if err := settings.Save(a.configDir+"/settings.json", a.settings); err != nil {
		return translateErr(err)
	}
	return nil
}

// ReadLogs 读取日志目录中最近的日志内容（设置页「日志」tab）。
// limit 为每条日志的最大行数；按修改时间取最新的日志文件。
func (a *App) ReadLogs(limit int) ([]string, error) {
	if limit <= 0 {
		limit = 200
	}
	logDir := filepath.Join(a.configDir, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, translateErr(err)
	}
	// 按修改时间倒序取日志文件（stat 失败的条目跳过，避免 nil 解引用）
	type logFile struct {
		name string
		mod  time.Time
	}
	files := make([]logFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, logFile{name: e.Name(), mod: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	if len(files) == 0 {
		return []string{}, nil
	}
	raw, err := os.ReadFile(filepath.Join(logDir, files[0].name))
	if err != nil {
		return nil, translateErr(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines, nil
}

// ExportLogs 将最新的日志文件复制到 dest（设置页「日志」tab 导出按钮）。
// 返回实际写入的目标路径；日志目录为空时返回错误。
func (a *App) ExportLogs(dest string) (string, error) {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return "", fmt.Errorf("导出路径不能为空")
	}
	logDir := filepath.Join(a.configDir, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return "", translateErr(fmt.Errorf("%w: 读取日志目录失败: %v", errs.ErrFileRead, err))
	}
	var newest string
	var newestMod time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestMod) {
			newest = e.Name()
			newestMod = info.ModTime()
		}
	}
	if newest == "" {
		return "", translateErr(fmt.Errorf("%w: 暂无日志文件", errs.ErrFileRead))
	}
	srcPath := filepath.Join(logDir, newest)
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return "", translateErr(err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", translateErr(err)
	}
	if err := os.WriteFile(dest, raw, 0o644); err != nil {
		return "", translateErr(err)
	}
	slog.Info("日志已导出", "src", srcPath, "dest", dest, "bytes", len(raw))
	return dest, nil
}

// UpdateInfo 更新检查结果（前端「更新」tab 展示）。
type UpdateInfo struct {
	Current   string  `json:"current"`
	Latest    string  `json:"latest"`
	HasUpdate bool    `json:"has_update"`
	ZipURL    *string `json:"zip_url,omitempty"`     // 更新包下载地址
	ZipSize   *int64  `json:"zip_size,omitempty"`    // 更新包字节数
	Notes     string  `json:"notes,omitempty"`       // 更新说明
	Error     string  `json:"error,omitempty"`
}

// CheckUpdate 检查最新版本（复用 internal/updater，更新源取设置 update_url）。
func (a *App) CheckUpdate() UpdateInfo {
	if a.settings.UseSystemProxy {
		updater.ApplySystemProxy() // 打包版无 shell 代理环境：从系统代理补上
	}
	client := &http.Client{Timeout: 10 * time.Second}
	info := UpdateInfo{Current: version.Version}
	release, err := updater.CheckLatestRelease(a.settings.UpdateURL, client)
	if err != nil {
		info.Error = fmt.Sprintf("检查失败：%v", err)
		slog.Warn("检查更新失败", "error", err.Error())
		return info
	}
	info.Latest = strings.TrimPrefix(release.TagName, "v")
	info.HasUpdate = updater.CompareVersions(version.Version, info.Latest) < 0
	info.ZipURL = release.ZipURL
	info.ZipSize = release.ZipSize
	info.Notes = release.Notes
	slog.Info("检查更新完成", "latest", info.Latest, "has_update", info.HasUpdate)
	return info
}
