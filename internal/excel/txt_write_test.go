package excel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTxtWriteRoundTrip txt 按分隔符写出后内容可读。
func TestTxtWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	req := &WriteRequest{
		FilePath:  path,
		Headers:   []string{"姓名", "手机"},
		DataRows:  [][]string{{"张三", "13800000000"}},
		Separator: "\t",
		Encoding:  "utf-8",
	}
	w := &TxtWriter{}
	if err := w.WriteExport(req); err != nil {
		t.Fatalf("WriteExport: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "姓名\t手机") {
		t.Errorf("表头应按 tab 分隔: %q", text)
	}
	if !strings.Contains(text, "张三\t13800000000") {
		t.Errorf("数据行应按 tab 分隔: %q", text)
	}
}

// TestVCFWriteExport_写出文件 BuildVCFLines 写入磁盘成功。
func TestVCFWriteExport_写出文件(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.vcf")
	req := &WriteRequest{
		FilePath:  path,
		Headers:   []string{"姓名", "手机", "备注"},
		FieldKeys: []string{"name", "phone", "custom_1"},
		VCFFields: []string{"name", "phone", "custom_1"},
		VCFProps:  map[string]string{"custom_1": "NOTE"},
		DataRows:  [][]string{{"王五", "13700000000", "VIP"}},
	}
	w := &VcfWriter{}
	if err := w.WriteExport(req); err != nil {
		t.Fatalf("WriteExport: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "BEGIN:VCARD") || !strings.Contains(text, "NOTE:VIP") {
		t.Errorf("vcf 内容异常: %q", text)
	}
}
