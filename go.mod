module wps-enhancer-go

go 1.25.0

require (
	github.com/extrame/xls v0.0.2-0.20200426124601-4a6cf263071b
	github.com/google/go-cmp v0.7.0
	github.com/wailsapp/wails/v3 v3.0.0-beta.7
	github.com/xuri/excelize/v2 v2.11.0
	golang.org/x/text v0.39.0
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/extrame/goyymmdd v0.0.0-20210114090516-7cc815f00d1a // indirect
	github.com/extrame/ole2 v0.0.0-20160812065207-d69429661ad7 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/richardlehane/mscfb v1.0.7 // indirect
	github.com/richardlehane/msoleps v1.0.6 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/xuri/efp v0.0.1 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
)

replace github.com/extrame/xls => ./third_party/xls

// wps-enhancer patch: 本地 Wails 副本（关闭 macOS 整窗背景拖动 movableByWindowBackground，
// 窗口移动仅保留 InvisibleTitleBarHeight 顶部拖拽区）。升级 Wails 时需重新应用 patch：
// third_party/wails/pkg/application/webview_window_darwin.m 第 52 行附近 YES→NO。
replace github.com/wailsapp/wails/v3 => ./third_party/wails
