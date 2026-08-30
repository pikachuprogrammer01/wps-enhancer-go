package app

// 卸载命令：清理项列表 + 逐项执行（对齐 Python 版 ui/components/settings/tab_about.py 流程）。

import (
	"log/slog"
	"path/filepath"

	"wps-enhancer-go/internal/uninstall"
)

// uninstallPaths 解析当前平台的卸载路径集合。
func (a *App) uninstallPaths() uninstall.Paths {
	return uninstall.Paths{
		AppPath:      uninstall.DefaultAppPath(),
		DataDir:      a.configDir,
		LogsDir:      filepath.Join(a.configDir, "logs"),
		DownloadsDir: a.settings.DownloadDir,
	}
}

// UninstallItems 返回当前平台的卸载清理项（含存在性检测，前端勾选展示用）。
func (a *App) UninstallItems() []uninstall.Item {
	return uninstall.Items(a.uninstallPaths())
}

// UninstallRemove 按 key 逐项执行清理；单项失败不中断后续项，结果写入日志。
func (a *App) UninstallRemove(keys []string) []uninstall.Result {
	results := uninstall.Remove(a.uninstallPaths(), keys)
	for _, res := range results {
		if res.Error != "" {
			slog.Warn("卸载清理项失败", "key", res.Key, "error", res.Error)
		} else {
			slog.Info("卸载清理项完成", "key", res.Key)
		}
	}
	return results
}
