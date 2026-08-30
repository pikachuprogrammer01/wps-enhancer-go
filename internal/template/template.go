// Package template 定义模板系统的数据结构与列匹配引擎（纯函数，L1 迁移）。
package template

import "strings"

// TemplateColumn 模板中的一列。
type TemplateColumn struct {
	Key     string `json:"key"`     // 语义键（稳定标识：name/phone/company/website/custom_<n>）
	Name    string `json:"name"`    // 列显示名（导出表头）
	Enabled bool   `json:"enabled"` // 导出时是否包含
}

// Template 一个模板 = 名称 + 列集合 + 可选建议映射（key → 源列名，应用时优先恢复）。
type Template struct {
	Name         string                    `json:"name"`
	Columns      []TemplateColumn          `json:"columns"`
	Mappings     map[string]string         `json:"mappings"`
	SourceFormat *string                   `json:"source_format,omitempty"` // "excel"/"text"；None=不限
}

// BuiltinColumn 内置列（语义字段），可增删改查。
type BuiltinColumn struct {
	Key     string   `json:"key"`               // 语义键（创建时分配，不再修改）
	Label   string   `json:"label"`             // 显示名（用户可改）
	Aliases []string `json:"aliases"`           // 匹配别名（用户可维护）
	VCFProp string   `json:"vcf_prop,omitempty"` // vCard 属性名（如 NOTE/EMAIL）；空则自定义字段默认 NOTE
}

// ColumnMatch 单个模板列的匹配结果。
type ColumnMatch struct {
	TemplateCol TemplateColumn `json:"template_col"`
	SourceCol   *string        `json:"source_col"` // 匹配到的源表列名；未匹配为 nil
	Status      string         `json:"status"`     // "manual" | "exact" | "alias" | "none"
}

// findSource 在未占用的源列中查找首个满足条件的列，返回原始列名。
func findSource(headers, strippedHeaders []string, used map[string]bool, predicate func(string) bool) *string {
	for i, orig := range headers {
		if used[strippedHeaders[i]] {
			continue
		}
		if predicate(strippedHeaders[i]) {
			return &orig
		}
	}
	return nil
}

// MatchColumns 按优先级匹配模板列与源表列（纯函数）：manual > exact > alias > none。
func MatchColumns(headers []string, tmpl *Template, builtinColumns []BuiltinColumn, manualMap map[string]string) []ColumnMatch {
	aliasMap := make(map[string][]string, len(builtinColumns))
	for _, col := range builtinColumns {
		aliasMap[col.Key] = col.Aliases
	}
	strippedHeaders := make([]string, len(headers))
	for i, h := range headers {
		strippedHeaders[i] = trimSpace(h)
	}
	used := make(map[string]bool)
	matches := make([]ColumnMatch, 0, len(tmpl.Columns))

	for _, tcol := range tmpl.Columns {
		if src, ok := manualMap[tcol.Key]; ok {
			// 手动指定：采用指定值（可为空字符串表示不映射）
			if src != "" {
				used[trimSpace(src)] = true
			}
			var source *string
			if src != "" {
				s := src
				source = &s
			}
			matches = append(matches, ColumnMatch{TemplateCol: tcol, SourceCol: source, Status: "manual"})
			continue
		}

		if target := findSource(headers, strippedHeaders, used, func(s string) bool { return s == tcol.Name }); target != nil {
			used[trimSpace(*target)] = true
			matches = append(matches, ColumnMatch{TemplateCol: tcol, SourceCol: target, Status: "exact"})
			continue
		}

		aliases := aliasMap[tcol.Key]
		if target := findSource(headers, strippedHeaders, used, func(s string) bool {
			for _, a := range aliases {
				if s == a {
					return true
				}
			}
			return false
		}); target != nil {
			used[trimSpace(*target)] = true
			matches = append(matches, ColumnMatch{TemplateCol: tcol, SourceCol: target, Status: "alias"})
			continue
		}

		matches = append(matches, ColumnMatch{TemplateCol: tcol, SourceCol: nil, Status: "none"})
	}
	return matches
}

// trimSpace 等价 Python str.strip()。
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
