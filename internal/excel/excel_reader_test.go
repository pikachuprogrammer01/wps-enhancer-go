package excel

import (
	"path/filepath"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// fixturePath 返回读取夹具目录路径（由 docs/migration/testdata/gen_read_fixtures.py 生成）。
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return "../../docs/migration/testdata/read_fixtures/" + name
}

// loadBaseline 加载 Python 版读取基准 JSON。
func loadBaseline(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, "read_baseline.json"))
	if err != nil {
		t.Fatalf("读取基准失败: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("解析基准失败: %v", err)
	}
	return out
}

// TestXlsxRead 对照验证 xlsx 读取（Sheet 名/行数/单元格，含数字手机号转换）。
func TestXlsxRead(t *testing.T) {
	baseline := loadBaseline(t)
	reader, err := GetReader(fixturePath(t, "sample.xlsx"))
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}

	names, err := reader.GetSheetNames(fixturePath(t, "sample.xlsx"))
	if err != nil {
		t.Fatalf("GetSheetNames: %v", err)
	}
	if !reflect.DeepEqual(normalizeJSON(names), baseline["xlsx_sheetnames"]) {
		t.Errorf("sheetnames 不一致: got %v want %v", names, baseline["xlsx_sheetnames"])
	}

	summaries, err := reader.GetSheetSummaries(fixturePath(t, "sample.xlsx"))
	if err != nil {
		t.Fatalf("GetSheetSummaries: %v", err)
	}
	wantSummaries := []any{map[string]any{"name": "通讯录", "rows": 4.0}, map[string]any{"name": "说明", "rows": 2.0}}
	if !reflect.DeepEqual(normalizeJSON(summaries), wantSummaries) {
		t.Errorf("summaries 不一致: got %v want %v", summaries, wantSummaries)
	}

	data, err := reader.ReadSheet(fixturePath(t, "sample.xlsx"), "通讯录", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	got := normalizeJSON(map[string]any{
		"sheet_name": data.SheetName, "headers": data.Headers,
		"rows": data.Rows, "declaration_skipped": data.DeclarationSkipped,
	})
	want := normalizeJSON(baseline["xlsx_read"])
	if !reflect.DeepEqual(got, want) {
		gotB, _ := json.MarshalIndent(got, "", "  ")
		wantB, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("xlsx 读取与 Python 基准不一致\n--- got ---\n%s\n--- want ---\n%s", gotB, wantB)
	}
	// 数字手机号必须原样输出（精度关键）
	if data.Rows[0]["手机号"] != "13800000000" {
		t.Errorf("数字手机号转换错误: got %q", data.Rows[0]["手机号"])
	}
}

