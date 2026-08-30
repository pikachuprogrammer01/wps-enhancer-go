// Package errs 全局统一错误定义（sentinel error）。
// 与 Python 版 core/exceptions.py 一一对应；业务层用 errors.Is 判定类型。
package errs

import "errors"

var (
	// ErrDataProcess 数据处理失败（DataProcessError）
	ErrDataProcess = errors.New("data process error")
	// ErrFileRead 文件读取失败（FileReadError）
	ErrFileRead = errors.New("file read error")
	// ErrFileWrite 文件写入失败（FileWriteError）
	ErrFileWrite = errors.New("file write error")
	// ErrTemplate 模板错误（TemplateError）
	ErrTemplate = errors.New("template error")
	// ErrSettings 设置错误（SettingsError）
	ErrSettings = errors.New("settings error")
	// ErrNetwork 网络错误（NetworkError，updater/license 用）
	ErrNetwork = errors.New("network error")
)
