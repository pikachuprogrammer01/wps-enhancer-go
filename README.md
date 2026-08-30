# WPS Enhancer — Go 版

WPS 表格增强工具的 Go + Wails v3 重构版。当前功能：**Excel 批量导入通讯录**——把杂乱的客户表格一键整理成手机可导入的通讯录格式。

> 本仓库为 **WPS Enhancer 现行主仓（Go + Wails）**。Python / PyQt6 版已停止维护，见归档仓 https://github.com/pikachuprogrammer01/wps-enhancer 。  
> 文档在 `docs/`：`migration/`、`design/`、`template_system.md`、`update_sop.md`、`wps-activation-policy.md`、**`agent-license-version.md`**、**`follow-up.md`**、**`release.md`（日常发版）**、**`gitee-releases.md`（发布仓对接）**。  
> 远程：https://github.com/pikachuprogrammer01/wps-enhancer-go

## 功能特性

### 批量导入通讯录（三步向导）

1. **数据源**：选择 xlsx / xls / csv / txt 源文件与模板
2. **列映射**：自动匹配（精确 / 别名 / 手动三级），未匹配列可手动指定或留空
3. **预览与导出**：所见即所得预览，导出 vcf / xlsx / csv / txt

### 数据处理能力

- **手机号**：同一单元格多号码拆分（支持 `,` `，` `;` `；` `、` 空格 换行 `|`）、合法性校验、非法号码标红、同姓名多号码合并单元格（xlsx）；读表后检测科学计数法/超长尾零等**疑似截断**并提示（可继续或中止）
- **声明行跳过**：自动识别企查查 / 天眼查 / 爱企查等平台导出表格的声明行（关键词可配置）
- **编码兼容**：csv / txt 源自动探测编码（BOM → UTF-8 → GBK → UTF-16）与分隔符；导出编码可选（UTF-8 BOM / UTF-8 / GBK / UTF-16）
- **vcf 导出**：vCard 3.0、字段过滤（含自定义内置列 / 自填 vCard 属性）、姓名前后缀、年月日时间戳、75 字节规范折叠；导出发送「vCard 导入指南」可在设置中关闭
- **模板系统**：内置默认模板（姓名/手机/公司名/网址），支持新建、从表头一键生成、重命名、删除；别名匹配可自定义；**会话内**改列/排序，导出后再选是否写回模板

### 应用能力

- **自动更新**：自定义更新源（Gitee）优先、GitHub API 回退；下载进度、zip 完整性校验、分平台替换指引；系统代理自动探测（macOS scutil / Windows 注册表）
- **订阅授权**：RSA 验签激活码、在线激活、设备指纹绑定、断网离线放行（待在线确认）、解绑
- **日志**：按天轮转、保留天数自动清理，设置页可直接查看
- **卸载**：注册式清理项（应用本体 / 数据 / 日志 / 更新包），勾选式删除，用户数据默认保护

## 安装与使用

