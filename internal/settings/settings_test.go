package settings

import (
	"os"
	"path/filepath"
	"testing"

	"wps-enhancer-go/internal/template"
)

// TestLoadRealSettingsFile 兼容验证：读取 Python 版生成的真实 settings.json 夹具。
func TestLoadRealSettingsFile(t *testing.T) {
	st, err := Load("../../docs/migration/testdata/settings_real.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// 文件存在且字段可读（字段值以 Python 版为准，这里只验证不崩溃 + 默认结构完整）
	if st.PhoneSeparators == nil || len(st.PhoneSeparators) == 0 {
		t.Error("phone_separators 应为非空")
	}
	if st.VCFFields == nil || len(st.VCFFields) == 0 {
		t.Error("vcf_fields 应为非空")
	}
	t.Logf("读取成功: phone_validate=%v csv_encoding=%s", st.PhoneValidate, st.CSVEncoding)
}

// TestSaveLoadRoundTrip 验证设置保存→读取一致。
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	st := New()
	st.PhoneMerge = true
	st.CSVEncoding = "gbk"
	st.TxtSeparator = "、"
	st.VCFNamePrefix = "客户-"
	if err := Save(path, st); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.PhoneMerge != true || got.CSVEncoding != "gbk" || got.TxtSeparator != "、" || got.VCFNamePrefix != "客户-" {
		t.Errorf("round-trip 不一致: %+v", got)
	}
}

// TestLoadCorruptFallback 验证损坏文件回退默认值（不崩溃）。
func TestLoadCorruptFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	_ = os.WriteFile(path, []byte("{corrupt json"), 0o644)
	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !st.PhoneValidate {
		t.Error("损坏文件应回退默认值")
	}
}

// TestTemplateStoreRoundTrip 模板保存→读取→列表→删除全流程。
func TestTemplateStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	tmpl := &template.Template{
		Name:    "通讯录模板",
		Columns: []template.TemplateColumn{{Key: "name", Name: "姓名", Enabled: true}, {Key: "phone", Name: "手机"}},
		Mappings: map[string]string{"name": "联系人"},
	}
	path := filepath.Join(dir, template.SanitizeFilename(tmpl.Name)+".json")
	if err := template.SaveTemplate(tmpl, path); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	got, err := template.LoadTemplate(path)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if got.Name != "通讯录模板" || len(got.Columns) != 2 || got.Mappings["name"] != "联系人" {
		t.Errorf("模板 round-trip 不一致: %+v", got)
	}
	// 损坏模板不阻塞列表
	_ = os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("bad"), 0o644)
	all := template.ListTemplates(dir)
	if len(all) != 1 || all[0].Name != "通讯录模板" {
		t.Errorf("ListTemplates 容错异常: %d 个", len(all))
	}
	if err := template.DeleteTemplate(path); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("删除后文件应不存在")
	}
	// 模板名安全化
	if template.SanitizeFilename(`a/b:c*d?`) != "a_b_c_d_" {
		t.Errorf("SanitizeFilename 异常: %q", template.SanitizeFilename(`a/b:c*d?`))
	}
}

// TestLoadMissingFile_默认值 缺失文件回退 New()。
func TestLoadMissingFile_默认值(t *testing.T) {
	st, err := Load(filepath.Join(t.TempDir(), "no-such.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !st.VCFShowImportGuide || st.DownloadDir == "" || st.InstallDir == "" {
		t.Errorf("默认值不完整: %+v", st)
	}
	if st.UpdateURL != DefaultUpdateURL {
		t.Errorf("UpdateURL 默认异常: %s", st.UpdateURL)
	}
}

// TestLoadEmptyUpdateURL_回退默认 空 update_url 视为未设置。
func TestLoadEmptyUpdateURL_回退默认(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	body := `{"app_settings":{"update_url":"","vcf_show_import_guide":false}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.UpdateURL != DefaultUpdateURL {
		t.Errorf("空 update_url 应回退默认: %s", st.UpdateURL)
	}
	if st.VCFShowImportGuide {
		t.Error("vcf_show_import_guide=false 应保留")
	}
}

// TestVCFPropMap_自定义字段 仅非核心列且有 VCFProp 进入 map。
func TestVCFPropMap_自定义字段(t *testing.T) {
	if New().VCFPropMap() != nil {
		t.Error("默认内置列不应产生自定义 VCFPropMap")
	}
	st := New()
	st.BuiltinColumns = append(st.BuiltinColumns, template.BuiltinColumn{
		Key: "email", Label: "邮箱", VCFProp: "EMAIL",
	})
	m := st.VCFPropMap()
	if m == nil || m["email"] != "EMAIL" {
		t.Errorf("VCFPropMap 异常: %v", m)
	}
	if (*AppSettings)(nil).VCFPropMap() != nil {
		t.Error("nil receiver 应返回 nil")
	}
}
