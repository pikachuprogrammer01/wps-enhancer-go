# 08 — Go 迁移现状评估报告（完善 / 不完善）

> 状态：评估快照（**2026-08-30** 增补）。基于 `wps-enhancer-go/` 当前代码与文档；权威参照：`00-overview.md`、`docs/testing.md`、`docs/agent-license-version.md`、`docs/follow-up.md`、`features/contacts_import/SPEC.md`。
> 本地跑测试需先安装 **Task**（`brew install go-task`），在 `wps-enhancer-go/` 下执行 `task test`；详见 `docs/testing.md`。

---

## 〇、总体结论

Go+Wails 版已完成**代码主体迁移**（核心逻辑 / 文件 IO / 模板 / 设置 / 订阅 / UI 主流程 / 测试体系），CI 测试门禁已落地，可运行二进制已产出（`bin/wps-enhancer-go`）。

**距"可替代 Python 版上线"主要差**：

1. **打包/发布未闭环**（L7：正式 `.app` / 安装包、双端 release 流水线跑通验证）；
2. **自动更新安装引导**（下载/校验已有，替换安装/重启引导需对齐 Python `update_flow.py` 体验）；
3. **xlsx 表格预览 WYSIWYG**（文本格式预览已走服务端；xlsx 表格列仍用 `preview.rows`，未走 `BuildPreviewDisplay`）。

**2026-08-30 已补**：导入会话列编辑与导出后写回、vcf 自定义字段 / 预览页设置、双向链表导航 + 首页、版本授权前端解耦（`agent-license-version.md`）、`DetectTruncatedNumbers` 读表弹窗。

---

## 一、完善的部分（✅ 已完成且质量达标）

### L1 核心逻辑层 `internal/core` — ✅

- `processor.go`：`SplitPhones` / `ValidatePhone` / `DetectTruncatedNumbers` / `BuildPreviewData` / `BuildWriteRequest` / `BuildPreviewDisplay` / `BuildTextPreview` / vCard 序号与合并单元格等。
- **Golden 对照**：`processor_test.go` + `docs/migration/testdata/processor_golden.json`（`scripts/gen_golden.go` 生成，`TestGoldenFileInSync` 防漂移）。

### L2 文件 IO 层 `internal/excel` — ✅（写 xls 有意放弃，见 §二.3）

- xlsx / xls / csv / txt / vcf 读写；xls 读有防护层；写侧仅 xlsx/csv/txt/vcf；自定义 vcf 字段经 `VCFProps` 写出。

### L3 模板 / 设置 / 日志 — ✅

- `internal/template`：匹配引擎 + 存储 + **应用层** `Suggest` / `Apply`；内置列可带 `vcf_prop`。
- `internal/settings`：含 `vcf_show_import_guide` 等字段，`settings.json` 零迁移语义（未知键忽略、缺省兜底）。
- `internal/logger`：按天轮转 + LogCall。

### L4 命令层 `internal/app` — ⚠️（更新安装引导见 §二.1）

- 导入主流程；`SessionMatch` / `PreviewWithColumns` / `ExportWithColumns` / `DetectTruncatedNumbers` / `PreviewSummary`；模板 Suggest/Apply。

### L5 前端 `frontend/` — ⚠️（xlsx 预览列见 §二.2）

- 三步向导；会话列编辑；vcf 预览选项与扩展字段；导航历史（`navHistory.ts`）；版本授权经 `useAppMeta` / `VersionLicensePanel`。

### L6 订阅体系 — ✅（代码；真实平台联调未验证）

- 约定文档：`docs/agent-license-version.md`。

### 测试体系 — ✅

- 规范：`docs/testing.md`（含 **Task 安装说明**）；Taskfile：`test` / `test:cover` / `test:cover:gate` / `test:e2e` / `test:all`。
- CI：根仓库 `.github/workflows/test-go.yml`。

---

## 二、不完善的部分（❌ / ⚠️），按严重度排序

