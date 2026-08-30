package excel

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"wps-enhancer-go/internal/errs"
)

// vcf 核心字段与内置列 key 的映射（VERSION:3.0）。
var keyToVCF = map[string]string{
	"name":    "FN",
	"phone":   "TEL;TYPE=CELL",
	"company": "ORG",
	"website": "URL",
}

// maxLineBytes vCard 3.0 折叠行最大字节数。
const maxLineBytes = 75

// ResolveVCFProp 解析字段 key 对应的 vCard 属性名（核心四字段优先，其次自定义映射，默认 NOTE）。
func ResolveVCFProp(key string, extra map[string]string) string {
	if p, ok := keyToVCF[key]; ok {
		return p
	}
	if extra != nil {
		if p := strings.TrimSpace(extra[key]); p != "" {
			return p
		}
	}
	return "NOTE"
}

// BuildVCFLines 按 vcf 规则生成文件行（预览与写入共用，保证所见即所得）。
// 仅导出 vcf_fields 中已映射且有值的字段；姓名应用前后缀；扩展内置列按 VCFProps/NOTE 写出。
func BuildVCFLines(request *WriteRequest) ([]string, error) {
	if request.FieldKeys == nil {
		return nil, fmt.Errorf("%w: vcf 导出需要 field_keys（模板列语义 key）", errs.ErrFileWrite)
	}
	allowed := request.VCFFields
	if allowed == nil {
		allowed = request.FieldKeys
	}
	type selectedCol struct {
		idx  int
		key  string
		prop string
	}
	selected := make([]selectedCol, 0)
	for i, key := range request.FieldKeys {
		inAllowed := false
		for _, a := range allowed {
			if a == key {
				inAllowed = true
				break
			}
		}
		if !inAllowed {
			continue
		}
		selected = append(selected, selectedCol{
			idx:  i,
			key:  key,
			prop: ResolveVCFProp(key, request.VCFProps),
		})
	}
	lines := make([]string, 0)
	for _, row := range request.DataRows {
		lines = append(lines, "BEGIN:VCARD")
		lines = append(lines, "VERSION:3.0")
		for _, col := range selected {
			value := strings.TrimSpace(row[col.idx])
			if value == "" {
				continue
			}
			if col.key == "name" {
				value = request.VCFNamePrefix + value + request.VCFNameSuffix
			}
			lines = append(lines, foldLine(col.prop+":"+escapeVCF(value)))
		}
		lines = append(lines, "END:VCARD")
		lines = append(lines, "")
	}
	return lines, nil
}

// escapeVCF 转义 vCard 文本特殊字符（\ , ; 与 CR/LF）。
func escapeVCF(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		",", "\\,",
		";", "\\;",
		"\r", "\\r",
		"\n", "\\n",
	)
	return replacer.Replace(value)
}

// foldLine 按 vCard 3.0 规则折叠超过 75 字节的行（续行以空格开头）。
// 与 Python 版一致：按 UTF-8 字节切片，切分边界的不完整字节用 errors="ignore" 语义丢弃。
func foldLine(line string) string {
	if len(line) <= maxLineBytes {
		return line
	}
	b := []byte(line)
	parts := make([]string, 0, len(b)/maxLineBytes+1)
	for i := 0; i < len(b); i += maxLineBytes {
		end := i + maxLineBytes
		if end > len(b) {
			end = len(b)
		}
		parts = append(parts, pyDecodeIgnore(b[i:end]))
	}
	return strings.Join(parts, "\r\n ")
}

// pyDecodeIgnore 精确模拟 Python bytes.decode("utf-8", errors="ignore")：丢弃所有非法字节序列。
func pyDecodeIgnore(b []byte) string {
	var sb strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			b = b[1:] // 非法字节，忽略
			continue
		}
		sb.Write(b[:size])
		b = b[size:]
	}
	return sb.String()
}

// VcfWriter vCard 3.0 写入器：一个手机号一条 vCard，字段按设置过滤。
type VcfWriter struct{}

// WriteExport 写入 vcf 文件（UTF-8，行结束 CRLF）。
func (w *VcfWriter) WriteExport(request *WriteRequest) error {
	lines, err := BuildVCFLines(request)
	if err != nil {
		return err
	}
	// 对齐 Python 版写入：每行以 \r\n 结尾（build_vcf_lines 含尾部空行）
	content := strings.Join(lines, "\r\n")
	if err := os.WriteFile(request.FilePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}
	return nil
}
