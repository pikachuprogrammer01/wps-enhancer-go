# 05 — internal/license：订阅体系

> 来源文件：无（全新模块）
> 上游契约：`docs/wps-activation-policy.md`（权威，激活码格式/API/状态机全部以它为准）
> 平台状态：LicenseHub 待上线（Vercel），联调按契约 §8 清单执行

---

## 1. 迁移目标

```
internal/license/
├── key.go          # 激活码解析（← 契约 §1：前缀/分组符/分割/base64）
├── verify.go       # RSA 本地验签（← 契约 §2.1 §5：内置 PEM 公钥，失败降级在线）
├── client.go       # /activate /deactivate HTTP 客户端（← 契约 §2.2 §4：3s 超时）
├── fingerprint.go  # 设备指纹（← 契约 §3：维度组合，随版本可升级）
├── store.go        # 本地持久化（← 契约 §2.3：os.UserConfigDir，与主配置分离）
├── state.go        # 状态判定 IsPro()/过期检查（内存缓存）
└── *_test.go
```

**架构铁律**：业务层只依赖 `license.State.IsPro()`，**core/ 零侵入**——订阅是独立模块，V2 上线时业务代码不需要动。

---

## 2. 模块设计（对照契约逐节）

### 2.1 key.go — 激活码解析（契约 §1）

```go
// ParseKey 解析激活码：去 WPS- 前缀与全部 '-' 分组符，按 '.' 分割载荷/签名，
// 标准 base64 解码（注意：不是 base64url），返回载荷 JSON + 签名原文
func ParseKey(code string) (*Payload, []byte, error)
```

要点：
- `encoding/base64` 的 `StdEncoding`（带填充）；先 `strings.TrimPrefix(code, "WPS-")`，再 `strings.ReplaceAll(s, "-", "")`
- 载荷字段：`id/version/type/expiresAt/maxDevices/batchId/issuedAt/features/metadata` → Go struct 带 `json` tag
- **V1 冻结语义**：未知字段忽略（`json.Decoder` 默认行为），按 `version` 分派解析器

### 2.2 verify.go — 本地验签（契约 §2.1 §5）

```go
// VerifyLocal 用内置公钥验签（RSA-2048/SHA-256，PKCS1v15）
func VerifyLocal(payload []byte, signature []byte) error
// PublicKey 内置公钥：PEM 常量 + base64+反转轻混淆（与 Python 版策略一致）
```

- `crypto/x509.ParsePKIXPublicKey` 或 `ParsePKCS1PublicKey`（按 PEM 块类型）
- `rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed, sig)`
- **降级策略（契约 §5 推荐）**：`VerifyLocal` 失败**不直接拒绝**，仍发在线验证，以平台裁决为准——本地验签只做快速失败（格式错误/签名明显损坏直接提示"激活码无效"）

### 2.3 client.go — 在线验证（契约 §2.2 §4）

```go
// Activate 调用 POST /activate，返回平台状态机结果
func (c *Client) Activate(key string, fp string) (*ActivateResult, error)
// Deactivate 调用 POST /deactivate
func (c *Client) Deactivate(key string, fp string) error
```

- `http.Client{Timeout: 3 * time.Second}`（契约硬性要求）
- 状态机六分支映射：200/400 INVALID_KEY/403 REVOKED/404 NOT_FOUND/409 ALREADY_ACTIVATED/410 EXPIRED → 返回语义化结果（`ActivateResult{Code, ExpiresAt}`），文案由前端映射（契约 §6 文案表放前端 i18n）
- **网络失败不降级**：直接返回"网络不可用"错误，不写入任何激活状态（契约 §2.2）
- 解绑频率限制（409/429）→ "稍后再试"提示

### 2.4 fingerprint.go — 设备指纹（契约 §3）

```go
// Fingerprint 采集硬件维度 → SHA-256 hex
func Fingerprint() (string, error)
```

| 平台 | 命令 | 说明 |
|------|------|------|
| Windows | `powershell -NoProfile -Command "Get-CimInstance Win32_BaseBoard \| Select-Object -ExpandProperty SerialNumber"` | **⚠️ 不要用 `wmic`（Win11 已移除）** |
| Windows（补充维度） | `Get-CimInstance Win32_DiskDrive ...` 磁盘序列号 | 组合维度，增加区分度 |
| macOS | `ioreg -rd1 -c IOPlatformExpertDevice` 提取 `IOPlatformUUID` | 稳定跨重启 |

