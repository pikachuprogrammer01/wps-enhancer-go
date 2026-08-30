package core

// ExportRow 单条导出数据行（一个手机号占一行；无手机映射时一源行一行）。
type ExportRow struct {
	Values         []string `json:"values"`           // 与模板 enabled 列一一对应的值列表
	PhoneValid     bool     `json:"phone_valid"`      // 该行手机号是否通过校验（未启用校验或空号恒为 true）
	SourceRowIndex int      `json:"source_row_index"` // 对应源表数据行号，从 1 开始计数，表头行不计
	MergeSpan      int      `json:"merge_span"`       // 同一源行拆分出的行数（1=无拆分）
	IsFirstOfSplit bool     `json:"is_first_of_split"` // 拆分组首行（姓名合并起始行）
}

// PreviewData 数据转换的完整预览结果。
type PreviewData struct {
	Rows           []ExportRow `json:"rows"`
	InvalidCount   int         `json:"invalid_count"`
	InvalidSummary []string    `json:"invalid_summary"`
}
