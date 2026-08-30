package excel

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"wps-enhancer-go/internal/errs"
)

// 分隔符候选（中文环境常见分号分隔，默认 csv.reader 仅认逗号会导致整列错位）。
var delimiterCandidates = []string{",", ";", "\t", "|"}

// detectEncoding 探测 csv 文件编码（BOM → UTF-8 → GBK）。
func detectEncoding(raw []byte) string {
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		return "utf-8-sig"
	}
	if bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) || bytes.HasPrefix(raw, []byte{0xFE, 0xFF}) {
		return "utf-16"
	}
	if isUTF8(raw) {
		return "utf-8"
	}
	return "gbk"
}

// isUTF8 判定字节串是否为合法 UTF-8。
func isUTF8(raw []byte) bool {
	return utf8.Valid(raw)
}

// detectDelimiter 探测 csv 分隔符（逗号/分号/制表符/竖线），取首个数据行中出现最多者。
func detectDelimiter(text string) string {
	sample := ""
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "//") {
			sample = line
			break
		}
	}
	best, bestCount := ",", 0
	for _, cand := range delimiterCandidates {
		count := strings.Count(sample, cand)
		if count > bestCount {
			best, bestCount = cand, count
		}
	}
	return best
}

// sepDisplay 分隔符的用户可读名称（错误提示用）。
func sepDisplay(sep string) string {
	switch sep {
	case "\t":
		return "制表符 Tab"
	case ",":
		return "逗号 ,"
	case ";":
		return "分号 ;"
	case "|":
		return "竖线 |"
	}
	return sep
}

// decodeText 按指定编码解码文本；非法编码返回明确错误。
func decodeText(raw []byte, encoding string) (string, error) {
	switch encoding {
	case "utf-8-sig", "utf-8":
		if encoding == "utf-8-sig" {
			raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
		}
		return string(raw), nil
	case "utf-16", "unicode":
		// 支持 BOM 或默认 LE
		dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
		out, _, err := transform.Bytes(dec, raw)
		return string(out), err
	case "gbk":
		out, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), raw)
		return string(out), err
	}
	return "", fmt.Errorf("%w: 未知编码 %s", errs.ErrFileRead, encoding)
}

// CsvReader 基于标准库 csv 的读取器（编码自动探测，无 Sheet 概念）。
type CsvReader struct{}

// GetSheetNames 返回单个伪 Sheet 名称（文件名不含扩展名）。
func (r *CsvReader) GetSheetNames(filePath string) ([]string, error) {
	stem := filepath.Base(filePath)
	stem = strings.TrimSuffix(stem, filepath.Ext(stem))
	return []string{stem}, nil
}

// GetSheetSummaries 返回单个伪 Sheet（文件名不含扩展名 + 二进制行数近似）。
func (r *CsvReader) GetSheetSummaries(filePath string) ([]SheetSummary, error) {
	rows := 0
	if f, err := os.Open(filePath); err == nil {
		buf := make([]byte, 64*1024)
		for {
			n, err := f.Read(buf)
			rows += bytes.Count(buf[:n], []byte("\n"))
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
		}
		_ = f.Close()
	}
	stem := filepath.Base(filePath)
	if ext := filepath.Ext(stem); ext != "" {
		stem = strings.TrimSuffix(stem, ext)
	}
	return []SheetSummary{{Name: stem, Rows: rows}}, nil
}

