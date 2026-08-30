#!/usr/bin/env bash
# 一键发版：改版本 → 提交 → 打 tag → push（触发 CI → GitHub Release → Gitee）
# 详见 docs/gitee-releases.md
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION_GO="internal/version/version.go"
CONFIG_YML="build/config.yml"
DRY_RUN=0
NO_PUSH=0
FORCE_TAG=0
NOTES=""
NEW_VERSION=""

usage() {
  cat <<'EOF'
用法:
  bash scripts/release.sh <版本> [选项]

示例:
  bash scripts/release.sh 1.2.0
  bash scripts/release.sh 1.2.0 --notes "修复更新源"
  bash scripts/release.sh 1.2.0 --dry-run
  bash scripts/release.sh 1.2.0 --no-push

流程:
  1. 写入 internal/version/version.go 与 build/config.yml info.version
  2. git commit（若有变更）
  3. 打 tag v<版本> 并 push main + tag
  4. GitHub Actions Release 自动构建；若已配 GITEE_TOKEN 则同步公开仓

选项:
  --notes <文本>   写入 commit message 附加说明（可选）
  --dry-run        只打印将要做的事，不改文件、不 git
  --no-push        本地 commit + tag，不 push
  --force-tag      若本地/远程已有同名 tag，删除后重建（慎用）
  -h, --help       帮助
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --notes) NOTES="${2:?}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --no-push) NO_PUSH=1; shift ;;
    --force-tag) FORCE_TAG=1; shift ;;
    -h|--help) usage; exit 0 ;;
    -*)
      echo "未知选项: $1" >&2
      usage
      exit 1
      ;;
    *)
      if [[ -n "$NEW_VERSION" ]]; then
        echo "多余参数: $1" >&2
        exit 1
      fi
      NEW_VERSION="$1"
      shift
      ;;
  esac
done

if [[ -z "$NEW_VERSION" ]]; then
  usage
  exit 1
fi

NEW_VERSION="${NEW_VERSION#v}"
if ! [[ "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.+-]+)?$ ]]; then
  echo "错误: 版本号格式无效: ${NEW_VERSION}（期望如 1.2.0）" >&2
  exit 1
fi

TAG="v${NEW_VERSION}"

read_code_version() {
  sed -n 's/^const Version = "\(.*\)"/\1/p' "$VERSION_GO" | head -1
}

read_config_version() {
  awk '/^info:/{f=1;next} f && /^[^[:space:]#]/{exit} f && /^[[:space:]]+version:/{gsub(/["'\'']/,""); print $2; exit}' "$CONFIG_YML"
}

OLD_CODE="$(read_code_version)"
OLD_CFG="$(read_config_version)"

echo "当前: code=${OLD_CODE} config=${OLD_CFG}"
echo "目标: ${NEW_VERSION}  tag=${TAG}"

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "[dry-run] 将更新 ${VERSION_GO} 与 ${CONFIG_YML}"
  echo "[dry-run] 将 commit + tag ${TAG} + $([ "$NO_PUSH" -eq 1 ] && echo '不 push' || echo 'push origin')"
  exit 0
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "错误: 工作区有未提交变更，请先处理后再发版：" >&2
  git status -sb >&2
  exit 1
fi

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$BRANCH" != "main" && "$BRANCH" != "master" ]]; then
  echo "警告: 当前分支是 ${BRANCH}（建议在 main 发版）。继续…" >&2
fi

# 远程已有同名 tag 时提前失败（除非 --force-tag）
if [[ "$FORCE_TAG" -eq 0 ]] && git ls-remote --exit-code --tags origin "refs/tags/${TAG}" >/dev/null 2>&1; then
  echo "错误: 远程已有 tag ${TAG}（需要重建请加 --force-tag）" >&2
  exit 1
fi

# 写入版本
python3 - "$VERSION_GO" "$NEW_VERSION" <<'PY'
from pathlib import Path
import re, sys
path, ver = Path(sys.argv[1]), sys.argv[2]
text = path.read_text(encoding="utf-8")
new, n = re.subn(
    r'^const Version = "[^"]*"',
    f'const Version = "{ver}"',
    text,
    count=1,
    flags=re.M,
)
if n != 1:
    raise SystemExit(f"无法更新 {path}: Version 常量匹配数={n}")
path.write_text(new, encoding="utf-8")
PY

python3 - "$CONFIG_YML" "$NEW_VERSION" <<'PY'
from pathlib import Path
import re, sys
path, ver = Path(sys.argv[1]), sys.argv[2]
lines = path.read_text(encoding="utf-8").splitlines(keepends=True)
out, in_info, done = [], False, False
for line in lines:
    if re.match(r"^info:\s*$", line):
        in_info = True
        out.append(line)
        continue
    if in_info and re.match(r"^[^\s#]", line):
        in_info = False
    if in_info and re.match(r"^(\s+)version:\s*", line) and not done:
        indent = re.match(r"^(\s+)", line).group(1)
        m = re.search(r"#\s*(.*)$", line.rstrip("\n"))
        comment = f" # {m.group(1)}" if m else ""
        out.append(f'{indent}version: "{ver}"{comment}\n')
        done = True
        continue
    out.append(line)
if not done:
    raise SystemExit(f"无法更新 {path}: 未找到 info.version")
path.write_text("".join(out), encoding="utf-8")
PY

CODE="$(read_code_version)"
CFG="$(read_config_version)"
if [[ "$CODE" != "$NEW_VERSION" || "$CFG" != "$NEW_VERSION" ]]; then
  echo "错误: 写入后版本不一致 code=${CODE} config=${CFG}" >&2
  exit 1
fi

MSG="chore: release ${TAG}"
if [[ -n "$NOTES" ]]; then
  MSG="${MSG}

${NOTES}"
fi

git add "$VERSION_GO" "$CONFIG_YML"
if git diff --cached --quiet; then
  echo "版本文件无变更（可能已是 ${NEW_VERSION}）"
else
  git commit -m "$MSG"
  echo "已提交: ${MSG%%$'\n'*}"
fi

if git rev-parse "$TAG" >/dev/null 2>&1; then
  if [[ "$FORCE_TAG" -eq 1 ]]; then
    git tag -d "$TAG"
    echo "已删除本地 tag ${TAG}"
  else
    echo "错误: 本地已有 tag ${TAG}（需要重建请加 --force-tag）" >&2
    exit 1
  fi
fi

git tag -a "$TAG" -m "Release ${TAG}${NOTES:+ — ${NOTES}}"
echo "已创建 tag ${TAG}"

if [[ "$NO_PUSH" -eq 1 ]]; then
  echo "已跳过 push（--no-push）。完成后请执行:"
  echo "  git push origin HEAD"
  echo "  git push origin ${TAG}"
  exit 0
fi

git push origin HEAD
if [[ "$FORCE_TAG" -eq 1 ]]; then
  git push origin ":refs/tags/${TAG}" 2>/dev/null || true
fi
git push origin "$TAG"

echo
echo "已推送 ${TAG}。请到 Actions 查看 Release workflow。"
echo "验收（跑绿后）:"
echo "  curl -i https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json"
echo "需已配置 GitHub secret GITEE_TOKEN，否则只会发 GitHub Release、不会同步 Gitee。"
