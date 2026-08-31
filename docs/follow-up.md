# 后续待办

> 更新：2026-08-31（许可改为个人免费 / 商业收费；Windows 签名暂缓）

## 已完成

- 测试补强、更新引导、覆盖率门禁
- 本地 mac `task package` / Windows 交叉 build
- 独立仓 + `.github/workflows/test.yml` / `release.yml`（tag `v*`）
- Gitee 对接：`DefaultUpdateURL`、`scripts/publish-gitee.sh`、`docs/gitee-releases.md`（产品分支 `wps-enhancer`）
- 许可：个人非商业免费、公司/商业须付费（[`LICENSE`](../LICENSE)、[`subscription-terms.md`](subscription-terms.md)）
- SignPath CI 骨架已预留（**暂不启用**）

## 下一步

### 发版 / 更新（L7）

1. ~~在 `my-software-releases` 建分支 `wps-enhancer`~~（已完成）
2. 确认 GitHub secret `GITEE_TOKEN` 稳定；同版本重发已带 `--force`
3. Windows 网页下载拦截：文档引导 PowerShell / 应用内更新；**签名暂缓**（见下）

### 签名（暂缓）

当前许可**不符合** SignPath OSS（需 OSI 开源）。若以后再做签名：

- 自购 OV 代码签名证书，或  
- 改回 OSI 许可后再申请 SignPath  

相关文件可留作参考：`SIGNPATH.md`、`docs/signpath-setup.md`、`.signpath/`（未配 Secret 时 CI 自动跳过签名）。

### 产品

1. xlsx `PreviewDisplay` WYSIWYG  
2. LicenseHub 联调（UI 已用 `SHOW_SUBSCRIPTION_UI=false` 隐藏，代码未删）  

## 发版

见 **`docs/release.md`**。一句话：

```bash
bash scripts/release.sh 1.2.0
```
