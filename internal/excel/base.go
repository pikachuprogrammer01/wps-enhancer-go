package excel

import "strings"

// IsEmptyRow 判断一行是否全空（nil/空串/纯空白）。
func IsEmptyRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// IsDeclarationFirstRow 判断首行是否为导出声明行（防误判优先，纯函数）。
//
// 规则（按序，命中即停）：
//  1. 首行全空 → 声明行（前导空行）
//  2. 首行仅 1 个非空单元格且次行非空 ≥ 2 → 声明行（结构判定，不依赖关键词）
//  3. 首行仅 1 个非空单元格且内容命中关键词 → 声明行（单格关键词判定）
//  4. 多格行（非空 ≥ 2）→ 不判为声明行（避免正常表头含「声明/数据来源」等词被误判）
func IsDeclarationFirstRow(firstRow, secondRow []string, keywords []string) bool {
	firstNonEmpty := nonEmptyCells(firstRow)
	if len(firstNonEmpty) == 0 {
		return true
	}
	if len(firstNonEmpty) != 1 {
		return false // 多格行不参与关键词判定（防误判）
	}
	cell := firstNonEmpty[0]
	if len(keywords) > 0 {
		for _, k := range keywords {
			if strings.Contains(cell, k) {
				return true
			}
		}
	}
	return len(nonEmptyCells(secondRow)) >= 2
}

// nonEmptyCells 返回行中非空单元格的 strip 后值列表。
func nonEmptyCells(row []string) []string {
	out := make([]string, 0)
	for _, c := range row {
		if s := strings.TrimSpace(c); s != "" {
			out = append(out, s)
		}
	}
	return out
}
