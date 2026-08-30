package template

import (
	"path/filepath"
	"testing"
)

// testBuiltins 测试用内置列（与 settings.defaultBuiltinColumns 一致）。
func testBuiltins() []BuiltinColumn {
	return []BuiltinColumn{
		{Key: "name", Label: "姓名", Aliases: []string{"姓名", "联系人", "名字"}},
		{Key: "phone", Label: "手机", Aliases: []string{"手机", "手机号", "电话"}},
		{Key: "company", Label: "公司名", Aliases: []string{"公司", "公司名", "单位"}},
		{Key: "website", Label: "网址", Aliases: []string{"网址", "官网", "网站"}},
	}
}

// saveTestTemplate 在 dir 下写入测试模板。
func saveTestTemplate(t *testing.T, dir, name string, columns []TemplateColumn) {
	t.Helper()
	tmpl := &Template{Name: name, Columns: columns}
	path := filepath.Join(dir, SanitizeFilename(name)+".json")
	if err := SaveTemplate(tmpl, path); err != nil {
		t.Fatalf("SaveTemplate(%s): %v", name, err)
	}
}

// TestApplicationSuggest 多模板按 Matched 降序、Matched=0 不出现、MissingCols 正确。
func TestApplicationSuggest(t *testing.T) {
	dir := t.TempDir()
	builtins := testBuiltins()
	headers := []string{"姓名", "手机号", "备注"}

	saveTestTemplate(t, dir, "全匹配", []TemplateColumn{
		{Key: "name", Name: "姓名", Enabled: true},
		{Key: "phone", Name: "手机", Enabled: true},
	})
	saveTestTemplate(t, dir, "部分匹配", []TemplateColumn{
		{Key: "name", Name: "姓名", Enabled: true},
		{Key: "phone", Name: "手机", Enabled: true},
		{Key: "company", Name: "公司名", Enabled: true},
		{Key: "website", Name: "网址", Enabled: true},
	})
	saveTestTemplate(t, dir, "零匹配", []TemplateColumn{
		{Key: "company", Name: "公司名", Enabled: true},
		{Key: "website", Name: "网址", Enabled: true},
	})

	app := Application{Store: &Store{}}
	suggestions := app.Suggest(headers, dir, builtins)

	if len(suggestions) != 2 {
		t.Fatalf("期望 2 条建议（零匹配不出现），实际 %d: %+v", len(suggestions), suggestions)
	}
	if suggestions[0].Name != "全匹配" || suggestions[0].Matched != 2 || suggestions[0].Total != 2 {
		t.Errorf("第一名应为全匹配(2/2): %+v", suggestions[0])
	}
	if suggestions[1].Name != "部分匹配" || suggestions[1].Matched != 2 || suggestions[1].Total != 4 {
		t.Errorf("第二名应为部分匹配(2/4): %+v", suggestions[1])
	}
	if len(suggestions[1].MissingCols) != 2 {
		t.Errorf("部分匹配 MissingCols 应有 2 项: %+v", suggestions[1].MissingCols)
	}
	for _, missing := range suggestions[1].MissingCols {
		if missing != "公司名" && missing != "网址" {
			t.Errorf("意外 MissingCol: %s", missing)
		}
	}
}

// TestApplicationApply 映射覆盖语义、模板不存在返回 error、MissingCols 正确。
func TestApplicationApply(t *testing.T) {
	dir := t.TempDir()
	builtins := testBuiltins()
	headers := []string{"联系人", "手机号", "备注"}

	saveTestTemplate(t, dir, "通讯录", []TemplateColumn{
		{Key: "name", Name: "姓名", Enabled: true},
		{Key: "phone", Name: "手机", Enabled: true},
		{Key: "company", Name: "公司名", Enabled: true},
	})

	app := Application{Store: &Store{}}
	tmpl, _ := LoadTemplate(filepath.Join(dir, SanitizeFilename("通讯录")+".json"))
	expectedMatches := MatchColumns(headers, tmpl, builtins, map[string]string{})

	applied, err := app.Apply(headers, dir, "通讯录", builtins)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.Name != "通讯录" {
		t.Errorf("Name 异常: %s", applied.Name)
	}
	if len(applied.Matches) != len(expectedMatches) {
		t.Fatalf("Matches 长度不一致: %d vs %d", len(applied.Matches), len(expectedMatches))
	}
	for i := range expectedMatches {
		em, am := expectedMatches[i], applied.Matches[i]
		if em.TemplateCol.Key != am.TemplateCol.Key || em.Status != am.Status {
			t.Errorf("match[%d] 不一致: %+v vs %+v", i, em, am)
		}
		es, as := em.SourceCol, am.SourceCol
		if (es == nil) != (as == nil) {
			t.Errorf("match[%d] source_col nil 不一致", i)
		} else if es != nil && *es != *as {
			t.Errorf("match[%d] source_col: %s vs %s", i, *es, *as)
		}
	}
	if applied.Mapping["name"] != "联系人" {
		t.Errorf("mapping[name] 应为联系人: %v", applied.Mapping)
	}
	if applied.Mapping["phone"] != "手机号" {
		t.Errorf("mapping[phone] 应为手机号: %v", applied.Mapping)
	}
	if _, ok := applied.Mapping["company"]; ok {
		t.Errorf("未匹配列不应出现在 mapping: %v", applied.Mapping)
	}
	if len(applied.MissingCols) != 1 || applied.MissingCols[0] != "公司名" {
		t.Errorf("MissingCols 异常: %+v", applied.MissingCols)
	}

	if _, err := app.Apply(headers, dir, "不存在", builtins); err == nil {
		t.Error("模板不存在应返回 error")
	}
}

// TestApplicationApplyUsesMappings 模板内 mappings 优先于自动匹配。
func TestApplicationApplyUsesMappings(t *testing.T) {
	dir := t.TempDir()
	builtins := testBuiltins()
	headers := []string{"联系人A", "手机号", "公司X"}

	path := filepath.Join(dir, SanitizeFilename("带映射")+".json")
	tmpl := &Template{
		Name: "带映射",
		Columns: []TemplateColumn{
			{Key: "name", Name: "姓名", Enabled: true},
			{Key: "phone", Name: "手机", Enabled: true},
			{Key: "company", Name: "公司名", Enabled: true},
		},
		// name 故意映射到「联系人A」（非别名），验证 mappings 优先
		Mappings: map[string]string{"name": "联系人A", "company": "公司X"},
	}
	if err := SaveTemplate(tmpl, path); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}

	app := Application{Store: &Store{}}
	applied, err := app.Apply(headers, dir, "带映射", builtins)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.Mapping["name"] != "联系人A" {
		t.Errorf("mappings 优先：name 应为联系人A，got %v", applied.Mapping)
	}
	if applied.Mapping["phone"] != "手机号" {
		t.Errorf("未建议列应走别名匹配：phone 应为手机号，got %v", applied.Mapping)
	}
	if applied.Mapping["company"] != "公司X" {
		t.Errorf("mappings 优先：company 应为公司X，got %v", applied.Mapping)
	}
}
