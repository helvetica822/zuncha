#!/bin/bash
# Claude Code Stop Hook — WIPコミット & GitHub Issue作成
# 配置場所: .claude/hooks/stop_create_issue.sh
set -euo pipefail

# ── ログ設定 ───────────────────────────────────────────────────────────────────
LOG_DIR="${HOME}/.claude/logs"
LOG_FILE="${LOG_DIR}/stop_hook.log"
mkdir -p "$LOG_DIR"

_log() {
  local level="$1"; shift
  local msg="$*"
  local timestamp
  timestamp=$(date '+%Y-%m-%d %H:%M:%S')
  echo "[${timestamp}] [${level}] ${msg}" | tee -a "$LOG_FILE" >&2
}
log_info()  { _log "INFO " "$*"; }
log_warn()  { _log "WARN " "$*"; }
log_error() { _log "ERROR" "$*"; }

# ログローテーション: 1MB超えたら .1 にリネーム
if [[ -f "$LOG_FILE" ]] && (( $(stat -c%s "$LOG_FILE" 2>/dev/null || echo 0) > 1048576 )); then
  mv "$LOG_FILE" "${LOG_FILE}.1"
fi

log_info "========== Stop hook 開始 =========="

# ── 前提チェック ──────────────────────────────────────────────────────────────
if ! command -v gh &>/dev/null; then
  log_warn "gh CLI が見つかりません。スキップします。"; exit 0
fi
if ! command -v jq &>/dev/null; then
  log_warn "jq が見つかりません。スキップします。"; exit 0
fi
if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  log_warn "gitリポジトリ外です。スキップします。"; exit 0
fi

# ── stdinからセッション情報を受け取る ─────────────────────────────────────────
HOOK_INPUT=$(cat)
SESSION_ID=$(echo "$HOOK_INPUT" | jq -r '.session_id // ""')
TRANSCRIPT_PATH=$(echo "$HOOK_INPUT" | jq -r '.transcript_path // ""')
log_info "session_id=${SESSION_ID}"
log_info "transcript_path=${TRANSCRIPT_PATH}"

