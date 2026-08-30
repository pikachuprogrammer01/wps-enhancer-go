package excel

import (
	"strings"
	"testing"
)

func TestBuildVCFLinesCustomFieldAsNOTE(t *testing.T) {
	req := &WriteRequest{
		Headers:   []string{"姓名", "手机", "职位"},
		FieldKeys: []string{"name", "phone", "custom_1"},
		VCFFields: []string{"name", "phone", "custom_1"},
		VCFProps:  map[string]string{"custom_1": "TITLE"},
		DataRows:  [][]string{{"张三", "13800138000", "经理"}},
	}
	lines, err := BuildVCFLines(req)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "FN:张三") {
		t.Errorf("缺 FN: %s", joined)
	}
	if !strings.Contains(joined, "TEL;TYPE=CELL:13800138000") {
		t.Errorf("缺 TEL: %s", joined)
	}
	if !strings.Contains(joined, "TITLE:经理") {
		t.Errorf("自定义字段应按 TITLE 写出: %s", joined)
	}
}

func TestResolveVCFPropDefaultNOTE(t *testing.T) {
	if got := ResolveVCFProp("custom_9", nil); got != "NOTE" {
		t.Errorf("默认应为 NOTE, got %s", got)
	}
	if got := ResolveVCFProp("name", nil); got != "FN" {
		t.Errorf("name 应为 FN, got %s", got)
	}
}