- 组合策略：`sha256(主板序列号 + "|" + 磁盘序列号 + "|" + CPU 型号)`（维度组合实现时定，记录在案）
- **容错约定**（契约 §3）：指纹随版本可变化，平台存当前+历史 5 次——客户端不承诺指纹永久稳定
- 命令执行失败（无权限/虚拟环境）→ 降级为低区分度组合（机器名+MAC），保证激活流程不卡死

### 2.5 store.go — 本地持久化（契约 §2.3）

```go
// Save 写入激活状态；Load 读取；Clear 解绑后清除
// 存储位置：os.UserConfigDir()/WPSEnhancer/license.json —— 与 settings.json 分离
```

```json
{ "key": "...", "payload": { ... }, "activatedAt": 1735689600000 }
```

- 完整性：文件损坏 → 视为未激活（提示重新激活），不崩溃
- 保护强度：普通文件即可（契约明确"本地逻辑只是体验层，强制力在服务端"），不引入 DPAPI/Keychain（cgo 依赖，收益低）

### 2.6 state.go — 状态判定（内存缓存）

```go
// State 内存态：启动时 Load 一次，运行期走内存判定
type State struct {
    mu        sync.RWMutex
    isPro     bool
    expiresAt int64
}

// IsPro 业务层唯一入口
func (s *State) IsPro() bool

// Refresh 激活/解绑/启动时调用，同步内存 + 持久化
func (s *State) Refresh(key string, payload *Payload) error
```

- 过期检查：每次 `IsPro()` 比较 `expiresAt` 与 `time.Now().UnixMilli()`（契约：启动时检查 + 运行期兜底）
- 网络不可用时：**不做离线宽限**（契约：网络失败不降级），保持上次状态直到重启再验

---

## 3. 命令层接入（internal/app）

```go
// handlers.go 新增四个命令：
//   LicenseActivate(key)      → 本地验签快速失败 → Activate → State.Refresh
//   LicenseStatus()           → {isPro, expiresAt, activatedAt}
//   LicenseDeactivate()       → Deactivate → State.Clear
//   LicenseRedeem(feature)    → （V3 云端功能预留，本期不做）
```

前端页面：设置 → "关于/激活"入口（契约 §9 待定项：UI 文案与交互，参考 §6 文案表）。

---

## 4. 迁移步骤（L6）

1. `key.go` + `verify.go` + 测试（构造已知激活码样例：改一位签名/过期/错类型，验证各自失败分支）
2. `fingerprint.go` + 双端实测（真机跑 `Fingerprint()`，记录输出稳定性）
3. `client.go` + 契约 §8 联调清单（平台上线后执行，8 项全过）
4. `store.go` + `state.go` + 测试（模拟激活/过期/清除状态机）
5. 接入 `internal/app` 命令 + 前端激活页
6. **V1 阶段**：license 模块接口先实现"恒真"空实现（`IsPro() bool { return true }`），功能全开——订阅逻辑与业务解耦，V2 上线时切换实现，业务零改动

## 5. 验收标准（L6 完成定义）

- [ ] 单元测试全绿（解析/验签/指纹/状态机，覆盖契约 §8 的 8 个联调场景）
- [ ] 契约 §8 联调清单 8 项在 LicenseHub 上线后全部通过
- [ ] 断网激活 → "网络不可用"提示，不写入激活状态
- [ ] 篡改本地 license.json → 启动后视为未激活（不崩溃、提示重新激活）
- [ ] 业务层 `core/` 代码零 license 依赖（`go list -deps internal/core` 验证）

## 6. 注意事项

1. **私钥只在平台**（Vercel env），仓库里**永远不出现私钥**；公钥混淆只是提高门槛，默认接受可提取（契约 §5 明示）
2. **平台换密钥**：客户端本地验签失败降级在线验证已兜底（契约 §5 推荐方案），但长期策略仍是在新版本发布时同步更新内置公钥
3. **指纹命令的进程执行**：`os/exec` 需设超时（3s），防 PowerShell 卡死拖住激活流程；输出 trim + 失败降级
4. **时钟篡改**：用户改系统时间可绕过本地过期检查——契约模型已接受（强制力在服务端），实现时不做对抗，注释标注
5. **测试禁连真平台**：client.go 测试用 `httptest.Server` 模拟六分支响应，联调才打真实端点

---

## 7. 前端解耦与后续 Agent 约定（2026-08 起）

完整约定见：**[`docs/agent-license-version.md`](../agent-license-version.md)**（必读）。

摘要：

- 业务层（`internal/core`、导入向导）**零 license 依赖**；门禁只用 `IsPro()`。
- 前端业务页**禁止**直接调 `App.Version` / `App.License*`；统一经 `useAppMeta` / `VersionLicensePanel` / `LicenseView`。
- 改激活协议只动 `internal/license`；改版本号只动 `internal/version`。
