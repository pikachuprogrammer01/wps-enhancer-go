# 06 — internal/updater：自动更新迁移

> 来源文件：`core/updater.py`（16.7KB，全项目最大单文件之一）+ `core/version.py`
> 目标结构：`internal/updater/`
> 上游参考：`docs/update_sop.md`（发布 SOP，权威）

---

## 1. 迁移目标

```
internal/updater/
├── updater.go    # 更新检查/版本比较/下载（← updater.py）
├── version.go    # APP_VERSION（← core/version.py）
└── updater_test.go
```

**功能等价**（与 Python 版逐条对应）：
1. GitHub Releases 检查（`update.json` / Releases API，按现有实现核对）
2. 版本比较（语义化版本，含预发布处理）
3. 下载（进度回调）
4. 校验（哈希/签名，如现有实现有）
5. 应用更新（替换可执行文件/重启）

---

## 2. 实现要点

### 2.1 版本号（version.go）

```go
// Version 应用版本号；发布 tag v{版本} 触发自动发布（GitHub Actions 约定不变）
const Version = "1.0.0"
```

- 保持 `core/version.py` 的 `APP_VERSION` 语义与发布约定（`docs/update_sop.md`）
- 构建时注入：`wails build` 支持 ldflags，`var Version = "dev"` + `-ldflags "-X main.Version=1.0.0"`，发布流水线注入

### 2.2 检查更新（updater.go）

```go
// CheckForUpdate 拉取远端版本信息，与本地版本比较
func CheckForUpdate(client *http.Client) (*ReleaseInfo, error)

// CompareVersions 语义化版本比较（主.次.补丁，可选 -pre 后缀）
func CompareVersions(local, remote string) (int, error)
```

- HTTP 客户端：`http.Client{Timeout: 10 * time.Second}`（Python 版超时策略对照 `updater.py`）
- **对比 Python 版的网络层依赖**：Go 标准库 `net/http` 直接可用；Python 版用了 certifi 证书包，Go 用系统证书池（`crypto/x509` 默认），**无需额外依赖**
- GitHub API 未认证限流 60 次/小时——保留 Python 版已有的处理（如走 `update.json` 静态文件而非 API，实现时核对 `update.json` 用途）

### 2.3 下载与进度

```go
// DownloadRelease 下载更新包，progress 回调推送进度（Wails Events → 前端进度条）
func DownloadRelease(client *http.Client, url, dest string, progress func(done, total int64)) error
```

- 断点续传/重试：按 Python 版现有行为对齐（`updater.py` 有重试/超时逻辑就直译）
- 下载到临时文件 → 校验 → 替换，**不允许直接覆盖运行中的可执行文件**（Windows 下会失败；走"下载到临时目录 → 提示用户重启后应用"流程，与 Python 版一致）

### 2.4 版本比较的坑

- 语义化版本规则（`1.2.3` > `1.2.3-rc1` > `1.2.2`）用 `golang.org/x/mod/semver`（**唯一允许的 x/ 依赖**，Go 模块生态事实标准）或自写 ~30 行比较器——**优先自写**（规则简单，避免新依赖）
- 注意 Python `packaging.version` 与语义化版本的差异（如 `1.0` vs `1.0.0` 归一化），用测试锁死行为

---

## 3. 平台差异（新出现的问题）

| 平台 | 更新应用方式 |
|------|-------------|
| macOS | 替换 `.app` 内可执行文件（`WPSEnhancer` 二进制）或整个 `.app`；**签名后更新需注意**：替换二进制会破坏签名 → 发布必须持续签名，或下载整个 `.app` 替换 |
| Windows | 替换 `.exe`；运行中文件被锁 → 下载到临时目录，用 `.bat` 延迟替换（标准做法：`cmd /c ping -n 2 127.0.0.1 & del old & move new`） |

> 这两点与 Python 版实现对齐（`updater.py` 已有平台分支），迁移时确认行为一致。

---

## 4. 迁移步骤（L4）

1. `version.go` + `CompareVersions` + 测试（含预发布/归一化用例）
2. `CheckForUpdate` + 测试（`httptest.Server` 模拟 GitHub 响应）
3. `DownloadRelease` + 进度回调 + 测试（临时目录，校验文件完整性）
4. 接入 `internal/app` 命令：`UpdateCheck`/`UpdateDownload`/`UpdateApply` + 前端更新页（`04-ui.md` §4.4）
5. 双端实测更新链路（本地起静态文件服务器模拟 Releases）

## 5. 验收标准（L4 完成定义）

- [ ] 版本比较测试全绿（含边界：相等/预发布/位数不同）
- [ ] 检查更新：本地版本 < 远端 → 提示可更新；相等 → 提示最新
- [ ] 下载进度事件前端可收到（进度条工作）
- [ ] 应用更新双端走通（macOS 替换 + Windows 延迟替换）
- [ ] 断网/404/超时均优雅降级（提示 + 不崩溃）

## 6. 注意事项

1. **证书**：Go 默认系统证书池，Windows/macOS 都可用；**不要**用自签证书测试（会养成坏习惯），测试用 `httptest` 即可
2. **更新是安全敏感面**：下载后校验哈希（如 Python 版有）必须保留；未来上订阅后，更新通道应考虑签名验证（`minisign`/`cosign` 方案留作 V3 演进项，不阻塞本期）
3. **别在 updater 里加业务**：只做"检查/下载/应用"，解压安装细节（如 zip 解包）用标准库 `archive/zip`
4. **与 license 的关系**：免费版也要能更新（更新不设门槛），订阅只锁功能不锁更新通道
