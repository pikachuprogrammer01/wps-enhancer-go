//go:build ignore

// 企查查夹具探测工具：填写 manifest 或调试读表/预览行数（读 manifest.json，不硬编码 Sheet）。
// 用法：cd wps-enhancer-go && go run scripts/qcc_probe.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"wps-enhancer-go/internal/app"
	"wps-enhancer-go/internal/excel"
)

func main() {
	root := "internal/e2e/testdata/qcc"
	raw, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		fmt.Println("read manifest:", err)
		os.Exit(1)
	}
	var m struct {
		Cases []struct {
			ID              string `json:"id"`
			File            string `json:"file"`
			Sheet           string `json:"sheet"`
			SkipDeclaration bool   `json:"skip_declaration"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Println("parse manifest:", err)
		os.Exit(1)
	}

	for _, c := range m.Cases {
		a := app.NewApp(os.TempDir())
		path := filepath.Join(root, c.File)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Println(c.ID, "file missing:", path)
			continue
		}
		sheet := c.Sheet
		if sheet == "" {
			reader, err := excel.GetReader(path)
			if err != nil {
				fmt.Println(c.ID, "GetReader:", err)
				continue
			}
			names, err := reader.GetSheetNames(path)
			if err != nil || len(names) == 0 {
				fmt.Println(c.ID, "no sheets")
				continue
			}
			sheet = names[0]
		}
		data, err := a.ReadSheet(path, sheet, c.SkipDeclaration)
		if err != nil {
			fmt.Println(c.ID, "read err", err)
			continue
		}
		tmplName := "e2e-" + c.ID
		if err := a.TemplateCreateFromHeaders(tmplName, data.Headers); err != nil {
			fmt.Println(c.ID, "template err", err)
			continue
		}
		sug := a.SuggestTemplates(data.Headers)
		prev, err := a.PreviewWithMapping(data, tmplName, nil)
		if err != nil {
			fmt.Println(c.ID, "preview err", err)
			continue
		}
		fmt.Printf("%s: decl=%v rows=%d preview=%d invalid=%d suggest=%d\n",
			c.ID, data.DeclarationSkipped, len(data.Rows), len(prev.Rows), prev.InvalidCount, len(sug))
		if len(sug) > 0 {
			fmt.Printf("  best: %s %d/%d missing=%v\n", sug[0].Name, sug[0].Matched, sug[0].Total, sug[0].MissingCols)
		}
	}
}
