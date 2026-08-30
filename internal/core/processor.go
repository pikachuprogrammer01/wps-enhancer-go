package core

import (
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"
	"time"

	"wps-enhancer-go/internal/errs"
	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/settings"
	"wps-enhancer-go/internal/template"
)

// validSecondDigits 规则二允许的手机号第二位字符。
const validSecondDigits = "3456789"

// 数字截断/补零检测：科学计数法 / 长数字尾补零（对齐 Python detect_truncated_numbers）。
var (
	sciRE      = regexp.MustCompile(`^[-+]?\d+(\.\d+)?[eE][-+]?\d+$`)
	longZeroRE = regexp.MustCompile(`^\d{15,}0{3,}$`)
)

// DetectTruncatedNumbers 检测疑似号码/身份证被截断补零的列提示。
// 特征（宁可漏检不误判）：① 科学计数法文本；② 15 位以上纯数字且末尾连续 3+ 个 0。
// 每列至多采 3 个样例；无问题返回空切片。
func DetectTruncatedNumbers(data *excel.SheetData) []string {
	if data == nil {
		return nil
	}
	hints := make([]string, 0)
	for _, col := range data.Headers {
		samples := make([]string, 0, 3)
		for _, row := range data.Rows {
			value := strings.TrimSpace(row[col])
			if value == "" {
				continue
			}
			if sciRE.MatchString(value) || longZeroRE.MatchString(value) {
				sample := value
				if len(sample) > 20 {
					sample = sample[:20]
				}
				samples = append(samples, sample)
				if len(samples) >= 3 {
					break
				}
			}
		}
		if len(samples) > 0 {
			hints = append(hints, fmt.Sprintf(
				"列「%s」：如 %s…（疑似号码/身份证被截断补零）",
				col, strings.Join(samples, ", "),
			))
		}
	}
	return hints
}

