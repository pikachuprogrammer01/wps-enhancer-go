package core

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// TestGoldenFileInSync 断言 processor_golden.json 与 Go 生成器一致（防漂移）。
func TestGoldenFileInSync(t *testing.T) {
	path := "../../docs/migration/testdata/processor_golden.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 golden: %v", err)
	}
	var onDisk GoldenFile
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("解析 golden: %v", err)
	}
	generated := GoldenFile{Cases: GenerateGoldenCases()}
	got := normalizeGolden(generated)
	want := normalizeGolden(onDisk)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("processor_golden.json 与 Go 生成器不一致；请运行: go run scripts/gen_golden.go")
	}
}
