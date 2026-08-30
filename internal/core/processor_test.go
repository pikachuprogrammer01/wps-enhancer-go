package core

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/settings"
	"wps-enhancer-go/internal/template"
)

// goldenCase 别名（测试内沿用旧名）。
type goldenCase = GoldenCase

type goldenFile = GoldenFile

// loadGolden 读取迁移基准夹具（由 scripts/gen_golden.go 生成）。
func loadGolden(t *testing.T) *goldenFile {
	t.Helper()
	b, err := os.ReadFile("../../docs/migration/testdata/processor_golden.json")
	if err != nil {
		t.Fatalf("读取 golden 文件失败: %v", err)
	}
	var g goldenFile
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("解析 golden 文件失败: %v", err)
	}
	return &g
}

// str 从 input 取字符串字段。
func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// strSlice 从 input 取字符串切片字段。
func strSlice(m map[string]any, key string) []string {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		out = append(out, v.(string))
	}
	return out
}

// strMap 从 input 取 string→string 映射字段。
func strMap(m map[string]any, key string) map[string]string {
	raw, ok := m[key].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = v.(string)
	}
	return out
}

// rawMap 把 input 里的 map 原样取出（settings/template/builtins 反序列化用）。
func rawMap(m map[string]any, key string) map[string]any {
	raw, _ := m[key].(map[string]any)
	return raw
}

// settingsFromInput 从 input.settings 反序列化 AppSettings（缺失字段用默认值）。
func settingsFromInput(in map[string]any) *settings.AppSettings {
	st := settings.New()
	if raw := rawMap(in, "settings"); raw != nil {
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, st)
	}
	return st
}

// templateFromInput 从 input.template 反序列化 Template。
func templateFromInput(in map[string]any) *template.Template {
	raw := rawMap(in, "template")
	if raw == nil {
		return nil
	}
	b, _ := json.Marshal(raw)
	var tmpl template.Template
	_ = json.Unmarshal(b, &tmpl)
	return &tmpl
}

// builtinsFromInput 从 input.builtins 反序列化内置列。
func builtinsFromInput(in map[string]any) []template.BuiltinColumn {
	raw, ok := in["builtins"].([]any)
	if !ok {
		return nil
	}
	b, _ := json.Marshal(raw)
	var cols []template.BuiltinColumn
	_ = json.Unmarshal(b, &cols)
	return cols
}

// sheetFromInput 从 input.headers/rows 构造 SheetData。
func sheetFromInput(in map[string]any) *excel.SheetData {
	data := &excel.SheetData{
		SheetName: "s",
		Headers:   strSlice(in, "headers"),
	}
	rawRows, ok := in["rows"].([]any)
	if !ok {
		return data
	}
	for _, rr := range rawRows {
		row := make(map[string]string)
		for k, v := range rr.(map[string]any) {
			row[k] = v.(string)
		}
		data.Rows = append(data.Rows, row)
	}
	return data
}

// normalize 将结果序列化后重新解析为 any（消除 int/float 与字段顺序差异）。
func normalize(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// TestGolden 用迁移基准夹具逐用例断言（41 个用例，覆盖 7 个函数）。
func TestGolden(t *testing.T) {
	g := loadGolden(t)
	executed := make(map[string]any)
	find := func(name string) *goldenCase {
		for i := range g.Cases {
			if g.Cases[i].Name == name {
				return &g.Cases[i]
			}
		}
		return nil
	}

	var run func(c goldenCase) any
	run = func(c goldenCase) any {
		if v, ok := executed[c.Name]; ok {
			return v
		}
		// 依赖用例：先执行 preview_ref 指向的用例
		if ref := str(c.Input, "preview_ref"); ref != "" {
			if dep := find(ref); dep != nil {
				run(*dep)
			}
		}
		result := dispatch(c, g, find, executed)
		executed[c.Name] = result
		return result
	}

	for _, c := range g.Cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			got := normalize(run(c))
			want := normalize(c.Output)
			if !reflect.DeepEqual(got, want) {
				gotB, _ := json.MarshalIndent(got, "", "  ")
				wantB, _ := json.MarshalIndent(want, "", "  ")
				t.Errorf("输出与 golden 不一致\n--- got ---\n%s\n--- want ---\n%s", gotB, wantB)
			}
		})
	}
}

