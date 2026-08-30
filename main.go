package main

import (
	"embed"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"wps-enhancer-go/internal/app"
	"wps-enhancer-go/internal/logger"
	"wps-enhancer-go/internal/settings"
	"wps-enhancer-go/internal/updater"
	"wps-enhancer-go/internal/version"
)

// 前端产物通过 go:embed 嵌入二进制（frontend/dist 由 vite build 生成）。

//go:embed all:frontend/dist
var assets embed.FS

// main 应用入口：初始化数据目录与日志 → 注册 Wails 服务 → 启动静默检查更新 → 运行应用。
func main() {
	// 数据目录（等价 Python 版 app_paths.get_data_dir）
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	configDir = filepath.Join(configDir, "WPSEnhancer")
	_ = os.MkdirAll(configDir, 0o755)

	// 初始化日志（等价 Python 版 core/logger：按天轮转 + 启动清理过期日志）
	st, _ := settings.Load(filepath.Join(configDir, "settings.json"))
	logDir := filepath.Join(configDir, "logs")
	if lg, err := logger.New(logDir, st.LogDebug); err == nil {
		slog.SetDefault(lg)
	} else {
		log.Printf("日志初始化失败（降级为标准输出）：%v", err)
	}
	if st.LogAutoClean && st.LogRetainDays > 0 {
		retainDays := st.LogRetainDays
		runLogCleanup := func() {
			deleted, _ := logger.Cleanup(logDir, retainDays)
			if deleted > 0 {
				slog.Info("已清理过期日志", "deleted", deleted, "retain_days", retainDays)
			}
		}
		runLogCleanup()
		go func() {
			time.Sleep(2 * time.Second) // 启动后延迟再清一次（对齐 Python 版）
			runLogCleanup()
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				logger.Cleanup(logDir, retainDays)
			}
		}()
	}

	appService := app.NewApp(configDir)

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	applicationInstance := application.New(application.Options{
		Name:        "WPSEnhancer",
		Description: "WPS 增强工具",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// 启动静默检查更新（等价 Python 版 auto_update_enabled；发现新版本时发事件通知前端）
	if st.AutoUpdateEnabled {
		go func() {
			time.Sleep(3 * time.Second) // 等 webview 就绪，避免事件早于页面监听
			if st.UseSystemProxy {
				updater.ApplySystemProxy()
			}
			client := &http.Client{Timeout: 10 * time.Second}
			release, err := updater.CheckLatestRelease(st.UpdateURL, client)
			if err != nil {
				slog.Warn("启动检查更新失败（静默）", "error", err.Error())
				return
			}
			latest := strings.TrimPrefix(release.TagName, "v")
			hasUpdate := updater.CompareVersions(version.Version, latest) < 0
			slog.Info("启动检查更新完成", "latest", latest, "has_update", hasUpdate)
			if !hasUpdate {
				return
			}
			if inst := application.Get(); inst != nil {
				inst.Event.Emit("update:available", map[string]any{
					"current": version.Version,
					"latest":  latest,
				})
			}
		}()
	}

	// 主窗口：标准原生标题栏（与 Python/Qt 版一致），拖动/红绿灯系统级处理
	applicationInstance.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "WPS 增强工具",
		Width:  1000,
		Height: 700,
		Mac: application.MacWindow{
			// 规避 macOS 26 隐形标题栏 + 全尺寸内容组合下的页面顶部裁剪问题
			TitleBar: application.MacTitleBarDefault,
		},
		BackgroundColour: application.NewRGB(245, 246, 248),
		URL:              "/",
	})

	// 运行应用（阻塞至退出）
	runErr := applicationInstance.Run()
	if runErr != nil {
		log.Fatal(runErr)
	}
}
