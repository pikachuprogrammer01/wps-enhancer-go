package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/xuri/excelize/v2"

	"wps-enhancer-go/internal/app"
	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/template"
)

// testApp 创建隔离 configDir 的 App（管道 E2E 专用）。
func testApp(t *testing.T) *app.App {
	t.Helper()
	return app.NewApp(t.TempDir())
}

// TestPipeline_声明行剔除_四格式导出读回 端到端：读表→模板→匹配→预览→导出→读回。
func TestPipeline_声明行剔除_四格式导出读回(t *testing.T) {
	dir := t.TempDir()
	src := writeSourceXlsx(t, dir)
	a := testApp(t)

	if err := a.SettingsUpdate(map[string]any{
		"phone_merge": true,
		"vcf_fields":  []string{"name", "phone", "company"},
	}); err != nil {
		t.Fatalf("SettingsUpdate: %v", err)
	}

	data, err := a.ReadSheet(src, "客户", true)
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	if !data.DeclarationSkipped {
		t.Error("应剔除声明行")
	}
	wantHeaders := []string{"姓名", "有效手机号", "公司", "官网"}
	if diff := cmp.Diff(wantHeaders, data.Headers); diff != "" {
		t.Errorf("headers mismatch (-want +got):\n%s", diff)
	}

	if err := a.TemplateCreateFromHeaders("企业通讯录", data.Headers); err != nil {
		t.Fatalf("TemplateCreateFromHeaders: %v", err)
	}
	matches, err := a.TemplateMatches(data.Headers, "企业通讯录", nil)
	if err != nil {
		t.Fatalf("TemplateMatches: %v", err)
	}
	wantStatus := map[string]string{
		"name": "exact", "phone": "exact", "company": "exact", "website": "exact",
	}
	gotStatus := make(map[string]string, len(matches))
	for _, m := range matches {
		gotStatus[m.TemplateCol.Key] = m.Status
	}
	if diff := cmp.Diff(wantStatus, gotStatus); diff != "" {
		t.Errorf("match status (-want +got):\n%s", diff)
	}

	preview, err := a.PreviewWithMapping(data, "企业通讯录", nil)
	if err != nil {
		t.Fatalf("PreviewWithMapping: %v", err)
	}
	if len(preview.Rows) != 3 {
		t.Errorf("预览应 3 行（张三 2 + 李四 1）: got %d", len(preview.Rows))
	}
	if preview.InvalidCount != 1 {
		t.Errorf("invalid_count 应为 1: got %d", preview.InvalidCount)
	}

	formats := []string{"xlsx", "csv", "vcf", "txt"}
	for _, fmt := range formats {
		out := filepath.Join(dir, "out."+fmt)
		if err := a.ExportWithTemplate(data, "企业通讯录", nil, out); err != nil {
			t.Fatalf("ExportWithTemplate(%s): %v", fmt, err)
		}
		if _, err := os.Stat(out); err != nil {
			t.Errorf("%s 导出文件不存在: %v", fmt, err)
		}
	}

	// xlsx 读回
	reader, err := excel.GetReader(filepath.Join(dir, "out.xlsx"))
	if err != nil {
		t.Fatalf("GetReader xlsx: %v", err)
	}
	back, err := reader.ReadSheet(filepath.Join(dir, "out.xlsx"), "Sheet1", excel.ReadOptions{})
	if err != nil {
		t.Fatalf("ReadSheet xlsx: %v", err)
	}
	if diff := cmp.Diff(wantHeaders, back.Headers); diff != "" {
		t.Errorf("xlsx 读回 headers (-want +got):\n%s", diff)
	}
	if len(back.Rows) != 3 {
		t.Errorf("xlsx 读回应 3 行: got %d", len(back.Rows))
	}

	// csv UTF-8 BOM
	csvBytes, err := os.ReadFile(filepath.Join(dir, "out.csv"))
	if err != nil {
		t.Fatalf("Read csv: %v", err)
	}
	csvText := strings.TrimPrefix(string(csvBytes), "\ufeff")
	if !strings.Contains(csvText, "张三") {
		t.Errorf("csv 应含张三: %q", csvText[:min(80, len(csvText))])
	}
	if !strings.HasPrefix(csvText, "姓名") {
		t.Error("csv 首行应为表头")
	}

	// txt 空格分隔表头
	txtBytes, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatalf("Read txt: %v", err)
	}
	txtText := strings.TrimPrefix(string(txtBytes), "\ufeff")
	txtLines := strings.Split(txtText, "\n")
	if txtLines[0] != "姓名 有效手机号 公司 官网" {
		t.Errorf("txt 表头行异常: %q", txtLines[0])
	}
	if !strings.Contains(txtText, "张三 13800000000 A公司 http://a.com") {
		t.Error("txt 应含张三首行数据")
	}

	// vcf 字段过滤 + 拆分行
	vcfText, err := os.ReadFile(filepath.Join(dir, "out.vcf"))
	if err != nil {
		t.Fatalf("Read vcf: %v", err)
	}
	vcf := string(vcfText)
	if !strings.Contains(vcf, "TEL;TYPE=CELL:13800000000") {
		t.Error("vcf 应含 13800000000")
	}
	if !strings.Contains(vcf, "TEL;TYPE=CELL:13900000000") {
		t.Error("vcf 应含 13900000000")
	}
	if strings.Contains(vcf, "URL:") {
		t.Error("vcf 不应含 URL（vcf_fields 未含 website）")
	}
	if strings.Count(vcf, "BEGIN:VCARD") != 3 {
		t.Errorf("vcf 应有 3 张卡片: got %d", strings.Count(vcf, "BEGIN:VCARD"))
	}
}

