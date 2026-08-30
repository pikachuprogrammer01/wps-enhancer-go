package excel

import (
	"fmt"
	"strings"

	"github.com/extrame/xls"

	"wps-enhancer-go/internal/errs"
)

// XlsReader 基于 extrame-xls 的 xls 读取器（已停滞库，必须包防护层）。
//
// 已知 bug 防护（open issues 实证）：
//   - #66/#70：特定文件上内部 panic → recover 转 error
//   - #99：部分列读不到导致数据错位 → 行短补空串到表头长度
//   - #101：数字误判日期 → 由库内格式化输出，golden 对照测试锁行为
type XlsReader struct{}

// GetSheetNames 读取文件中所有 Sheet 名称。
func (r *XlsReader) GetSheetNames(filePath string) (names []string, err error) {
	defer recoverToErr(&err, filePath)
	wb, err := xls.Open(filePath, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	for i := 0; i < wb.NumSheets(); i++ {
		names = append(names, wb.GetSheet(i).Name)
	}
	return names, nil
}

// GetSheetSummaries 读取所有 Sheet 的名称与数据行数（对齐 Python nrows 语义）。
func (r *XlsReader) GetSheetSummaries(filePath string) (summaries []SheetSummary, err error) {
	defer recoverToErr(&err, filePath)
	wb, err := xls.Open(filePath, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	for i := 0; i < wb.NumSheets(); i++ {
		ws := wb.GetSheet(i)
		rows := int(ws.MaxRow) + 1 // MaxRow 为最后行索引（count-1）
		if ws.MaxRow == 0 && ws.Row(0) == nil {
			rows = 0 // 空 sheet
		}
		summaries = append(summaries, SheetSummary{Name: ws.Name, Rows: rows})
	}
	return summaries, nil
}

// ReadSheet 读取指定 Sheet 的表头和数据行（可选剔除首行声明，第一行即表头）。
func (r *XlsReader) ReadSheet(filePath, sheetName string, opts ReadOptions) (data *SheetData, err error) {
	defer recoverToErr(&err, filePath)
	wb, err := xls.Open(filePath, "utf-8")
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	ws := findSheet(wb, sheetName)
	if ws == nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': Sheet '%s' 不存在", errs.ErrFileRead, filePath, sheetName)
	}

	// 逐行提取（nil 行 = 空行，跳过；行内列取到 LastCol）
	allRows := make([][]string, 0)
	for rowIdx := 0; rowIdx <= int(ws.MaxRow); rowIdx++ {
		row := ws.Row(rowIdx)
		if row == nil {
			continue
		}
		rowVals := make([]string, 0, row.LastCol()+1)
		for colIdx := 0; colIdx <= row.LastCol(); colIdx++ {
			rowVals = append(rowVals, row.Col(colIdx))
		}
		allRows = append(allRows, rowVals)
	}

	// 跳过前导空行（声明行前常见空行）
	headerRow := 0
	for headerRow < len(allRows) && IsEmptyRow(allRows[headerRow]) {
		headerRow++
	}
	if headerRow >= len(allRows) {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s'：Sheet '%s' 为空", errs.ErrFileRead, filePath, sheetName)
	}
	skipped := false
	if opts.SkipDeclaration && len(allRows)-headerRow >= 2 && IsDeclarationFirstRow(allRows[headerRow], allRows[headerRow+1], opts.DeclarationKeywords) {
		headerRow++
		skipped = true
		// 声明行后可能跟空行
		for headerRow < len(allRows) && IsEmptyRow(allRows[headerRow]) {
			headerRow++
		}
		if headerRow >= len(allRows) {
			return nil, fmt.Errorf("%w: 无法读取文件 '%s'：Sheet '%s' 为空", errs.ErrFileRead, filePath, sheetName)
		}
	}

	headers := append([]string(nil), allRows[headerRow]...)
	for len(headers) > 0 && strings.TrimSpace(headers[len(headers)-1]) == "" {
		headers = headers[:len(headers)-1] // 去掉尾部空表头列
	}

	data = &SheetData{
		SheetName:          sheetName,
		Headers:            headers,
		DeclarationSkipped: skipped,
	}
	for rowIdx := headerRow + 1; rowIdx < len(allRows); rowIdx++ {
		rowDict := make(map[string]string, len(headers))
		for colIdx, header := range headers {
			if colIdx < len(allRows[rowIdx]) {
				rowDict[header] = allRows[rowIdx][colIdx]
			} else {
				rowDict[header] = "" // #99 防护：行短补空串
			}
		}
		data.Rows = append(data.Rows, rowDict)
	}
	return data, nil
}

// findSheet 按名称查找 Sheet。
func findSheet(wb *xls.WorkBook, name string) *xls.WorkSheet {
	for i := 0; i < wb.NumSheets(); i++ {
		if ws := wb.GetSheet(i); ws != nil && ws.Name == name {
			return ws
		}
	}
	return nil
}

// recoverToErr 捕获 extrame-xls 的已知 panic（#66/#70）并转为 ErrFileRead。
func recoverToErr(err *error, filePath string) {
	if r := recover(); r != nil {
		*err = fmt.Errorf("%w: xls 解析异常: %v (文件 '%s')", errs.ErrFileRead, r, filePath)
	}
}
