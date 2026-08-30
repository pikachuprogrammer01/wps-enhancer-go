#!/usr/bin/env bash
# 将本仓 CI/本地产物发布到 Gitee 统一发布仓 my-software-releases。
# 约定：每个产品一条独立分支（分支名 = PRODUCT），清单为分支根目录 update.json。
# 详见 docs/gitee-releases.md
set -euo pipefail

PRODUCT="${GITEE_PRODUCT:-wps-enhancer}"
OWNER="${GITEE_OWNER:-pikachuprogrammer01}"
REPO="${GITEE_REPO:-my-software-releases}"
API="https://gitee.com/api/v5"
VERSION=""
NOTES=""
ASSETS_DIR=""
RELEASE_REPO_DIR="${RELEASE_REPO_DIR:-}"
DRY_RUN=0
FORCE=0
EXTRA_ASSETS=""
REL_ID=""
TMP_CLONE=0
ASKPASS_SCRIPT=""

usage() {
  cat <<'EOF'
用法:
  GITEE_TOKEN=xxx bash scripts/publish-gitee.sh \
    --version 1.1.0 \
    --assets-dir ./artifacts \
    [--notes "更新说明"] \
    [--release-repo-dir /path/to/my-software-releases] \
    [--asset /path/to/extra.file] \
    [--force] \
    [--dry-run]

环境变量:
  GITEE_TOKEN          必填（dry-run 除外）；私人令牌，需 Releases + 仓库写权限
  GITEE_PRODUCT        默认 wps-enhancer（= 发布仓分支名）
  GITEE_OWNER / REPO   默认 pikachuprogrammer01 / my-software-releases
  RELEASE_REPO_DIR     发布仓本地副本；未设则临时 clone 后 push

产物约定（从 --assets-dir 自动收集）:
  WPSEnhancer-macos-arm64.zip              → urls.macos-arm64
  WPSEnhancer-macos-x86_64.zip             → urls.macos-x86_64
  WPSEnhancer-windows-x86_64.zip           → urls.windows-x86_64（自动更新必填）
  WPSEnhancer-windows-x86_64-installer.exe → 仅上传 Release，不进 urls

分支模型: 每产品独立分支（本产品 → 分支 wps-enhancer，根目录 update.json）。
--force: 若同名 Release/tag 已存在则先删除再发（用于重试不完整发版）。
失败回滚: 若已创建 Release 但上传/推送失败，会删除该 Release 及其 git tag，便于重试。
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="${2:?}"; shift 2 ;;
    --notes) NOTES="${2:?}"; shift 2 ;;
    --assets-dir) ASSETS_DIR="${2:?}"; shift 2 ;;
    --release-repo-dir) RELEASE_REPO_DIR="${2:?}"; shift 2 ;;
    --asset) EXTRA_ASSETS="${EXTRA_ASSETS}${EXTRA_ASSETS:+$'\n'}${2:?}"; shift 2 ;;
    --product) PRODUCT="${2:?}"; shift 2 ;;
    --force) FORCE=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  echo "错误: 需要 --version" >&2
  exit 1
fi
VERSION="${VERSION#v}"
TAG="${PRODUCT}-v${VERSION}"
PRODUCT_BRANCH="$PRODUCT"
DOWNLOAD_BASE="https://gitee.com/${OWNER}/${REPO}/releases/download/${TAG}"
UPDATE_RAW_URL="https://gitee.com/${OWNER}/${REPO}/raw/${PRODUCT_BRANCH}/update.json"

if [[ "$DRY_RUN" -eq 0 && -z "${GITEE_TOKEN:-}" ]]; then
  echo "错误: 需要 GITEE_TOKEN（或使用 --dry-run）" >&2
  exit 1
fi

# curl JSON API：校验 HTTP 状态；成功时把 body 写到 stdout。
api_json() {
  local method="$1" path="$2"
  shift 2
  local tmp code
  tmp="$(mktemp)"
  code=$(curl -sS -o "$tmp" -w "%{http_code}" -X "$method" \
    -H "Authorization: token ${GITEE_TOKEN}" \
    -H "Accept: application/json" \
    "${API}${path}" \
    "$@") || {
    rm -f "$tmp"
    echo "错误: 请求失败 ${method} ${path}" >&2
    return 1
  }
  if [[ "$code" -lt 200 || "$code" -ge 300 ]]; then
    echo "错误: ${method} ${path} → HTTP ${code}" >&2
    head -c 2000 "$tmp" >&2 || true
    echo >&2
    rm -f "$tmp"
    return 1
  fi
  cat "$tmp"
  rm -f "$tmp"
}

