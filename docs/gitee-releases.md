# Gitee 统一发布仓对接说明（WPS Enhancer Go）

> **本文件用途**：转交任意 agent / 接手开发者，说明本产品如何对接公开发布仓 `my-software-releases`。  
> 收到本文档即可独立完成发版与排查，无需通读业务代码。  
> 产品仓：本仓库 `wps-enhancer-go`  
> 发布仓：https://gitee.com/pikachuprogrammer01/my-software-releases  

---

## 1. 背景（30 秒）

本应用是桌面端（macOS `.app` + Windows exe/NSIS），内置更新检测：读取静态 `update.json`，比较版本后按**当前平台+架构**下载对应 zip。

**GitHub 本仓已公开**（MIT）：Release 仍作备份；用户更新主源为 Gitee 公开仓。  
**`my-software-releases` 为公开仓**：只放产物与清单；**每个产品一条独立分支**。

客户端默认更新源（已写入代码）：

```
https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json
```

常量：`internal/settings.DefaultUpdateURL`（路径中的 `wps-enhancer` = 分支名）。

> **上线顺序**：先在发布仓创建产品分支并完成首次 `publish-gitee.sh`，再发带此默认 URL 的客户端。否则新装/空设置用户检查更新会 404。

---

## 2. 分支模型（与「目录共用 main」不同）

| | 约定 |
|--|------|
| 默认分支 `main` | 仅放总 README（产品矩阵），**不放**本产品安装包清单 |
| 产品分支 | 分支名 = 产品名，本产品为 **`wps-enhancer`** |
| 清单路径 | 产品分支**根目录** `update.json` |
| Release tag | 仍全局唯一：`wps-enhancer-v{版本}`，`target_commitish` = 产品分支 |
| 附件 | 挂在该 Release 上（全仓共享 Releases 列表，靠 tag 前缀区分产品） |

示例：

```
my-software-releases
├── main              → README.md（索引）
├── table-flow        → update.json +（其发布脚本管理的附件 tag）
└── wps-enhancer      → update.json   ← 本产品
```

---

## 3. 与 TableFlow 的差异

| | TableFlow | 本产品（WPS） |
|--|-----------|----------------|
| 隔离方式 | 视其现有约定 | **独立分支 `wps-enhancer`** |
| 产物数量 | 1 个 zip | 多平台多文件 |
| update.json | 可能单字段 `url` | **必须用 `urls` 多平台 map** |
| tag | `table-flow-v…` | `wps-enhancer-v1.1.0`，附件保留平台名 |
| 自动更新包 | 扩展 zip | **仅 `.zip`**；NSIS `.exe` 只作新装 |

本仓用 `scripts/publish-gitee.sh`（多资产 + 推产品分支）。

---

## 4. 命名约定

| 项 | 格式 | 示例 |
|----|------|------|
| 产品分支 | `<产品>` | `wps-enhancer` |
| 清单 raw URL | `.../raw/<产品>/update.json` | 见第 1 节 |
| Gitee Release tag | `<产品>-v<版本>` | `wps-enhancer-v1.1.0` |
| macOS 附件 | `WPSEnhancer-macos-{arm64\|x86_64}.zip` | 与 CI 一致 |
| Windows 更新包 | `WPSEnhancer-windows-x86_64.zip` | 进 `urls` |
| Windows 安装器 | `WPSEnhancer-windows-x86_64-installer.exe` | 仅 Release，不进 `urls` |
| 本仓 GitHub tag | `v<版本>` | `v1.1.0`（触发 CI） |

直链模板：

```
https://gitee.com/pikachuprogrammer01/my-software-releases/releases/download/wps-enhancer-v{版本}/{附件名}
```

---

## 5. update.json 规格