### 1. ⚠️ 自动更新：下载已有，安装引导已对齐 Python（第一期）

- ✅：`CheckUpdate`、静默启动检查、`StartDownloadUpdate`、`DownloadProgress`、zip 校验、`download_dir` / `install_dir`。
- ✅（2026-08-30）：替换指引文案对齐 Python（完全退出 / Gatekeeper / SmartScreen）；「打开所在目录」用 `open -R` / `explorer /select`；新增「打开安装目录」。
- ⚠️：发版资产上传到 `wps-enhancer-go` Releases + CI 流水线仍为第二期（见 `07-build-release.md`）。

### 2. ⚠️ xlsx 表格预览未走 `BuildPreviewDisplay`

- vcf/csv/txt 已通过 `PreviewText` 服务端生成，所见即所得。
- xlsx 模式表格仍直接渲染 `preview.rows`，格式切换时列过滤/姓名前后缀展示可能与导出文件有细微差异。
- **待办**：`App.PreviewDisplay` + 前端 xlsx 模式统一走服务端。

### 3. ✅ xls 导出：不提供（2026-08-25 决策）

导出格式：**vcf / xlsx / csv / txt** 四种；源读取仍支持 xls。

### 4. ⚠️ L7 打包发布（workflow 已落，待首次跑绿）

- `build/config.yml` 已定制；本地 mac `task package` 已冒烟；Windows NSIS 走 CI。
- ✅：根仓库 `.github/workflows/release-go.yml`（tag `go-v*`，与 Python `v*` 错开）。
- ⚠️：需配置 `WPS_ENHANCER_GO_TOKEN` 并实际打 tag 验收；签名/公证仍为二期。
- **待办**：见 `docs/follow-up.md`。

### 5. 轻微 — 行为对齐细节

| # | 差异 | 说明 |
|---|------|------|
| 5.1 | 预览默认行数 | Go 统一 20 行起、×4 展开至 480；Python 默认 30 |
| 5.2 | 死代码 | `App.Preview` / `App.Export`（默认模板路径）前端未用 |
| 5.3 | 更新源仓库 | GitHub 回退已指向 `wps-enhancer-go`；自定义 Gitee `update.json` 仍可为旧路径，发 Go 版时需同步 |

### 6. 未在规划内（备查）

- **卸载功能** ✅（2026-08-25）
- **Word 端功能**：规划中
- **LicenseHub 联调**：`cmd/license-probe` 已备，8 项契约清单无验证记录

---

## 三、路线图对照（L1–L7）

| 级别 | 目标 | 状态 | 说明 |
|------|------|------|------|
| L1 | 环境 + 骨架 + core | ✅ | golden 全绿 |
| L2 | excel 读写 | ✅ | 写 xls 除外 |
| L3 | template + settings + logger | ✅ | 含 Suggest/Apply、LogCall |
| L4 | updater + app | ⚠️ | 下载有，安装引导待对齐 |
| L5 | 前端 UI | ⚠️ | 主流程可跑；xlsx 预览列待 WYSIWYG |
| L6 | 订阅体系 | ✅（代码） | 联调未验证 |
| L7 | 打包发布双端 CI | ⚠️ | `release-go.yml` 已落；待首次 tag + secret |

---

## 四、建议的下一步（按优先级）

1. **L7 第一期本地打包验收**（`07-build-release.md` §4）— Windows NSIS + mac 无签名 `.app`。
2. **xlsx 预览走 `BuildPreviewDisplay`**（§二.2）— 预览 WYSIWYG 最后一块。
3. **L7 第二期：tag → Release CI**（上传到 `wps-enhancer-go` 仓库）— 端到端更新验收。
4. LicenseHub 8 项契约联调；顺手清理 §二.5 死代码。

---

## 五、一句话总结

**功能主体与测试体系已就绪**（golden + E2E + CI 门禁）；**缺口集中在发布链路（L7）、更新安装引导体验、xlsx 表格预览 WYSIWYG**。完成 §四.1–§四.3 即可替代 Python 版上线。
