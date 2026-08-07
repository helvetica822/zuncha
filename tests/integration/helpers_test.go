// I/O層テスト共通ヘルパー。テスト用DBが必要（環境変数ZUNCHA_TEST_DATABASE_URL）。
// 対応仕様: docs/03_unit_test/08_test_specification.md 6章
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("ZUNCHA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ZUNCHA_TEST_DATABASE_URL未設定のためスキップ（テスト用DBが必要）")
	}

	// 共有DBを使っていると、他プロセスの TRUNCATE と衝突して「広範囲かつ実行ごとに
	// 違うテストが落ちる」切り分け困難な失敗になる（scripts/test_env.sh 参照）。
	// t.Log はテスト失敗時と -v 時に出力されるため、まさに衝突で落ちたときに目に入る。
	// scripts/test_env.sh は未設定時に共有DBへフォールバックする一方、
	// scripts/create_test_db.sh は未設定をエラーにするという非対称があるので、
	// 設定漏れの再発口をここで塞ぐ。
	if strings.Contains(dsn, "/zuncha_test?") {
		t.Log("警告: 実行者ごとに分離されていない共有DB(zuncha_test)を使っています。" +
			"他プロセスが同時にテストを実行していると TRUNCATE が衝突し、この失敗は偽物の可能性があります。" +
			"`export ZUNCHA_TEST_DB_OWNER=<自分の名前> && ./scripts/create_test_db.sh` を実行してください。")
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	require.NoError(t, db.PingContext(context.Background()))

	t.Cleanup(func() {
		_, _ = db.Exec("TRUNCATE conversations, messages, audio_files CASCADE")
		db.Close()
	})

	_, err = db.Exec("TRUNCATE conversations, messages, audio_files CASCADE")
	require.NoError(t, err)

	return db
}

func insertConversation(t *testing.T, db *sql.DB, id string, startedAt time.Time) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO conversations (id, started_at) VALUES ($1, $2)`,
		id, startedAt,
	)
	require.NoError(t, err)
}

func insertMessage(t *testing.T, db *sql.DB, id, conversationID, role, content string, emotion *string, createdAt time.Time) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO messages (id, conversation_id, role, content, emotion, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		id, conversationID, role, content, emotion, createdAt,
	)
	require.NoError(t, err)
}

func insertAudioFile(t *testing.T, db *sql.DB, id, conversationID, messageID, filePath string, fetchedAt *time.Time) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO audio_files (id, conversation_id, message_id, file_path, fetched_at) VALUES ($1, $2, $3, $4, $5)`,
		id, conversationID, messageID, filePath, fetchedAt,
	)
	require.NoError(t, err)
}

// dbNow は DB 側の現在時刻を返す。
// そらの指摘（改善）: 「created_at が NOW() で補完されたこと」をホストの time.Now() と
// 比較すると、ホストとDBコンテナのクロック差に依存する。WSL2 のサスペンド復帰では
// 数秒のドリフトが起こり得るため、許容幅を広げても本質的に不安定なままになる。
// 判定基準をDB側の時計だけで揃えれば skew を完全に排除できる。
// 2ファイル（message / audio）から使うため共通ヘルパーとして置く。
func dbNow(t *testing.T, db *sql.DB) time.Time {
	t.Helper()
	var now time.Time
	require.NoError(t, db.QueryRow(`SELECT NOW()`).Scan(&now))
	return now
}

func countRows(t *testing.T, db *sql.DB, table, where string, args ...interface{}) int {
	t.Helper()
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, where)
	var count int
	require.NoError(t, db.QueryRow(query, args...).Scan(&count))
	return count
}

func ulidLike(seed int) string {
	// テストデータ用の簡易ULID風文字列（26文字、Crockford Base32相当の文字のみ使用）
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	base := "01ARZ3NDEKTSV4RRFFQ69G5FA"
	return base + string(alphabet[seed%len(alphabet)])
}
