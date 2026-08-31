# SignPath Windows 签名接入

> 仓库已 MIT 开源后，用 [SignPath Foundation](https://signpath.org/) 免费 Authenticode 签名，减少 Windows SmartScreen 拦截。  
> 策略说明见根目录 [`SIGNPATH.md`](../SIGNPATH.md)。

---

## 1. 申请 SignPath OSS（一次性）

1. 打开 https://signpath.io/product/open-source  
2. 填写仓库 URL：`https://github.com/pikachuprogrammer01/wps-enhancer-go`  
3. 说明：MIT 桌面工具，Gitee 公开发布，需 Windows 签名  
4. 审核通常数个工作日；通过后登录 SignPath 控制台

---

## 2. 在 SignPath 控制台配置

| 项 | 建议值 |
|----|--------|
| Project slug | `wps-enhancer-go` |
| Signing policy slug | `release-signing` |
| Trusted build system | GitHub |
| Repository | 本仓库 |
| Workflow file | `.github/workflows/release.yml` |
| Branch/tag filter | `refs/tags/v*` |

**导入两份** artifact configuration（slug 必须一致）：

| Slug | 文件 |
|------|------|
| `windows-app-exe` | [`.signpath/windows-app-exe.xml`](../.signpath/windows-app-exe.xml) |
| `windows-installer-exe` | [`.signpath/windows-installer-exe.xml`](../.signpath/windows-installer-exe.xml) |

安装 **SignPath GitHub App** 到本仓库（控制台会引导）。

---

## 3. GitHub Secrets / Variables

在仓库 **Settings → Secrets and variables → Actions** 添加：

| Name | 说明 |
|------|------|
| `SIGNPATH_API_TOKEN` | SignPath 用户 API Token（对该 project 有 submit 权限） |
| `SIGNPATH_ORGANIZATION_ID` | 组织 UUID（控制台 Organization settings） |

可选 **Variables**（与默认不一致时）：

| Name | 默认 |
|------|------|
| `SIGNPATH_PROJECT_SLUG` | `wps-enhancer-go` |
| `SIGNPATH_SIGNING_POLICY` | `release-signing` |

未配置上述两个 Secret 时，Release 仍发 **未签名** Windows 包（与改前行为一致）。

---

## 4. CI 流水线（签名顺序）

```
build-windows          → 未签名 wps-enhancer-go.exe
sign-app-exe           → SignPath 签 exe（或旁路）
build-installer        → 用已签名 exe 打 NSIS（不 rebuild）
sign-installer         → SignPath 签 installer → 产出 go-windows
release                → GitHub + Gitee
```

要点：

- **先签主程序，再打安装器**，这样安装出的 exe 也带签名。  
- 上传给 SignPath 的是**单个 exe**；由 `upload-artifact` 自动打 zip，配置根元素为 `<zip-file>`（见 XML）。  
- 回包从 `output-artifact-directory` 取已解压的 exe（兼容偶发 zip）。

---

## 5. 验证

1. 打 tag 触发 Release，或 Actions 手动 **Run workflow**  
2. 看 jobs：`sign-app-exe`、`sign-installer` 均成功  
3. 下载 `WPSEnhancer-windows-x86_64-installer.exe` 与 zip 内 exe  
4. 右键 → **属性 → 数字签名**：两者均应显示 SignPath Foundation  
5. 在 Windows 上再试 Gitee 下载 / 运行

---

## 6. 故障排查

| 现象 | 处理 |
|------|------|
| 签名 job 跳过 | 检查 `SIGNPATH_API_TOKEN` / `SIGNPATH_ORGANIZATION_ID` |
| Artifact configuration 不匹配 | 控制台需有 `windows-app-exe` **与** `windows-installer-exe` 两个 slug |
| 路径找不到 pe-file | 确认上传的是 exe 本体（勿再预打一层 zip） |
| Signing policy 拒绝 | Trusted build 是否绑定 `release.yml` + tag `v*` |
| 超时 | 控制台看 signing request；必要时加大 `wait-for-completion-timeout-in-seconds` |

---

## 7. 相关文件

- CI：`.github/workflows/release.yml`（`sign-app-exe` / `build-installer` / `sign-installer`）
- Task：`windows:create:nsis:installer:from-bin`（不 rebuild）
- 策略：[`SIGNPATH.md`](../SIGNPATH.md)
- 发版：[`release.md`](./release.md)
