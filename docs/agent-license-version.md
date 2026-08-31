# 版本与授权 — 后续 Agent 工作约定

> 权威补充：`docs/migration/05-license.md`（后端订阅模块）、`docs/wps-activation-policy.md`（激活契约）、`docs/subscription-terms.md`（Pro 订阅与 MIT 关系）。  
> 本文约束 **Go 版前端 + 命令层**，避免把版本/授权逻辑嵌进通讯录等业务。

---

## 1. 边界（必须遵守）

| 层 | 允许做什么 | 禁止做什么 |
|----|------------|------------|
| `internal/core`、`internal/excel`、导入/模板业务 | 无 license 依赖 | `import .../license`、读 `license.json`、写激活流程 |
| `internal/license` + `internal/app/license.go` | 解析/验签/激活/解绑/`IsPro()` | 依赖通讯录 processor、模板匹配 |
| `internal/version` | 仅版本常量 | 业务逻辑 |
| 前端业务页（`ImportView` 等） | 需要门禁时读 `useAppMeta().isPro` | 直接 `App.Version` / `App.License*`、内嵌激活表单 |
| 前端元信息 | `composables/useAppMeta.ts`、`components/VersionLicensePanel.vue`、`views/LicenseView.vue` | 把激活 UI 抄进业务页 |

**能力门禁唯一入口**

- 后端：`license.State.IsPro()`（经 App 注入的 `licenseState`）
- 前端：`useAppMeta().isPro`（展示或显式门禁）

不要在业务里复制「读 license.json / 调激活 API」的代码。

---

## 2. 前端文件地图

```
frontend/src/
├── composables/useAppMeta.ts          # 版本 + 授权状态单例（唯一数据入口）
├── components/VersionLicensePanel.vue # 首页「版本与授权」展示块
├── views/LicenseView.vue              # 激活/解绑专用页
├── views/HomeView.vue                 # 只挂 Panel + emit('license')，不调 License API
├── views/SettingsView.vue             # 关于/更新用 useAppMeta.version
└── views/ImportView.vue               # 业务；默认不碰授权
```

激活成功/解绑后必须 `await useAppMeta().refresh()`，保证首页卡片与设置页版本区同步。

---

## 3. 后续 Agent 常见任务怎么做

| 需求 | 正确做法 |
|------|----------|
| 首页/关于显示版本或 Pro 标记 | 用 `useAppMeta` 或 `VersionLicensePanel` |
| 某功能仅 Pro 可用 | 边界判断 `IsPro()` / `isPro`；未开通则提示去「激活与授权」，**不要**在业务里写激活表单 |
| 改激活码协议/验签 | 只改 `internal/license` + 契约文档；业务零改 |
| 改版本号 | 只改 `internal/version/version.go`（及发布 tag） |
| 新业务功能 | 禁止复制 `LicenseActivate` 调用链；需要状态就注入/调 `IsPro` |

---

## 4. 自检清单（改完相关代码后）

- [ ] `go list -deps ./internal/core` 无 `internal/license`
- [ ] `ImportView.vue`（及同类业务页）无 `App.License*` / `App.Version`
- [ ] 新增展示仍走 `useAppMeta`，未再造一套本地 `ref` 拉版本
- [ ] 激活/解绑后调用了 `refresh()`
- [ ] 本地测试：已装 Task（`brew install go-task`）且在 `wps-enhancer-go/` 下 `task test`；或直接 `go test ./internal/...`

---

## 5. 与 Python 版关系

Python 侧仍以 `core/version.py`、设置/关于 UI 为准；**主版本是 `wps-enhancer-go/`**。新功能优先按本文约定落在 Go 版。
