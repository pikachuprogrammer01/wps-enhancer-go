package e2e

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// writeSourceXlsx 写入企查查风格源表（声明行 + 表头 + 2 行数据）。
func writeSourceXlsx(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "源表.xlsx")
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "客户"
	if err := f.SetSheetName(f.GetSheetName(0), sheet); err != nil {
		t.Fatalf("SetSheetName: %v", err)
	}
	rows := [][]string{
		{"企查查导出数据"},
		{"姓名", "有效手机号", "公司", "官网"},
		{"张三", "13800000000;13900000000", "A公司", "http://a.com"},
		{"李四", "bad", "B公司", ""},
	}
	for ri, row := range rows {
		for ci, val := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+1)
			if err := f.SetCellValue(sheet, cell, val); err != nil {
				t.Fatalf("SetCellValue: %v", err)
			}
		}
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	return path
}
