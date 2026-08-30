# 后续待办

> 更新：2026-08-31（Gitee 统一发布仓对接已落代码；待首发验收）

## 已完成

- 测试补强、更新引导、覆盖率门禁
- 本地 mac `task package` / Windows 交叉 build
- 独立仓 + `.github/workflows/test.yml` / `release.yml`（tag `v*`）
- Gitee 对接：`DefaultUpdateURL`、`scripts/publish-gitee.sh`、`docs/gitee-releases.md`（产品分支 `wps-enhancer`）

## 下一步

1. 在 `my-software-releases` 建分支 `wps-enhancer`（或交给首次 `publish-gitee.sh`）+ `main` README 贴 snippet
2. 配置 GitHub secret `GITEE_TOKEN`，打 `v1.1.0` 试发（NSIS + mac zip → 公开仓）
3. 更新端到端人工验收（见 `docs/gitee-releases.md` §7）
4. xlsx `PreviewDisplay` WYSIWYG
5. LicenseHub 联调；签名/公证可选

## 发版

见 **`docs/release.md`**。一句话：

```bash
bash scripts/release.sh 1.2.0
```
