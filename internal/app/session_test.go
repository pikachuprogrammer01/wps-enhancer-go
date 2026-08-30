package app

import (
	"path/filepath"
	"testing"

	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/template"
)

// sessionSheet 构造会话列测试用源表。
func sessionSheet() *excel.SheetData {
	return &excel.SheetData{
		SheetName: "s",
		Headers:   []string{"姓名", "手机号", "公司"},
		Rows: []map[string]string{
			{"姓名": "张三", "手机号": "13800000000", "公司": "A公司"},
			{"姓名": "李四", "手机号": "bad", "公司": "B公司"},
		},
	}
}

// sessionCols 构造仅含姓名/手机的会话列（不含公司，验证不读磁盘模板）。
func sessionCols() []template.TemplateColumn {
	return []template.TemplateColumn{
		{Key: "name", Name: "姓名", Enabled: true},
		{Key: "phone", Name: "手机", Enabled: true},
	}
}

// TestSessionMatch_列编辑后重匹配 会话列匹配不依赖磁盘模板。
func TestSessionMatch_列编辑后重匹配(t *testing.T) {
	a := testApp(t)
	headers := []string{"姓名", "手机号", "公司"}
	matches, err := a.SessionMatch(headers, sessionCols(), nil)
	if err != nil {
		t.Fatalf("SessionMatch: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("应匹配 2 列: %d", len(matches))
	}
	got := map[string]string{}
	for _, m := range matches {
		got[m.TemplateCol.Key] = m.Status
	}
	phoneOK := got["phone"] == "alias" || got["phone"] == "exact"
	if got["name"] != "exact" || !phoneOK {
		t.Errorf("匹配状态异常: %+v", got)
	}

	if _, err := a.SessionMatch(headers, nil, nil); err == nil {
		t.Error("空列应拒绝")
	}

	matches2, err := a.SessionMatch(headers, sessionCols(), map[string]string{"name": "公司"})
	if err != nil {
		t.Fatalf("SessionMatch manual: %v", err)
	}
	if matches2[0].Status != "manual" || matches2[0].SourceCol == nil || *matches2[0].SourceCol != "公司" {
		t.Errorf("手动映射未生效: %+v", matches2[0])
	}
}

// TestPreviewWithColumns_不读磁盘模板 会话预览仅用传入列。
func TestPreviewWithColumns_不读磁盘模板(t *testing.T) {
	a := testApp(t)
	// 种冲突磁盘模板：若误读磁盘，列数会变成 3
	if err := a.TemplateCreate("磁盘模板", []template.TemplateColumn{
		{Key: "name", Name: "姓名", Enabled: true},
		{Key: "phone", Name: "手机", Enabled: true},
		{Key: "company", Name: "公司", Enabled: true},
	}); err != nil {
		t.Fatalf("TemplateCreate: %v", err)
	}

	preview, err := a.PreviewWithColumns(sessionSheet(), sessionCols(), nil)
	if err != nil {
		t.Fatalf("PreviewWithColumns: %v", err)
	}
	if len(preview.Rows) != 2 {
		t.Errorf("应 2 行: %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Values) != 2 {
		t.Errorf("会话仅 2 列，Values 长应为 2: %d", len(preview.Rows[0].Values))
	}
	if preview.InvalidCount != 1 {
		t.Errorf("bad 应计 1: %d", preview.InvalidCount)
	}
}

// TestExportWithColumns_写出成功 会话列导出不创建磁盘模板，读回仅两列。
func TestExportWithColumns_写出成功(t *testing.T) {
	a := testApp(t)
	out := filepath.Join(t.TempDir(), "session.xlsx")
	if err := a.ExportWithColumns(sessionSheet(), sessionCols(), nil, out); err != nil {
		t.Fatalf("ExportWithColumns: %v", err)
	}
	if len(a.TemplateList()) != 0 {
		t.Error("会话导出不应写入磁盘模板")
	}
	reader, err := excel.GetReader(out)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	back, err := reader.ReadSheet(out, "Sheet1", excel.ReadOptions{})
	if err != nil {
		t.Fatalf("读回: %v", err)
	}
	if len(back.Headers) != 2 || back.Headers[0] != "姓名" || back.Headers[1] != "手机" {
		t.Errorf("读回表头应为 [姓名 手机]: %v", back.Headers)
	}
	if len(back.Rows) < 1 || back.Rows[0]["姓名"] != "张三" {
		t.Errorf("读回数据异常: %+v", back.Rows)
	}
}

// TestDetectTruncatedNumbers_App层 命令层委托 core 检测。
func TestDetectTruncatedNumbers_App层(t *testing.T) {
	a := testApp(t)
	data := &excel.SheetData{
		Headers: []string{"手机"},
		Rows:    []map[string]string{{"手机": "1.38E+10"}},
	}
	hints := a.DetectTruncatedNumbers(data)
	if len(hints) == 0 {
		t.Error("科学计数法应有提示")
	}
	if a.DetectTruncatedNumbers(nil) != nil {
		t.Error("nil 应返回 nil")
	}
}