api_upload_file() {
  local release_id="$1" file_path="$2"
  local tmp code
  tmp="$(mktemp)"
  code=$(curl -sS -o "$tmp" -w "%{http_code}" -X POST \
    -H "Authorization: token ${GITEE_TOKEN}" \
    -F "file=@${file_path}" \
    "${API}/repos/${OWNER}/${REPO}/releases/${release_id}/attach_files") || {
    rm -f "$tmp"
    echo "错误: 上传请求失败: $(basename "$file_path")" >&2
    return 1
  }
  if [[ "$code" -lt 200 || "$code" -ge 300 ]]; then
    echo "错误: 上传 $(basename "$file_path") → HTTP ${code}" >&2
    head -c 2000 "$tmp" >&2 || true
    echo >&2
    rm -f "$tmp"
    return 1
  fi
  if ! python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); sys.exit(0 if (d.get("browser_download_url") or d.get("name") or d.get("id")) else 1)' "$tmp"; then
    echo "错误: 上传响应缺少附件字段: $(basename "$file_path")" >&2
    head -c 2000 "$tmp" >&2 || true
    echo >&2
    rm -f "$tmp"
    return 1
  fi
  rm -f "$tmp"
}

delete_release_if_any() {
  [[ -n "${REL_ID:-}" ]] || return 0
  echo "回滚: 删除未完成的 Release id=${REL_ID} tag=${TAG} ..." >&2
  local code
  code=$(curl -sS -o /tmp/gitee-del.json -w "%{http_code}" -X DELETE \
    -H "Authorization: token ${GITEE_TOKEN}" \
    "${API}/repos/${OWNER}/${REPO}/releases/${REL_ID}" || true)
  if [[ "$code" == "204" || "$code" == "200" ]]; then
    echo "回滚: Release 已删除" >&2
  else
    echo "回滚: 删除 Release 失败 HTTP ${code}（见 /tmp/gitee-del.json）" >&2
  fi
  code=$(curl -sS -o /tmp/gitee-del-tag.json -w "%{http_code}" -X DELETE \
    -H "Authorization: token ${GITEE_TOKEN}" \
    "${API}/repos/${OWNER}/${REPO}/tags/${TAG}" || true)
  if [[ "$code" == "204" || "$code" == "200" ]]; then
    echo "回滚: tag ${TAG} 已删除，可重试" >&2
  elif [[ "$code" == "404" ]]; then
    echo "回滚: tag ${TAG} 已不存在" >&2
  else
    echo "回滚: 删除 tag 失败 HTTP ${code}，请手动删除 ${TAG}（见 /tmp/gitee-del-tag.json）" >&2
  fi
  REL_ID=""
}

cleanup() {
  local ec=$?
  if [[ $ec -ne 0 && "$DRY_RUN" -eq 0 ]]; then
    delete_release_if_any || true
  fi
  [[ -n "${ASKPASS_SCRIPT:-}" && -f "${ASKPASS_SCRIPT:-}" ]] && rm -f "$ASKPASS_SCRIPT"
  [[ "$TMP_CLONE" -eq 1 && -n "${RELEASE_REPO_DIR:-}" && -d "${RELEASE_REPO_DIR:-}" ]] && rm -rf "$RELEASE_REPO_DIR"
  [[ -n "${TMP_LIST:-}" && -f "${TMP_LIST:-}" ]] && rm -f "$TMP_LIST"
  [[ -n "${URL_MAP:-}" && -f "${URL_MAP:-}" ]] && rm -f "$URL_MAP"
  exit "$ec"
}
trap cleanup EXIT

url_key_for() {
  case "$(basename "$1")" in
    WPSEnhancer-macos-arm64.zip) echo "macos-arm64" ;;
    WPSEnhancer-macos-x86_64.zip) echo "macos-x86_64" ;;
    WPSEnhancer-windows-x86_64.zip) echo "windows-x86_64" ;;
    *) echo "" ;;
  esac
}

FILES=""
if [[ -n "$ASSETS_DIR" ]]; then
  if [[ ! -d "$ASSETS_DIR" ]]; then
    echo "错误: --assets-dir 不存在: $ASSETS_DIR" >&2
    exit 1
  fi
  while IFS= read -r f; do
    [[ -z "$f" ]] && continue
    FILES="${FILES}${FILES:+$'\n'}${f}"
  done < <(find "$ASSETS_DIR" -type f -name 'WPSEnhancer-*' | sort)
