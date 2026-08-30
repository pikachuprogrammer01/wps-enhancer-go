package e2e

import (
	"testing"
)

// TestQCC_Phase2_Mapping 阶段 2：模板建议、Apply、预览（依赖 manifest expect）。
func TestQCC_Phase2_Mapping(t *testing.T) {
	m := loadQCCManifest(t)
	a := testApp(t)

	for _, c := range m.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			requireQCCFile(t, c)
			requireQCCReady(t, c)

			data, _, tmplName := setupQCCCase(t, a, c)

			t.Run("suggest_templates", func(t *testing.T) {
				suggestions := a.SuggestTemplates(data.Headers)
				if len(suggestions) < c.Expect.SuggestMin {
					t.Errorf("SuggestTemplates: got %d want >= %d", len(suggestions), c.Expect.SuggestMin)
				}
				if len(suggestions) == 0 || suggestions[0].Name != tmplName {
					t.Errorf("最佳建议应为 %q: %+v", tmplName, suggestions)
				}
				if suggestions[0].Matched == 0 {
					t.Error("最佳建议 Matched 应 > 0")
				}
			})

			t.Run("apply_template", func(t *testing.T) {
				applied, err := a.ApplyTemplate(data.Headers, tmplName)
				if err != nil {
					t.Fatalf("ApplyTemplate: %v", err)
				}
				if applied.Name != tmplName {
					t.Errorf("Applied.Name: got %q want %q", applied.Name, tmplName)
				}
				if len(applied.Mapping) == 0 {
					t.Error("mapping 不应为空")
				}
			})

			t.Run("preview", func(t *testing.T) {
				preview, err := a.PreviewWithMapping(data, tmplName, nil)
				if err != nil {
					t.Fatalf("PreviewWithMapping: %v", err)
				}
				if len(preview.Rows) != c.Expect.PreviewRows {
					t.Errorf("preview rows: got %d want %d", len(preview.Rows), c.Expect.PreviewRows)
				}
				if preview.InvalidCount != c.Expect.InvalidPhones {
					t.Errorf("invalid_phones: got %d want %d", preview.InvalidCount, c.Expect.InvalidPhones)
				}
			})
		})
	}
}
