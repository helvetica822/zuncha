#!/bin/bash
# integrationテスト用DB接続情報。使い方: source scripts/test_env.sh && go test ./tests/integration/...
#
# 【並行実行の衝突対策】
# integration の setupTestDB は毎回 `TRUNCATE conversations, messages, audio_files CASCADE`
# を実行する。したがって同一DBに対して2プロセスが同時に走ると互いの行を消し合い、
# 「広範囲かつ実行ごとに違うテストが落ちる」という切り分けの難しい失敗になる。
# チーム(めたん/ずんだもん/そら)がそれぞれテストを回す体制ではこれが繰り返し起きるため、
# 実行者ごとにDBを分ける。
#
#   export ZUNCHA_TEST_DB_OWNER=metan      # 各自ひとつ決める（metan / zundamon / sora ...）
#   ./scripts/create_test_db.sh            # 初回のみ: DB作成 + migration適用
#   source scripts/test_env.sh && go test ./tests/...
#
# ZUNCHA_TEST_DB_OWNER 未設定なら従来の zuncha_test を使う（既存手順との後方互換）。
# その場合は他プロセスと衝突し得るので、衝突を疑ったら下記で他の接続を確認する:
#   psql "$ZUNCHA_TEST_DATABASE_URL" -c \
#     "SELECT pid, application_name, state, query FROM pg_stat_activity WHERE datname=current_database()"

ZUNCHA_TEST_DB_NAME="zuncha_test"
if [[ -n "${ZUNCHA_TEST_DB_OWNER:-}" ]]; then
  ZUNCHA_TEST_DB_NAME="zuncha_test_${ZUNCHA_TEST_DB_OWNER}"
fi
export ZUNCHA_TEST_DB_NAME

export ZUNCHA_TEST_DATABASE_URL="postgres://zuncha:zuncha@127.0.0.1:55432/${ZUNCHA_TEST_DB_NAME}?sslmode=disable"

if [[ -z "${ZUNCHA_TEST_DB_OWNER:-}" ]]; then
  echo "注意: ZUNCHA_TEST_DB_OWNER が未設定のため共有DB ${ZUNCHA_TEST_DB_NAME} を使います（他プロセスと衝突し得ます）。" >&2
fi
