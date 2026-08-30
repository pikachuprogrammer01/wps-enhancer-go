package template

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSanitizeFilename_非法字符 非法字符替换为下划线。
func TestSanitizeFilename_非法字符(t *testing.T) {
	cases := map[string]string{
		"正常":          "正常",
		`a/b:c*?"<>|`: "a_b_c______",
		"  ":          "_",
		"":            "_",
	}
	for in, want := range cases {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q)=%q want %q", in, got, want)
		}
	}
}

// TestSaveLoadRoundTrip 模板原子写入后可读回。
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "通讯录.json")
	sf := "excel"
	tmpl := &Template{
		Name:         "通讯录",
		Columns:      []TemplateColumn{{Key: "name", Name: "姓名", Enabled: true}},
		Mappings:     map[string]string{"name": "联系人"},
		SourceFormat: &sf,
	}
	if err := SaveTemplate(tmpl, path); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	got, err := LoadTemplate(path)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if got.Name != "通讯录" || len(got.Columns) != 1 || got.Mappings["name"] != "联系人" {
		t.Errorf("回读异常: %+v", got)
	}
	if got.SourceFormat == nil || *got.SourceFormat != "excel" {
		t.Errorf("SourceFormat 丢失: %+v", got.SourceFormat)
	}
}

// TestListTemplates_跳过损坏 损坏文件不影响其他模板。
func TestListTemplates_跳过损坏(t *testing.T) {
	dir := t.TempDir()
	ok := &Template{Name: "好模板", Columns: []TemplateColumn{{Key: "phone", Name: "手机", Enabled: true}}}
	if err := SaveTemplate(ok, filepath.Join(dir, "好模板.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "坏.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := ListTemplates(dir)
	if len(list) != 1 || list[0].Name != "好模板" {
		t.Errorf("应仅返回好模板: %+v", list)
	}
	if len(ListTemplates(filepath.Join(dir, "不存在"))) != 0 {
		t.Error("目录不存在应返回空列表")
	}
}

// TestDeleteTemplate_幂等 不存在也成功。
func TestDeleteTemplate_幂等(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	if err := DeleteTemplate(path); err != nil {
		t.Fatalf("不存在应幂等: %v", err)
	}
	tmpl := &Template{Name: "x", Columns: []TemplateColumn{{Key: "name", Name: "姓名", Enabled: true}}}
	if err := SaveTemplate(tmpl, path); err != nil {
		t.Fatal(err)
	}
	if err := DeleteTemplate(path); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("文件应已删除")
	}
}
