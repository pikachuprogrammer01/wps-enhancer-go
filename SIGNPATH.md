# SignPath Code Signing（暂缓）

> **状态：暂不启用。** 本仓库许可为「个人免费 / 商业收费」，**不是** OSI 开源，不符合 [SignPath Foundation OSS](https://signpath.org/) 申请条件。  
> CI 在未配置 `SIGNPATH_*` Secret 时会自动跳过签名。若将来改回 OSI 许可或自购 OV 证书，可再启用。  
> 操作备忘见 [`docs/signpath-setup.md`](docs/signpath-setup.md)。

## Product

- **Name**: WPS Enhancer (Go)
- **Repository**: https://github.com/pikachuprogrammer01/wps-enhancer-go
- **License**: [LICENSE](LICENSE)（proprietary; free for personal non-commercial use only）
- **Distribution**: https://gitee.com/pikachuprogrammer01/my-software-releases/releases (tags `wps-enhancer-v*`)

## Pipeline (when secrets are set)

1. Build `wps-enhancer-go.exe` → sign (`windows-app-exe`) → NSIS from signed exe → sign installer (`windows-installer-exe`).
2. Artifact configs: [`.signpath/windows-app-exe.xml`](.signpath/windows-app-exe.xml), [`.signpath/windows-installer-exe.xml`](.signpath/windows-installer-exe.xml).
3. Private keys would be held by SignPath (HSM); this project does not store code signing private keys.
