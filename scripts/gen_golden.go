//go:build ignore

// 生成 processor golden 夹具（替代 docs/migration/testdata/gen_golden.py）。
// 用法：cd wps-enhancer-go && go run scripts/gen_golden.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"wps-enhancer-go/internal/core"
)

func main() {
	out := filepath.Join("docs", "migration", "testdata", "processor_golden.json")
	cases := core.GenerateGoldenCases()
	body, err := json.MarshalIndent(core.GoldenFile{Cases: cases}, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("golden 生成完成：%d 个用例 → %s\n", len(cases), out)
}
