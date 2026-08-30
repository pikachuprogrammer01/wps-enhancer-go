package excel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"wps-enhancer-go/internal/errs"
)

// XlsxReader 基于 excelize 的 xlsx 文件读取器。
type XlsxReader struct{}

// GetSheetNames 读取文件中所有 Sheet 名称。
func (r *XlsxReader) GetSheetNames(filePath string) ([]string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	defer f.Close()
	return f.GetSheetList(), nil
}

// GetSheetSummaries 读取所有 Sheet 的名称与数据行数（近似值，对齐 Python max_row 语义）。
func (r *XlsxReader) GetSheetSummaries(filePath string) ([]SheetSummary, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	summaries := make([]SheetSummary, 0, len(sheets))
	for _, name := range sheets {
		rows, err := f.GetRows(name)
		if err != nil {
			return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
		}
		summaries = append(summaries, SheetSummary{Name: name, Rows: len(rows)})
	}
	return summaries, nil
}

// ReadSheet 读取指定 Sheet 的表头和数据行（可选剔除首行声明，第一行即表头）。
// separator/encoding 仅 csv/txt 数据源使用，xlsx 忽略。
func (r *XlsxReader) ReadSheet(filePath, sheetName string, opts ReadOptions) (*SheetData, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	defer f.Close()
	if _, err := f.GetSheetIndex(sheetName); err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': Sheet '%s' 不存在", errs.ErrFileRead, filePath, sheetName)
	}
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	allRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		allRows = append(allRows, row)
	}
	// 跳过前导空行（声明行前常见空行）
	for len(allRows) > 0 && IsEmptyRow(allRows[0]) {
		allRows = allRows[1:]
	}
	if len(allRows) == 0 {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s'：Sheet '%s' 为空", errs.ErrFileRead, filePath, sheetName)
	}
	skipped := false
	if opts.SkipDeclaration && len(allRows) >= 2 && IsDeclarationFirstRow(allRows[0], allRows[1], opts.DeclarationKeywords) {
		allRows = allRows[1:]
		skipped = true
		// 声明行后可能跟空行
		for len(allRows) > 0 && IsEmptyRow(allRows[0]) {
			allRows = allRows[1:]
		}
	}
	headers := make([]string, 0, len(allRows[0]))
	for _, cell := range allRows[0] {
		headers = append(headers, cell)
	}
	for len(headers) > 0 && headers[len(headers)-1] == "" {
		headers = headers[:len(headers)-1] // 去掉尾部空表头列（文件最大列宽超出表头）
	}
	data := &SheetData{
		SheetName:          sheetName,
		Headers:            headers,
		DeclarationSkipped: skipped,
	}
	for _, row := range allRows[1:] {
		rowDict := make(map[string]string, len(headers))
		for i, header := range headers {
			if i < len(row) {
				rowDict[header] = row[i]
			} else {
				rowDict[header] = ""
			}
		}
		data.Rows = append(data.Rows, rowDict)
	}
	return data, nil
}

// XlsxWriter 基于 excelize 的 xlsx 文件写入器。
type XlsxWriter struct{}

// WriteExport 写入导出数据，含合并单元格和背景色标记。
func (w *XlsxWriter) WriteExport(request *WriteRequest) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())

	// 表头加粗
	boldStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}
	for colIdx, header := range request.Headers {
		cell := cellName(1, colIdx+1)
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
		}
		_ = f.SetCellStyle(sheet, cell, cell, boldStyle)
	}
	// 数据行
	for rowIdx, row := range request.DataRows {
		for colIdx, value := range row {
			if err := f.SetCellValue(sheet, cellName(rowIdx+2, colIdx+1), value); err != nil {
				return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
			}
		}
	}
	// 合并单元格（合并后清空非首行值，对齐 Python 版）
	for _, merge := range request.MergeRanges {
		startRow, endRow := merge.RowStart+2, merge.RowEnd+2
		col := merge.ColIndex + 1
		if err := f.MergeCell(sheet, cellName(startRow, col), cellName(endRow, col)); err != nil {
			return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
		}
		for dataRow := merge.RowStart + 1; dataRow <= merge.RowEnd; dataRow++ {
			_ = f.SetCellValue(sheet, cellName(dataRow+2, col), "")
		}
	}
	// 背景色样式
	for key, style := range request.CellStyles {
		if style.BackgroundColor == nil {
			continue
		}
		rowIdx, colIdx := parseCellKey(key)
		if rowIdx < 0 {
			continue
		}
		color := strings.TrimPrefix(*style.BackgroundColor, "#")
		fillStyle, err := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Color: []string{color}, Pattern: 1},
		})
		if err != nil {
			return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
		}
		cell := cellName(rowIdx+2, colIdx+1)
		_ = f.SetCellStyle(sheet, cell, cell, fillStyle)
	}
	if err := f.SaveAs(request.FilePath); err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}
	return nil
}

// cellName 生成 excelize 单元格名（行列恒合法，忽略错误）。
func cellName(row, col int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// parseCellKey 解析 "(row, col)" 形式的样式 key（与 Python str(tuple) 一致）；非法返回 -1。
func parseCellKey(key string) (row, col int) {
	inner := strings.TrimSuffix(strings.TrimPrefix(key, "("), ")")
	parts := strings.Split(inner, ", ")
	if len(parts) != 2 {
		return -1, -1
	}
	r, err1 := strconv.Atoi(parts[0])
	c, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return -1, -1
	}
	return r, c
}