// SplitPhones 按配置的分隔符依次拆分手机号，去除空白并过滤空段。
func SplitPhones(rawPhone string, separators []string) []string {
	pieces := []string{rawPhone}
	for _, sep := range separators {
		if sep == "" {
			continue
		}
		var next []string
		for _, piece := range pieces {
			next = append(next, strings.Split(piece, sep)...)
		}
		pieces = next
	}
	out := make([]string, 0)
	for _, s := range pieces {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ValidatePhone 校验单个手机号是否合法（调用方负责 strip）。
func ValidatePhone(phone string) bool {
	if phone == "" {
		return true
	}
	if strings.HasPrefix(phone, "+") {
		return true
	}
	if len(phone) != 11 {
		return false
	}
	if !isAllDigits(phone) {
		return false
	}
	if phone[0] != '1' {
		return false
	}
	if !strings.ContainsRune(validSecondDigits, rune(phone[1])) {
		return false
	}
	return true
}

// enabledMatches 返回模板中 enabled 列的匹配结果（顺序与模板列一致）。
func enabledMatches(matches []template.ColumnMatch) []template.ColumnMatch {
	out := make([]template.ColumnMatch, 0, len(matches))
	for _, m := range matches {
		if m.TemplateCol.Enabled {
			out = append(out, m)
		}
	}
	return out
}

// sourceValue 取源行中该匹配列的值（strip 后；未匹配返回空串）。
func sourceValue(row map[string]string, match template.ColumnMatch) string {
	if match.SourceCol == nil {
		return ""
	}
	return strings.TrimSpace(row[*match.SourceCol])
}

// buildValues 按 enabled 列顺序构建一行导出值（phone 列用拆分后的单段值）。
func buildValues(row map[string]string, matches []template.ColumnMatch, phoneValue string) []string {
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.TemplateCol.Key == "phone" {
			values = append(values, phoneValue)
		} else {
			values = append(values, sourceValue(row, match))
		}
	}
	return values
}

// processRow 处理单个源数据行，返回 ExportRow 列表与无效手机号描述列表。
func processRow(row map[string]string, matches []template.ColumnMatch, hasPhone bool, rowIndex int, validate bool, separators []string) ([]ExportRow, []string) {
	if !hasPhone {
		return []ExportRow{{
			Values:         buildValues(row, matches, ""),
			PhoneValid:     true,
			SourceRowIndex: rowIndex,
			MergeSpan:      1, // 与 Python dataclass 默认值一致（无拆分）
		}}, nil
	}

	var phoneMatch template.ColumnMatch
	for _, m := range matches {
		if m.TemplateCol.Key == "phone" {
			phoneMatch = m
			break
		}
	}
	phones := SplitPhones(sourceValue(row, phoneMatch), separators)
	if len(phones) == 0 {
		phones = []string{""}
	}
	rows := make([]ExportRow, 0, len(phones))
	invalids := make([]string, 0)
	for i, phone := range phones {
		valid := ValidatePhone(phone)
		if !validate {
			valid = true
		}
		rows = append(rows, ExportRow{
			Values:         buildValues(row, matches, phone),
			PhoneValid:     valid,
			SourceRowIndex: rowIndex,
			MergeSpan:      len(phones),
			IsFirstOfSplit: i == 0,
		})
		if !valid {
			invalids = append(invalids, fmt.Sprintf("第 %d 行：%s 不是合法手机号", rowIndex, phone))
		}
	}
	return rows, invalids
}

// groupByName 按姓名分组（保持首次出现顺序）：组内除首行外姓名置空，并标记合并跨度。
// 未启用合并或无姓名列时原样返回；空姓名行不参与分组（独立成行）。
func groupByName(rows []ExportRow, nameIndex int, mergeEnabled bool) []ExportRow {
	if !mergeEnabled || nameIndex < 0 {
		return rows
	}
	groups := make(map[string][]ExportRow)
	order := make([]string, 0)
	result := make([]ExportRow, 0, len(rows))
	for _, row := range rows {
		name := row.Values[nameIndex]
		if name == "" {
			result = append(result, row) // 空姓名行独立，不参与合并
			continue
		}
		if _, ok := groups[name]; !ok {
			order = append(order, name)
		}
		groups[name] = append(groups[name], row)
	}
	for _, name := range order {
		group := groups[name]
		for i := range group {
			if i > 0 {
				group[i].Values[nameIndex] = "" // 合并单元格：仅组首行显示姓名
			}
			group[i].IsFirstOfSplit = (i == 0)
			group[i].MergeSpan = len(group)
			result = append(result, group[i])
		}
	}
	return result
}

// BuildPreviewData 将 SheetData 转换为 PreviewData（按映射填充 + 按设置拆分/校验/分组合并）。
// 与 Python 版 build_preview_data 一致：内部 panic 包装为 ErrDataProcess。
func BuildPreviewData(data *excel.SheetData, tmpl *template.Template, matches []template.ColumnMatch, st *settings.AppSettings) (preview *PreviewData, err error) {
	defer func() {
		if r := recover(); r != nil {
			preview = nil
			err = fmt.Errorf("%w: 数据处理失败: %v", errs.ErrDataProcess, r)
		}
	}()

	enabled := enabledMatches(matches)
	hasPhone := false
	for _, m := range enabled {
		if m.TemplateCol.Key == "phone" && m.SourceCol != nil {
			hasPhone = true
			break
		}
	}
	nameIndex := -1
	for i, m := range enabled {
		if m.TemplateCol.Key == "name" {
			nameIndex = i
			break
		}
	}

	allRows := make([]ExportRow, 0)
	invalidSummary := make([]string, 0)
	for rowIndex, row := range data.Rows { // rowIndex 从 0 起，Python enumerate(start=1)
		rows, invalids := processRow(row, enabled, hasPhone, rowIndex+1, st.PhoneValidate, st.PhoneSeparators)
		allRows = append(allRows, rows...)
		invalidSummary = append(invalidSummary, invalids...)
	}
	allRows = groupByName(allRows, nameIndex, st.PhoneMerge)

	return &PreviewData{
		Rows:           allRows,
		InvalidCount:   len(invalidSummary),
		InvalidSummary: invalidSummary,
	}, nil
}

// buildMergeRanges 构建姓名列合并范围（拆分组连续行合并）。
func buildMergeRanges(rows []ExportRow, nameColIndex int, mergeEnabled bool) []excel.MergeRange {
	if !mergeEnabled || nameColIndex < 0 {
		return []excel.MergeRange{}
	}
	ranges := make([]excel.MergeRange, 0)
	for i, row := range rows {
		if row.IsFirstOfSplit && row.MergeSpan > 1 {
			ranges = append(ranges, excel.MergeRange{
				RowStart: i,
				RowEnd:   i + row.MergeSpan - 1,
				ColIndex: nameColIndex,
			})
		}
	}
	return ranges
}

// buildCellStyles 构建非法手机号单元格的红色背景样式。
func buildCellStyles(rows []ExportRow, phoneColIndex int, highlightEnabled bool) map[string]excel.CellStyle {
	styles := make(map[string]excel.CellStyle)
	if !highlightEnabled || phoneColIndex < 0 {
		return styles
	}
	red := "#FF0000"
	for i, row := range rows {
		if !row.PhoneValid {
			styles[fmt.Sprintf("(%d, %d)", i, phoneColIndex)] = excel.CellStyle{BackgroundColor: &red}
		}
	}
	return styles
}

// outputSuffix 返回输出文件后缀（无后缀返回空串），等价 Python _output_suffix。
func outputSuffix(outputPath string) string {
	idx := strings.LastIndex(outputPath, ".")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(outputPath[idx+1:])
}

// vcfIndexedRows vcf 导出行：同姓名多手机号（merge_span>1 的组）姓名追加 _1/_2 序号。
// 序号按组内位置从 1 累加；单行组不加序号；姓名已为空的行不追加。
func vcfIndexedRows(rows [][]string, spans []int, nameIndex int) [][]string {
	if nameIndex < 0 {
		return rows
	}
	result := make([][]string, len(rows))
	for i, r := range rows {
		result[i] = append([]string(nil), r...)
	}
	i := 0
	for i < len(result) {
		span := spans[i]
		if span > 1 {
			groupEnd := i + span
			if groupEnd > len(result) {
				groupEnd = len(result) // 预览截断时组可能被切断：实际组内行数 = min(span, 剩余行数)
			}
			// vcf 无合并概念：组内被置空的姓名恢复为组首姓名
			baseName := ""
			for j := i; j < groupEnd; j++ {
				if result[j][nameIndex] != "" {
					baseName = result[j][nameIndex]
					break
				}
			}
			for k := 0; k < groupEnd-i; k++ {
				idx := i + k
				if result[idx][nameIndex] != "" {
					result[idx][nameIndex] = fmt.Sprintf("%s_%d", result[idx][nameIndex], k+1)
				} else if baseName != "" {
					result[idx][nameIndex] = fmt.Sprintf("%s_%d", baseName, k+1)
				}
			}
			i = groupEnd
		} else {
			i++
		}
	}
	return result
}

// vcfTimestamp 返回年月日时间戳（开关关闭时为空串）。
func vcfTimestamp(st *settings.AppSettings) string {
	if !st.VCFTimestamp {
		return ""
	}
	return time.Now().Format("20060102")
}

// EffectiveVCFPrefix 返回 vcf 姓名前缀实际值（时间戳在姓名前时附加年月日）。
func EffectiveVCFPrefix(st *settings.AppSettings) string {
	ts := vcfTimestamp(st)
	if ts == "" || st.VCFTimestampPosition != "prefix" {
		return st.VCFNamePrefix
	}
	return st.VCFNamePrefix + ts
}

// PreviewSummaryLine 生成预览区顶部汇总文案（对齐 SPEC 步骤 7）。
func PreviewSummaryLine(st *settings.AppSettings, total int, format string, invalidCount int) string {
	line := fmt.Sprintf("共 %d 行", total)
	if format == "vcf" {
		line += fmt.Sprintf("，vcf 姓名前缀：%s", EffectiveVCFPrefix(st))
	}
	if st.PhoneValidate && invalidCount > 0 {
		line += fmt.Sprintf("，其中 %d 个手机号格式异常", invalidCount)
	}
	return line
}

// effectiveVCFSuffix 返回 vcf 姓名后缀实际值（时间戳在姓名后时附加年月日）。
func effectiveVCFSuffix(st *settings.AppSettings) string {
	ts := vcfTimestamp(st)
	if ts == "" || st.VCFTimestampPosition != "suffix" {
		return st.VCFNameSuffix
	}
	return st.VCFNameSuffix + ts
}

// BuildWriteRequest 将 PreviewData 转换为 WriteRequest（headers 来自模板 enabled 列）。
func BuildWriteRequest(preview *PreviewData, tmpl *template.Template, matches []template.ColumnMatch, st *settings.AppSettings, outputPath string) *excel.WriteRequest {
	enabled := enabledMatches(matches)
	headers := make([]string, 0, len(enabled))
	fieldKeys := make([]string, 0, len(enabled))
	phoneIndex := -1
	nameIndex := -1
	for i, m := range enabled {
		headers = append(headers, m.TemplateCol.Name)
		fieldKeys = append(fieldKeys, m.TemplateCol.Key)
		if m.TemplateCol.Key == "phone" {
			phoneIndex = i
		}
		if m.TemplateCol.Key == "name" {
			nameIndex = i
		}
	}
	dataRows := make([][]string, 0, len(preview.Rows))
	for _, row := range preview.Rows {
		dataRows = append(dataRows, row.Values)
	}
	if outputSuffix(outputPath) == "vcf" {
		spans := make([]int, 0, len(preview.Rows))
		for _, r := range preview.Rows {
			spans = append(spans, r.MergeSpan)
		}
		dataRows = vcfIndexedRows(dataRows, spans, nameIndex)
	}
	mergeRanges := buildMergeRanges(preview.Rows, nameIndex, st.PhoneMerge)
	cellStyles := buildCellStyles(preview.Rows, phoneIndex, st.PhoneHighlight)
	return &excel.WriteRequest{
		FilePath:      outputPath,
		Headers:       headers,
		DataRows:      dataRows,
		MergeRanges:   mergeRanges,
		CellStyles:    cellStyles,
		FieldKeys:     fieldKeys,
		Encoding:      outputEncoding(outputPath, st),
		Separator:     st.TxtSeparator,
		VCFFields:     append([]string(nil), st.VCFFields...),
		VCFProps:      st.VCFPropMap(),
		VCFNamePrefix: EffectiveVCFPrefix(st),
		VCFNameSuffix: effectiveVCFSuffix(st),
	}
}

// outputEncoding 按输出格式选择编码：csv 用 csv 编码，txt 用 txt 编码，其余固定 utf-8。
func outputEncoding(outputPath string, st *settings.AppSettings) string {
	suffix := outputSuffix(outputPath)
	if suffix == "txt" {
		return st.TxtEncoding
	}
	if suffix == "csv" {
		return st.CSVEncoding
	}
	return "utf-8" // vcf/xlsx/xls 固定 UTF-8
}

// BuildPreviewDisplay 按导出格式生成预览表头与行内容（与导出文件展示一致）。
// vcf：仅保留 vcf_fields 且 vcf 支持的字段列，姓名应用前后缀；其他格式：模板 enabled 全部列，原值展示。
func BuildPreviewDisplay(preview *PreviewData, matches []template.ColumnMatch, st *settings.AppSettings, format string) ([]string, [][]string) {
	enabled := enabledMatches(matches)
	keep := make([]int, 0)
	for i, m := range enabled {
		if format == "vcf" {
			for _, f := range st.VCFFields {
				if m.TemplateCol.Key == f {
					keep = append(keep, i)
					break
				}
			}
		} else {
			keep = append(keep, i)
		}
	}
	headers := make([]string, 0, len(keep))
	for _, i := range keep {
		headers = append(headers, enabled[i].TemplateCol.Name)
	}
	nameKeyIdx := -1
	for i, m := range enabled {
		if m.TemplateCol.Key == "name" {
			nameKeyIdx = i
			break
		}
	}
	rows := make([][]string, 0, len(preview.Rows))
	for _, row := range preview.Rows {
		values := make([]string, 0, len(keep))
		for _, i := range keep {
			values = append(values, row.Values[i])
		}
		if format == "vcf" && nameKeyIdx >= 0 {
			namePos := -1
			for j, i := range keep {
				if i == nameKeyIdx {
					namePos = j
					break
				}
			}
			if namePos >= 0 && values[namePos] != "" {
				values[namePos] = st.VCFNamePrefix + values[namePos] + st.VCFNameSuffix
			}
		}
		rows = append(rows, values)
	}
	return headers, rows
}

// BuildTextPreview 生成 csv/txt/vcf 的文本预览（与导出文件内容一致，最多前 rowLimit 行数据）。
// csv/txt：表头 + 分隔符连接的数据行；vcf：完整 vCard 文本（含姓名前后缀）。
func BuildTextPreview(preview *PreviewData, matches []template.ColumnMatch, st *settings.AppSettings, format string, rowLimit int) (string, error) {
	enabled := enabledMatches(matches)
	headers := make([]string, 0, len(enabled))
	for _, m := range enabled {
		headers = append(headers, m.TemplateCol.Name)
	}
	if format == "vcf" {
		nameIndex := -1
		for i, m := range enabled {
			if m.TemplateCol.Key == "name" {
				nameIndex = i
				break
			}
		}
		// 先对全量行做序号（组完整性），再截断显示行数
		allRows := make([][]string, 0, len(preview.Rows))
		spans := make([]int, 0, len(preview.Rows))
		for _, r := range preview.Rows {
			allRows = append(allRows, r.Values)
			spans = append(spans, r.MergeSpan)
		}
		allRows = vcfIndexedRows(allRows, spans, nameIndex)
		dataRows := allRows
		if rowLimit < len(dataRows) {
			dataRows = dataRows[:rowLimit]
		}
		fieldKeys := make([]string, 0, len(enabled))
		for _, m := range enabled {
			fieldKeys = append(fieldKeys, m.TemplateCol.Key)
		}
		request := &excel.WriteRequest{
			CSVSeparator:  st.CSVSeparator,
			Headers:       headers,
			DataRows:      dataRows,
			FieldKeys:     fieldKeys,
			VCFFields:     append([]string(nil), st.VCFFields...),
			VCFProps:      st.VCFPropMap(),
			VCFNamePrefix: EffectiveVCFPrefix(st),
			VCFNameSuffix: effectiveVCFSuffix(st),
		}
		vcfLines, err := excel.BuildVCFLines(request)
		if err != nil {
			return "", err
		}
		return strings.Join(vcfLines, "\n"), nil
	}
	if format == "csv" {
		return buildCSVText(headers, previewRowsLimited(preview, rowLimit), st.CSVSeparator), nil
	}
	return buildTxtText(headers, previewRowsLimited(preview, rowLimit), st.TxtSeparator), nil
}

// previewRowsLimited 返回预览数据行（最多前 rowLimit 行）。
func previewRowsLimited(preview *PreviewData, rowLimit int) [][]string {
	dataRows := make([][]string, 0, len(preview.Rows))
	for _, r := range preview.Rows {
		dataRows = append(dataRows, r.Values)
	}
	if rowLimit < len(dataRows) {
		dataRows = dataRows[:rowLimit]
	}
	return dataRows
}

// buildCSVText 按 csv 规则生成文件文本（预览与写入共用；行结束符归一为 \n 便于展示）。
// 与 Python csv.writer(lineterminator="\n") 行为一致（Go encoding/csv 默认 QUOTE_MINIMAL + \n）。
func buildCSVText(headers []string, rows [][]string, separator string) string {
	if sep := []rune(separator); len(sep) == 1 {
		// 自定义单字符分隔符；空值回退逗号
	} else if len(separator) > 0 {
		separator = "," // 多字符非法，回退逗号
	}
	if separator == "" {
		separator = ","
	}
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	w.Comma = []rune(separator)[0]
	_ = w.Write(headers)
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	return buf.String()
}

// buildTxtText 按 txt 规则生成文件文本（预览与写入共用）：行内用分隔符连接，末尾换行。
func buildTxtText(headers []string, rows [][]string, separator string) string {
	sep := separator
	if sep == "" {
		sep = " "
	}
	lines := []string{strings.Join(headers, sep)}
	for _, row := range rows {
		lines = append(lines, strings.Join(row, sep))
	}
	return strings.Join(lines, "\n") + "\n"
}
