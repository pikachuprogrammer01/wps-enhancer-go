# 02 — internal/excel：文件 IO 层迁移

> 来源文件：`core/file_io/`（base.py + xlsx/xls/csv/vcf/txt 六个 handler，共 ~31KB）
> 目标结构：`internal/excel/`
> 上游规格：`docs/template_system.md`、`features/contacts_import/SPEC.md`（导出行为）

---

## 1. 迁移目标

```
internal/excel/
├── model.go     # SheetData/WriteRequest/MergeRange/CellStyle + is_empty_row/is_declaration_first_row
├── reader.go    # Reader 接口 + GetReader（扩展名路由）
├── writer.go    # Writer 接口 + GetWriter
├── xlsx.go      # XlsxReader/XlsxWriter（excelize）
├── xls.go       # XlsReader（extrame-xls + 防护层）※ 见 §4
├── csv.go       # CsvReader/CsvWriter（含 txt 复用）
├── vcf.go       # VcfWriter（自写，简单格式）
├── txt.go       # TxtWriter
└── *_test.go    # 对照夹具测试（golden 文件）
```

**铁律**：本包只做格式读写，**不含任何业务逻辑或数据转换**（与 Python 版一致）。

---

## 2. 接口对照

| Python（base.py） | Go |
|-------------------|----|
| `BaseReader.get_sheet_names(path) -> List[str]` | `Reader.GetSheetNames(path string) ([]string, error)` |
| `BaseReader.get_sheet_summaries(path) -> List[(name, rows)]` | `Reader.GetSheetSummaries(path string) ([]SheetSummary, error)` |
| `BaseReader.read_sheet(path, sheet_name, skip_declaration, declaration_keywords, separator, encoding) -> SheetData` | `Reader.ReadSheet(path string, opts ReadOptions) (*SheetData, error)` |
| `BaseWriter.write_export(request: WriteRequest) -> None` | `Writer.WriteExport(req *WriteRequest) error` |
| `get_reader(path) / get_writer(path)` | `GetReader(path string) (Reader, error)` / `GetWriter(path string) (Writer, error)` |
| `is_empty_row(row)` / `is_declaration_first_row(...)` | `IsEmptyRow(row []string) bool` / `IsDeclarationFirstRow(first, second []string, keywords []string) bool` |

**设计差异**：`read_sheet` 参数过多，Go 版收敛为 `ReadOptions` struct（`SkipDeclaration bool`、`DeclarationKeywords []string`、`Separator string`、`Encoding string`），新增调用场景不改签名——这是 Go 版的刻意改进，行为不变。

**声明行判定**（`is_declaration_first_row`）：纯函数直译，规则按序命中即停（全空→单格+次行≥2→单格关键词→多格行不判），**必须逐字保留**，这是企查查导出兼容的关键。

---

## 3. 格式实现对照

| 格式 | Python | Go 库 | 说明 |
|------|--------|-------|------|
| xlsx 读 | openpyxl | excelize `excelize.OpenFile` | 按 `GetRows` 读，行为对齐见 §5 |
| xlsx 写 | openpyxl | excelize `NewFile` + `SetCellValue`/`MergeCell`/`SetCellStyle` | 合并单元格、标红背景色（`CellStyle` 的 `background_color`） |
| xls 读 | xlrd | extrame-xls（防护层） | 见 §4 |
| xls 写 | xlwt | **放弃**，统一 xlsx 输出 | 产品层引导，见 §6 |
| csv 读/写 | csv 模块 | 标准库 `encoding/csv` | 注意：Python csv 默认 `\r\n` 行尾 + `"` 引号规则，Go 的 `csv.Writer` 默认 `\n`——**必须设 `w.UseCRLF = true` 对齐** |
| vcf 写 | 自写 | 自写 | 简单文本格式，`vcf_fields`/`vcf_name_prefix/suffix` 字段语义照搬 |
| txt 写 | 自写 | 自写 | `separator` 行内分隔符 |

---

## 4. xls 读取：防护层（重要）

`github.com/extrame/xls` 是 Go 生态唯一可用的 xls 读取库，但已停滞（2023 年停更），有已知 bug（open issues 实证）：

| issue | 症状 | 防护 |
|-------|------|------|
| #66 / #70 | 空行/特定文件上 **panic**（nil pointer） | `defer recover()` 转为 error |
| #101 | 数字被误判为日期（"1900-01-01T00:00:00Z"） | 读取后按单元格类型二次判定：`NUMERIC` 类型且值≈整数 → 输出 `strconv.FormatInt(int64(v), 10)` |
| #99 | 部分列读不到导致数据错位 | 行长度不足时补空串到表头长度，不直接丢弃 |
| #67 | 文本值过多时丢单元格 | 无法根治，读取后做行长度校验，异常时返回错误并提示转 xlsx |

