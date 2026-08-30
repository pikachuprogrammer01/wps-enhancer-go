# 后续待办

> 更新：2026-08-30（已迁出独立仓 https://github.com/pikachuprogrammer01/wps-enhancer-go）

## 已完成

- 测试补强、更新引导、覆盖率门禁
- 本地 mac `task package` / Windows 交叉 build
- 独立仓 + `.github/workflows/test.yml` / `release.yml`（tag `v*`）

## 下一步

1. Actions 首次跑绿（push / PR）
2. 打 `v1.1.0` 试发 Release（NSIS + mac zip）
3. 更新端到端人工验收（注意：私有仓 Release 匿名不可下，更新源优先 Gitee/`update_url`）
4. xlsx `PreviewDisplay` WYSIWYG
5. LicenseHub 联调；签名/公证可选

## 发版

```bash
# Version ≡ build/config.yml ≡ tag
git tag v1.1.0
git push origin v1.1.0
```
