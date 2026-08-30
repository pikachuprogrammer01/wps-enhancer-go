package app

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"wps-enhancer-go/internal/core"
	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/logger"
	"wps-enhancer-go/internal/template"
)

// templatesDir 模板目录（等价 Python 版 <configDir>/template/）。
func (a *App) templatesDir() string {
	return filepath.Join(a.configDir, "template")
}

// TemplateList 返回全部模板（按名称排序）。
func (a *App) TemplateList() []*template.Template {
	return template.ListTemplates(a.templatesDir())
}

// TemplateCreate 新建模板（列 = key/name 列表；key 为空时按 custom_<n> 生成）。
func (a *App) TemplateCreate(name string, columns []template.TemplateColumn) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("模板名不能为空")
	}
	clean, err := sanitizeTemplateColumns(columns)
	if err != nil {
		return err
	}
	tmpl := &template.Template{
		Name:    strings.TrimSpace(name),
		Columns: clean,
	}
	return template.SaveTemplate(tmpl, filepath.Join(a.templatesDir(), template.SanitizeFilename(tmpl.Name)+".json"))
}

// TemplateUpdate 更新已有模板的列集合（列映射页增删改后持久化）。
func (a *App) TemplateUpdate(name string, columns []template.TemplateColumn) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("模板名不能为空")
	}
	tmpl, err := a.resolveTemplate(name)
	if err != nil {
		return err
	}
	clean, err := sanitizeTemplateColumns(columns)
	if err != nil {
		return err
	}
	tmpl.Columns = clean
	return template.SaveTemplate(tmpl, filepath.Join(a.templatesDir(), template.SanitizeFilename(tmpl.Name)+".json"))
}

// sanitizeTemplateColumns 列集合清洗：去空名、补 key（custom_<n>）、去重、默认启用。
func sanitizeTemplateColumns(columns []template.TemplateColumn) ([]template.TemplateColumn, error) {
	seen := make(map[string]bool)
	clean := make([]template.TemplateColumn, 0, len(columns))
	for i, col := range columns {
		if col.Name == "" {
			continue
		}
		key := strings.TrimSpace(col.Key)
		if key == "" {
			key = fmt.Sprintf("custom_%d", i+1)
		}
		for seen[key] {
			key += "_2"
		}
		seen[key] = true
		col.Key = key
		col.Enabled = true
		clean = append(clean, col)
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("模板至少需要一列")
	}
	return clean, nil
}

// TemplateCreateFromHeaders 从源表头一键生成模板（SPEC 步骤 4 快捷入口）。
// 列 key：命中内置列别名/列名 → 用内置 key；否则 custom_<n>。
// 同时写入 mappings（key→源列名），下次 Apply 可直接恢复。
func (a *App) TemplateCreateFromHeaders(name string, headers []string) error {
	tmpl := &template.Template{
		Name:     strings.TrimSpace(name),
		Mappings: make(map[string]string),
	}
	builtins := a.settings.BuiltinColumns
	used := make(map[string]bool)
	for i, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		col := template.TemplateColumn{Name: header, Enabled: true}
		for _, b := range builtins {
			if used[b.Key] {
				continue
			}
			if header == b.Label || contains(b.Aliases, header) {
				col.Key = b.Key
				used[b.Key] = true
				break
			}
		}
		if col.Key == "" {
			col.Key = fmt.Sprintf("custom_%d", i+1)
		}
		tmpl.Columns = append(tmpl.Columns, col)
		tmpl.Mappings[col.Key] = header
	}
	if len(tmpl.Columns) == 0 {
		return fmt.Errorf("源表没有可用表头")
	}
	return template.SaveTemplate(tmpl, filepath.Join(a.templatesDir(), template.SanitizeFilename(tmpl.Name)+".json"))
}

// TemplateRename 重命名模板（文件随模板名）。
func (a *App) TemplateRename(oldName, newName string) error {
	if strings.TrimSpace(newName) == "" {
		return fmt.Errorf("模板名不能为空")
	}
	dir := a.templatesDir()
	oldPath := filepath.Join(dir, template.SanitizeFilename(oldName)+".json")
	tmpl, err := template.LoadTemplate(oldPath)
	if err != nil {
		return translateErr(err)
	}
	tmpl.Name = strings.TrimSpace(newName)
	if err := template.SaveTemplate(tmpl, filepath.Join(dir, template.SanitizeFilename(tmpl.Name)+".json")); err != nil {
		return translateErr(err)
	}
	return template.DeleteTemplate(oldPath)
}

// TemplateDelete 删除模板。
func (a *App) TemplateDelete(name string) error {
	return template.DeleteTemplate(filepath.Join(a.templatesDir(), template.SanitizeFilename(name)+".json"))
}