// dispatch 按用例 func 分发到对应函数；matches 从 preview_ref 依赖用例的 input 重建。
func dispatch(c goldenCase, g *goldenFile, find func(string) *goldenCase, executed map[string]any) any {
	in := c.Input
	// 依赖用例的 input（用于重建 matches/模板等）
	depInput := in
	if ref := str(in, "preview_ref"); ref != "" {
		if dep := find(ref); dep != nil {
			depInput = dep.Input
		}
	}

	switch c.Func {
	case "split_phones":
		return SplitPhones(str(in, "raw_phone"), strSlice(in, "separators"))
	case "validate_phone":
		return ValidatePhone(str(in, "phone"))
	case "match_columns":
		return template.MatchColumns(
			strSlice(in, "headers"), templateFromInput(in), builtinsFromInput(in), strMap(in, "manual_map"))
	case "build_preview_data":
		preview, err := BuildPreviewData(
			sheetFromInput(in), templateFromInput(in),
			template.MatchColumns(strSlice(in, "headers"), templateFromInput(in), builtinsFromInput(in), strMap(in, "manual_map")),
			settingsFromInput(in))
		if err != nil {
			return err.Error()
		}
		return preview
	case "build_write_request":
		preview, _ := executed[str(in, "preview_ref")].(*PreviewData)
		if preview == nil {
			return "missing preview_ref dependency"
		}
		matches := template.MatchColumns(
			strSlice(depInput, "headers"), templateFromInput(depInput), builtinsFromInput(depInput), strMap(depInput, "manual_map"))
		return BuildWriteRequest(preview, templateFromInput(depInput), matches, settingsFromInput(depInput), str(in, "output_path"))
	case "build_preview_display":
		preview, _ := executed[str(in, "preview_ref")].(*PreviewData)
		if preview == nil {
			return "missing preview_ref dependency"
		}
		matches := template.MatchColumns(
			strSlice(depInput, "headers"), templateFromInput(depInput), builtinsFromInput(depInput), strMap(depInput, "manual_map"))
		headers, rows := BuildPreviewDisplay(preview, matches, settingsFromInput(depInput), str(in, "fmt"))
		return map[string]any{"headers": headers, "rows": rows}
	case "build_text_preview":
		preview, _ := executed[str(in, "preview_ref")].(*PreviewData)
		if preview == nil {
			return "missing preview_ref dependency"
		}
		matches := template.MatchColumns(
			strSlice(depInput, "headers"), templateFromInput(depInput), builtinsFromInput(depInput), strMap(depInput, "manual_map"))
		text, err := BuildTextPreview(preview, matches, settingsFromInput(depInput), str(in, "fmt"), 30)
		if err != nil {
			return err.Error()
		}
		return text
	}
	return "unknown func: " + c.Func
}

// TestPreviewSummaryLine 预览汇总文案（vcf 前缀 + 异常计数）。
func TestPreviewSummaryLine(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(*settings.AppSettings)
		total  int
		format string
		invalid int
		want   string
		wantContains []string // 与 want 二选一；用于含日期的 vcf 前缀
	}{
		{
			name: "xlsx_with_invalid",
			setup: func(st *settings.AppSettings) { st.PhoneValidate = true },
			total: 10, format: "xlsx", invalid: 2,
			want: "共 10 行，其中 2 个手机号格式异常",
		},
		{
			name: "xlsx_no_invalid",
			setup: func(st *settings.AppSettings) { st.PhoneValidate = true },
			total: 10, format: "xlsx", invalid: 0,
			want: "共 10 行",
		},
		{
			name: "vcf_prefix_no_invalid",
			setup: func(st *settings.AppSettings) {
				st.VCFNamePrefix = "vcf_"
				st.VCFTimestamp = false
			},
			total: 5, format: "vcf", invalid: 0,
			want: "共 5 行，vcf 姓名前缀：vcf_",
		},
		{
			name: "vcf_timestamp_prefix_with_invalid",
			setup: func(st *settings.AppSettings) {
				st.PhoneValidate = true
				st.VCFNamePrefix = "vcf_"
				st.VCFTimestamp = true
				st.VCFTimestampPosition = "prefix"
			},
			total: 3, format: "vcf", invalid: 1,
			wantContains: []string{"共 3 行", "vcf 姓名前缀：vcf_", "其中 1 个手机号格式异常"},
		},
		{
			name: "validate_off_omits_invalid",
			setup: func(st *settings.AppSettings) {
				st.PhoneValidate = false
				st.VCFNamePrefix = "vcf_"
				st.VCFTimestamp = false
			},
			total: 3, format: "vcf", invalid: 5,
			want: "共 3 行，vcf 姓名前缀：vcf_",
		},
		{
			name: "non_vcf_ignores_prefix_settings",
			setup: func(st *settings.AppSettings) {
				st.VCFNamePrefix = "vcf_"
				st.PhoneValidate = true
			},
			total: 4, format: "csv", invalid: 1,
			want: "共 4 行，其中 1 个手机号格式异常",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := settings.New()
			if tc.setup != nil {
				tc.setup(st)
			}
			got := PreviewSummaryLine(st, tc.total, tc.format, tc.invalid)
			if tc.want != "" && got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
			for _, sub := range tc.wantContains {
				if !strings.Contains(got, sub) {
					t.Errorf("got %q should contain %q", got, sub)
				}
			}
		})
	}
}