fi
if [[ -n "$EXTRA_ASSETS" ]]; then
  FILES="${FILES}${FILES:+$'\n'}${EXTRA_ASSETS}"
fi

if [[ -z "$FILES" ]]; then
  echo "错误: 未找到任何产物（检查 --assets-dir / --asset）" >&2
  exit 1
fi

TMP_LIST="$(mktemp)"
URL_MAP="$(mktemp)"
printf '%s\n' "$FILES" > "$TMP_LIST"

HAS_WIN_ZIP=0
HAS_MAC=0
while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  if [[ ! -f "$f" ]]; then
    echo "错误: 文件不存在: $f" >&2
    exit 1
  fi
  base="$(basename "$f")"
  key="$(url_key_for "$f")"
  echo "将上传: $base${key:+ → urls.$key}"
  if [[ -n "$key" ]]; then
    printf '%s\t%s\n' "$key" "${DOWNLOAD_BASE}/${base}" >> "$URL_MAP"
    case "$key" in
      windows-x86_64) HAS_WIN_ZIP=1 ;;
      macos-*) HAS_MAC=1 ;;
    esac
  fi
done < "$TMP_LIST"

if [[ "$HAS_WIN_ZIP" -ne 1 ]]; then
  echo "错误: 缺少 WPSEnhancer-windows-x86_64.zip（自动更新必填）" >&2
  exit 1
fi
if [[ "$HAS_MAC" -ne 1 ]]; then
  echo "错误: 至少需要一个 macOS zip（arm64 或 x86_64）" >&2
  exit 1
fi

NOTES_ESC="${NOTES:-WPS Enhancer ${VERSION}}"

UPDATE_JSON=$(python3 - "$VERSION" "$NOTES_ESC" "$URL_MAP" <<'PY'
import json, sys
version, notes, path = sys.argv[1], sys.argv[2], sys.argv[3]
urls = {}
order = ["macos-arm64", "macos-x86_64", "windows-x86_64"]
with open(path, encoding="utf-8") as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        k, v = line.split("\t", 1)
        urls[k] = v
ordered = {k: urls[k] for k in order if k in urls}
print(json.dumps({"version": version, "urls": ordered, "notes": notes}, ensure_ascii=False, indent=2))
PY
)

echo "---- update.json ----"
echo "$UPDATE_JSON"
echo "---- branch: ${PRODUCT_BRANCH}  tag: ${TAG} ----"
echo "---- raw: ${UPDATE_RAW_URL} ----"

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "[dry-run] 跳过 Gitee API / git push"
  exit 0
fi

# Gitee 对不存在的 tag 可能返回 HTTP 200 + body null；须有 id 才算「已存在」
release_exists() {
  local body
  body=$(api_json GET "/repos/${OWNER}/${REPO}/releases/tags/${TAG}" 2>/dev/null || true)
  echo "$body" | python3 -c 'import json,sys
raw=sys.stdin.read().strip()
if not raw or raw=="null":
  raise SystemExit(1)
try:
  d=json.loads(raw)
except Exception:
  raise SystemExit(1)
raise SystemExit(0 if isinstance(d, dict) and d.get("id") else 1)'
}

delete_existing_release_by_tag() {
  local body id
  body=$(api_json GET "/repos/${OWNER}/${REPO}/releases/tags/${TAG}" 2>/dev/null || true)
  id=$(echo "$body" | python3 -c 'import json,sys
raw=sys.stdin.read().strip()
if not raw or raw=="null":
  print("")
else:
  try:
    d=json.loads(raw)
    print(d.get("id") or "")
  except Exception:
    print("")')
  if [[ -n "$id" ]]; then
    echo " --force: 删除已有 Release id=${id} tag=${TAG}"
    api_json DELETE "/repos/${OWNER}/${REPO}/releases/${id}" >/dev/null || true
  fi
  # 顺带删 git tag（可能残留）
  code=$(curl -sS -o /tmp/gitee-del-tag.json -w "%{http_code}" -X DELETE \
    -H "Authorization: token ${GITEE_TOKEN}" \
    "${API}/repos/${OWNER}/${REPO}/tags/${TAG}" || true)
  echo " --force: 删除 tag ${TAG} → HTTP ${code}"
}

if release_exists; then
  if [[ "$FORCE" -eq 1 ]]; then
    delete_existing_release_by_tag
  else
    echo "错误: Release tag 已存在: ${TAG}（重发请加 --force）" >&2
    exit 1
  fi
fi

# —— 先确保产品分支存在（Release 的 target_commitish 必须已存在）——
if [[ -z "$RELEASE_REPO_DIR" ]]; then
  RELEASE_REPO_DIR="$(mktemp -d)"
  TMP_CLONE=1
fi

ASKPASS_SCRIPT="$(mktemp)"
chmod 700 "$ASKPASS_SCRIPT"
cat > "$ASKPASS_SCRIPT" <<'ASK'
#!/bin/sh
case "$1" in
  *Username*) printf '%s\n' "oauth2" ;;
  *) printf '%s\n' "${GITEE_TOKEN}" ;;
