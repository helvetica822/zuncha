// I/O層テスト共通ヘルパー。テスト用DBが必要（環境変数ZUNCHA_TEST_DATABASE_URL）。
// 対応仕様: docs/03_unit_test/08_test_specification.md 6章
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"os"
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
