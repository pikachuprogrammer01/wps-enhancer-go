package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/template"
)

// testApp 创建临时 configDir 的 App 实例。
func testApp(t *testing.T) *App {
	t.Helper()
	return NewApp(t.TempDir())
}

// TestTemplateCRUD 模板管理：新建/列表/重命名/删除/从表头导入。
func TestTemplateCRUD(t *testing.T) {
	a := testApp(t)

	// 新建
	err := a.TemplateCreate("通讯录", []template.TemplateColumn{
		{Key: "name", Name: "姓名"}, {Key: "phone", Name: "手机"},
	})
	if err != nil {
		t.Fatalf("TemplateCreate: %v", err)
	}
	// 空列应拒绝
	if err := a.TemplateCreate("空模板", nil); err == nil {
		t.Error("空列模板应拒绝")
	}
	list := a.TemplateList()
	if len(list) != 1 || list[0].Name != "通讯录" {
		t.Fatalf("TemplateList 异常: %+v", list)
	}

	// 从表头导入（命中内置别名）
	err = a.TemplateCreateFromHeaders("导入模板", []string{"联系人", "手机号", "备注"})
	if err != nil {
		t.Fatalf("TemplateCreateFromHeaders: %v", err)
	}
	list = a.TemplateList()
	if len(list) != 2 {
		t.Fatalf("应有 2 个模板: %d", len(list))
	}
	// 导入的列 key：联系人→name、手机号→phone、备注→custom_3
	var imported *template.Template
	for _, tmpl := range list {
		if tmpl.Name == "导入模板" {
			imported = tmpl
		}
	}
	if imported == nil || len(imported.Columns) != 3 {
		t.Fatalf("导入模板异常: %+v", imported)
	}
	if imported.Columns[0].Key != "name" || imported.Columns[1].Key != "phone" {
		t.Errorf("内置列 key 匹配失败: %+v", imported.Columns)
	}
	if imported.Columns[2].Key != "custom_3" {
		t.Errorf("自定义列 key 异常: %+v", imported.Columns[2])
	}

	// 重命名
	if err := a.TemplateRename("通讯录", "通讯录V2"); err != nil {
		t.Fatalf("TemplateRename: %v", err)
	}
	list = a.TemplateList()
	found := false
	for _, tmpl := range list {
		if tmpl.Name == "通讯录V2" {
			found = true
		}
		if tmpl.Name == "通讯录" {
			t.Error("旧名模板应已删除")
		}
	}
	if !found {
		t.Error("重命名后新名应存在")
	}

	// 删除
	if err := a.TemplateDelete("通讯录V2"); err != nil {
		t.Fatalf("TemplateDelete: %v", err)
	}
	if len(a.TemplateList()) != 1 {
		t.Error("删除后应剩 1 个模板")
	}
}

// TestPreviewWithMapping 映射预览：自动匹配 + 手动映射 + 未映射列空值。
func TestPreviewWithMapping(t *testing.T) {
	a := testApp(t)
	data := &excel.SheetData{
		SheetName: "s",
		Headers:   []string{"姓名", "手机号", "公司"},
		Rows: []map[string]string{
			{"姓名": "张三", "手机号": "13800000000;13900000000", "公司": "A公司"},
			{"姓名": "李四", "手机号": "bad", "公司": "B公司"},
		},
	}

	// 默认模板自动匹配（姓名→姓名 exact、手机→手机号 alias、公司→公司 exact）
	preview, err := a.PreviewWithMapping(data, "", nil)
	if err != nil {
		t.Fatalf("PreviewWithMapping: %v", err)
	}
	if len(preview.Rows) != 3 { // 张三 2 行 + 李四 1 行
		t.Errorf("默认模板应拆出 3 行: %d", len(preview.Rows))
	}
	if preview.Rows[0].Values[0] != "张三" || preview.Rows[0].Values[1] != "13800000000" {
		t.Errorf("映射取值异常: %+v", preview.Rows[0].Values)
	}
	if preview.InvalidCount != 1 {
		t.Errorf("bad 应计 1 个异常: %d", preview.InvalidCount)
	}

	// 手动映射覆盖（姓名→公司列）
	preview2, err := a.PreviewWithMapping(data, "", map[string]string{"name": "公司"})
	if err != nil {
		t.Fatalf("PreviewWithMapping manual: %v", err)
	}
	if preview2.Rows[0].Values[0] != "A公司" {
		t.Errorf("手动映射未生效: %+v", preview2.Rows[0].Values)
	}

	// 自定义模板 + 未映射列
	err = a.TemplateCreate("带网址", []template.TemplateColumn{
		{Key: "name", Name: "姓名"}, {Key: "phone", Name: "手机"}, {Key: "website", Name: "网址"},
	})
	if err != nil {
		t.Fatalf("TemplateCreate: %v", err)
	}
	preview3, err := a.PreviewWithMapping(data, "带网址", nil)
	if err != nil {
		t.Fatalf("PreviewWithMapping custom: %v", err)
	}
	if preview3.Rows[0].Values[2] != "" {
		t.Errorf("未映射列应为空: %+v", preview3.Rows[0].Values)
	}
}