1. 从 [Gitee 发布仓 Releases](https://gitee.com/pikachuprogrammer01/my-software-releases/releases) 下载对应平台包（macOS `.app` zip / Windows zip 或 NSIS 安装器）
2. 首次启动后在「激活与授权」中输入激活码完成激活（未激活可试用基础功能）
3. 首页选择「Excel → 批量导入通讯录」，按三步向导完成导出
4. 导出的 .vcf 可直接导入手机通讯录；.xlsx 可回贴 WPS 表格

## 架构

```
frontend (Vue3 + Naive UI)
    │  Wails bindings
internal/app      命令层：流程编排、错误码翻译（唯一允许捕获错误的地方）
    ├── internal/core      纯函数处理层：拆分/校验/合并/预览/写出请求
    ├── internal/excel     文件 IO 层：xlsx/xls/csv/txt/vcf 读写
    ├── internal/template  模板系统：存储 + 列匹配引擎
    └── internal/settings  全局设置：settings.json（与 Python 版逐字兼容）
internal/license  订阅授权      internal/updater  自动更新
internal/logger   日志          internal/errs     统一错误定义
third_party/      vendored fork（wails / xls，本地补丁见各自目录说明）
```

约定：`internal/core` 全部为纯函数（相同输入必得相同输出）；文件 IO 层不含业务逻辑；模块间传递使用定义好的结构体；失败通过 sentinel error 逐层上抛，只在命令层翻译为前端错误码。

## 开发

环境要求：

| 工具 | 用途 | 安装 |
|------|------|------|
| Go 1.25+ | 后端编译与测试 | [go.dev/dl](https://go.dev/dl/) 或 `brew install go` |
| Node 20+ / pnpm | 前端构建 | `brew install fnm` → `fnm install 20`；`npm i -g pnpm` |
| **[Task](https://taskfile.dev/)**（`go-task`） | 跑 `Taskfile.yml`（`task test` / `task dev` 等） | **macOS：`brew install go-task`**；或 `go install github.com/go-task/task/v3/cmd/task@latest` |
| wails3 CLI（可选） | 热重载 / 打包 | `go install github.com/wailsapp/wails/v3/cmd/wails3@latest` |

> `task: command not found` → 尚未安装 Task。装好后在**本仓库根目录**执行。不装 Task 时可用下文「等价 `go test`」代替。

```bash
cd wps-enhancer-go

# 前端构建（产物经 go:embed 嵌入二进制）
cd frontend && pnpm install && pnpm build && cd ..

# 编译
go build -o bin/wps-enhancer-go .

# Wails 热重载开发（需 Task 或 wails3 CLI）
task dev
# 或
wails3 dev -config ./build/config.yml
```

## 测试

完整规范见 **[docs/testing.md](docs/testing.md)**（分层、写法、CI 门禁、企查查夹具）。

### 日常命令（推荐用 Taskfile）

前置：已安装 Task（见上表），当前目录为**本仓库根**。

| 命令 | 用途 | 何时跑 |
|------|------|--------|
| `task test` | 单元 + 集成（`internal/...`） | 日常开发、PR |
| `task test:cover` | 同上 + 覆盖率摘要 | 关注覆盖率时 |
| `task test:cover:gate` | 同上 + **覆盖率门禁**（`scripts/check_coverage.sh`） | **发 PR 前** |
| `task test:e2e` | 管道 E2E（合成夹具） | 发版前 |
| `task test:all` | 单元 + E2E 全量 | 发版前 |

等价 `go test` 写法（**无需 Task**）：

```bash
# 单元 + 集成
go test ./internal/... -count=1 -timeout 120s

# 静态检查
go vet ./internal/...

# 管道 E2E
go test ./internal/e2e/... -count=1 -timeout 300s

# 只跑某包 / 某测试
go test ./internal/core/... -run TestPreviewSummaryLine -v
go test ./internal/logger/... -run TestLogCall -v
```

### Golden 夹具

```bash
go run scripts/gen_golden.go                          # 重新生成 processor_golden.json
go test ./internal/core/ -run TestGoldenFileInSync    # 校验 golden 未漂移
go test ./internal/core/ -run TestGolden              # 跑 golden 对照
```

Golden 文件：`docs/migration/testdata/processor_golden.json`（由 `scripts/gen_golden.go` 生成，不再依赖 Python）。

### 企查查真实夹具（本地可选）

夹具目录：`internal/e2e/testdata/qcc/`（`case_01.xlsx`、`case_02.xlsx` + `manifest.json`，**不提交 git**）。

```bash
# 探测表头/行数（填写 manifest 用）
QCC_FIXTURE_DUMP=1 go test ./internal/e2e/ -run TestQCC_BootstrapDump -v

# 分阶段回归（manifest expect.ready=true 且本地有 xlsx 时才会跑）
go test ./internal/e2e/ -run TestQCC_Phase1 -v
go test ./internal/e2e/ -run TestQCC_Phase2 -v
go test ./internal/e2e/ -run TestQCC_Phase3 -v
go test ./internal/e2e/ -run TestQCC -v
```

详见 `internal/e2e/testdata/qcc/README.md`。

### 测试分层速查

| 层级 | 目录 | 示例 |
|------|------|------|
| L0 纯函数 | `internal/core`, `internal/template` | `processor_test.go`, `application_test.go` |
| L1 IO | `internal/excel`, `internal/settings` | `excel_reader_test.go` |
| L2 集成 | `internal/app` | `app_test.go`, `preview_test.go` |
| L3 管道 E2E | `internal/e2e` | `pipeline_test.go`, `qcc_phase*_test.go` |

### CI

`.github/workflows/test.yml`：push/PR 触发 `go test` + vet + 覆盖率门禁 + frontend build + E2E。

## 打包与发布

> GitHub 回退更新源即本仓 Releases：[`pikachuprogrammer01/wps-enhancer-go`](https://github.com/pikachuprogrammer01/wps-enhancer-go)。  
> **用户更新主源**：Gitee [`my-software-releases`](https://gitee.com/pikachuprogrammer01/my-software-releases) 分支 **`wps-enhancer`**（见 `docs/gitee-releases.md`）。  
> **签名**：第一期可不签名（macOS 右键打开 / Windows SmartScreen「仍要运行」）。

在仓库根目录：

```bash
task build      # → bin/
task package    # macOS → .app；Windows → NSIS（需 makensis）
```

- 更新闭环见 `docs/update_sop.md`。
- **日常发版**：见 **`docs/release.md`**（`bash scripts/release.sh 1.2.0`）。对接细节见 `docs/gitee-releases.md`。
- 后续待办见 `docs/follow-up.md`。

## 授权与激活

激活码为离线验签 + 在线绑定的双段结构，规则详见 `docs/wps-activation-policy.md`。联调工具：

```bash
go run ./cmd/license-probe "WPS-xxxx.yyyy"   # 验签 + 在线激活
go run ./cmd/license-probe "WPS-xxxx.yyyy" --deactivate   # 解绑
go run ./cmd/license-probe --fingerprint     # 显示本机设备指纹
```

## 导出格式说明

支持 vcf / xlsx / csv / txt 四种格式。**不提供 xls 导出**：Go 生态无成熟 xls 写出库，且 xlsx 兼容性更好（评估详见 `docs/migration/00-overview.md` §6.8）；源文件读取仍支持 xls。

## 与 Python 版的差异

- 不提供 xls 导出（自动转换方案一并废弃，见上）
- 新增订阅授权系统与 `cmd/license-probe` 联调工具
- 更新下载/安装闭环、系统代理、日志系统为原生 Go 实现
- 行为一致性由 golden 对照 + 管道 E2E 测试锁定
- 测试体系见 `docs/testing.md`；CI 见根仓库 `.github/workflows/test-go.yml`

## 数据目录

`~/Library/Application Support/WPSEnhancer`（macOS）／`%APPDATA%\WPSEnhancer`（Windows）：
`settings.json`（全局设置）、`template/`（用户模板）、`logs/`（日志）、`license.json`（授权状态）。

## 许可证

本软件为**专有软件**，采用付费订阅授权，详见 [LICENSE](LICENSE)。未经授权禁止复制、分发、逆向工程或衍生开发。
