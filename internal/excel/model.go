// Package excel 定义文件 IO 层的数据结构（L1：仅纯数据结构，handler 在 L2 迁移）。
package excel

// SheetData 从单个 Sheet 读取的全部数据。
type SheetData struct {
	SheetName           string              `json:"sheet_name"`
	Headers             []string            `json:"headers"`
	Rows                []map[string]string `json:"rows"`
	DeclarationSkipped  bool                `json:"declaration_skipped"` // 是否跳过了首行声明（企查查导出）
}

// CellStyle 单元格样式描述。
type CellStyle struct {
	BackgroundColor *string `json:"background_color"`
}

// MergeRange 合并单元格范围（数据区 0 索引，不含表头行）。
type MergeRange struct {
	RowStart int `json:"row_start"`
	RowEnd   int `json:"row_end"`
	ColIndex int `json:"col_index"`
}

// WriteRequest 写入输出文件所需的所有信息（通用列结构）。
type WriteRequest struct {
	CSVSeparator string `json:"CSVSeparator,omitempty"` // csv 导出分隔符（单字符；空 = 逗号）
	FilePath       string                `json:"file_path"`
	Headers        []string              `json:"headers"`
	DataRows       [][]string            `json:"data_rows"`
	MergeRanges    []MergeRange          `json:"merge_ranges"`
	CellStyles     map[string]CellStyle  `json:"cell_styles"` // key 为 "(row, col)"，与 Python 版 str(tuple) 一致
	FieldKeys      []string              `json:"field_keys"`  // 与 headers 对应的语义 key（vcf 导出必需）
	Encoding       string                `json:"encoding"`    // csv/txt 输出编码
	Separator      string                `json:"separator"`   // txt 行内分隔符
	VCFFields      []string              `json:"vcf_fields"`  // vcf 导出字段（内置列 key 列表，None=全部）
	VCFProps       map[string]string     `json:"vcf_props,omitempty"` // 非核心四字段的 key → vCard 属性
	VCFNamePrefix  string                `json:"vcf_name_prefix"`
	VCFNameSuffix  string                `json:"vcf_name_suffix"`
}
