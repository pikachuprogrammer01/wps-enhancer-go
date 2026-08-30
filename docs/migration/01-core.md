# 01 — internal/core：逻辑层迁移

> 来源文件：`features/contacts_import/processor.py`（16KB，纯函数）+ `core/exceptions.py`（异常体系）
> 目标结构：`internal/core/`（纯 Go，无 I/O、无 UI、无全局状态）
> 上游规格：`features/contacts_import/SPEC.md`（权威）

---

## 1. 迁移目标

```
internal/core/
├── model.go       # 数据结构：ExportRow/PreviewData/SheetData/WriteRequest/MergeRange/CellStyle
├── processor.go   # 纯函数直译：processor.py 全部函数
├── errors.go      # 错误体系：sentinel error 定义
└── processor_test.go  # ← tests/test_processor.py 直译（验收基准）
```

**铁律（与 Python 版逐条对应）**：
- 无文件读写、无网络、无 UI、无全局可变状态 → 违反即编译不过（不引入任何 I/O 依赖）
- 相同输入始终相同输出 → 表驱动测试锁死
- 错误用返回值传递（`error`），禁止 panic 表示业务失败

---

## 2. 数据结构对照（dataclass → struct）

| Python dataclass | Go struct | 备注 |
|------------------|-----------|------|
| `ExportRow` | `type ExportRow struct` | `values []string`、`phoneValid bool`、`sourceRowIndex int`、`mergeSpan int`、`isFirstOfSplit bool` |
| `PreviewData` | `type PreviewData struct` | `rows []ExportRow`、`invalidCount int`、`invalidSummary []string` |
| `SheetData`（core/file_io/base.py） | `core.SheetData` 或 `excel.SheetData` | 归属建议：放 `internal/excel`，core 引 excel 类型 |
| `WriteRequest` | `excel.WriteRequest` | 同上，含 `fieldKeys/encoding/separator/vcfFields/vcfNamePrefix/Suffix` |
| `MergeRange` / `CellStyle` | `excel.MergeRange` / `excel.CellStyle` | 同上 |

**命名约定**：Go 字段用驼峰（`PhoneValid`），JSON 序列化时用 tag 保持与 Python 端一致（如有导出场景）。

---

## 3. 纯函数直译对照表（processor.py → processor.go）

先列出 processor.py 全部公开函数（迁移前用 `grep '^def ' processor.py` 核对完整清单），典型对应：

| Python 函数 | Go 函数 | 关键转换点 |
|-------------|---------|-----------|
| `detect_truncated_numbers(sheet_data)` | `DetectTruncatedNumbers(sd *excel.SheetData) []string` | ✅ 已实现；正则直译 `_SCI_RE` / `_LONG_ZERO_RE`；前端读表后弹窗（可继续/中止） |
| `validate_phone(phone)` | `ValidatePhone(p string) bool` | 逻辑直译，`_VALID_SECOND_DIGITS` → `var validSecondDigits = "3456789"` + `strings.ContainsRune` |
| 数据转换主流程（`preview`/`export` 相关，见 SPEC） | `BuildPreview(...)` / `BuildExportRows(...)` | 输入改为显式参数（`template.Template`、`excel.SheetData`、`settings.AppSettings`）——**Python 版从 settings 读默认值的地方，Go 版全部改为参数传入**，呼应"函数所有依赖通过参数传入"铁律 |
| 手机号拆分/合并逻辑（SPEC 规则） | 对应函数 | 逐行直译，边界条件写测试 |

**正则差异注意**：Python `re` 与 Go `regexp` 语法 99% 兼容，但：Go 不支持 `(?P<name>)` 命名组（用位置组）；`re.IGNORECASE` 对应 `(?i)` 内联。迁移时跑 `go vet` 的正则校验兜底。

---

## 4. 错误体系（exceptions.py → errors.go）

```go
// errors.go —— 与 core/exceptions.py 一一对应
var (
    ErrDataProcess = errors.New("data process error")   // DataProcessError
    ErrFileRead    = errors.New("file read error")      // FileReadError
    ErrFileWrite   = errors.New("file write error")     // FileWriteError
    ErrTemplate    = errors.New("template error")       // TemplateError
    ErrSettings    = errors.New("settings error")       // SettingsError
    ErrNetwork     = errors.New("network error")        // NetworkError（updater/license 用）
)
```

**用法约定**：
- 业务层：`return fmt.Errorf("%w: 第 %d 行手机号无效", ErrDataProcess, i)`
- 判定层（仅 `internal/app`）：`errors.Is(err, core.ErrDataProcess)` → 映射为前端错误码
- **与 Python 铁律一致**：只有 `internal/app`（panel 等价层）捕获并翻译错误；core/excel/template 内部不 catch，直接上抛

---

## 5. 迁移步骤（L1）

1. **建仓库骨架**：`wails init -n wps-enhancer -t vanilla`（先不引任何前端框架）
2. **建 `internal/core/`**：先写 `model.go`（struct 全部字段带注释）→ `errors.go` → `processor.go`
3. **测试直译**：把 `tests/test_processor.py` 的每个 `def test_xxx` 译为 `func TestXxx(t *testing.T)`（表驱动），**先用 Python 版跑一遍生成 golden 数据**（真实样例输入→输出），Go 测试直接断言 golden 数据
4. **验证**：`go test ./internal/core/...` 全绿
5. **临时 CLI 验证**：写一个 `cmd/probe/main.go`（验证完删除或留在 tools/），用真实 Excel 样例喂 `BuildPreview`，输出与 Python 版 `python -c` 对照一致

**工作量提示**：processor.py 16KB ≈ 500 行，Go 直译后约 700-900 行（错误处理更显式）。这是全项目**最安全**的迁移模块——纯函数 + 测试对照，行为零漂移。

---

## 6. 验收标准（L1 完成定义）

- [ ] `go test ./...` 全绿（含直译的 processor 测试）
- [ ] golden 对照：≥20 组真实/构造样例输入，Go 输出与 Python 版逐字段一致
- [ ] `wails dev` 空窗口可启动（骨架验证）
- [ ] `internal/core` 无任何 `os.`/`net.`/`encoding/json` 之外的 I/O 依赖（`go list -deps` 检查）

---

## 7. 注意事项

1. **手机号/身份证字符串语义**：Python 版依赖字符串处理（`str(cell)`），Go 版在 excel 读层保证单元格→字符串转换与 Python 一致（见 `02-file-io.md` §5），core 层不做数字转换——**core 只认 string**。
2. **不可变切片**：Go 的 slice 是引用语义，函数内不要改输入 slice；需要改就复制（`slices.Clone`）。测试里加"输入未被修改"断言。
3. **并发安全**：core 函数要能被前端并行调用（Wails 命令默认并发），不要在 core 里引入任何共享可变状态；需要缓存一律放调用方。
4. **日志**：core 层不直接打日志（Python 版的 `@log_call` 装饰器在 Go 版由 `internal/app` 的命令层统一记录进出），保持纯函数纯净。