// resolveTemplate 按名称加载模板（为空返回内置默认模板）。
func (a *App) resolveTemplate(templateName string) (*template.Template, error) {
	if templateName == "" {
		return defaultTemplate(), nil
	}
	t, err := template.LoadTemplate(filepath.Join(a.templatesDir(), template.SanitizeFilename(templateName)+".json"))
	if err != nil {
		return nil, translateErr(err)
	}
	return t, nil
}

// SuggestTemplates 对全部已存模板打分，返回 Matched>0 的建议列表（读入数据源后提示可用模板）。
func (a *App) SuggestTemplates(headers []string) []template.Suggestion {
	app := template.Application{Store: &template.Store{}}
	suggestions := app.Suggest(headers, a.templatesDir(), a.settings.BuiltinColumns)
	slog.Info("模板建议", "headers", len(headers), "templates", len(template.ListTemplates(a.templatesDir())), "matched", len(suggestions))
	return suggestions
}

// ApplyTemplate 将指定模板应用到源表头（覆盖语义：按模板 mappings + 自动匹配重建映射）。
func (a *App) ApplyTemplate(headers []string, name string) (*template.Applied, error) {
	return logger.LogCall1("ApplyTemplate", func() (*template.Applied, error) {
		app := template.Application{Store: &template.Store{}}
		applied, err := app.Apply(headers, a.templatesDir(), name, a.settings.BuiltinColumns)
		if err != nil {
			return nil, translateErr(err)
		}
		slog.Info("已应用模板", "name", applied.Name, "mapped", len(applied.Mapping), "missing", len(applied.MissingCols))
		return applied, nil
	})
}

// TemplateSetMappings 更新已有模板的建议映射（导出后持久化，下次 Apply 优先恢复）。
func (a *App) TemplateSetMappings(name string, mappings map[string]string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("模板名不能为空")
	}
	path := filepath.Join(a.templatesDir(), template.SanitizeFilename(name)+".json")
	tmpl, err := template.LoadTemplate(path)
	if err != nil {
		return translateErr(err)
	}
	if mappings == nil {
		mappings = map[string]string{}
	}
	tmpl.Mappings = mappings
	return translateErr(template.SaveTemplate(tmpl, path))
}

// TemplateMatches 返回模板列与源表列的自动匹配结果（SPEC 步骤 5：exact/alias/none 状态）。
// manualMap 为已手动调整的映射（key → 源列名），用于增量更新。
func (a *App) TemplateMatches(headers []string, templateName string, manualMap map[string]string) ([]template.ColumnMatch, error) {
	tmpl, err := a.resolveTemplate(templateName)
	if err != nil {
		return nil, err
	}
	return template.MatchColumns(headers, tmpl, a.settings.BuiltinColumns, manualMap), nil
}

// SessionMatch 用会话内列集合匹配（不读磁盘模板；列编辑后重匹配用）。
func (a *App) SessionMatch(headers []string, columns []template.TemplateColumn, manualMap map[string]string) ([]template.ColumnMatch, error) {
	clean, err := sanitizeTemplateColumns(columns)
	if err != nil {
		return nil, err
	}
	tmpl := &template.Template{Name: "_session", Columns: clean}
	return template.MatchColumns(headers, tmpl, a.settings.BuiltinColumns, manualMap), nil
}

// PreviewWithMapping 用指定模板 + 手动映射构建预览（SPEC 步骤 5-7）。
// templateName 为空时用内置默认模板；manualMap = {模板列 key: 源列名}。
func (a *App) PreviewWithMapping(data *excel.SheetData, templateName string, manualMap map[string]string) (*core.PreviewData, error) {
	return logger.LogCall1("PreviewWithMapping", func() (*core.PreviewData, error) {
		tmpl, err := a.resolveTemplate(templateName)
		if err != nil {
			return nil, err
		}
		return a.previewWith(data, tmpl, manualMap)
	})
}

// PreviewWithColumns 用会话列集合构建预览（不读磁盘模板）。
func (a *App) PreviewWithColumns(data *excel.SheetData, columns []template.TemplateColumn, manualMap map[string]string) (*core.PreviewData, error) {
	return logger.LogCall1("PreviewWithColumns", func() (*core.PreviewData, error) {
		tmpl, err := a.sessionTemplate(columns)
		if err != nil {
			return nil, err
		}
		return a.previewWith(data, tmpl, manualMap)
	})
}

