package excel

import (
	"fmt"
	"strings"

	"wps-enhancer-go/internal/errs"
)

// ReadOptions read_sheet 的可选参数（Python 版 read_sheet 的 keyword 参数收敛为 struct）。
type ReadOptions struct {
	SkipDeclaration     bool
	DeclarationKeywords []string
	Separator           string // 仅 csv/txt 生效（空=自动检测）
	Encoding            string // 仅 csv/txt 生效（空=自动检测）
}

// SheetSummary 单个 Sheet 的名称与数据行数（下拉选择时展示）。
type SheetSummary struct {
	Name string `json:"name"`
	Rows int    `json:"rows"`
}

// Reader 文件读取抽象接口（features 只能通过此层访问文件）。
type Reader interface {
	// GetSheetNames 读取文件中所有 Sheet 名称（csv 返回单个文件名）。
	GetSheetNames(filePath string) ([]string, error)
	// GetSheetSummaries 读取所有 Sheet 的名称与数据行数。
	GetSheetSummaries(filePath string) ([]SheetSummary, error)
	// ReadSheet 读取指定 Sheet 的表头和数据行；SkipDeclaration 时按声明规则剔除首行声明。
	ReadSheet(filePath, sheetName string, opts ReadOptions) (*SheetData, error)
}

// Writer 文件写入抽象接口。
type Writer interface {
	// WriteExport 写入导出数据；xlsx/xls 含合并与标红，csv/vcf/txt 按 request 参数。
	WriteExport(request *WriteRequest) error
}

// GetReader 根据文件扩展名返回对应的 Reader 实例。
func GetReader(filePath string) (Reader, error) {
	switch suffix(filePath) {
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return &XlsxReader{}, nil
	case ".xls":
		return &XlsReader{}, nil
	case ".csv":
		return &CsvReader{}, nil
	}
	return nil, fmt.Errorf("%w: 不支持的文件格式：%s（仅支持 xlsx / xls / csv）", errs.ErrFileRead, suffix(filePath))
}

// GetWriter 根据文件扩展名返回对应的 Writer 实例。
func GetWriter(filePath string) (Writer, error) {
	switch suffix(filePath) {
	case ".xlsx", ".xlsm", ".xltx", ".xltm":
		return &XlsxWriter{}, nil
	case ".csv":
		return &CsvWriter{}, nil
	case ".vcf":
		return &VcfWriter{}, nil
	case ".txt":
		return &TxtWriter{}, nil
	}
	return nil, fmt.Errorf("%w: 不支持的输出格式：%s", errs.ErrFileWrite, suffix(filePath))
}

// suffix 返回小写扩展名（含点）；无扩展名返回空串。
func suffix(filePath string) string {
	idx := strings.LastIndex(filePath, ".")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(filePath[idx:])
}