esac
ASK

export GITEE_TOKEN
export GIT_ASKPASS="$ASKPASS_SCRIPT"
export GIT_TERMINAL_PROMPT=0

if [[ "$TMP_CLONE" -eq 1 ]]; then
  git -c credential.helper= clone --depth 1 \
    "https://gitee.com/${OWNER}/${REPO}.git" \
    "$RELEASE_REPO_DIR"
fi

ensure_product_branch() {
  (
    cd "$RELEASE_REPO_DIR"
    git config user.email "ci@wps-enhancer-go.local"
    git config user.name "wps-enhancer-go-release"
    export GIT_ASKPASS GITEE_TOKEN GIT_TERMINAL_PROMPT=0

    if git ls-remote --exit-code --heads origin "${PRODUCT_BRANCH}" >/dev/null 2>&1; then
      git fetch --depth 1 origin "${PRODUCT_BRANCH}"
      git checkout -B "${PRODUCT_BRANCH}" "FETCH_HEAD"
      echo "已检出产品分支 ${PRODUCT_BRANCH}"
    else
      git checkout -B "${PRODUCT_BRANCH}"
      # 勿推送空 urls 的 update.json（客户端会判「格式错误」）；用空提交即可满足 target_commitish
      git commit --allow-empty -m "chore(${PRODUCT}): bootstrap branch"
      git push -u origin "HEAD:${PRODUCT_BRANCH}"
      echo "已创建并推送产品分支 ${PRODUCT_BRANCH}"
    fi
  )
}
ensure_product_branch

# Release 挂在产品分支上
CREATE=$(api_json POST "/repos/${OWNER}/${REPO}/releases" \
  --data-urlencode "tag_name=${TAG}" \
  --data-urlencode "name=${TAG}" \
  --data-urlencode "body=${NOTES_ESC}" \
  --data-urlencode "target_commitish=${PRODUCT_BRANCH}" \
  --data-urlencode "prerelease=false")

REL_ID=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("id") or "")' <<<"$CREATE")
if [[ -z "$REL_ID" ]]; then
  echo "错误: 创建 Release 成功但无 id: $CREATE" >&2
  exit 1
fi
echo "已创建 Release id=${REL_ID} tag=${TAG}（target=${PRODUCT_BRANCH}）"

while IFS= read -r f; do
  [[ -z "$f" ]] && continue
  echo "上传 $(basename "$f") ..."
  api_upload_file "$REL_ID" "$f"
done < "$TMP_LIST"

# 附件就绪后再写清单，避免 urls 指向尚未上传的文件
(
  cd "$RELEASE_REPO_DIR"
  export GIT_ASKPASS GITEE_TOKEN GIT_TERMINAL_PROMPT=0
  # 确保仍在产品分支（勿用 checkout -B，避免误把分支重置到错误 tip）
  git checkout "${PRODUCT_BRANCH}"
  python3 -c 'import pathlib,sys; pathlib.Path(sys.argv[1]).write_text(sys.argv[2]+"\n", encoding="utf-8")' \
    "update.json" "$UPDATE_JSON"
  git add update.json
  if git diff --cached --quiet; then
    echo "update.json 无变更"
  else
    git commit -m "release(${PRODUCT}): v${VERSION}"
    git push origin "HEAD:${PRODUCT_BRANCH}"
    echo "已推送 update.json → 分支 ${PRODUCT_BRANCH}"
  fi
)

REL_ID=""

echo "完成: ${TAG}"
echo "清单: ${UPDATE_RAW_URL}"
