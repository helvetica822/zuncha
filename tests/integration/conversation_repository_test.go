// I/O層テスト（実DB接続）。対応仕様: docs/03_unit_test/08_test_specification.md 4.1 I/O層（観点2-1、TC-2-1-08〜15）
package integration

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/model"
	"zuncha/internal/postgres"
	"zuncha/internal/service"
)

func queryConversationRow(t *testing.T, db *sql.DB, id string) (startedAt, expiresAt time.Time, firstText sql.NullString) {
	t.Helper()
	err := db.QueryRow(
		`SELECT started_at, expires_at, first_text FROM conversations WHERE id = $1`, id,
	).Scan(&startedAt, &expiresAt, &firstText)
	require.NoError(t, err)
	return
}

// startedAtForExpiry は、expires_at（生成カラム = started_at + 30日）が
// ちょうどwantExpiresAtになるようなstarted_atを逆算する。
// GC呼び出しに渡すnowと同じ基準時刻から算出することで、
// 「expires_atとnowの関係」をテスト側で確定的に固定できるようにする
// （そらの指摘：Postgres側のNOW()に依存する設計では境界値テストが原理的に成立しないため）。
func startedAtForExpiry(wantExpiresAt time.Time) time.Time {
	return wantExpiresAt.Add(-30 * 24 * time.Hour)
}

func TestConversationRepository_InsertConversation(t *testing.T) {
	db := setupTestDB(t)
	repo := postgres.NewConversationRepository(db)

	t.Run("TC-2-1-08_期限切れなしでも新規レコードが作成される", func(t *testing.T) {
		conv := &model.Conversation{ID: ulid.Make().String(), StartedAt: time.Now()}

		err := repo.InsertConversation(context.Background(), conv)

		require.NoError(t, err)
		assert.Equal(t, 1, countRows(t, db, "conversations", "id = $1", conv.ID))
	})

	t.Run("TC-2-1-10_新規レコードのカラム初期値が正しい", func(t *testing.T) {
		before := time.Now()
		conv := &model.Conversation{ID: ulid.Make().String(), StartedAt: time.Now()}

		err := repo.InsertConversation(context.Background(), conv)
		after := time.Now()

		require.NoError(t, err)
		startedAt, expiresAt, firstText := queryConversationRow(t, db, conv.ID)
		assert.False(t, firstText.Valid, "first_textはNULLであるべき")
		assert.WithinRange(t, startedAt, before.Add(-time.Second), after.Add(time.Second))
		assert.WithinDuration(t, startedAt.Add(30*24*time.Hour), expiresAt, time.Second)
	})

	t.Run("TC-2-1-14_ULID衝突時は即エラーを返す", func(t *testing.T) {
		id := ulidLike(1)
		insertConversation(t, db, id, time.Now())
		conv := &model.Conversation{ID: id, StartedAt: time.Now()}

		err := repo.InsertConversation(context.Background(), conv)

		assert.Error(t, err)
	})
}

func TestConversationRepository_GC(t *testing.T) {
	db := setupTestDB(t)
	repo := postgres.NewConversationRepository(db)

	t.Run("TC-2-1-09_GCでCASCADE削除が連鎖する", func(t *testing.T) {
		now := time.Now()
		convID := ulidLike(2)
		msgID := ulidLike(3)
		audioID := ulidLike(4)
		insertConversation(t, db, convID, startedAtForExpiry(now.Add(-time.Hour))) // 1時間前に期限切れ
		insertMessage(t, db, msgID, convID, "user", "こんにちは", nil, time.Now())
		insertAudioFile(t, db, audioID, convID, msgID, "/tmp/audio/"+audioID+".wav", nil)

		deleted, err := repo.GC(context.Background(), now)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, int64(1))
		assert.Equal(t, 0, countRows(t, db, "conversations", "id = $1", convID))
		assert.Equal(t, 0, countRows(t, db, "messages", "id = $1", msgID))
		assert.Equal(t, 0, countRows(t, db, "audio_files", "id = $1", audioID))
	})

	t.Run("TC-2-1-11_ちょうどNOWのレコードはGC対象外", func(t *testing.T) {
		// そらの指摘（⚠要修正）: Postgres側のNOW()に依存する設計だと、
		// INSERTからGC呼び出しまでの経過時間分だけexpires_atが必ず過去になり、
		// 「ちょうど一致」の境界値テストが原理的に成立しなかった。
		// GCにnowを明示的に注入する設計に変更し、expires_atとnowを同じ基準値から
		// 確定的に導出することで、この境界値を確実に検証できるようにする。
		now := time.Now()
		convID := ulidLike(5)
		insertConversation(t, db, convID, startedAtForExpiry(now)) // expires_at == now ちょうど一致

		_, err := repo.GC(context.Background(), now)

		require.NoError(t, err)
		assert.Equal(t, 1, countRows(t, db, "conversations", "id = $1", convID),
			"expires_atがちょうどnowと一致する場合、`<`の等号非対称性によりGC対象外（削除されない）であること")
	})

	t.Run("TC-2-1-12_期限切れ1件が削除される", func(t *testing.T) {
		now := time.Now()
		convID := ulidLike(6)
		insertConversation(t, db, convID, startedAtForExpiry(now.Add(-time.Hour)))

		deleted, err := repo.GC(context.Background(), now)

		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)
	})

	t.Run("TC-2-1-13_期限切れ1000件が全件削除される", func(t *testing.T) {
		now := time.Now()
		const n = 1000
		for i := 0; i < n; i++ {
			insertConversation(t, db, ulid.Make().String(), startedAtForExpiry(now.Add(-time.Hour)))
		}

		deleted, err := repo.GC(context.Background(), now)

		require.NoError(t, err)
		assert.Equal(t, int64(n), deleted)
	})
}

func TestConversationRepository_10並列実行(t *testing.T) {
	// TC-2-1-15
	db := setupTestDB(t)
	repo := postgres.NewConversationRepository(db)
	svc := service.NewCreateConversationService(repo)

	const parallelism = 10
	var wg sync.WaitGroup
	errs := make([]error, parallelism)
	ids := make([]string, parallelism)

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conv, err := svc.CreateConversation(context.Background())
			errs[idx] = err
			if conv != nil {
				ids[idx] = conv.ID
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool)
	for i, err := range errs {
		require.NoError(t, err)
		assert.False(t, seen[ids[i]], "ULIDが重複してはならない: %s", ids[i])
		seen[ids[i]] = true
	}
}
