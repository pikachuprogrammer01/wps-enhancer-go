package app

import (
	"wps-enhancer-go/internal/core"
	"wps-enhancer-go/internal/excel"
)

// PreviewSummary 返回预览区顶部汇总文案（vcf 前缀说明 + 异常手机号计数）。
func (a *App) PreviewSummary(total int, format string, invalidCount int) string {
	return core.PreviewSummaryLine(a.settings, total, format, invalidCount)
}

// DetectTruncatedNumbers 检测 Sheet 中疑似号码/身份证截断补零的列提示（读表后弹窗用）。
func (a *App) DetectTruncatedNumbers(data *excel.SheetData) []string {
	return core.DetectTruncatedNumbers(data)
}