```json
{
  "version": "1.1.0",
  "urls": {
    "macos-arm64": "https://gitee.com/pikachuprogrammer01/my-software-releases/releases/download/wps-enhancer-v1.1.0/WPSEnhancer-macos-arm64.zip",
    "windows-x86_64": "https://gitee.com/pikachuprogrammer01/my-software-releases/releases/download/wps-enhancer-v1.1.0/WPSEnhancer-windows-x86_64.zip"
  },
  "notes": "可选更新说明"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `version` | ✅ | semver，与 `internal/version.Version`、tag 去 `v` 后一致 |
| `urls` | ✅ | key=`{platform}-{arch}`：`macos-arm64` / `macos-x86_64` / `windows-x86_64` |
| `notes` | ❌ | 展示给用户 |

解析：`internal/updater.checkViaCustom`。有 `urls` 时不会回退单字段 `url`。

---

## 6. 发布流程

### 6.1 一键发版（推荐）

完整操作手册：**[`docs/release.md`](./release.md)**。

```bash
bash scripts/release.sh 1.2.0
# 可选：--notes "说明"  --dry-run  --no-push  --force-tag
```

脚本会：改 `Version` + `info.version` → commit → 打 `v1.2.0` → push（触发 CI）。  
需已配置 GitHub secret **`GITEE_TOKEN`**，CI 才会同步 Gitee。

### 6.2 构建（本仓 CI，由 tag 触发）

```bash
git tag v1.1.0 && git push origin v1.1.0
```

CI 产出 mac/win zip + NSIS，发 GitHub Release；若配置了 `GITEE_TOKEN` 则调用 `publish-gitee.sh`。

### 6.3 手动发布到 Gitee（救急）

```bash
GITEE_TOKEN=<私人令牌> bash scripts/publish-gitee.sh \
  --version 1.1.0 \
  --assets-dir ./artifacts \
  --notes "本次更新说明（可选）"
```

脚本自动：

1. 校验必需 zip  
2. 在产品分支 `wps-enhancer` 上创建 tag Release（分支不存在时先从当前 HEAD 建分支再推）  
3. 上传全部 `WPSEnhancer-*` 附件  
4. 写入并 push 分支根目录 `update.json`  

**失败回滚**：删除未完成的 Release **及同名 git tag**，可直接重试。

### 6.4 首次初始化（仅一次）

在发布仓：

```bash
git clone https://gitee.com/pikachuprogrammer01/my-software-releases.git
cd my-software-releases
git checkout main   # 或默认分支：只维护 README
# README 增加 release-repo/README.snippet.md 一节后 push main

git checkout -b wps-enhancer
cp /path/to/wps-enhancer-go/release-repo/update.json ./update.json
git add update.json && git commit -m "init: wps-enhancer branch"
git push -u origin wps-enhancer
```

占位 `update.json` 在首次正式 `publish-gitee.sh` 后会被覆盖。  
若跳过手工建分支，脚本首次发布时会用**空提交**自动创建并推送 `wps-enhancer`（不会推送非法空 `urls` 清单）；附件上传成功后再写入正式 `update.json`。

---

## 7. 验收

```bash
curl -i https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json
curl -iL "<urls.macos-arm64>"
curl -iL "<urls.windows-x86_64>"
```

期望：均 200；`version` 一致；zip 可解压。

---

## 8. 边界

| 事项 | 原因 |
|------|------|
| 不把源码放进发布仓 | 仅产物 + 清单 |
| 不把 NSIS exe 写进 `urls` | 更新链路只接受 zip |
| 不把本产品清单放在 `main` | 产品隔离靠分支 |
| 不清理旧 Release 附件 | update.json 始终指向当前版 |

---

## 9. 信息索引

| 想了解 | 看哪里 |
|--------|--------|
| 日常发版操作 | [`docs/release.md`](./release.md) |
| 默认更新 URL | `internal/settings.DefaultUpdateURL` |
| 解析 update.json | `internal/updater/updater.go` |
| GitHub 构建 | `.github/workflows/release.yml` |
| GitHub 一键发版 | `scripts/release.sh` |
| Gitee 多资产发布 | `scripts/publish-gitee.sh` |
| 分支初始化模板 | `release-repo/` |
| 许可（MIT）与 Pro 订阅 | [`LICENSE`](../LICENSE)、[`docs/subscription-terms.md`](./subscription-terms.md) |
| 开源 / SignPath 待办 | [`docs/follow-up.md`](./follow-up.md) §开源 |
| SignPath Windows 签名 | [`SIGNPATH.md`](../SIGNPATH.md)、[`docs/signpath-setup.md`](./signpath-setup.md) |

---

## 10. CI Secret

| Name | 说明 |
|------|------|
| `GITEE_TOKEN` | Gitee 私人令牌；有则 Release job 自动同步公开仓产品分支 |
