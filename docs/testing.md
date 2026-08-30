# Go 版测试规范

> 适用范围：本仓库（独立主仓）。快速入口：**[README.md](../README.md)** §测试。

## 测试分层

| 层级 | 目录 | 职责 | CI |
|------|------|------|-----|
| L0 纯函数 | `internal/core`, `internal/template` | 输入→输出，无 IO | PR 必过 |
| L1 IO | `internal/excel`, `internal/settings` | 读写夹具、settings 兼容 | PR 必过 |
| L2 集成 | `internal/app` | Wails 命令编排 | PR 必过 |
| L3 管道 E2E | `internal/e2e` | 读表→映射→导出→读回 | PR + 发版 |
| L4 契约 | `internal/license`, `internal/updater` | httptest 模拟外部 API | PR 必过 |

GUI（WebView 点击、系统文件对话框）不做自动化；由 L2+L3 覆盖业务逻辑。

## 依赖

| 库 | 用途 |
|----|------|
| `testing`（标准库） | 测试入口 |
| `github.com/google/go-cmp/cmp` | 结构体/切片 diff 断言 |
| `net/http/httptest`（标准库） | license/updater 契约测试 |

暂不引入：gomock、testcontainers、Playwright。

## 前置：安装 Task（跑 `task test` 必需）

