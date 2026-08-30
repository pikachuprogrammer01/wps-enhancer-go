package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wps-enhancer-go/internal/errs"
)

// Store 模板文件存储访问（无全局状态，目录经调用方注入）。
type Store struct{}

// templateFileVersion 模板文件格式版本（与 Python 版 TEMPLATE_FILE_VERSION 一致）。
const templateFileVersion = 2

// illegalChars 模板名非法字符（与 Python 版 _ILLEGAL_CHARS 一致）。
const illegalChars = `\/:*?"<>|`

// SanitizeFilename 将模板名转换为安全文件名（非法字符替换为 _，空名返回 _）。
func SanitizeFilename(name string) string {
	for _, ch := range illegalChars {
		name = strings.ReplaceAll(name, string(ch), "_")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "_"
	}
	return name
}

// columnsFromRaw 从 JSON 原始数据构建 TemplateColumn 列表（未知键忽略，缺键用默认值）。
func columnsFromRaw(raw []map[string]any) []TemplateColumn {
	columns := make([]TemplateColumn, 0)
	for _, item := range raw {
		columns = append(columns, TemplateColumn{
			Key:     fmt.Sprintf("%v", item["key"]),
			Name:    fmt.Sprintf("%v", item["name"]),
			Enabled: boolOr(item["enabled"], true),
		})
	}
	return columns
}

// boolOr 读取 JSON 布尔值，缺省时返回默认。
func boolOr(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// SaveTemplate 将模板写入 JSON 文件（原子写入，先写临时文件再替换）。
func SaveTemplate(tmpl *Template, path string) error {
	data := map[string]any{
		"name":          tmpl.Name,
		"version":       templateFileVersion,
		"columns":       tmpl.Columns,
		"mappings":      tmpl.Mappings,
		"source_format": tmpl.SourceFormat,
	}
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: 无法写入模板文件 '%s': %v", errs.ErrTemplate, path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("%w: 无法写入模板文件 '%s': %v", errs.ErrTemplate, path, err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0o644); err != nil {
		return fmt.Errorf("%w: 无法写入模板文件 '%s': %v", errs.ErrTemplate, path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("%w: 无法写入模板文件 '%s': %v", errs.ErrTemplate, path, err)
	}
	return nil
}

// LoadTemplate 从 JSON 文件读取模板（损坏或缺失返回 ErrFileRead）。
func LoadTemplate(path string) (*Template, error) {
	rawText, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取模板文件 '%s': %v", errs.ErrFileRead, path, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(rawText, &raw); err != nil {
		return nil, fmt.Errorf("%w: 模板文件 '%s' 格式损坏：%v", errs.ErrFileRead, path, err)
	}
	name := filepath.Base(path)
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	if rawName, ok := raw["name"].(string); ok && rawName != "" {
		name = rawName
	}
	columns := make([]TemplateColumn, 0)
	if rawCols, ok := raw["columns"].([]any); ok {
		cols := make([]map[string]any, 0, len(rawCols))
		for _, c := range rawCols {
			if m, ok := c.(map[string]any); ok {
				cols = append(cols, m)
			}
		}
		columns = columnsFromRaw(cols)
	}
	mappings := make(map[string]string)
	if rawMappings, ok := raw["mappings"].(map[string]any); ok {
		for k, v := range rawMappings {
			mappings[k] = fmt.Sprintf("%v", v)
		}
	}
	tmpl := &Template{Name: name, Columns: columns, Mappings: mappings}
	if sf, ok := raw["source_format"].(string); ok && sf != "" {
		tmpl.SourceFormat = &sf
	}
	return tmpl, nil
}

// ListTemplates 扫描模板目录，返回全部模板（按名称排序；目录不存在返回空列表）。
// 容错：单个模板损坏跳过，不影响其他模板加载（SPEC 定义行为）。
func ListTemplates(templatesDir string) []*Template {
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		return []*Template{}
	}
	templates := make([]*Template, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		tmpl, err := LoadTemplate(filepath.Join(templatesDir, entry.Name()))
		if err != nil {
			continue // 单个模板损坏不影响其他模板加载
		}
		templates = append(templates, tmpl)
	}
	// 按名称排序（稳定排序）
	for i := 1; i < len(templates); i++ {
		for j := i; j > 0 && templates[j].Name < templates[j-1].Name; j-- {
			templates[j], templates[j-1] = templates[j-1], templates[j]
		}
	}
	return templates
}

// DeleteTemplate 删除模板文件（文件不存在时幂等成功）。
func DeleteTemplate(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("%w: 无法删除模板文件 '%s': %v", errs.ErrTemplate, path, err)
	}
	return nil
}
