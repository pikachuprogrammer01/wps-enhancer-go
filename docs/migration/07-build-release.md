# 07 — 打包与发布（双端构建 + CI）

> 来源：`build/config.yml` + `Taskfile.yml` + `docs/update_sop.md`  
> 目标：`task build` / `task package` 双端产物 +（后续）GitHub Actions 发版 + 自动更新链路  
> **现状（2026-08-30）**：本地 `task package` macOS 已冒烟；Windows NSIS 由 CI 构建；根仓库 **`.github/workflows/release-go.yml`**（tag `go-v*`）；第一期无 Developer ID 公证；源码暂留 monorepo 子目录。

---

## 1. 产物对比

| | Python（现状） | Go+Wails（目标） |
|---|---|---|
| 打包工具 | PyInstaller | `task build` / `task package`（Wails v3） |
| macOS 产物 | `WPS增强工具.app` | `.app`（`task package`，第一期可无签名） |
| Windows 产物 | exe + 目录 | `wps-enhancer-go.exe` + **NSIS 安装器**（捆绑 WebView2 bootstrapper） |
| 更新仓库 | 本 monorepo Releases | **`pikachuprogrammer01/wps-enhancer-go` Releases**（源码目录暂不搬迁） |

---

## 2. 本地命令（在 `wps-enhancer-go/`）

```bash
brew install go-task          # 若未装 Task
task build                    # 当前平台 → bin/
task package                  # macOS → .app；Windows → NSIS installer（需 makensis）

# 交叉：在非 Windows 上可 task build GOOS=windows 得到 exe；
# NSIS 安装器建议在 Windows runner / 本机打（makensis）。
```

配置：`build/config.yml`（产品名 / bundle id / version，须与 `internal/version.Version` 一致）。

### Windows NSIS

- 脚本：`build/windows/nsis/project.nsi`
- `task package`（`GOOS=windows`）默认 `FORMAT=nsis`：生成 bootstrapper + `*-installer.exe`
- **不签名**：用户若遇 SmartScreen → 指引文案已含「更多信息 → 仍要运行」
- 可选二期：`task windows:sign:installer`（需证书）

### macOS

- 第一期：无签名 `.app` / zip；Gatekeeper → 右键打开（更新指引已写）
- 二期：Developer ID + notarytool

---

## 3. 发版与更新源

> **现行权威**：独立仓 `wps-enhancer-go` + **`docs/gitee-releases.md`**。下文「go-v* / monorepo」为迁出前快照，勿再按此操作。

1. 改 `internal/version.Version` 与 `build/config.yml` `info.version` → 提交  
2. 打 tag **`v{版本}`** 并 push（或 Actions 手动跑 **Release**）  
3. CI 产出：`WPSEnhancer-macos-*.zip`、`WPSEnhancer-windows-x86_64.zip`、NSIS installer  
4. GitHub Release（私有仓备份）+ 若配置 `GITEE_TOKEN` → 同步公开仓 **`my-software-releases` 分支 `wps-enhancer`**  
5. 客户端默认读：`…/raw/wps-enhancer/update.json`（`urls` 多平台）

详见 `docs/gitee-releases.md`、`scripts/publish-gitee.sh`。

---

## 4. 验收标准（L7，分期）

**第一期（当前范围）**

- [x] 本地 `task build` / `task package` macOS 可运行（adhoc 签名）
- [x] Windows：交叉 `task build GOOS=windows` 得 exe；NSIS 由 CI `windows-latest` 构建
- [x] 更新引导：文案对齐 Python；可打开下载目录与安装目录
- [x] GitHub 回退 URL 为 `.../wps-enhancer-go/releases/latest`
- [x] 根仓库 `release-go.yml`（`go-v*`）

**第二期**

- [ ] 配置 `GITEE_TOKEN` + 首次同步 `my-software-releases` 跑绿（见 `docs/gitee-releases.md`）
- [ ] 签名/公证（可选）；端到端：旧版 → 检查 → 下载 → 替换 → 新版本号

---

## 5. 双轨与目录策略

- Python / Go 并行期间共享用户数据目录（零迁移）  
- **暂不**把 `wps-enhancer-go/` 升到仓库根；文档与发布完善后再搬  
- 升格前：模块路径、CI `working-directory`、bindings 包名保持现状