`task` 来自 **[go-task](https://taskfile.dev/)**，不是 Go 自带命令。未安装时会出现 `task: command not found`。

```bash
# macOS（推荐）
brew install go-task

# 或任意平台（需已配置 GOPATH/bin 在 PATH）
go install github.com/go-task/task/v3/cmd/task@latest

which task   # 应有路径
```

然后在**本仓库根目录**执行：

```bash
task test
```

不装 Task 时直接用下方 `go test` 等价命令即可。

## 本地命令

在本仓库根目录下：

```bash
# 单元 + 集成（日常开发、PR 必过）
task test
# 或（无需 Task）
go test ./internal/... -count=1 -timeout 120s

# 覆盖率摘要
task test:cover

# 覆盖率门禁（发 PR 前推荐）
task test:cover:gate

# 管道 E2E（发版前）
task test:e2e

# 全部（单元 + E2E）
task test:all

# 静态检查
go vet ./internal/...
```

### Golden 夹具

```bash
go run scripts/gen_golden.go
go test ./internal/core/ -run TestGoldenFileInSync
go test ./internal/core/ -run TestGolden
```

### 企查查真实夹具（本地可选，不阻塞 CI）

```bash
QCC_FIXTURE_DUMP=1 go test ./internal/e2e/ -run TestQCC_BootstrapDump -v
go test ./internal/e2e/ -run TestQCC -v
```

夹具说明：`internal/e2e/testdata/qcc/README.md`。

## 写法约定

1. **命名**：`Test<标识>_<场景>`，函数顶部一行中文注释说明职责。
2. **辅助函数**：必须 `t.Helper()`；临时目录只用 `t.TempDir()`。
3. **表驱动**：多场景用 `[]struct{}` + `t.Run`；纯函数测试可 `t.Parallel()`。
4. **断言**：复杂结构用 `cmp.Diff`；简单值用 `t.Errorf`。
5. **分层禁跨界**：`core`/`template` 测试禁止读写文件；`excel` 不测业务规则。
6. **Golden 文件**：放 `docs/migration/testdata/`，变更须在 PR 说明生成命令。
7. **慢/网络测试**：`testing.Short()` 跳过或 `//go:build integration` 标签。

### 示例（L0 表驱动）

```go
func TestValidatePhone_边界(t *testing.T) {
    tests := []struct {
        name  string
        phone string
        want  bool
    }{
        {name: "11位合法", phone: "13800000000", want: true},
        {name: "过短", phone: "138", want: false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := ValidatePhone(tt.phone); got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### 示例（L2 集成）

```go
func testApp(t *testing.T) *App {
    t.Helper()
    return NewApp(t.TempDir())
}
```

## 夹具目录

| 路径 | 用途 |
|------|------|
| `docs/migration/testdata/processor_golden.json` | core 纯函数 golden |
| `docs/migration/testdata/read_fixtures/` | excel 读取基线 |
| `internal/e2e/`（测试内生成） | 管道 E2E 源表（excelize 动态写入） |

禁止测试读写用户真实 `~/Library/Application Support/WPSEnhancer`。

## 验收标准

### PR 合并门禁

- `go test ./internal/... -count=1` 全绿
- `go vet ./internal/...` 无告警
- `cd frontend && pnpm build` 成功

### 发版门禁

- 上述 + `go test ./internal/e2e/...`
- `go build -o bin/wps-enhancer-go .` 成功

### 覆盖率目标（`internal/`，参考线）

| 包 | 理想目标 | CI 门禁（`check_coverage.sh`） |
|----|---------|-------------------------------|
| `internal/core` | ≥ 90% | ≥ 88% |
| `internal/template` | ≥ 85% | ≥ 85% |
| `internal/settings` | ≥ 75% | ≥ 74% |
| `internal/license` | ≥ 70% | ≥ 70% |
| `internal/app` | ≥ 70% | ≥ 30%（逐步上调） |
| `internal/excel` | ≥ 75% | ≥ 55%（逐步上调） |
| **合计** | ≥ 75% | ≥ 62% |

覆盖率不作为唯一指标；L3 管道场景必须覆盖导入主流程。

### 新功能 Definition of Done

1. L0/L1 核心逻辑有表驱动测试
2. 新增 Wails 命令在 `app_test.go` 或 `e2e` 有集成覆盖
3. bindings 手工同步（FNV-1a ID）
4. 不降低已有包覆盖率

## L3 管道 E2E 必覆盖场景

见 `internal/e2e/pipeline_test.go`（合成源表）与 **`internal/e2e/testdata/qcc/`**（真实企查查源表，分阶段）：

1. 企查查声明行剔除
2. 从表头建模板 + 四级匹配
3. 预览行数 / invalid_count
4. 导出 xlsx / csv / vcf / txt 并读回
5. SuggestTemplates / ApplyTemplate
6. phone_merge 合并单元格
7. 会话列：`SessionMatch` / `PreviewWithColumns` / `ExportWithColumns`（不落盘模板）

### 企查查真实夹具（分阶段）

| 阶段 | 测试 | 说明 |
|------|------|------|
| Bootstrap | `TestQCC_BootstrapDump` | `QCC_FIXTURE_DUMP=1` 探测表头/行数，填写 manifest |
| Phase 1 | `TestQCC_Phase1_Read` | 读入冒烟 |
| Phase 2 | `TestQCC_Phase2_Mapping` | 模板 + 预览 |
| Phase 3 | `TestQCC_Phase3_Export` | 导出读回 |

夹具目录：`internal/e2e/testdata/qcc/`（`case_01.xlsx`、`case_02.xlsx` + `manifest.json`）。  
未放入文件或 `expect.ready=false` 时自动 Skip，不阻塞 CI。详见该目录 `README.md`。

## CI

GitHub Actions：**`.github/workflows/test.yml`**。

- `test` job：`go test ./internal/...` + vet + 覆盖率门禁 + frontend build
- `e2e` job：`go test ./internal/e2e/...`

## 演进路线

| 阶段 | 内容 | 状态 |
|------|------|------|
| P0 | go-cmp + CI + Taskfile | ✅ |
| P1 | `internal/e2e` 管道 + 企查查夹具框架 | ✅ |
| P2 | app Suggest/Apply 集成测试 | ✅ |
| P3 | 覆盖率门禁 `scripts/check_coverage.sh` | ✅ |
| P4 | `scripts/gen_golden.go` + `TestGoldenFileInSync` | ✅ |
| P5 | 可选 testify | 待评估 |

### Golden 夹具维护

```bash
go run scripts/gen_golden.go                          # 重新生成 processor_golden.json
go test ./internal/core/ -run TestGoldenFileInSync    # 校验未漂移
```
