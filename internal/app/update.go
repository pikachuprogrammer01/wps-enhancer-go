package app

// 更新下载/安装闭环（对齐 Python 版 ui/components/update_flow.py）：
// 前端 StartDownloadUpdate 发起后台下载 → DownloadProgress 轮询进度 →
// 完成后 zip 校验 + 生成替换指引 + 打开文件管理器定位。

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"wps-enhancer-go/internal/errs"
	"wps-enhancer-go/internal/updater"
)

// DownloadProgress 下载进度状态（前端轮询展示）。
type DownloadProgress struct {
	Status   string  `json:"status"`           // idle / downloading / done / error
	Percent  float64 `json:"percent"`          // 0-100（总大小未知时为 0）
	Done     int64   `json:"done"`             // 已下载字节
	Total    int64   `json:"total"`            // 总字节（未知为 0）
	FilePath string  `json:"file_path,omitempty"`
	Guide    string  `json:"guide,omitempty"`  // 安装替换指引文案
	Error    string  `json:"error,omitempty"`
}

// dlMu 保护下载状态（单例下载任务）。
var (
	dlMu      sync.Mutex
	dlRunning bool
	dlState   DownloadProgress
)

// StartDownloadUpdate 启动后台更新包下载（进行中时拒绝重复启动）。
func (a *App) StartDownloadUpdate() error {
	dlMu.Lock()
	if dlRunning {
		dlMu.Unlock()
		return fmt.Errorf("已有更新下载正在进行")
	}
	dlRunning = true
	dlState = DownloadProgress{Status: "downloading"}
	dlMu.Unlock()

	go a.runDownload()
	return nil
}

// runDownload 执行完整下载链路：查源 → 下载 → zip 校验 → 指引生成。
func (a *App) runDownload() {
	defer func() {
		dlMu.Lock()
		dlRunning = false
		dlMu.Unlock()
	}()

	client := &http.Client{Timeout: 30 * time.Second}
	if a.settings.UseSystemProxy {
		updater.ApplySystemProxy() // 打包版无 shell 代理环境：从系统代理补上
	}
	release, err := updater.CheckLatestRelease(a.settings.UpdateURL, client)
	if err != nil {
		a.failDownload(fmt.Sprintf("检查失败：%v", err))
		return
	}
	if release.ZipURL == nil || *release.ZipURL == "" {
		a.failDownload("该版本未提供当前平台的更新包")
		return
	}

	dir := a.settings.DownloadDir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.failDownload(fmt.Sprintf("无法创建下载目录 '%s'：%v", dir, err))
		return
	}
	dest := filepath.Join(dir, fmt.Sprintf("WPS增强工具_%s.zip", release.TagName))

	slog.Info("开始下载更新包", "url", *release.ZipURL, "dest", dest)
	err = updater.DownloadFile(*release.ZipURL, dest, client, func(done, total int64) {
		p := DownloadProgress{Status: "downloading", Done: done, Total: total}
		if total > 0 {
			p.Percent = float64(done) / float64(total) * 100
		}
		dlMu.Lock()
		dlState = p
		dlMu.Unlock()
	})
	if err != nil {
		a.failDownload(fmt.Sprintf("下载失败：%v", err))
		return
	}
	if err := updater.VerifyZipIntegrity(dest); err != nil {
		_ = os.Remove(dest)
		a.failDownload(fmt.Sprintf("更新包损坏，已删除：%v", err))
		return
	}

	guide := replaceGuide(a.settings.InstallDir)
	slog.Info("更新包下载完成", "path", dest)
	dlMu.Lock()
	dlState = DownloadProgress{
		Status:   "done",
		Percent:  100,
		Done:     1,
		Total:    1,
		FilePath: dest,
		Guide:    guide,
	}
	dlMu.Unlock()
}

// failDownload 标记下载失败并记录日志。
func (a *App) failDownload(msg string) {
	slog.Error("更新包下载失败", "reason", msg)
	dlMu.Lock()
	dlState = DownloadProgress{Status: "error", Error: msg}
	dlMu.Unlock()
}

// DownloadProgress 返回当前下载进度（前端轮询）。
func (a *App) DownloadProgress() DownloadProgress {
	dlMu.Lock()
	defer dlMu.Unlock()
	return dlState
}

// OpenPath 用系统文件管理器打开目录，或定位并选中文件（对齐 update_sop：open -R / explorer /select）。
func (a *App) OpenPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return translateErr(fmt.Errorf("%w: 路径为空", errs.ErrFileRead))
	}
	cmd := revealCommand(path)
	if err := cmd.Start(); err != nil {
		return translateErr(fmt.Errorf("%w: 无法打开 '%s': %v", errs.ErrFileRead, path, err))
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// revealCommand 构造「打开目录 / 选中文件」的系统命令（可单测参数，不真启进程）。
func revealCommand(path string) *exec.Cmd {
	info, err := os.Stat(path)
	isFile := err == nil && !info.IsDir()
	// 路径已不存在时打开父目录，避免把失效文件路径当目录 open
	if err != nil {
		path = filepath.Dir(path)
		isFile = false
	}
	switch runtime.GOOS {
	case "darwin":
		if isFile {
			return exec.Command("open", "-R", path)
		}
		return exec.Command("open", path)
	case "windows":
		if isFile {
			// 路径必须单独成参（对齐 Python update_flow），否则含空格时 /select 失效
			return exec.Command("explorer", "/select,", filepath.Clean(path))
		}
		return exec.Command("explorer", path)
	default:
		target := path
		if isFile {
			target = filepath.Dir(path)
		}
		return exec.Command("xdg-open", target)
	}
}

// replaceGuide 生成分平台安装替换指引（对齐 Python 版 _REPLACE_GUIDE_MAC/_WIN）。
func replaceGuide(installDir string) string {
	if installDir == "" {
		installDir = defaultInstallDirFallback()
	}
	if runtime.GOOS == "darwin" {
		return fmt.Sprintf(
			"替换方法：\n"+
				"1. 完全退出 WPS 增强工具\n"+
				"2. 解压 zip，把新的 WPS增强工具.app 拖到「%s」覆盖旧版\n"+
				"3. 若提示「无法验证开发者」，请右键点按 App → 打开\n", installDir)
	}
	return fmt.Sprintf(
		"替换方法：\n"+
			"1. 完全退出 WPS 增强工具\n"+
			"2. 解压 zip，用其中的文件覆盖安装目录（%s）\n"+
			"3. 若出现 SmartScreen 提示，点「更多信息」→「仍要运行」\n", installDir)
}

// defaultInstallDirFallback 安装目录兜底值（避免依赖 settings 包内部函数）。
func defaultInstallDirFallback() string {
	if runtime.GOOS == "windows" {
		local := os.Getenv("LOCALAPPDATA")
		if local != "" {
			return filepath.Join(local, "WPSEnhancer")
		}
	}
	if runtime.GOOS == "darwin" {
		return "/Applications"
	}
	return "."
}