// TestExportWithTemplate 模板导出端到端（xlsx 写出成功 + 文件存在）。
func TestExportWithTemplate(t *testing.T) {
	a := testApp(t)
	data := &excel.SheetData{
		SheetName: "s",
		Headers:   []string{"姓名", "手机"},
		Rows:      []map[string]string{{"姓名": "张三", "手机": "13800000000"}},
	}
	out := filepath.Join(t.TempDir(), "out.xlsx")
	if err := a.ExportWithTemplate(data, "", nil, out); err != nil {
		t.Fatalf("ExportWithTemplate: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("导出文件不存在: %v", err)
	}
}

// TestNormalizeExportPath xls 扩展名自动转换为 xlsx（决策：docs/migration/00-overview.md §6.8）。
func TestNormalizeExportPath(t *testing.T) {
	cases := map[string]string{
		"/tmp/out.xls":       "/tmp/out.xlsx",
		"/tmp/OUT.XLS":       "/tmp/OUT.xlsx",
		"/tmp/out.xlsx":      "/tmp/out.xlsx",
		"/tmp/out.csv":       "/tmp/out.csv",
		"/tmp/out.vcf":       "/tmp/out.vcf",
		"/tmp/out.txt":       "/tmp/out.txt",
		"/tmp/无扩展名":          "/tmp/无扩展名",
		"/tmp/通讯录_20260824.xls": "/tmp/通讯录_20260824.xlsx",
	}
	for in, want := range cases {
		if got := normalizeExportPath(in); got != want {
			t.Errorf("normalizeExportPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestExportXlsAutoConvert 用户选 xls 导出时实际写出 xlsx 文件（扩展名纠正端到端）。
func TestExportXlsAutoConvert(t *testing.T) {
	a := testApp(t)
	data := &excel.SheetData{
		SheetName: "s",
		Headers:   []string{"姓名", "手机"},
		Rows:      []map[string]string{{"姓名": "李四", "手机": "13900000000"}},
	}
	out := filepath.Join(t.TempDir(), "out.xls") // 用户输入 .xls
	if err := a.ExportWithTemplate(data, "", nil, out); err != nil {
		t.Fatalf("ExportWithTemplate: %v", err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("不应生成 .xls 文件（应转换为 .xlsx）")
	}
	converted := strings.TrimSuffix(out, ".xls") + ".xlsx"
	if _, err := os.Stat(converted); err != nil {
		t.Errorf("转换后的 .xlsx 不存在: %v", err)
	}
}

// TestSuggestTemplates 多模板按 Matched 降序、零匹配不出现。
func TestSuggestTemplates(t *testing.T) {
	a := testApp(t)
	headers := []string{"姓名", "手机号", "备注"}

	if err := a.TemplateCreate("全匹配", []template.TemplateColumn{
		{Key: "name", Name: "姓名"}, {Key: "phone", Name: "手机"},
	}); err != nil {
		t.Fatalf("TemplateCreate 全匹配: %v", err)
	}
	if err := a.TemplateCreate("部分", []template.TemplateColumn{
		{Key: "name", Name: "姓名"}, {Key: "phone", Name: "手机"},
		{Key: "company", Name: "公司名"}, {Key: "website", Name: "网址"},
	}); err != nil {
		t.Fatalf("TemplateCreate 部分: %v", err)
	}
	if err := a.TemplateCreate("零匹配", []template.TemplateColumn{
		{Key: "company", Name: "公司名"}, {Key: "website", Name: "网址"},
	}); err != nil {
		t.Fatalf("TemplateCreate 零匹配: %v", err)
	}

	suggestions := a.SuggestTemplates(headers)
	if len(suggestions) != 2 {
		t.Fatalf("应有 2 条建议: %+v", suggestions)
	}
	if suggestions[0].Name != "全匹配" || suggestions[0].Matched != 2 {
		t.Errorf("第一名异常: %+v", suggestions[0])
	}
}

// TestApplyTemplate 加载模板并返回覆盖语义的完整映射。
func TestApplyTemplate(t *testing.T) {
	a := testApp(t)
	headers := []string{"联系人", "手机号", "备注"}
	if err := a.TemplateCreateFromHeaders("通讯录", headers); err != nil {
		t.Fatalf("TemplateCreateFromHeaders: %v", err)
	}

	applied, err := a.ApplyTemplate(headers, "通讯录")
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if applied.Mapping["name"] != "联系人" {
		t.Errorf("mapping[name] 应为联系人: %v", applied.Mapping)
	}
	if applied.Mapping["phone"] != "手机号" {
		t.Errorf("mapping[phone] 应为手机号: %v", applied.Mapping)
	}
	if applied.Mapping["custom_3"] != "备注" {
		t.Errorf("mapping[custom_3] 应为备注: %v", applied.Mapping)
	}
	if len(applied.MissingCols) != 0 {
		t.Errorf("MissingCols 应为空: %+v", applied.MissingCols)
	}

	if _, err := a.ApplyTemplate(headers, "不存在"); err == nil {
		t.Error("模板不存在应返回 error")
	}
}

// TestTemplateSetMappings 导出映射可持久化并在 Apply 时优先恢复。
func TestTemplateSetMappings(t *testing.T) {
	a := testApp(t)
	if err := a.TemplateCreate("映射模板", []template.TemplateColumn{
		{Key: "name", Name: "姓名"},
		{Key: "phone", Name: "手机"},
	}); err != nil {
		t.Fatalf("TemplateCreate: %v", err)
	}
	if err := a.TemplateSetMappings("映射模板", map[string]string{
		"name":  "联系人A",
		"phone": "手机号",
	}); err != nil {
		t.Fatalf("TemplateSetMappings: %v", err)
	}
	applied, err := a.ApplyTemplate([]string{"联系人A", "手机号"}, "映射模板")
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if applied.Mapping["name"] != "联系人A" || applied.Mapping["phone"] != "手机号" {
		t.Errorf("应恢复 mappings: %v", applied.Mapping)
	}
}
