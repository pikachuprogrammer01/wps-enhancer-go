# WPS Enhancer — Python → Go+Wails 迁移总览

> **状态**：决策已定（2026-08）。本文档是迁移的权威总纲，其余 `docs/migration/0*.md` 为分模块细则。
> **结论**：独立桌面应用 + Go + Wails v3 + excelize。体积 123MB → ~15-20MB，性能数量级提升，订阅体系可防破解，Go1 兼容承诺保证长期不重构。

---

## 1. 为什么迁移（决策记录）

| 驱动 | Python+PyQt（现状） | Go+Wails（目标） |
|------|--------------------|------------------|
| 打包体积 | 123 MB | 15-20 MB（↓85%） |
| 启动时间 | 3-5 秒 | <300ms |
| 10 万行 xlsx 读取 | 2-5 秒 | <200ms |
| 订阅防破解 | ❌ 字节码明文可改 | ✅ 编译二进制难逆向 |
| 长期稳定性 | 依赖 PyInstaller/Qt 生态 | Go1 兼容承诺 + 单一成熟库 |

**已排除方案**：Rust+Tauri（性能/体积边际收益不抵 2-3 倍开发成本）、Flutter（Excel 生态荒漠、桌面端未稳）、WPS 加载项（用户确认保持独立应用形态）、保持 Python（订阅体系不可行）。

---

## 2. 目标架构总览

```
wps-enhancer/  (新 Go 仓库，与 Python 仓库并行)
├── main.go                  # 入口：Wails 启动 + 日志清理定时器（← main.py）
├── internal/
│   ├── core/                # 领域逻辑纯函数（← features/contacts_import/processor.py）
│   ├── excel/               # 文件 IO 层（← core/file_io/）
│   ├── template/            # 模板系统（← core/template/）
│   ├── settings/            # 全局设置（← core/settings.py + app_paths.py）
│   ├── logger/              # 统一日志（← core/logger.py）
│   ├── license/             # 订阅体系（新增，对接 docs/wps-activation-policy.md）
│   ├── updater/             # 自动更新（← core/updater.py）
│   └── app/                 # Wails 命令层（← features/contacts_import/panel.py）
├── frontend/                # Web 前端（← ui/ + features/contacts_import/ui/）
└── wails.json
```

**架构铁律（延续 Python 版约定，Go 语言表达）**：

| Python 铁律 | Go 等价 |
|------------|---------|
| processor.py 纯函数 | `internal/core/` 无 I/O、无全局状态，错误用返回 `error` |
| 禁止裸 dict 传数据 | 全部用 `struct`（`SheetData`/`WriteRequest`/`ExportRow` 等） |
| 失败抛异常而非 return None | 返回 `error`，业务层用 `errors.Is` 判定类型 |
| file_io 无业务逻辑 | `internal/excel/` 只做格式读写，不做数据转换 |
| 禁止 features 直接 import openpyxl | `internal/app/` 只允许通过 `internal/excel/` 接口访问文件 |
| 全部类型注解 | Go 编译器强制（天然满足） |
| 每函数一行注释 | 导出函数必须有 doc comment（`golint` 强制） |

---

## 3. 依赖清单（全部锁定）

| 用途 | 库 | 版本策略 | 说明 |
|------|----|---------|------|
| GUI 框架 | `github.com/wailsapp/wails/v3` | 锁定 v3.x | 系统 WebView，不打包浏览器 |
| xlsx 读写 | `github.com/xuri/excelize/v2` | 锁定 v2.x | 读写一体，唯一维护活跃的 xlsx 库 |
| xls 读取 | `github.com/extrame/xls` | **锁 commit**（非发布版） | 已停滞（2023 停更），必须包防护层，见 `02-file-io.md` §4 |
| 激活码验签 | Go 标准库 `crypto/rsa` + `crypto/sha256` + `crypto/x509` + `encoding/pem` | — | 零依赖 |
| HTTP | 标准库 `net/http` | — | updater + license client |
| 日志 | 标准库 `log/slog` | — | 自写按天轮转 writer |
| 设置/模板 JSON | 标准库 `encoding/json` | — | 保持 settings.json 格式兼容 |
| 设备指纹 | 标准库 `os/exec` | — | 调 PowerShell/ioreg，见 `05-license.md` |
| 前端构建 | Node.js 24 LTS + pnpm（Vite，Wails 模板自带） | 仅构建期依赖，不进最终产物 |

