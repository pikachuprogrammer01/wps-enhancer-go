// Package core 领域逻辑纯函数层（L1：processor.py 直译，无 I/O、无 UI、无全局状态）。
package core

import (
	"unicode"

	"wps-enhancer-go/internal/errs"
)

// 错误体系别名：与 core/exceptions.py 一一对应（业务层用 errors.Is 判定）。
// 定义在 internal/errs（excel 等包也需引用，避免循环依赖）。
var (
	// ErrDataProcess 数据处理失败（DataProcessError）
	ErrDataProcess = errs.ErrDataProcess
	// ErrFileRead 文件读取失败（FileReadError）
	ErrFileRead = errs.ErrFileRead
	// ErrFileWrite 文件写入失败（FileWriteError）
	ErrFileWrite = errs.ErrFileWrite
	// ErrTemplate 模板错误（TemplateError）
	ErrTemplate = errs.ErrTemplate
	// ErrSettings 设置错误（SettingsError）
	ErrSettings = errs.ErrSettings
	// ErrNetwork 网络错误（NetworkError，updater/license 用）
	ErrNetwork = errs.ErrNetwork
)

// isAllDigits 等价 Python str.isdigit()（unicode 数字字符判定，含全角数字）。
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
