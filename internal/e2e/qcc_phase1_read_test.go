package e2e

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestQCC_BootstrapDump 探测夹具读表摘要（填写 manifest 用）；需 QCC_FIXTURE_DUMP=1。
func TestQCC_BootstrapDump(t *testing.T) {
	if os.Getenv("QCC_FIXTURE_DUMP") == "" {
		t.Skip("设置 QCC_FIXTURE_DUMP=1 以打印夹具摘要（用于填写 manifest.json）")
	}

	m := loadQCCManifest(t)
	a := testApp(t)
	entries := make([]qccDumpEntry, 0, len(m.Cases))

	for _, c := range m.Cases {
		path := qccFilePath(c)
		entry := qccDumpEntry{
			ID:            c.ID,
			File:          c.File,
			ManifestReady: c.Expect.Ready,
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			entry.FileExists = false
			entries = append(entries, entry)
			continue
		}
		entry.FileExists = true

		reader, err := requireReader(t, path)
		if err != nil {
			t.Fatalf("%s: %v", c.ID, err)
		}
		names, err := reader.GetSheetNames(path)
		if err != nil {
			t.Fatalf("%s GetSheetNames: %v", c.ID, err)
		}
		entry.Sheets = names

		sheet := c.Sheet
		if sheet == "" && len(names) > 0 {
			sheet = names[0]
		}
		entry.Sheet = sheet

		fullRows, _ := readQCCSourceFull(t, c)
		entry.SourceRowCount = fullRows

		data, err := a.ReadSheet(path, sheet, c.SkipDeclaration)
		if err != nil {
			t.Fatalf("%s ReadSheet: %v", c.ID, err)
		}
		entry.DeclarationSkipped = data.DeclarationSkipped
		entry.Headers = data.Headers
		entry.RowCount = len(data.Rows)
		entry.ReadCapped = fullRows > len(data.Rows)

		tmplName := "e2e-" + c.ID
		if err := a.TemplateCreateFromHeaders(tmplName, data.Headers); err != nil {
			t.Fatalf("%s TemplateCreateFromHeaders: %v", c.ID, err)
		}
		suggestions := a.SuggestTemplates(data.Headers)
		entry.SuggestCount = len(suggestions)
		if len(suggestions) > 0 {
			entry.SuggestMin = 1
		}
		preview, err := a.PreviewWithMapping(data, tmplName, nil)
		if err != nil {
			t.Fatalf("%s PreviewWithMapping: %v", c.ID, err)
		}
		entry.PreviewRows = len(preview.Rows)
		entry.InvalidPhones = preview.InvalidCount

		entries = append(entries, entry)
	}

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	t.Logf("\n--- QCC 夹具摘要（复制到 manifest.json expect 段）---\n%s", string(out))
}

// TestQCC_Phase1_Read 阶段 1：真实企查查源表读入冒烟（声明行、表头、行数）。
func TestQCC_Phase1_Read(t *testing.T) {
	m := loadQCCManifest(t)
	a := testApp(t)

	for _, c := range m.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			requireQCCFile(t, c)
			requireQCCReady(t, c)

			data, sheet := readQCCCase(t, a, c)
			t.Logf("[%s] sheet=%q desc=%s", c.ID, sheet, c.Description)

			t.Run("declaration_skipped", func(t *testing.T) {
				if data.DeclarationSkipped != c.Expect.DeclarationSkipped {
					t.Errorf("declaration_skipped: got %v want %v", data.DeclarationSkipped, c.Expect.DeclarationSkipped)
				}
			})
			t.Run("headers", func(t *testing.T) {
				if diff := cmp.Diff(c.Expect.Headers, data.Headers); diff != "" {
					t.Errorf("headers (-want +got):\n%s", diff)
				}
			})
			t.Run("row_count", func(t *testing.T) {
				if len(data.Rows) != c.Expect.RowCount {
					t.Errorf("row_count (app): got %d want %d", len(data.Rows), c.Expect.RowCount)
				}
			})
			t.Run("source_row_count", func(t *testing.T) {
				full, _ := readQCCSourceFull(t, c)
				wantFull := expectSourceRows(c)
				if full != wantFull {
					t.Errorf("source_row_count: got %d want %d", full, wantFull)
				}
				if full > appReadRowCap && c.Expect.RowCount != appReadRowCap {
					t.Errorf("源表 %d 行超过 App 读入上限 %d，manifest row_count 应设为 %d",
						full, appReadRowCap, appReadRowCap)
				}
			})
		})
	}
}
