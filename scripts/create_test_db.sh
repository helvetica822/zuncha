#!/bin/bash
# 実行者ごとのテスト用DBを作成し migration を適用する（冪等・初回のみ実行すればよい）。
#
# 使い方:
#   export ZUNCHA_TEST_DB_OWNER=metan
#   ./scripts/create_test_db.sh
#   source scripts/test_env.sh && go test ./tests/...
#
# なぜDBを分けるのか: setupTestDB が毎回 TRUNCATE するため、同一DBを2プロセスが
# 同時に使うと互いの行を消し合い、広範囲かつ非決定的な失敗になる（詳細は test_env.sh）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PG_CONTAINER="${PG_CONTAINER:-zuncha-test-pg}"
PG_SUPERUSER="${PG_SUPERUSER:-zuncha}"
# 管理接続用のDB。ユーザー名と同名のDBは存在しないため明示する。
PG_ADMIN_DB="${PG_ADMIN_DB:-postgres}"

if [[ -z "${ZUNCHA_TEST_DB_OWNER:-}" ]]; then
  echo "エラー: ZUNCHA_TEST_DB_OWNER を設定してください（例: export ZUNCHA_TEST_DB_OWNER=metan）" >&2
  echo "        未設定のまま共有DBを使うと他プロセスのTRUNCATEと衝突します。" >&2
  exit 1
fi
DB_NAME="zuncha_test_${ZUNCHA_TEST_DB_OWNER}"

if ! docker ps --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
  echo "エラー: PostgreSQLコンテナ '$PG_CONTAINER' が起動していません（docker start $PG_CONTAINER）" >&2
  exit 1
fi

# CREATE DATABASE は IF NOT EXISTS を持たないため、存在チェックで冪等にする。
exists=$(docker exec -i "$PG_CONTAINER" psql -U "$PG_SUPERUSER" -d "$PG_ADMIN_DB" -tAc \
  "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'")
if [[ "$exists" == "1" ]]; then
  echo "DB ${DB_NAME} は既に存在します（migration適用のみ行います）。"
else
  docker exec -i "$PG_CONTAINER" psql -U "$PG_SUPERUSER" -d "$PG_ADMIN_DB" -c "CREATE DATABASE ${DB_NAME}"
  echo "DB ${DB_NAME} を作成しました。"
fi

# migration を適用する。テーブルが既にある場合は CREATE TABLE が落ちるため、
# 適用済みかを1テーブルの存在で判定する（migration管理ツールは未導入）。
#
# 【注意】この判定は「conversationsテーブルが存在するか」だけを見ており、
# DDL変更（カラム追加・FK制約の変更など）を検知できない。0001_initial_schema.up.sql
# を変更した場合、既存DBに対してこのスクリプトを再実行しても何も起きず、
# 既存DBは古いスキーマのまま取り残される（2026-08-10、そらの実測で
# zuncha_test_metanが旧FKスキーマのまま残っていたことが判明・要修正の原因になった）。
# DDLを変更したら、各自 `DROP DATABASE zuncha_test_<owner>` してから
# このスクリプトを再実行すること。
has_tables=$(docker exec -i "$PG_CONTAINER" psql -U "$PG_SUPERUSER" -d "$DB_NAME" -tAc \
  "SELECT 1 FROM information_schema.tables WHERE table_name = 'conversations'")
if [[ "$has_tables" == "1" ]]; then
  echo "スキーマは既に適用済みです（DDL変更を反映済みか不安な場合は、DROP DATABASEしてから再実行してください）。"
else
  docker exec -i "$PG_CONTAINER" psql -U "$PG_SUPERUSER" -d "$DB_NAME" \
    < "$REPO_ROOT/migrations/0001_initial_schema.up.sql"
  echo "migration を適用しました。"
fi

echo
echo "完了。以降は次で実行してください:"
echo "  export ZUNCHA_TEST_DB_OWNER=${ZUNCHA_TEST_DB_OWNER}"
echo "  source scripts/test_env.sh && go test ./tests/..."
