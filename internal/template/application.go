package template

import (
	"path/filepath"
	"sort"
)

// Suggestion 单个模板的匹配建议（按 Matched 降序排列）。
type Suggestion struct {
	Name        string   `json:"name"`
	Matched     int      `json:"matched"`      // 匹配到的模板列数
	Total       int      `json:"total"`        // 模板列总数
	MissingCols []string `json:"missing_cols"` // 未匹配的模板列名
}

// Applied 模板应用到数据源后的完整映射结果。
type Applied struct {
	Name        string            `json:"name"`
	Matches     []ColumnMatch     `json:"matches"`
	Mapping     map[string]string `json:"mapping"` // 模板列 key → 源列名
	MissingCols []string          `json:"missing_cols"`
}

// Application 模板应用领域服务（无全局状态，dir/builtins 经参数注入）。
type Application struct {
	Store *Store
}

// Suggest 对所有已存模板打分，只返回 Matched>0 的模板（按 Matched 降序）。
// 打分时优先恢复模板内保存的 mappings，其余列走 exact/alias 自动匹配。
func (a *Application) Suggest(headers []string, dir string, builtins []BuiltinColumn) []Suggestion {
	templates := ListTemplates(dir)
	suggestions := make([]Suggestion, 0)
	for _, tmpl := range templates {
		matched, missing := scoreMatches(MatchColumns(headers, tmpl, builtins, seedManualMap(tmpl, headers)))
		if matched == 0 {
			continue
		}
		suggestions = append(suggestions, Suggestion{
			Name:        tmpl.Name,
			Matched:     matched,
			Total:       len(tmpl.Columns),
			MissingCols: missing,
		})
	}
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Matched != suggestions[j].Matched {
			return suggestions[i].Matched > suggestions[j].Matched
		}
		return suggestions[i].Name < suggestions[j].Name
	})
	return suggestions
}

// Apply 加载指定模板并计算完整映射（覆盖语义）。
// 优先恢复模板内 mappings，未建议列走 exact/alias 自动匹配。
func (a *Application) Apply(headers []string, dir string, name string, builtins []BuiltinColumn) (*Applied, error) {
	path := filepath.Join(dir, SanitizeFilename(name)+".json")
	tmpl, err := LoadTemplate(path)
	if err != nil {
		return nil, err
	}
	matches := MatchColumns(headers, tmpl, builtins, seedManualMap(tmpl, headers))
	mapping, missing := mappingFromMatches(matches)
	return &Applied{
		Name:        tmpl.Name,
		Matches:     matches,
		Mapping:     mapping,
		MissingCols: missing,
	}, nil
}

// seedManualMap 从模板建议映射构建匹配引擎种子（仅保留当前表头中存在的非空目标列）。
func seedManualMap(tmpl *Template, headers []string) map[string]string {
	seed := make(map[string]string)
	if tmpl == nil || len(tmpl.Mappings) == 0 {
		return seed
	}
	allowed := make(map[string]bool, len(headers))
	for _, h := range headers {
		allowed[trimSpace(h)] = true
	}
	for k, v := range tmpl.Mappings {
		if v != "" && allowed[trimSpace(v)] {
			seed[k] = v
		}
	}
	return seed
}

// scoreMatches 统计匹配列数与未匹配列名。
func scoreMatches(matches []ColumnMatch) (matched int, missing []string) {
	missing = make([]string, 0)
	for _, m := range matches {
		if m.Status != "none" {
			matched++
			continue
		}
		missing = append(missing, m.TemplateCol.Name)
	}
	return matched, missing
}

// mappingFromMatches 从匹配结果构建 key→源列名映射与未匹配列名列表。
func mappingFromMatches(matches []ColumnMatch) (map[string]string, []string) {
	mapping := make(map[string]string)
	missing := make([]string, 0)
	for _, m := range matches {
		if m.SourceCol != nil {
			mapping[m.TemplateCol.Key] = *m.SourceCol
			continue
		}
		missing = append(missing, m.TemplateCol.Name)
	}
	return mapping, missing
}
