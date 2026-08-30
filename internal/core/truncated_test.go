package core

import (
	"strings"
	"testing"

	"wps-enhancer-go/internal/excel"
)

func TestDetectTruncatedNumbers(t *testing.T) {
	data := &excel.SheetData{
		Headers: []string{"姓名", "手机号", "备注"},
		Rows: []map[string]string{
			{"姓名": "张三", "手机号": "1.38123E+10", "备注": "ok"},
			{"姓名": "李四", "手机号": "13800000000", "备注": "123456789012345000"},
		},
	}
	hints := DetectTruncatedNumbers(data)
	if len(hints) < 2 {
		t.Fatalf("应检出手机号科学计数与备注长尾零，got %v", hints)
	}
	joined := strings.Join(hints, "\n")
	if !strings.Contains(joined, "手机号") || !strings.Contains(joined, "备注") {
		t.Errorf("提示应含列名: %s", joined)
	}
	if DetectTruncatedNumbers(nil) != nil {
		t.Error("nil 应返回 nil")
	}
}
