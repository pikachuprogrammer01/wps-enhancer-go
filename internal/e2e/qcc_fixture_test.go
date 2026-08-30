package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"wps-enhancer-go/internal/app"
	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/settings"
)

// appReadRowCap App.ReadSheet 预览读入的最大行数（与 internal/app/app.go 一致）。
const appReadRowCap = 200

// qccRoot 企查查夹具根目录（相对 internal/e2e 测试包）。
const qccRoot = "testdata/qcc"

// qccManifest 夹具清单（manifest.json）。
type qccManifest struct {
	Version int       `json:"version"`
	Cases   []qccCase `json:"cases"`
}

// qccCase 单个企查查源表用例。
type qccCase struct {
	ID              string    `json:"id"`
	File            string    `json:"file"`
	Description     string    `json:"description"`
	Sheet           string    `json:"sheet"`
	SkipDeclaration bool      `json:"skip_declaration"`
	Expect          qccExpect `json:"expect"`
}

// qccExpect 可断言的期望（ready=false 时该 case 在 Phase 测试中 Skip）。
type qccExpect struct {
	Ready              bool     `json:"ready"`
	DeclarationSkipped bool     `json:"declaration_skipped"`
	Headers            []string `json:"headers"`
	RowCount           int      `json:"row_count"`            // App.ReadSheet 路径（≤ appReadRowCap）
	SourceRowCount     int      `json:"source_row_count"`     // excel 直读全量行数（0=与 row_count 相同）
	SuggestMin         int      `json:"suggest_min"`
	PreviewRows        int      `json:"preview_rows"`
	InvalidPhones      int      `json:"invalid_phones"`
	ExportFormats      []string `json:"export_formats"`
}

// qccDumpEntry Bootstrap 探测输出的一条摘要。
type qccDumpEntry struct {
	ID                 string   `json:"id"`
	File               string   `json:"file"`
	Sheet              string   `json:"sheet"`
	Sheets             []string `json:"sheets"`
	DeclarationSkipped bool     `json:"declaration_skipped"`
	Headers            []string `json:"headers"`
	RowCount           int      `json:"row_count"`
	SourceRowCount     int      `json:"source_row_count"`
	PreviewRows        int      `json:"preview_rows"`
	InvalidPhones      int      `json:"invalid_phones"`
	SuggestMin         int      `json:"suggest_min"`
	SuggestCount       int      `json:"suggest_count"`
	FileExists         bool     `json:"file_exists"`
	ManifestReady      bool     `json:"manifest_ready"`
	ReadCapped         bool     `json:"read_capped"`
}

// loadQCCManifest 加载 manifest.json。
func loadQCCManifest(t *testing.T) *qccManifest {
	t.Helper()
	path := filepath.Join(qccRoot, "manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s: %v", path, err)
	}
	var m qccManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解析 manifest: %v", err)
	}
	return &m
}

// qccFilePath 返回 case 源文件的相对路径（相对 internal/e2e 包目录）。
func qccFilePath(c qccCase) string {
	return filepath.Join(qccRoot, c.File)
}

// requireQCCFile 源文件必须存在，否则 Skip（便于 CI 在夹具未提交时不失败）。
func requireQCCFile(t *testing.T, c qccCase) string {
	t.Helper()
	path := qccFilePath(c)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("夹具未放入：%s（请将表格放到 internal/e2e/testdata/qcc/%s）", path, c.File)
	}
	return path
}

// requireQCCReady manifest 期望已填写（ready=true），否则 Skip。
func requireQCCReady(t *testing.T, c qccCase) {
	t.Helper()
	if !c.Expect.Ready {
		t.Skipf("%s: manifest expect.ready=false，请先运行 QCC_FIXTURE_DUMP=1 探测并填写 manifest", c.ID)
	}
}

// resolveQCCSheet 解析目标 Sheet（manifest 为空则用第一个）。
func resolveQCCSheet(t *testing.T, path string, c qccCase) string {
	t.Helper()
	if c.Sheet != "" {
		return c.Sheet
	}
	reader, err := excel.GetReader(path)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	names, err := reader.GetSheetNames(path)
	if err != nil {
		t.Fatalf("GetSheetNames: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("文件无 Sheet")
	}
	return names[0]
}

// readQCCCase 通过 App.ReadSheet 读入 case（与生产路径一致，最多 appReadRowCap 行）。
func readQCCCase(t *testing.T, a *app.App, c qccCase) (*excel.SheetData, string) {
	t.Helper()
	path := requireQCCFile(t, c)
	sheet := resolveQCCSheet(t, path, c)
	data, err := a.ReadSheet(path, sheet, c.SkipDeclaration)
	if err != nil {
		t.Fatalf("%s ReadSheet(%q): %v", c.ID, sheet, err)
	}
	return data, sheet
}

// readQCCSourceFull 经 excel 直读全量行数（声明行剔除规则与 App 默认设置一致）。
func readQCCSourceFull(t *testing.T, c qccCase) (int, string) {
	t.Helper()
	path := qccFilePath(c)
	sheet := resolveQCCSheet(t, path, c)
	st := settings.New()
	reader, err := excel.GetReader(path)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	data, err := reader.ReadSheet(path, sheet, excel.ReadOptions{
		SkipDeclaration:     c.SkipDeclaration && st.DeclarationDetect,
		DeclarationKeywords: st.DeclarationKeywords,
	})
	if err != nil {
		t.Fatalf("%s ReadSheet full: %v", c.ID, err)
	}
	return len(data.Rows), sheet
}

// setupQCCCase 读表并建模板（Phase 2/3 共享 setup，避免子测试隐式顺序依赖）。
func setupQCCCase(t *testing.T, a *app.App, c qccCase) (*excel.SheetData, string, string) {
	t.Helper()
	data, sheet := readQCCCase(t, a, c)
	tmplName := "e2e-" + c.ID
	if err := a.TemplateCreateFromHeaders(tmplName, data.Headers); err != nil {
		t.Fatalf("TemplateCreateFromHeaders: %v", err)
	}
	return data, sheet, tmplName
}

// expectSourceRows 返回 manifest 中的全量源行数期望（未填则等于 row_count）。
func expectSourceRows(c qccCase) int {
	if c.Expect.SourceRowCount > 0 {
		return c.Expect.SourceRowCount
	}
	return c.Expect.RowCount
}

// requireReader 获取 Reader 实例（Bootstrap 用）。
func requireReader(t *testing.T, path string) (excel.Reader, error) {
	t.Helper()
	return excel.GetReader(path)
}
