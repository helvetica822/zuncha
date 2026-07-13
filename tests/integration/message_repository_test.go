// I/O層テスト（実DB接続）。対応仕様: docs/03_unit_test/08_test_specification.md 4.2 I/O層（観点2-2、TC-2-2-05〜15）
package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/model"
	"zuncha/internal/postgres"
)

func seedMessages(t *testing.T, db *sql.DB, conversationID string, count int, base time.Time, roleFn func(i int) string) []string {
	t.Helper()
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		id := ulid.Make().String()
		ids[i] = id
		insertMessage(t, db, id, conversationID, roleFn(i), "本文"+id, nil, base.Add(time.Duration(i)*time.Second))
	}
	return ids
}

func messageIDsFromResult(messages []model.Message) []string {
	ids := make([]string, len(messages))
	for i, m := range messages {
		ids[i] = m.ID
	}
	return ids
}

func alternatingRole(i int) string {
	if i%2 == 0 {
		return "user"
	}
	return "assistant"
}

func TestMessageRepository_GetRecentMessages(t *testing.T) {
	db := setupTestDB(t)
	repo := postgres.NewMessageRepository(db)
	base := time.Now().Add(-1 * time.Hour)

	t.Run("TC-2-2-05_5件は全件古い順で返る", func(t *testing.T) {
		convID := ulidLike(10)
		insertConversation(t, db, convID, time.Now())
		ids := seedMessages(t, db, convID, 5, base, alternatingRole)

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 5)
		for i, m := range got {
			assert.Equal(t, ids[i], m.ID)
		}
	})

	t.Run("TC-2-2-06_30件は直近20件のみ古い順で返る", func(t *testing.T) {
		convID := ulidLike(11)
		insertConversation(t, db, convID, time.Now())
		ids := seedMessages(t, db, convID, 30, base, alternatingRole)

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 20)
		wantIDs := ids[10:30] // 直近20件＝末尾20件、古い→新しい順
		for i, m := range got {
			assert.Equal(t, wantIDs[i], m.ID)
		}
	})

	t.Run("TC-2-2-07_交互20件は10往復として一致する", func(t *testing.T) {
		convID := ulidLike(12)
		insertConversation(t, db, convID, time.Now())
		ids := seedMessages(t, db, convID, 20, base, alternatingRole)

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 20)
		for i, m := range got {
			assert.Equal(t, ids[i], m.ID)
		}
	})

	t.Run("TC-2-2-08_0件は空スライスを返す", func(t *testing.T) {
		convID := ulidLike(13)
		insertConversation(t, db, convID, time.Now())

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("TC-2-2-09_存在しないconversation_idは空配列を返す", func(t *testing.T) {
		got, err := repo.GetRecentMessages(context.Background(), ulidLike(99))

		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("TC-2-2-10_ちょうど20件は全件取得される", func(t *testing.T) {
		convID := ulidLike(14)
		insertConversation(t, db, convID, time.Now())
		ids := seedMessages(t, db, convID, 20, base, alternatingRole)

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 20)
		assert.Equal(t, ids[0], got[0].ID)
		assert.Equal(t, ids[19], got[19].ID)
	})

	t.Run("TC-2-2-11_19件は全件取得される", func(t *testing.T) {
		convID := ulidLike(15)
		insertConversation(t, db, convID, time.Now())
		ids := seedMessages(t, db, convID, 19, base, alternatingRole)

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 19)
		assert.Equal(t, ids[0], got[0].ID)
	})

	t.Run("TC-2-2-12_21件は最古の1件が除外される", func(t *testing.T) {
		convID := ulidLike(16)
		insertConversation(t, db, convID, time.Now())
		ids := seedMessages(t, db, convID, 21, base, alternatingRole)

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 20)
		for _, m := range got {
			assert.NotEqual(t, ids[0], m.ID, "最も古い1件（ids[0]）は除外されるべき")
		}
		assert.Equal(t, ids[1], got[0].ID)
		assert.Equal(t, ids[20], got[19].ID)
	})

	t.Run("TC-2-2-13_同一ミリ秒でも順序が安定する", func(t *testing.T) {
		convID := ulidLike(17)
		insertConversation(t, db, convID, time.Now())
		sameInstant := time.Now()
		// ulid.Make()は同一プロセス内で単調増加する値を返すため、idsは生成順（古い→新しい）に並ぶ
		ids := seedMessages(t, db, convID, 5, sameInstant, func(i int) string { return "user" })
		for _, id := range ids {
			_, err := db.Exec(`UPDATE messages SET created_at = $1 WHERE id = $2`, sameInstant, id)
			require.NoError(t, err)
		}

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 5)
		// そらの指摘（軽微）: 「2回取得して結果が一致する」だけでは、たまたま安定した
		// 不定順序に乗っているだけの可能性を排除できない。ORDER BY created_at DESCのみでは
		// 同時刻レコードの順序が不定になり得るため、id（ULID）をタイブレーカーとして
		// 用いていることを、期待順序（ids＝生成順＝古い→新しい）と直接突き合わせて検証する。
		assert.Equal(t, ids, messageIDsFromResult(got),
			"created_atが同一でも、idタイブレーカーにより生成順（古い→新しい）で安定的に返ること")

		// 複数回取得しても同じ順序が再現されること（安定性の裏付け）
		got2, err2 := repo.GetRecentMessages(context.Background(), convID)
		require.NoError(t, err2)
		assert.Equal(t, messageIDsFromResult(got), messageIDsFromResult(got2))
	})

	t.Run("TC-2-2-14_role非交互でも単純20件上限で取得される", func(t *testing.T) {
		convID := ulidLike(18)
		insertConversation(t, db, convID, time.Now())
		ids := seedMessages(t, db, convID, 22, base, func(i int) string {
			if i < 2 {
				return "user" // 冒頭2件をuser連続にして非交互データを作る
			}
			return alternatingRole(i)
		})

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 20, "往復単位ではなく単純な最大20件の件数上限で取得される")
		assert.Equal(t, ids[2], got[0].ID)
	})

	t.Run("TC-2-2-15_実DB経由でも古い新しい順の契約が成立する", func(t *testing.T) {
		convID := ulidLike(19)
		insertConversation(t, db, convID, time.Now())
		ids := seedMessages(t, db, convID, 30, base, alternatingRole)

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 20)
		for i := 1; i < len(got); i++ {
			assert.False(t, got[i].CreatedAt.Before(got[i-1].CreatedAt), "常に古い→新しい順であること")
		}
		assert.Equal(t, ids[len(ids)-1], got[len(got)-1].ID, "最後の要素は最新メッセージであること")
	})
}