// TestPipeline_模板SuggestApply 读表后 Suggest/Apply 覆盖映射语义。
func TestPipeline_模板SuggestApply(t *testing.T) {
	dir := t.TempDir()
	src := writeSourceXlsx(t, dir)
	a := testApp(t)

	data, err := a.ReadSheet(src, "客户", true)
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	if err := a.TemplateCreateFromHeaders("企业通讯录", data.Headers); err != nil {
		t.Fatalf("TemplateCreateFromHeaders: %v", err)
	}

	suggestions := a.SuggestTemplates(data.Headers)
	if len(suggestions) == 0 {
		t.Fatal("应有可用模板建议")
	}
	if suggestions[0].Name != "企业通讯录" {
		t.Errorf("最佳匹配应为 企业通讯录: got %s", suggestions[0].Name)
	}
	if suggestions[0].Matched != 4 {
		t.Errorf("Matched 应为 4: got %d", suggestions[0].Matched)
	}

	applied, err := a.ApplyTemplate(data.Headers, "企业通讯录")
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if applied.Name != "企业通讯录" {
		t.Errorf("Applied.Name 异常: %s", applied.Name)
	}
	if len(applied.MissingCols) != 0 {
		t.Errorf("全匹配模板 MissingCols 应为空: %+v", applied.MissingCols)
	}
	if applied.Mapping["name"] != "姓名" {
		t.Errorf("mapping[name] 异常: %v", applied.Mapping)
	}
}

// TestPipeline_合并单元格 合并开启时 xlsx 输出含合并区域。
func TestPipeline_合并单元格(t *testing.T) {
	dir := t.TempDir()
	src := writeSourceXlsx(t, dir)
	a := testApp(t)

	if err := a.SettingsUpdate(map[string]any{"phone_merge": true}); err != nil {
		t.Fatalf("SettingsUpdate: %v", err)
	}

	data, err := a.ReadSheet(src, "客户", true)
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	if err := a.TemplateCreateFromHeaders("企业通讯录", data.Headers); err != nil {
		t.Fatalf("TemplateCreateFromHeaders: %v", err)
	}

	out := filepath.Join(dir, "merged.xlsx")
	if err := a.ExportWithTemplate(data, "企业通讯录", nil, out); err != nil {
		t.Fatalf("ExportWithTemplate: %v", err)
	}

	f, err := excelize.OpenFile(out)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer f.Close()
	merges, err := f.GetMergeCells("Sheet1")
	if err != nil {
		t.Fatalf("GetMergeCells: %v", err)
	}
	if len(merges) != 1 {
		t.Errorf("合并区域应有 1 个: got %d", len(merges))
	}
}

// TestPipeline_会话列导出读回 不落盘模板：SessionMatch → PreviewWithColumns → ExportWithColumns。
func TestPipeline_会话列导出读回(t *testing.T) {
	dir := t.TempDir()
	src := writeSourceXlsx(t, dir)
	a := testApp(t)

	data, err := a.ReadSheet(src, "客户", true)
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	tmplCols := []template.TemplateColumn{
		{Key: "name", Name: "姓名", Enabled: true},
		{Key: "phone", Name: "手机", Enabled: true},
		{Key: "company", Name: "公司", Enabled: true},
	}
	matches, err := a.SessionMatch(data.Headers, tmplCols, nil)
	if err != nil {
		t.Fatalf("SessionMatch: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("应 3 列匹配: %d", len(matches))
	}
	status := map[string]string{}
	for _, m := range matches {
		status[m.TemplateCol.Key] = m.Status
	}
	if status["name"] != "exact" {
		t.Errorf("name 应为 exact: %v", status)
	}
	if status["phone"] != "alias" && status["phone"] != "exact" {
		t.Errorf("phone 应为 alias/exact: %v", status)
	}
	if status["company"] != "exact" && status["company"] != "alias" {
		t.Errorf("company 应为 exact/alias: %v", status)
	}

	preview, err := a.PreviewWithColumns(data, tmplCols, nil)
	if err != nil {
		t.Fatalf("PreviewWithColumns: %v", err)
	}
	// 张三双号拆 2 + 李四 1
	if len(preview.Rows) != 3 {
		t.Errorf("预览应 3 行: got %d", len(preview.Rows))
	}
	if len(preview.Rows[0].Values) != 3 {
		t.Errorf("会话 3 列 Values 长应为 3: %d", len(preview.Rows[0].Values))
	}
	if preview.InvalidCount != 1 {
		t.Errorf("invalid_count 应为 1: got %d", preview.InvalidCount)
	}

	out := filepath.Join(dir, "session_out.xlsx")
	if err := a.ExportWithColumns(data, tmplCols, nil, out); err != nil {
		t.Fatalf("ExportWithColumns: %v", err)
	}
	if len(a.TemplateList()) != 0 {
		t.Error("会话导出不应产生磁盘模板")
	}
	reader, err := excel.GetReader(out)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	back, err := reader.ReadSheet(out, "Sheet1", excel.ReadOptions{})
	if err != nil {
		t.Fatalf("读回: %v", err)
	}
	wantHeaders := []string{"姓名", "手机", "公司"}
	if diff := cmp.Diff(wantHeaders, back.Headers); diff != "" {
		t.Errorf("读回表头 (-want +got):\n%s", diff)
	}
	if len(back.Rows) < 1 || back.Rows[0]["姓名"] != "张三" {
		t.Errorf("读回数据异常: %+v", back.Rows)
	}
}
