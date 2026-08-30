package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wps-enhancer-go/internal/excel"
)

// TestQCC_Phase3_Export 阶段 3：四格式导出并读回（依赖 Phase 2 manifest）。
func TestQCC_Phase3_Export(t *testing.T) {
	m := loadQCCManifest(t)
	a := testApp(t)

	for _, c := range m.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			requireQCCFile(t, c)
			requireQCCReady(t, c)

			data, _, tmplName := setupQCCCase(t, a, c)

			dir := t.TempDir()
			for _, fmt := range c.Expect.ExportFormats {
				fmt := fmt
				t.Run("export_"+fmt, func(t *testing.T) {
					out := filepath.Join(dir, c.ID+"."+fmt)
					if err := a.ExportWithTemplate(data, tmplName, nil, out); err != nil {
						t.Fatalf("ExportWithTemplate(%s): %v", fmt, err)
					}
					assertExportReadable(t, out, fmt, c.Expect.PreviewRows)
				})
			}
		})
	}
}

// assertExportReadable 导出文件存在且读回行数与 preview_rows 一致。
func assertExportReadable(t *testing.T, path, format string, wantRows int) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("导出文件不存在 %s: %v", path, err)
	}

	switch format {
	case "xlsx":
		actualPath := path
		if strings.EqualFold(filepath.Ext(path), ".xls") {
			actualPath = strings.TrimSuffix(path, filepath.Ext(path)) + ".xlsx"
		}
		reader, err := excel.GetReader(actualPath)
		if err != nil {
			t.Fatalf("GetReader: %v", err)
		}
		names, err := reader.GetSheetNames(actualPath)
		if err != nil || len(names) == 0 {
			t.Fatalf("GetSheetNames: %v", err)
		}
		back, err := reader.ReadSheet(actualPath, names[0], excel.ReadOptions{})
		if err != nil {
			t.Fatalf("ReadSheet: %v", err)
		}
		if len(back.Rows) != wantRows {
			t.Errorf("xlsx 读回行数: got %d want %d", len(back.Rows), wantRows)
		}
	case "csv", "txt":
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		text := strings.TrimPrefix(string(raw), "\ufeff")
		dataLines := exportDataLineCount(text)
		if dataLines != wantRows {
			t.Errorf("%s 数据行数: got %d want %d", format, dataLines, wantRows)
		}
	case "vcf":
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		count := strings.Count(string(raw), "BEGIN:VCARD")
		if count != wantRows {
			t.Errorf("vcf 卡片数: got %d want %d", count, wantRows)
		}
	default:
		t.Fatalf("未知格式: %s", format)
	}
}

// exportDataLineCount 统计导出文本中的数据行数（首行为表头）。
func exportDataLineCount(text string) int {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) <= 1 {
		return 0
	}
	return len(lines) - 1
}