**禁止引入**（防过度工程）：Web 框架（React/Vue 可选但非必须，先用原生 JS）、ORM、依赖注入容器、`wails` 之外的任何 GUI 库。

---

## 4. 环境安装

### macOS（开发机）

```bash
# 1. Xcode Command Line Tools（必需；见下方说明）
xcode-select --install

# 2. Go（建议官网 pkg 或 brew；brew 版本可能滞后）
brew install go          # 或 https://go.dev/dl/ 下载 pkg
go version               # 期望 go1.24+

# 3. Node.js（前端构建；Node 24 LTS 已验证可用，无需降级）
#    已有 Node 24 可跳过；没有则用 fnm 管理版本
brew install fnm
fnm install 24 && fnm use 24

# 4. pnpm（包管理器；npm/pnpm/yarn 均可，本方案用 pnpm）
npm install -g pnpm
pnpm --version

# 5. Task（跑 Taskfile：`task test` / `task dev` 等；未装会报 command not found）
brew install go-task
# 或：go install github.com/go-task/task/v3/cmd/task@latest
which task

# 6. Wails CLI
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
wails3 doctor             # 校验环境（旧文档中的 `wails` 命令以本机 CLI 为准）
```

**关于 Xcode 的澄清**：
- **只需要 Xcode Command Line Tools**（`xcode-select --install`，约 1-2GB）：提供 clang（CGO 编译）、git、make 等，WebView 运行时是 macOS 系统自带的 WKWebView，**不需要 Xcode 提供任何东西**
- **不需要完整 Xcode.app**（App Store 约 12GB）：那是给 iOS/macOS 原生开发用的，Wails 桌面开发用不到
- 唯一例外：正式发布做**公证（notarytool）**时，若 CLT 版本过旧可能缺失该工具——到发布环节再确认，开发阶段完全不需要

### Windows（构建机 / CI）

```powershell
# 1. Visual Studio Build Tools（必须，Go 调 WebView2 需要）
#    安装时勾选 "Desktop development with C++"（MSVC + Windows SDK）

# 2. Go + Node.js 24 + pnpm + Task + Wails CLI（同上思路）
#    Task: go install github.com/go-task/task/v3/cmd/task@latest
#    或 scoop/choco 安装 go-task

# 3. WebView2 运行时
#    Win10 1809+ 系统自带；旧系统在安装器里捆绑 WebView2 bootstrapper

wails3 doctor             # 校验环境
```

### 开发工作流

```bash
wails dev    # 前端热更新 + Go 后端，改 CSS 秒刷，改 Go 增量编译
wails build  # 产出双端产物（macOS .app / Windows .exe）
```

---

## 5. 迁移级别路线图（按依赖顺序，每级独立验收）

> 原则：**每级可独立验收、可独立上线、不阻塞后续级别**。级别内按"测试先行"执行——
> Python 版 `tests/test_processor.py` 等纯逻辑测试先直译为 Go test，作为迁移正确性的对照基准。

