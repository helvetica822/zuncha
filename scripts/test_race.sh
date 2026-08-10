#!/bin/bash
# 競合検出器つきテスト実行。
#
# なぜ docker 経由か: 開発ホストは CGO_ENABLED=0 かつ C コンパイラ未導入で、
# `go test -race` は cgo を要求するため原理的に実行できない。ホスト環境の変更
# (gcc 導入) を前提にせず、コンテナ内で担保する。CI でも同じ手順で再現できる。
#
# 使い方: source scripts/test_env.sh && ./scripts/test_race.sh [追加のgo testフラグ]
#   例) ./scripts/test_race.sh -run TestHub -v
set -euo pipefail

# go.mod の go ディレクティブに合わせる。anthropic-sdk-go が go >= 1.24 を要求するため
# W-08 で 1.22 から引き上げた（1.22 のままだと GOTOOLCHAIN=local で起動すらしない）。
GO_IMAGE="${GO_IMAGE:-golang:1.24}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# --network host: integration テストがホストの 55432 の PostgreSQL に到達するため。
# ZUNCHA_TEST_DATABASE_URL 未設定だと integration は t.Skip される（緑に見えるだけ）。
if [[ -z "${ZUNCHA_TEST_DATABASE_URL:-}" ]]; then
  echo "警告: ZUNCHA_TEST_DATABASE_URL が未設定です。integration はスキップされます。" >&2
  echo "      先に 'source scripts/test_env.sh' を実行してください。" >&2
fi

exec docker run --rm --network host \
  -v "$REPO_ROOT":/app -w /app \
  -e HOME=/tmp \
  -e ZUNCHA_TEST_DATABASE_URL="${ZUNCHA_TEST_DATABASE_URL:-}" \
  "$GO_IMAGE" \
  go test -race -count=1 "$@" ./tests/...