# ── トランスクリプトからテキストを抽出するヘルパー ────────────────────────────
# 形式1: {"role":"assistant","content":[...]}
# 形式2: {"type":"assistant","message":{"role":"assistant","content":[...]}}
_extract_text() {
  local json="$1"
  echo "$json" | jq -r '
    (.message.content // .content // null) |
    if . == null then ""
    elif type == "array" then
      [ .[] | select(.type == "text") | .text ] | join("\n")
    elif type == "string" then .
    else ""
    end
  ' 2>/dev/null || echo ""
}

# ── トランスクリプト解析 ───────────────────────────────────────────────────────
FIRST_USER_MSG="(取得できませんでした)"
LAST_CLAUDE_MSG="(取得できませんでした)"
TOOL_STATS="(取得できませんでした)"

if [[ -n "$TRANSCRIPT_PATH" && -f "$TRANSCRIPT_PATH" ]]; then

  # 最初のユーザーメッセージ（作業指示）
  RAW_FIRST_USER=$(grep -a '"role"[[:space:]]*:[[:space:]]*"user"' "$TRANSCRIPT_PATH" 2>/dev/null | head -1 || true)
  if [[ -n "$RAW_FIRST_USER" ]]; then
    FIRST_USER_MSG=$(_extract_text "$RAW_FIRST_USER" | head -c 1000)
    [[ -z "$FIRST_USER_MSG" ]] && FIRST_USER_MSG="(パース失敗)"
  fi

  # 最後のアシスタントメッセージ
  RAW_LAST_ASSISTANT=$(grep -a '"role"[[:space:]]*:[[:space:]]*"assistant"' "$TRANSCRIPT_PATH" 2>/dev/null | tail -1 || true)
  if [[ -n "$RAW_LAST_ASSISTANT" ]]; then
    LAST_CLAUDE_MSG=$(_extract_text "$RAW_LAST_ASSISTANT" | head -c 8000)
    [[ -z "$LAST_CLAUDE_MSG" ]] && LAST_CLAUDE_MSG="(パース失敗)"
  fi

  # ツール使用統計（作業フロー）
  TOOL_STATS=$(cat "$TRANSCRIPT_PATH" 2>/dev/null \
    | jq -r '
        (.message.content // .content // []) |
        if type == "array" then
          .[] | select(.type == "tool_use") | .name
        else empty
        end
      ' 2>/dev/null \
    | sort | uniq -c | sort -rn \
    | awk '{printf "- %s: %d回\n", $2, $1}' \
    | head -20 || echo "(取得失敗)")
  [[ -z "$TOOL_STATS" ]] && TOOL_STATS="(ツール使用なし)"

fi

# ── セッションサマリーの抽出 ──────────────────────────────────────────────────
# CLAUDE.md の指示に従い Claude が書いたマーカー付きサマリーを優先して使用する
# マーカーがなければ最後のアシスタントメッセージ全体をフォールバックとして使用
SESSION_SUMMARY=""
if echo "$LAST_CLAUDE_MSG" | grep -q 'CLAUDE_SESSION_SUMMARY_START'; then
  SESSION_SUMMARY=$(echo "$LAST_CLAUDE_MSG" \
    | sed -n '/<!-- CLAUDE_SESSION_SUMMARY_START -->/,/<!-- CLAUDE_SESSION_SUMMARY_END -->/p' \
    | grep -v 'CLAUDE_SESSION_SUMMARY_' || true)
  log_info "構造化サマリーを検出しました。"
else
  SESSION_SUMMARY="$LAST_CLAUDE_MSG"
  log_warn "構造化サマリーが見つかりません。最後のメッセージをフォールバックとして使用します。"
fi

# ── git情報 ────────────────────────────────────────────────────────────────────
ORIGINAL_BRANCH=$(git branch --show-current)
if [[ -z "$ORIGINAL_BRANCH" ]]; then
  log_warn "detached HEAD のためスキップします。"; exit 0
fi
WIP_BRANCH="wip/${ORIGINAL_BRANCH}"
log_info "branch=${ORIGINAL_BRANCH} → wip=${WIP_BRANCH}"

# ── 変更の有無を確認 ───────────────────────────────────────────────────────────
HAS_CHANGES=false
if ! git diff --quiet || ! git diff --cached --quiet; then
  HAS_CHANGES=true
fi
if [[ -n "$(git ls-files --others --exclude-standard)" ]]; then
  HAS_CHANGES=true
fi

if [[ "$HAS_CHANGES" == "false" ]]; then
  log_info "変更がないため WIP コミットをスキップします。"
  exit 0
fi

# ── WIP コミット ───────────────────────────────────────────────────────────────
git add -A
COMMIT_MSG="WIP: claude session $(date '+%Y-%m-%d %H:%M:%S')"
git commit -m "$COMMIT_MSG"
COMMIT_SHA=$(git rev-parse --short HEAD)
log_info "WIPコミット完了: ${COMMIT_SHA}"

# ── wip/ブランチへ push ────────────────────────────────────────────────────────
if ! git push origin "HEAD:refs/heads/${WIP_BRANCH}" --force 2>&1; then
  log_error "push に失敗しました。Issueの作成をスキップします。"
  exit 1
fi
log_info "push完了: ${WIP_BRANCH}"

# ── 変更ファイル一覧 ───────────────────────────────────────────────────────────
CHANGED_FILES=$(git diff --name-status HEAD~1 HEAD 2>/dev/null \
  | awk '{printf "%-2s %s\n", $1, $2}' \
  | head -30 || echo "(取得失敗)")

# ── 未解決の TODO / FIXME を抽出 ──────────────────────────────────────────────
TODO_LIST=$(git diff HEAD~1 HEAD -U0 2>/dev/null \
  | grep '^\+[^+]' \
  | grep -E '\b(TODO|FIXME)\b' \
  | sed 's/^\+//' \
  | sed 's/^[[:space:]]*//' \
  | head -20 || true)

# ── Issue本文を組み立て ────────────────────────────────────────────────────────
BODY=$(cat <<EOF
## 🤖 Claude Code セッション引き継ぎ

| 項目 | 値 |
|---|---|
| 作業ブランチ | \`${ORIGINAL_BRANCH}\` |
| WIPブランチ | \`${WIP_BRANCH}\` |
| コミット | \`${COMMIT_SHA}\` |
| 日時 | $(date '+%Y-%m-%d %H:%M:%S') |

---

### 📋 作業指示（最初のユーザーメッセージ）

${FIRST_USER_MSG}

---

### 📊 セッションサマリー

${SESSION_SUMMARY}

---

### 🔧 使用ツール統計（作業フロー）

${TOOL_STATS}

---

### 📁 変更ファイル一覧

\`\`\`
${CHANGED_FILES}
\`\`\`
*(記号: A=追加, M=変更, D=削除, R=リネーム)*

---

### ⚠️ 未解決の TODO / FIXME

\`\`\`
${TODO_LIST:-"なし"}
\`\`\`

---

### 🔁 次のセッションでの再開方法

\`\`\`bash
# WIPブランチから作業ブランチへマージして再開
git fetch origin
git checkout ${ORIGINAL_BRANCH}
git merge origin/${WIP_BRANCH}

# またはWIPブランチで直接続きを作業する場合
git checkout ${WIP_BRANCH}
\`\`\`

---
*このIssueはClaude Code Stop hookが自動生成しました。*
EOF
)

TITLE="[WIP] ${ORIGINAL_BRANCH} — $(date '+%m/%d %H:%M')"

# ── ラベルの存在確認・自動作成 ────────────────────────────────────────────────
LABEL="claude-session"
if ! gh label list --limit 100 2>/dev/null | grep -q "^${LABEL}"; then
  gh label create "$LABEL" --color "0075ca" --description "Claude Code自動生成セッション" 2>/dev/null || true
  log_info "ラベル作成: ${LABEL}"
fi

# ── Issue作成 ──────────────────────────────────────────────────────────────────
ISSUE_URL=$(gh issue create \
  --title "$TITLE" \
  --body "$BODY" \
  --label "$LABEL" \
  2>&1)

log_info "Issue作成完了: ${ISSUE_URL}"
log_info "WIPブランチ: ${WIP_BRANCH}"
log_info "========== Stop hook 終了 (exit 0) =========="