// previewWith 构建预览（共享实现）。
func (a *App) previewWith(data *excel.SheetData, tmpl *template.Template, manualMap map[string]string) (*core.PreviewData, error) {
	matches := template.MatchColumns(data.Headers, tmpl, a.settings.BuiltinColumns, manualMap)
	preview, err := core.BuildPreviewData(data, tmpl, matches, a.settings)
	if err != nil {
		return nil, translateErr(err)
	}
	return preview, nil
}

// sessionTemplate 清洗列并构造会话模板。
func (a *App) sessionTemplate(columns []template.TemplateColumn) (*template.Template, error) {
	clean, err := sanitizeTemplateColumns(columns)
	if err != nil {
		return nil, err
	}
	return &template.Template{Name: "_session", Columns: clean}, nil
}

// PreviewText 返回与导出文件内容一致的文本预览（vcf/csv/txt 所见即所得，最多前 rowLimit 行数据）。
// xlsx 表格预览请用 PreviewWithMapping 的结构化结果。
func (a *App) PreviewText(data *excel.SheetData, templateName string, manualMap map[string]string, format string, rowLimit int) (string, error) {
	tmpl, err := a.resolveTemplate(templateName)
	if err != nil {
		return "", err
	}
	return a.previewTextWith(data, tmpl, manualMap, format, rowLimit)
}

// PreviewTextWithColumns 用会话列集合生成文本预览。
func (a *App) PreviewTextWithColumns(data *excel.SheetData, columns []template.TemplateColumn, manualMap map[string]string, format string, rowLimit int) (string, error) {
	tmpl, err := a.sessionTemplate(columns)
	if err != nil {
		return "", err
	}
	return a.previewTextWith(data, tmpl, manualMap, format, rowLimit)
}

// previewTextWith 文本预览共享实现。
func (a *App) previewTextWith(data *excel.SheetData, tmpl *template.Template, manualMap map[string]string, format string, rowLimit int) (string, error) {
	matches := template.MatchColumns(data.Headers, tmpl, a.settings.BuiltinColumns, manualMap)
	preview, err := core.BuildPreviewData(data, tmpl, matches, a.settings)
	if err != nil {
		return "", translateErr(err)
	}
	if rowLimit <= 0 {
		rowLimit = 30
	}
	text, err := core.BuildTextPreview(preview, matches, a.settings, format, rowLimit)
	if err != nil {
		return "", translateErr(err)
	}
	return text, nil
}

// ExportWithTemplate 用指定模板 + 手动映射导出（Spec 步骤 8-10）。
func (a *App) ExportWithTemplate(data *excel.SheetData, templateName string, manualMap map[string]string, outputPath string) error {
	return logger.LogCall("ExportWithTemplate", func() error {
		tmpl, err := a.resolveTemplate(templateName)
		if err != nil {
			return err
		}
		return a.exportWith(data, tmpl, manualMap, outputPath)
	})
}

// ExportWithColumns 用会话列集合导出（不读磁盘模板）。
func (a *App) ExportWithColumns(data *excel.SheetData, columns []template.TemplateColumn, manualMap map[string]string, outputPath string) error {
	return logger.LogCall("ExportWithColumns", func() error {
		tmpl, err := a.sessionTemplate(columns)
		if err != nil {
			return err
		}
		return a.exportWith(data, tmpl, manualMap, outputPath)
	})
}

// exportWith 导出共享实现。
func (a *App) exportWith(data *excel.SheetData, tmpl *template.Template, manualMap map[string]string, outputPath string) error {
	matches := template.MatchColumns(data.Headers, tmpl, a.settings.BuiltinColumns, manualMap)
	preview, err := core.BuildPreviewData(data, tmpl, matches, a.settings)
	if err != nil {
		return translateErr(err)
	}
	request := core.BuildWriteRequest(preview, tmpl, matches, a.settings, outputPath)
	request.FilePath = normalizeExportPath(request.FilePath)
	writer, err := excel.GetWriter(request.FilePath)
	if err != nil {
		return translateErr(err)
	}
	slog.Info("开始导出", "format", filepath.Ext(request.FilePath), "rows", len(preview.Rows), "output", request.FilePath)
	if err := writer.WriteExport(request); err != nil {
		return translateErr(err)
	}
	slog.Info("导出完成", "output", request.FilePath)
	return nil
}

// normalizeExportPath 规范化导出路径：xls 扩展名自动转换为 xlsx。
// Go 生态无 xlwt 等价物，不实现 xls 写出；xlsx 兼容性更好且对外保持 5 种格式口径
// （决策记录：docs/migration/00-overview.md §6.8）。
func normalizeExportPath(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".xls") {
		return strings.TrimSuffix(path, filepath.Ext(path)) + ".xlsx"
	}
	return path
}

// contains 判断切片是否包含元素。
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