// TestCSVRead 对照验证 csv 读取（编码检测/分隔符/引号/空字段）。
func TestCSVRead(t *testing.T) {
	baseline := loadBaseline(t)
	reader, err := GetReader(fixturePath(t, "sample.csv"))
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	data, err := reader.ReadSheet(fixturePath(t, "sample.csv"), "sample", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	got := normalizeJSON(map[string]any{
		"sheet_name": data.SheetName, "headers": data.Headers,
		"rows": data.Rows, "declaration_skipped": data.DeclarationSkipped,
	})
	want := normalizeJSON(baseline["csv_read"])
	if !reflect.DeepEqual(got, want) {
		gotB, _ := json.MarshalIndent(got, "", "  ")
		wantB, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("csv 读取与 Python 基准不一致\n--- got ---\n%s\n--- want ---\n%s", gotB, wantB)
	}
}

// TestXlsRead 对照验证 xls 读取（数字手机号/中文/防护层）。
func TestXlsRead(t *testing.T) {
	baseline := loadBaseline(t)
	reader, err := GetReader(fixturePath(t, "sample.xls"))
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	names, err := reader.GetSheetNames(fixturePath(t, "sample.xls"))
	if err != nil {
		t.Fatalf("GetSheetNames: %v", err)
	}
	if !reflect.DeepEqual(normalizeJSON(names), baseline["xls_sheetnames"]) {
		t.Errorf("xls sheetnames 不一致: got %v want %v", names, baseline["xls_sheetnames"])
	}
	data, err := reader.ReadSheet(fixturePath(t, "sample.xls"), "通讯录", ReadOptions{})
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	got := normalizeJSON(map[string]any{
		"sheet_name": data.SheetName, "headers": data.Headers,
		"rows": data.Rows, "declaration_skipped": data.DeclarationSkipped,
	})
	want := normalizeJSON(baseline["xls_read"])
	if !reflect.DeepEqual(got, want) {
		gotB, _ := json.MarshalIndent(got, "", "  ")
		wantB, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("xls 读取与 Python 基准不一致\n--- got ---\n%s\n--- want ---\n%s", gotB, wantB)
	}
}

// TestWriteRoundTrip 验证写入→读回自洽（xlsx 合并/标红 + csv BOM 编码）。
func TestWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	red := "#FF0000"

	// xlsx：表头 + 数据 + 合并 + 标红
	req := &WriteRequest{
		FilePath: dir + "/out.xlsx",
		Headers:  []string{"姓名", "手机"},
		DataRows: [][]string{{"张三", "13800000000"}, {"", "13900000000"}, {"李四", "bad"}},
		MergeRanges: []MergeRange{{RowStart: 0, RowEnd: 1, ColIndex: 0}},
		CellStyles:  map[string]CellStyle{"(2, 1)": {BackgroundColor: &red}},
	}
	writer, err := GetWriter(req.FilePath)
	if err != nil {
		t.Fatalf("GetWriter: %v", err)
	}
	if err := writer.WriteExport(req); err != nil {
		t.Fatalf("WriteExport xlsx: %v", err)
	}
	reader, _ := GetReader(req.FilePath)
	data, err := reader.ReadSheet(req.FilePath, "Sheet1", ReadOptions{})
	if err != nil {
		t.Fatalf("回读 xlsx: %v", err)
	}
	if len(data.Rows) != 3 || data.Headers[0] != "姓名" {
		t.Errorf("xlsx 回读异常: headers=%v rows=%d", data.Headers, len(data.Rows))
	}

	// csv：BOM + 编码
	csvReq := &WriteRequest{
		FilePath: dir + "/out.csv",
		Headers:  []string{"姓名", "手机"},
		DataRows: [][]string{{"张三", "13800000000"}},
		Encoding: "utf-8-bom",
	}
	writer, err = GetWriter(csvReq.FilePath)
	if err != nil {
		t.Fatalf("GetWriter csv: %v", err)
	}
	if err := writer.WriteExport(csvReq); err != nil {
		t.Fatalf("WriteExport csv: %v", err)
	}
	raw, _ := os.ReadFile(csvReq.FilePath)
	if raw[0] != 0xEF || raw[1] != 0xBB || raw[2] != 0xBF {
		t.Error("csv 缺少 UTF-8 BOM")
	}
	reader, _ = GetReader(csvReq.FilePath)
	data, err = reader.ReadSheet(csvReq.FilePath, "out", ReadOptions{})
	if err != nil {
		t.Fatalf("回读 csv: %v", err)
	}
	if len(data.Rows) != 1 || data.Rows[0]["手机"] != "13800000000" {
		t.Errorf("csv 回读异常: %v", data.Rows)
	}
}

// normalizeJSON 序列化→反序列化（消除 int/float 差异）。
func normalizeJSON(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// TestCSVCustomerSeparator 自定义单字符分隔符（含多字节字符如「、」）按设置生效。
func TestCSVCustomerSeparator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.csv")
	content := "姓名、手机\n张三、13800000000\n李四、13900000001\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &CsvReader{}
	data, err := r.ReadSheet(path, "", ReadOptions{Separator: "、"})
	if err != nil {
		t.Fatalf("ReadSheet: %v", err)
	}
	if len(data.Headers) != 2 || data.Headers[0] != "姓名" || data.Headers[1] != "手机" {
		t.Errorf("表头错误: %v", data.Headers)
	}
	if len(data.Rows) != 2 || data.Rows[0]["手机"] != "13800000000" {
		t.Errorf("数据行错误: %v", data.Rows)
	}

	// 多字符分隔符应报明确错误
	if _, err := r.ReadSheet(path, "", ReadOptions{Separator: "、、"}); err == nil {
		t.Error("多字符分隔符应报错")
	}
}
