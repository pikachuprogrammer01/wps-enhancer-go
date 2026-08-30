# 03 — internal/template + settings + logger：基础设施迁移

> 来源文件：`core/template/`（config/manager/matcher/store，共 ~14KB）+ `core/settings.py`（10.8KB）+ `core/logger.py`（6.1KB）+ `core/app_paths.py`
> 目标结构：`internal/template/`、`internal/settings/`、`internal/logger/`
> 上游权威：`docs/template_system.md`（模板系统设计文档，行为以它为准）

---

## 1. 迁移目标

```
internal/template/
├── config.go    # Template 数据结构（← template/config.py）
├── matcher.go   # 匹配引擎（← template/matcher.py，ColumnMatch）
├── manager.go   # 模板管理器（← template/manager.py）
├── store.go     # 模板存储（← template/store.py，JSON 持久化）
└── *_test.go

internal/settings/
├── settings.go  # AppSettings + settings.json 读写（← core/settings.py）
└── paths.go     # 数据目录/日志目录解析（← core/app_paths.py + mac_paths.py）

internal/logger/
├── logger.go    # slog 封装 + 按天轮转 writer（← core/logger.py）
└── logger_test.go
```

---

## 2. 模板系统（internal/template/）

### 2.1 数据结构

| Python | Go | 说明 |
|--------|-----|------|
| `Template`（config.py） | `type Template struct` | 字段按 `docs/template_system.md` 定义；**JSON tag 与 Python 序列化字段名完全一致**（`template/` 目录下的模板文件是老用户资产，必须零迁移读取） |
| `ColumnMatch`（matcher.py） | `type ColumnMatch struct` | 匹配结果：列名、置信度、匹配方式 |
| `TemplateStore`（store.py） | `type Store struct` | 增删改查 + `ListTemplates` 容错 |

### 2.2 关键行为（必须保留）

1. **存储格式**：模板文件 = `template/<模板名>` 目录（文件名 = 模板名），内含 JSON。**JSON 结构与字段名必须与 Python 版完全一致**，老用户模板直接可用——这是兼容性红线。
2. **匹配引擎**（matcher.py）：列匹配算法（关键词/语义匹配/置信度排序）逐行直译，`docs/template_system.md` 是权威行为定义，**先写测试后迁移**。
3. **容错读取**（store.py 的 `list_templates`）：损坏模板跳过不崩溃——Go 版对应：`ListTemplates() ([]Template, error)` 内部对单个模板 `recover`/错误判断后跳过，**不把单个损坏模板的错误上抛**（SPEC 定义的行为，Python 版是唯一允许捕获异常的地方，Go 版同样允许）。
4. **默认模板**：应用自带内置模板（如有），迁移时作为 `embed.FS` 打进二进制（`go:embed`），省去运行时文件分发。

### 2.3 迁移步骤

1. 先读 `docs/template_system.md` 全文，把模板 JSON 样例拷为测试夹具
2. `config.go` → `matcher.go`（匹配算法纯函数化，无 I/O）→ `store.go`（I/O）→ `manager.go`（编排）
3. `template/` 目录路径由 `settings.Paths()` 提供（见 §3），保持目录名 `template` 不变

---

## 3. 全局设置（internal/settings/）

### 3.1 AppSettings

`core/settings.py` 定义 `AppSettings` dataclass（含 `log_auto_clean`、`log_retain_days` 等全部字段）。Go 版：

```go
// settings.go
type AppSettings struct {
    LogAutoClean bool `json:"log_auto_clean"`   // 字段名 = Python 版 settings.json 的 key
    LogRetainDays int `json:"log_retain_days"`
    // ... 其余字段逐一对照 core/settings.py
}

// Load 读取 settings.json；文件缺失/损坏 → 返回默认值 + 重建（与 Python 版行为一致）
func Load() (*AppSettings, error)
func Save(s *AppSettings) error
```

**兼容红线**：settings.json 的 key 名与 Python 版**逐字一致**，老用户配置零迁移。读取时未知字段忽略（Go `json.Decoder` 默认行为，天然满足"只追加不删改"的版本语义）。

**并发**：settings 是全局单例，Go 版提供 `Get()`（sync.Once 初始化）+ `Update(fn func(*AppSettings))` 写锁接口，禁止裸 map 访问。

### 3.2 路径（app_paths.py → paths.go）

| Python | Go 标准库 | 实际目录 |
|--------|----------|---------|
| `get_data_dir()` | `os.UserConfigDir()` | macOS `~/Library/Application Support/WPSEnhancer`；Windows `%AppData%/WPSEnhancer` |
| 日志目录 | `os.UserCacheDir()` | macOS `~/Library/Caches/WPSEnhancer/logs`；Windows `%LocalAppData%/WPSEnhancer/logs` |
| `template/` | `filepath.Join(configDir, "template")` | **目录名保持 `template`**（老用户资产） |
| 设置文件 | `filepath.Join(configDir, "settings.json")` | 文件名保持 `settings.json` |

`mac_paths.py` 的 macOS 特殊分支在 `os.UserConfigDir` 下不再需要（Go 标准库已处理），删除。

---

## 4. 日志（internal/logger/）

Python 版 `core/logger.py`：统一入口 + `log_call` AOP 装饰器 + 按天轮转清理（`cleanup_logs(retain_days)`）。

Go 版用标准库 `log/slog`：

```go
// logger.go — New 创建带文件输出的 slog.Logger（按天轮转）
func New(logDir string, debug bool) (*slog.Logger, error)

// logcall.go — LogCall / LogCall1 命令层进出日志（等价 Python @log_call，debug 级）
func LogCall(name string, fn func() error) error
func LogCall1[T any](name string, fn func() (T, error)) (T, error)
```

**实现要点**（**已实现，2026-08-27**）：
- 轮转 writer：`app-YYYY-MM-DD.log`
- `main.go`：启动清理 + 2s 后再清 + `time.Ticker` 每 24h
- 命令层：`ReadSheet` / `PreviewWithMapping` / `ExportWithTemplate` 已包 `LogCall`
- 日志级别：`settings.log_debug` 控制 Debug；发布默认 Info

---

## 5. 迁移步骤（L3）

1. `internal/logger`：writer + 轮转 + 清理 + 测试（构造过期文件验证清理）
2. `internal/settings`：settings.go + paths.go + 测试（临时目录读写、旧格式兼容）
3. `internal/template`：config → matcher → store → manager，测试夹具用老模板文件
4. 验证：`go test ./internal/...` 全绿

## 6. 验收标准（L3 完成定义）

- [ ] 老用户 settings.json 直接读取成功，字段值完整
- [ ] `template/` 目录下现有模板文件可被 Go 版完整读取（把仓库 `template/` 实际文件拷为夹具）
- [ ] 日志按天轮转 + 保留天数清理测试通过
- [ ] 模板匹配结果与 Python 版一致（≥10 组列名对照）

## 7. 注意事项

1. **JSON 字段名是兼容红线**：模板与 settings 的 key 必须与 Python 版逐字一致，迁移期间任何字段改名都要记录在案并双写兼容
2. **匹配算法先测后迁**：matcher 是最容易"顺手改逻辑"的地方，禁止优化，先 1:1 直译，锁行为后再谈优化
3. **settings 单例**：全部模块通过 `settings.Get()` 读取，禁止复制粘贴默认值常量
4. **日志写失败**：写日志失败只 `os.Stderr` 兜底，绝不阻塞业务（Python 版同样静默）
