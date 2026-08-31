# my-software-releases — 仓库说明（交接用）

> 本仓**只存放各产品安装包与更新清单**，不放源码。  
> 构建与发版脚本在各**产品仓**内执行；本仓一般只需看 README、分支上的 `update.json`、以及 Releases 附件。

仓库：https://gitee.com/pikachuprogrammer01/my-software-releases

---

## 1. 结构（每产品一条分支）

| 分支 | 内容 |
|------|------|
| `main` | 本 README（产品矩阵索引） |
| `table-flow` | TableFlow 的 `update.json`（若沿用其现有约定） |
| `wps-enhancer` | WPS 增强工具（Go）的 `update.json` |

产品分支**根目录**只有清单，例如：

```
wps-enhancer 分支
└── update.json
```

安装包在 **Releases** 里，不进 git 树。tag 命名：`<产品>-v<版本>`，例如 `wps-enhancer-v1.1.0`。

---

## 2. 谁负责发版（重要）

| 产品 | 源码 / 构建仓 | 发版方式 |
|------|----------------|----------|
| WPS 增强工具 | GitHub `wps-enhancer-go`（MIT，已公开） | 打 tag `v*` → CI 签名（SignPath）→ `publish-gitee.sh` |
| TableFlow | 其产品仓 | 按其 `docs` / `scripts/release.sh` |

**不要**在本仓手写附件或手改 `update.json` 做正式发版（除非救急）。  
WPS 产品仓日常发版见：`docs/release.md`；对接细节：`docs/gitee-releases.md`。

本仓维护者需要知道的只有：

1. 保持仓库**公开**（匿名可读 raw + 可下 Release）
2. 发版用的 Gitee Token 对本仓有 **Releases + 推送分支** 权限
3. 产品矩阵 README 与真实分支/tag 命名一致

---

## 3. WPS 增强工具（`wps-enhancer`）

| 项 | 值 |
|----|-----|
| 分支 | `wps-enhancer` |
| 清单 | https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json |
| Release tag | `wps-enhancer-v{版本}` |
| 附件示例 | `WPSEnhancer-macos-arm64.zip`、`WPSEnhancer-windows-x86_64.zip`、（可选）`…-installer.exe` |
| 客户端默认更新源 | 上表 raw URL（多平台字段在 `urls` 里，不是单字段 `url`） |

`update.json` 形状（由产品仓脚本生成）：

```json
{
  "version": "1.1.0",
  "urls": {
    "macos-arm64": "https://gitee.com/pikachuprogrammer01/my-software-releases/releases/download/wps-enhancer-v1.1.0/WPSEnhancer-macos-arm64.zip",
    "windows-x86_64": "https://gitee.com/pikachuprogrammer01/my-software-releases/releases/download/wps-enhancer-v1.1.0/WPSEnhancer-windows-x86_64.zip"
  },
  "notes": "…"
}
```

首次发版时产品仓脚本可**自动创建**分支 `wps-enhancer`；`main` 上本 README 需人工维护索引。

---

## 4. 验收（发版后）

```bash
curl -i https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json
curl -iL "<update.json 里某个 urls 的值>"
```

期望：HTTP 200；`version` 与 tag 一致；zip 可匿名下载。

---

## 5. 不要做的事

- 不往本仓提交产品源码  
- 不把 NSIS `.exe` 当作 WPS 自动更新唯一包（更新走 zip；exe 仅新装）  
- 不删旧 Release（无妨；清单始终指向当前版）  
- 不改 WPS 的 `urls` 为 TableFlow 那种单字段 `url`（客户端不兼容）

---

## 6. 产品矩阵（摘要）

### WPS 增强工具（Go）

- 分支 / 清单：见 §3  
- 说明：Excel → 通讯录批量导出；安装包见对应 Release  

### TableFlow

- 按其现有分支/目录与 `update.json` 约定维护  
