# SignPath Code Signing

Free code signing provided by [SignPath.io](https://signpath.io/), certificate by [SignPath Foundation](https://signpath.org/).

## Product

- **Name**: WPS Enhancer (Go)
- **Repository**: https://github.com/pikachuprogrammer01/wps-enhancer-go
- **License**: [MIT](LICENSE)
- **Distribution**: https://gitee.com/pikachuprogrammer01/my-software-releases/releases (tags `wps-enhancer-v*`)

## Signing policy

1. Only artifacts built by GitHub Actions workflow [`.github/workflows/release.yml`](.github/workflows/release.yml) on tag **`v*`** are submitted for signing.
2. Signing order (required so the NSIS payload is also signed):
   1. Build `wps-enhancer-go.exe`
   2. Sign the app exe (`windows-app-exe`)
   3. Build the NSIS installer **from the signed exe**
   4. Sign the installer (`windows-installer-exe`)
3. macOS `.app` bundles are ad-hoc signed in CI only (not submitted to SignPath).
4. Private keys are held by SignPath (HSM-backed). **This project does not store code signing private keys.**

## Artifact configurations

| Slug | File | Description |
|------|------|-------------|
| `windows-app-exe` | [`.signpath/windows-app-exe.xml`](.signpath/windows-app-exe.xml) | Portable / update zip contents |
| `windows-installer-exe` | [`.signpath/windows-installer-exe.xml`](.signpath/windows-installer-exe.xml) | NSIS installer |

Create a SignPath project with slug **`wps-enhancer-go`** (or set repo variable `SIGNPATH_PROJECT_SLUG`) and import **both** configurations above.

## Approvers

Repository maintainers with SignPath **submitter** role on signing policy **`release-signing`**.

## Setup

See [`docs/signpath-setup.md`](docs/signpath-setup.md) for GitHub Secrets and first release verification.
