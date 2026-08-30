package excel

import (
	"fmt"
	"os"
	"strings"

	"wps-enhancer-go/internal/errs"
)

// TxtWriter 文本导出写入器（行内用分隔符连接，末尾换行）。
type TxtWriter struct{}

// WriteExport 写入 txt 文件（编码按 request.encoding）。
func (w *TxtWriter) WriteExport(request *WriteRequest) error {
	sep := request.Separator
	if sep == "" {
		sep = " "
	}
	lines := []string{strings.Join(request.Headers, sep)}
	for _, row := range request.DataRows {
		lines = append(lines, strings.Join(row, sep))
	}
	text := strings.Join(lines, "\n") + "\n"

	encoded, err := encodeForWrite([]byte(text), request.Encoding)
	if err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}
	if err := os.WriteFile(request.FilePath, encoded, 0o644); err != nil {
		return fmt.Errorf("%w: 无法写入文件 '%s': %v", errs.ErrFileWrite, request.FilePath, err)
	}
	return nil
}
