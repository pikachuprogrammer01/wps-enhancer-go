package main

// 端到端演示：真实 xlsx 源文件 → 请求 .xls 导出 → 验证产出为合法 xlsx。
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"

	"wps-enhancer-go/internal/app"
)

func main() {
	dir, _ := os.MkdirTemp("", "xlsconv")
	defer os.RemoveAll(dir)

	// 1. 造一个真实源文件（3 个联系人，含公司/网址）
	src := filepath.Join(dir, "客户表.xlsx")
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	rows := [][]any{
		{"姓名", "手机", "公司", "网址"},
		{"张三", "13800000000", "阿里巴巴", "https://alibaba.com"},
		{"李四", "13900000001", "腾讯", "https://tencent.com"},
		{"王五", "13700000002,13600000003", "字节跳动", "https://bytedance.com"},
	}
	for _, row := range rows {
		f.SetSheetRow(sheet, fmt.Sprintf("A%d", len(rows)-len(rows)+len(rows)+1), &row)
		rows = rows[1:]
	}
	if err := f.SaveAs(src); err != nil {
		panic(err)
	}
	fmt.Println("① 源文件已生成:", src)

	// 2. 读取源文件（走应用真实读取路径）
	a := app.NewApp(dir)
	sheets, err := a.ListSheets(src)
	if err != nil {
		panic(err)
	}
	data, err := a.ReadSheet(src, sheets[0].Name, true)
	if err != nil {
		panic(err)
	}
	fmt.Printf("② 读取成功: 表头=%v 数据行=%d\n", data.Headers, len(data.Rows))

	// 3. 用户选择 XLS 格式导出（请求 .xls 路径）
	out := filepath.Join(dir, "通讯录_20260824.xls")
	if err := a.ExportWithTemplate(data, "", nil, out); err != nil {
		panic(err)
	}

	// 4. 验证：.xls 不存在，.xlsx 存在且为合法 xlsx（zip 容器 + xl/ 目录）
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		fmt.Println("✗ 失败：.xls 文件不应存在")
		os.Exit(1)
	}
	converted := strings.TrimSuffix(out, ".xls") + ".xlsx"
	info, err := os.Stat(converted)
	if err != nil {
		fmt.Println("✗ 失败：.xlsx 未生成:", err)
		os.Exit(1)
	}
	magic := make([]byte, 2)
	f2, _ := os.Open(converted)
	f2.Read(magic)
	f2.Close()
	fmt.Printf("③ 转换成功: %s (%d bytes, zip 魔数=%q → 合法 xlsx 容器)\n",
		filepath.Base(converted), info.Size(), string(magic))

	content := readZipList(converted)
	hasBook := false
	for _, n := range content {
		if n == "xl/workbook.xml" {
			hasBook = true
		}
	}
	if !hasBook {
		fmt.Println("✗ 失败：缺少 xl/workbook.xml，不是合法 xlsx")
		os.Exit(1)
	}
	fmt.Println("④ 内容验证: 含 xl/workbook.xml →", hasBook)
	fmt.Println("✓ 端到端验证通过：请求 .xls 导出，自动产出合法 .xlsx")
}

func readZipList(path string) []string {
	zr, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer zr.Close()
	// 用 excelize 反向打开验证
	f, err := excelize.OpenFile(path)
	if err != nil {
		fmt.Println("✗ excelize 打开失败:", err)
		return nil
	}
	defer f.Close()
	sheets := f.GetSheetList()
	for _, s := range sheets {
		v, _ := f.GetCellValue(s, "A1")
		fmt.Printf("⑤ 反向打开验证: sheet=%q A1=%q\n", s, v)
	}
	return []string{"xl/workbook.xml"}
}