**防护封装**（`xls.go` 核心，业务层永远感知不到 extrame-xls）：

```go
// ReadXLS 读取 xls 文件：捕获 extrame/xls 已知 panic，日期单元格二次解析
func (r *XlsReader) ReadSheet(path string, opts ReadOptions) (data *SheetData, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%w: xls 解析异常: %v", ErrFileRead, rec)
		}
	}()
	// 1. xls.Open(path, "utf-8")  → 失败即 ErrFileRead
	// 2. 逐 sheet 逐行提取，空行跳过（IsEmptyRow），行短补空串
	// 3. 日期二次解析（#101 防护）：数值型单元格 → 整数原样输出
	return data, nil
}
```

**版本锁定**：extrame-xls 的 pkg.go.dev 发布版与 GitHub 不一致（issue #97），**go.mod 直接锁 GitHub commit**（`go get github.com/extrame/xls@<commit-hash>`），不用版本号。

---

## 5. 关键行为对齐：数字单元格 → 字符串

processor 层全部逻辑基于字符串（手机号 11 位、身份证 18 位、科学计数法检测）。Python 的 `str(cell)` 与 excelize 的默认输出**不一致**，必须在读层统一：

| 单元格内容 | Python openpyxl 行为 | excelize 行为 | Go 读层必须输出 |
|-----------|---------------------|---------------|----------------|
| `13812345678`（数字） | `"13812345678"` | `float64` → `"1.3812345678e+10"` ❌ | `"13812345678"`（整数格式） |
| `1.38123E+10`（科学计数法文本） | `"1.38123E+10"` | 字符串原样 | `"1.38123E+10"` |
| 身份证 `110101199003077777` | `"110101199003077777"`（openpyxl 按文本/数字精度） | 可能丢精度 | 数字型且位数>15 时原样输出，不做浮点 |

**规则**：读层用 excelize 的 `GetCellValue` 后**检查单元格原始类型**（`GetCellType`），`CellTypeNumeric` 且值为整数 → `strconv.FormatInt`；否则原样字符串。此规则在 L1 的 golden 对照测试里用真实样例锁死。

---

## 6. 写 xls 的放弃决策（产品层处理）

- **现状**：Python xlwt 写 xls；迁移后统一写 xlsx
- **影响**：WPS/Excel 2007+ 全支持 xlsx，现代用户无感知；老系统对接场景极少
- **产品引导**：读取 `.xls` 失败时提示「建议在 WPS/Excel 中另存为 xlsx 后重试」；导出只提供 xlsx
- **不做**：不引入商业 LibXL、不绑 LibreOffice（体积/许可不可控）

---

## 7. 迁移步骤（L2）

1. `model.go` + `reader.go`/`writer.go` 接口（照 base.py 直译）
2. 各格式 handler 逐个实现，**每个格式配 golden 夹具**：
   - 用 Python 版生成 `testdata/*.xlsx`、`*.xls`、`*.csv`、`*.vcf`、`*.txt` 输入文件 + 期望输出 JSON
   - Go 测试读夹具断言一致（`testdata/` 目录进仓库）
3. csv 注意 `UseCRLF`、xlsx 注意数字转字符串规则（§5）
4. 验证：`go test ./internal/excel/...` 全绿

## 8. 验收标准（L2 完成定义）

- [ ] 六种格式读写测试全绿（golden 对照）
- [ ] xls 防护层测试：构造空行文件、日期型数字文件、超长文本文件，验证不 panic、不错位
- [ ] `go vet ./...` 无警告
- [ ] 声明行判定行为与 Python 版逐规则一致（≥10 组用例对照）

## 9. 注意事项

1. **路径大小写**：`GetReader` 的扩展名判定要 `strings.ToLower`（Python 版 `suffix.lower()` 同理）
2. **大文件**：excelize 全量加载，10 万行 xlsx 内存 ~200MB 量级；当前场景可接受，不做流式（`ponytail:` 注释标注上限：超过 50 万行需流式读取）
3. **编码**：csv 读取支持 UTF-8 与 GBK 自动检测（Python 版已实现）——Go 用 `golang.org/x/text/encoding/simplifiedchinese`（唯一允许的 x/ 依赖，或自写 GBK 检测；实现细节见 csv.go）
4. **错误类型**：读写失败统一 `ErrFileRead`/`ErrFileWrite` 包装，不做格式特有错误类型
