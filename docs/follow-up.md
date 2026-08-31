# 后续待办

> 更新：2026-08-31（MIT 已公开；SignPath CI 已接入，待控制台 + Secrets）

## 已完成

- 测试补强、更新引导、覆盖率门禁
- 本地 mac `task package` / Windows 交叉 build
- 独立仓 + `.github/workflows/test.yml` / `release.yml`（tag `v*`）
- Gitee 对接：`DefaultUpdateURL`、`scripts/publish-gitee.sh`、`docs/gitee-releases.md`（产品分支 `wps-enhancer`）
- **MIT License** + Pro 订阅条款 [`docs/subscription-terms.md`](subscription-terms.md)
- **SignPath**：`SIGNPATH.md`、`.signpath/windows-*.xml`、`release.yml`（签 exe → NSIS → 签 installer）

## 下一步

### 发版 / 更新（L7）

1. ~~在 `my-software-releases` 建分支 `wps-enhancer`~~（已完成）
2. 配置 GitHub secret `GITEE_TOKEN`，端到端更新验收（见 `docs/gitee-releases.md` §7）
3. **SignPath**：申请 OSS → 控制台建 project → 配 Secrets → 打 tag 验证签名（见 [`docs/signpath-setup.md`](signpath-setup.md)）

### 开源（进行中）

1. ~~GitHub 公开 + MIT~~（已完成）
2. 确认无密钥泄露（`.env`、LicenseHub 私钥、`GITEE_TOKEN` 等不进仓库）
3. 完成 SignPath 审核与首次签名发版
4. Pro 订阅仍走 LicenseHub，条款见 `docs/subscription-terms.md`

### 产品

1. xlsx `PreviewDisplay` WYSIWYG
2. LicenseHub 联调

## 发版

见 **`docs/release.md`**。一句话：

```bash
bash scripts/release.sh 1.2.0
```