| 级别 | 模块 | 工作量 | 依赖 | 验收标准 |
|------|------|--------|------|---------|
| **L1** | 环境 + 骨架 + `internal/core` | ⭐⭐ | 无 | `wails dev` 空窗口跑通；`go test ./...` 全绿；processor 纯函数输出与 Python 版逐项一致 |
| **L2** | `internal/excel`（file_io） | ⭐⭐⭐ | L1 | 六种格式（xlsx/xls/csv/vcf/txt）读写与 Python 版输出一致（测试对照夹具） |
| **L3** | `internal/template` + `settings` + `logger` | ⭐⭐ | L1 | 模板匹配结果与 Python 版一致；settings.json 旧文件可直接读 |
| **L4** | `internal/updater` + `internal/app` | ⭐⭐ | L2, L3 | 更新检查/下载走通；panel 流程编排可被 CLI 调用（无 UI 验证逻辑） |
| **L5** | `frontend`（UI 重写） | ⭐⭐⭐⭐ | L2-L4 | 用户可完成完整导入流程（选文件→映射→预览→导出），与 Python 版 UX 对齐 |
| **L6** | `internal/license`（订阅） | ⭐⭐⭐ | L4 | 对接 `docs/wps-activation-policy.md` 联调清单 8 项全过 |
| **L7** | 打包发布（双端 CI） | ⭐⭐ | L5, L6 | GitHub Actions 双端构建产物可用；自动更新链路通 |

**关键决策点**：L5 完成后可发布 v1.0（免费版），L6 完成后发布 v2.0（订阅版）。L1-L4 全程 Python 版仍可维护，双轨并行不阻塞。

---

## 6. 全局注意事项（所有模块适用）

1. **测试先行**：每个模块迁移前，先把对应 Python 测试直译为 Go test。Go 的 `testing` + 表驱动测试写法与 pytest 一一对应，这是"迁移不改行为"的唯一保证。
2. **中文文案**：Python 版 UI 文案硬编码，迁移到前端时放入 `frontend/src/i18n.js`（简单键值映射即可，不引 i18n 框架），一次到位。
3. **数字精度**：excelize 读单元格返回 `float64`，而 processor 逻辑依赖字符串（手机号/身份证长度校验）。读层必须把数字单元格转为与 Python `str(cell)` 行为一致的字符串（见 `02-file-io.md` §5），否则 L1 测试直接暴露差异。
4. **路径语义**：Python `app_paths.get_data_dir()` 对应 Go `os.UserConfigDir()/WPSEnhancer`；日志目录对应 `os.UserCacheDir()/WPSEnhancer/logs`。`template/` 目录名保持不变，老用户模板零迁移。
5. **错误体系**：Python 自定义异常 → Go sentinel error（`var ErrFileRead = errors.New(...)`）+ `fmt.Errorf("%w", ...)` 包装。`internal/app` 统一转换为 Wails 错误返回给前端。
6. **日志**：`log/slog` + 自写按天轮转 writer（~30 行），保留 `log_call` 等价物（slog 的 `Debug` 级函数进入/退出记录）。
7. **不回退**：迁移期间不往 Python 版加新功能（除紧急 bugfix），避免双份维护成本。
8. **xls 妥协（2026-08-25 最终决策）**：读尽力兼容（extrame-xls + 防护层）。写侧 Go 生态无成熟 xls 写出库（唯一候选 shakinm/xlsWriter 小众且功能基础，自研 BIFF8 成本 1-2 周且兼容风险高，性价比评估后放弃），**导出格式不提供 xls 选项**，对外口径为 4 种：vcf / xlsx / csv / txt。源文件读取仍支持 xls。防御性保留 `normalizeExportPath`：用户手输 .xls 路径时自动纠正为 .xlsx。

---

## 7. 文档索引

| 文档 | 覆盖模块 |
|------|---------|
| `docs/testing.md` | 测试分层、命令、CI 门禁、企查查夹具 |
| `01-core.md` | internal/core（processor 纯函数 + 错误体系） |
| `02-file-io.md` | internal/excel（六格式读写 + xls 防护层） |
| `03-template.md` | internal/template + settings + logger |
| `04-ui.md` | frontend（PyQt → Web 重写） |
| `05-license.md` | internal/license（订阅体系，对接 wps-activation-policy.md） |
| `06-updater.md` | internal/updater（自动更新） |
| `07-build-release.md` | 打包、双端构建、GitHub Actions、发布 SOP |

上游权威文档（不变）：`docs/template_system.md`（模板系统设计）、`features/contacts_import/SPEC.md`（功能规格）、`docs/wps-activation-policy.md`（激活码契约）。
