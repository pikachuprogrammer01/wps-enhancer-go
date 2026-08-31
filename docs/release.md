# 发版与更新操作手册

> 给维护者：以后每次发新版本按本文操作即可。  
> 对接细节 / 排障见 [`gitee-releases.md`](./gitee-releases.md)。

---

## 一分钟流程

```bash
# 1. 工作区干净、在 main
git status

# 2. 一键改版本 + 提交 + 打 tag + push（触发 CI）
bash scripts/release.sh 1.2.0
# 可选：--notes "修复某某问题"

# 3. 打开 GitHub Actions → Release，等跑绿

# 4. 验收
curl -i https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json
```

用户端无需任何操作：已安装的客户端会读上述 `update.json`，发现新版本后提示下载。

---

## 角色分工（心里有数即可）

| 谁 | 做什么 |
|----|--------|
| 你（本仓） | `scripts/release.sh` 改版本并推 tag |
| GitHub Actions | 构建 mac/win 包 → 发 GitHub Release → 调 `publish-gitee.sh` |
| Gitee `my-software-releases` | 存公开附件 + 分支 `wps-enhancer` 上的 `update.json` |
| 客户端 | 启动/检查更新时 fetch 清单，按平台下载 zip |

**不要**去发布仓手改 `update.json`，也**不要**用发布仓里 TableFlow 的 `release.sh`。

---

## 日常发新版（推荐）

### 前置（一次性，已配好可跳过）

1. GitHub 本仓 Secret：`GITEE_TOKEN`（Gitee **私人令牌**，要有 Releases + 推送；不是登录密码）
2. 发布仓公开：https://gitee.com/pikachuprogrammer01/my-software-releases  
3. `main` 上 README 已有本产品索引（可选但建议）

### 每次发版

1. **确认代码已合并到 `main`，工作区干净**
2. **执行**

   ```bash
   bash scripts/release.sh <新版本>
   # 例：bash scripts/release.sh 1.2.0 --notes "支持某某功能"
   ```

   脚本会：

   - 改 `internal/version/version.go` 的 `Version`
   - 改 `build/config.yml` 的 `info.version`
   - `git commit`
   - 打 annotated tag `v<版本>`
   - `git push` 分支 + tag

3. **看 CI**：Actions → **Release**  
   - 绿：GitHub 有包；若配了 `GITEE_TOKEN`，Gitee 也会更新  
   - 红：点进失败 step；常见是 Gitee token、NSIS、网络

4. **验收**

   ```bash
   curl -i https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json
   # 确认 version 为新版本；再对 urls 里两个 zip：
   curl -iL "<urls.macos-arm64>"
   curl -iL "<urls.windows-x86_64>"
   ```

   期望：均 HTTP 200；客户端「检查更新」能提示新版本。

### `release.sh` 常用选项

| 选项 | 作用 |
|------|------|
| `--notes "..."` | 写入 commit / tag 说明 |
| `--dry-run` | 只打印，不改不推 |
| `--no-push` | 只本地 commit + tag |
| `--force-tag` | 删掉同名本地/远程 tag 再打（慎用） |

---

## 用户如何「更新」

1. 打开应用 → 设置/关于 → **检查更新**（或自动检查）  
2. 有新版本 → 下载 zip → 按指引替换安装  
3. 无需用户改更新源（默认已是 Gitee 公开仓）

默认更新地址（代码常量）：

```
https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json
```

清单格式是多平台 `urls`（不是 TableFlow 单字段 `url`）：

```json
{
  "version": "1.2.0",
  "urls": {
    "macos-arm64": "https://gitee.com/.../WPSEnhancer-macos-arm64.zip",
    "windows-x86_64": "https://gitee.com/.../WPSEnhancer-windows-x86_64.zip"
  },
  "notes": "可选说明"
}
```

---

## 救急：CI 没同步到 Gitee

例如 Secret 未配、或只想补传附件：

1. 从 Actions 下载 artifact，放到 `./artifacts/`（文件名须符合约定）  
2. 本机执行：

   ```bash
   GITEE_TOKEN=<PAT> bash scripts/publish-gitee.sh \
     --version 1.2.0 \
     --assets-dir ./artifacts \
     --notes "说明"
   ```

3. 若提示 tag 已存在且上次不完整：加 `--force` 后重试  

产物约定：

| 文件 | 用途 |
|------|------|
| `WPSEnhancer-macos-arm64.zip`（或 x86_64） | 进 `urls` |
| `WPSEnhancer-windows-x86_64.zip` | 进 `urls`（自动更新必填） |
| `WPSEnhancer-windows-x86_64-installer.exe` | 仅 Release，不进 `urls` |

---

## 命名对照

| 项 | 例子 |
|----|------|
| 本仓 git tag | `v1.2.0` |
| Gitee Release tag | `wps-enhancer-v1.2.0` |
| 清单分支 | `wps-enhancer` |
| 清单路径 | 该分支根目录 `update.json` |

---

## 常见问题

**Q: 只 push 了 main，没有发版？**  
A: 必须打 `v*` tag（或跑 `release.sh`）。只推代码不会触发 Release。

**Q: Actions 绿了但 Gitee 没更新？**  
A: 检查 Secret `GITEE_TOKEN` 是否为 PAT、权限是否够；看 Release job 里「Publish to Gitee」日志。CI 已默认带 `--force`，同版本重发会覆盖 Gitee Release。

**Q: 客户端检查更新 404 / 格式错误？**  
A: 确认 raw URL 能打开且含 `urls`；不要用手写成单字段 `url`。

**Q: 版本号要改几处？**  
A: 用 `release.sh` 只传一个版本即可，脚本改两处并保证一致。

**Q: 发布仓还要维护什么？**  
A: 平时不用动。`main` README 产品索引偶发更新即可；分支与清单由本仓脚本维护。

**Q: Windows 在 Gitee 网页下载 zip / installer 都被拦截？**  
A: 当前包**未做 Windows 代码签名**，Edge / Defender SmartScreen 对 Gitee 上的未知发布者常会同时拦 `.zip` 与 `.exe`（zip 内也有 exe）。可依次尝试：

1. **已有旧版**：应用内 **设置 → 更新 → 下载更新包**（不走浏览器，通常可下）。
2. **PowerShell 直链下载**（管理员打开 PowerShell）：

   ```powershell
   $url = "https://gitee.com/pikachuprogrammer01/my-software-releases/releases/download/wps-enhancer-v1.1.0/WPSEnhancer-windows-x86_64.zip"
   $out = "$env:USERPROFILE\Downloads\WPSEnhancer-windows-x86_64.zip"
   Invoke-WebRequest -Uri $url -OutFile $out -UseBasicParsing
   Unblock-File -Path $out
   ```

   若仍被删，打开 **Windows 安全中心 → 病毒和威胁防护 → 保护历史记录**，看是否「已阻止」；可临时关「实时保护」再下，下完立刻打开并 `Unblock-File`。
3. **浏览器**：下载栏里被拦项 → `⋯` → **保留**；运行 exe 时 SmartScreen → **更多信息 → 仍要运行**。
4. **换网络 / 换设备**：手机流量热点、Mac 下载后 U 盘拷贝，有时比公司网 + 360 等杀软组合更顺。
5. **长期**：Windows 代码签名暂缓。可选自购 OV 证书；SignPath OSS 需 OSI 开源许可，与当前「商业收费」许可不兼容（见 `docs/follow-up.md`）。
