package excel

// xls → xlsx 转换：源文件统一走 xlsx 处理管线（决策：docs/migration/08-status-report.md）。
// 读取用 vendored extrame/xls fork，写出用 excelize。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"

	"wps-enhancer-go/internal/errs"
)

// ConvertXlsToXlsx 将 .xls 源文件转换为 .xlsx（全部 Sheet、保留单元格文本值）。
// 返回生成的 xlsx 路径：与源文件同目录，命名为 <原名>_converted.xlsx；
// 目标已存在时直接复用（按源文件修改时间判断是否需要重新转换由调用方决定）。
func ConvertXlsToXlsx(srcPath string) (string, error) {
	if strings.ToLower(filepath.Ext(srcPath)) != ".xls" {
		return "", fmt.Errorf("%w: 仅支持 .xls 源文件，收到 %s", errs.ErrFileRead, srcPath)
	}
	dst := strings.TrimSuffix(srcPath, filepath.Ext(srcPath)) + "_converted.xlsx"
	if _, err := os.Stat(dst); err == nil {
		return dst, nil // 已转换过，复用
	}

	reader := &XlsReader{}
	summaries, err := reader.GetSheetSummaries(srcPath)
	if err != nil {
		return "", err
	}

	out := excelize.NewFile()
	defer out.Close()
	for i, sum := range summaries {
		sheetName := sum.Name
		if i == 0 {
			out.SetSheetName("Sheet1", sheetName)
		} else {
			out.NewSheet(sheetName)
		}
		data, err := reader.ReadSheet(srcPath, sheetName, ReadOptions{})
		if err != nil {
			return "", fmt.Errorf("读取 Sheet %q 失败: %v", sheetName, err)
		}
		for c, h := range data.Headers {
			cell, _ := excelize.CoordinatesToCellName(c+1, 1)
			out.SetCellValue(sheetName, cell, h)
		}
		for ri, row := range data.Rows {
			for c, h := range data.Headers {
				cell, _ := excelize.CoordinatesToCellName(c+1, ri+2)
				out.SetCellValue(sheetName, cell, row[h])
			}
		}
	}

	if err := out.SaveAs(dst); err != nil {
		return "", fmt.Errorf("%w: 保存转换后的 xlsx 失败: %v", errs.ErrFileWrite, err)
	}
	return dst, nil
}