// ReadSheet 读取 csv/txt 内容：第一行为表头（可选剔除首行声明）。
// separator/encoding 为自动时检测；指定时按用户设置执行，格式不符报明确错误。
func (r *CsvReader) ReadSheet(filePath, sheetName string, opts ReadOptions) (*SheetData, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	enc := opts.Encoding
	var text string
	if enc != "" && enc != "auto" {
		text, err = decodeText(raw, enc)
		if err != nil {
			return nil, fmt.Errorf("%w: 文件 '%s' 不是 %s 编码（请在设置中选择正确的数据源编码）", errs.ErrFileRead, filePath, enc)
		}
	} else {
		text, err = decodeText(raw, detectEncoding(raw))
		if err != nil {
			return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
		}
	}

	delim := detectDelimiter(text)
	if opts.Separator != "" && opts.Separator != "auto" {
		delim = opts.Separator
		if delim == "tab" {
			delim = "\t" // Python 版设置值 "tab" → 制表符
		}
		// 分隔符必须为单个字符（与 Python csv.reader 一致；支持多字节字符如「、」）
		runes := []rune(delim)
		if len(runes) != 1 {
			return nil, fmt.Errorf("%w: 分隔符必须为单个字符，收到 %q（%d 个字符）",
				errs.ErrFileRead, delim, len(runes))
		}
		// 指定分隔符校验：数据行中完全不存在该分隔符 → 明确报错（防静默错位）
		sample := ""
		for _, line := range strings.Split(text, "\n") {
			if strings.TrimSpace(line) != "" {
				sample = line
				break
			}
		}
		if sample != "" && !strings.Contains(sample, delim) {
			return nil, fmt.Errorf("%w: 文件 '%s' 未发现「%s」分隔符，请检查设置中的数据源分隔符是否正确",
				errs.ErrFileRead, filePath, sepDisplay(delim))
		}
	}

	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = []rune(delim)[0]
	cr.LazyQuotes = true // Python csv.reader 默认宽松引号处理
	all, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s': %v", errs.ErrFileRead, filePath, err)
	}
	rows := all
	// 跳过前导空行（声明行前常见空行）
	for len(rows) > 0 && IsEmptyRow(rows[0]) {
		rows = rows[1:]
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s'：文件为空", errs.ErrFileRead, filePath)
	}
	skipped := false
	if opts.SkipDeclaration && len(rows) >= 2 && IsDeclarationFirstRow(rows[0], rows[1], opts.DeclarationKeywords) {
		rows = rows[1:]
		skipped = true
		for len(rows) > 0 && IsEmptyRow(rows[0]) {
			rows = rows[1:]
		}
	}
	headers := make([]string, 0, len(rows[0]))
	for _, cell := range rows[0] {
		headers = append(headers, strings.TrimSpace(cell))
	}
	hasHeader := false
	for _, h := range headers {
		if h != "" {
			hasHeader = true
			break
		}
	}
	if !hasHeader {
		return nil, fmt.Errorf("%w: 无法读取文件 '%s'：首行为空，无法确定表头", errs.ErrFileRead, filePath)
	}
	// 列数校验：仅多列表头时启用；某行列数超过表头即格式异常（尾空字段被 csv 截断属正常）
	badRows := make([][2]int, 0)
	for i, row := range rows[1:] {
		if len(headers) >= 2 && len(row) > len(headers) {
			badRows = append(badRows, [2]int{i + 2, len(row)})
		}
	}
	if len(badRows) > 0 {
		firstLine, cols := badRows[0][0], badRows[0][1]
		return nil, fmt.Errorf("%w: 文件 '%s' 第 %d 行有 %d 列，超出表头 %d 列：请检查文件是否为纯文本表格，并在设置中选择正确的数据源分隔符",
			errs.ErrFileRead, filePath, firstLine, cols, len(headers))
	}
	stem := filepath.Base(filePath)
	if ext := filepath.Ext(stem); ext != "" {
		stem = strings.TrimSuffix(stem, ext)
	}
	data := &SheetData{
		SheetName:          stem,
		Headers:            headers,
		DeclarationSkipped: skipped,
	}
	for _, row := range rows[1:] {
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

// CsvWriter 基于标准库 csv 的写入器（编码按设置，含 BOM 支持）。
type CsvWriter struct{}

// WriteExport 写入 csv 文件：首行表头 + 数据行，编码按 request.encoding。
func (w *CsvWriter) WriteExport(request *WriteRequest) error {
	sep := request.CSVSeparator
	if sep == "" {
		sep = ","
	}
	runes := []rune(sep)
	if len(runes) != 1 {
		return fmt.Errorf("%w: csv 分隔符必须为单个字符，收到 %q", errs.ErrFileWrite, sep)
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = runes[0]
	writer.UseCRLF = true // Python csv.writer(lineterminator="\r\n")
	if err := writer.Write(request.Headers); err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}
	for _, row := range request.DataRows {
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}

	encoded, err := encodeForWrite(buf.Bytes(), request.Encoding)
	if err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}
	if err := os.WriteFile(request.FilePath, encoded, 0o644); err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}
	return nil
}

// encodeForWrite 按 csv 编码映射输出字节（unicode 即 UTF-16 LE 带 BOM）。
func encodeForWrite(utf8Bytes []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "", "utf-8-bom":
		return append([]byte{0xEF, 0xBB, 0xBF}, utf8Bytes...), nil
	case "utf-8":
		return utf8Bytes, nil
	case "gbk":
		out, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), utf8Bytes)
		return out, err
	case "utf-16", "unicode":
		enc := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder()
		out, _, err := transform.Bytes(enc, utf8Bytes)
		return out, err
	}
	return nil, fmt.Errorf("%w: 未知编码 %s", errs.ErrFileWrite, encoding)
}
