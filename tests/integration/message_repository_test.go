// I/O層テスト（実DB接続）。対応仕様: docs/03_unit_test/08_test_specification.md 4.2 I/O層（観点2-2、TC-2-2-05〜15）
package integration

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

// queryMessageRow は保存されたメッセージ行を全カラム取得する。
// conversation_repository_test.go の queryConversationRow と同じ流儀（読み取り検証用のファイルローカルヘルパー）。
func queryMessageRow(t *testing.T, db *sql.DB, id string) (conversationID, role, content string, emotion sql.NullString, createdAt time.Time) {
	t.Helper()
	err := db.QueryRow(
		`SELECT conversation_id, role, content, emotion, created_at FROM messages WHERE id = $1`, id,
	).Scan(&conversationID, &role, &content, &emotion, &createdAt)
	require.NoError(t, err)
	return
}

func TestMessageRepository_InsertMessage(t *testing.T) {
	db := setupTestDB(t)
	repo := postgres.NewMessageRepository(db)

	t.Run("W-01-01_userメッセージが全カラム保存されemotionはNULLになる", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		msgID := ulid.Make().String()
		createdAt := time.Now().Add(-10 * time.Minute)

		err := repo.InsertMessage(context.Background(), &model.Message{
			ID:             msgID,
			ConversationID: convID,
			Role:           "user",
			Content:        "ずんだもんこんにちは",
			CreatedAt:      createdAt,
		})

		require.NoError(t, err)
		gotConvID, gotRole, gotContent, gotEmotion, _ := queryMessageRow(t, db, msgID)
		assert.Equal(t, convID, gotConvID)
		assert.Equal(t, "user", gotRole)
		assert.Equal(t, "ずんだもんこんにちは", gotContent)
		assert.False(t, gotEmotion.Valid, "userメッセージのemotionはNULLであるべき")
	})

	t.Run("W-01-02_assistantメッセージのemotionが保存される", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		msgID := ulid.Make().String()
		emotion := "喜び"

		err := repo.InsertMessage(context.Background(), &model.Message{
			ID:             msgID,
			ConversationID: convID,
			Role:           "assistant",
			Content:        "やったのだ！",
			Emotion:        &emotion,
			CreatedAt:      time.Now(),
		})

		require.NoError(t, err)
		_, gotRole, gotContent, gotEmotion, _ := queryMessageRow(t, db, msgID)
		assert.Equal(t, "assistant", gotRole)
		assert.Equal(t, "やったのだ！", gotContent)
		require.True(t, gotEmotion.Valid)
		assert.Equal(t, "喜び", gotEmotion.String)
	})

	t.Run("W-01-03_CreatedAt明示指定はNOWに上書きされない", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		msgID := ulid.Make().String()
		want := time.Date(2026, 3, 1, 12, 34, 56, 0, time.UTC)

		err := repo.InsertMessage(context.Background(), &model.Message{
			ID: msgID, ConversationID: convID, Role: "user", Content: "過去の発話", CreatedAt: want,
		})

		require.NoError(t, err)
		_, _, _, _, gotCreatedAt := queryMessageRow(t, db, msgID)
		assert.True(t, want.Equal(gotCreatedAt),
			"明示したCreatedAt(%v)がそのまま保存されるべきだが %v だった", want, gotCreatedAt)
	})

	t.Run("W-01-04_CreatedAtゼロ値はDB側のNOWで補完される", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		msgID := ulid.Make().String()
		// 判定基準はDB側の時計のみで揃える（ホストとDBコンテナのクロック差を排除）。
		before := dbNow(t, db)

		err := repo.InsertMessage(context.Background(), &model.Message{
			ID: msgID, ConversationID: convID, Role: "user", Content: "現在の発話",
		})
		after := dbNow(t, db)

		require.NoError(t, err)
		_, _, _, _, gotCreatedAt := queryMessageRow(t, db, msgID)
		assert.False(t, gotCreatedAt.IsZero(), "ゼロ値をそのまま保存してはならない")
		assert.WithinRange(t, gotCreatedAt, before, after,
			"created_atはNOW()で補完され、INSERT前後にDBから取得した時刻の間に入るべき")
	})

	t.Run("W-01-05_content1000文字が切り捨てられず全長保存される", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		msgID := ulid.Make().String()
		long := strings.Repeat("あ", 1000)

		err := repo.InsertMessage(context.Background(), &model.Message{
			ID: msgID, ConversationID: convID, Role: "user", Content: long, CreatedAt: time.Now(),
		})

		require.NoError(t, err)
		_, _, gotContent, _, _ := queryMessageRow(t, db, msgID)
		assert.Equal(t, 1000, utf8.RuneCountInString(gotContent))
		assert.Equal(t, long, gotContent)
	})

	t.Run("W-01-06_content空文字列も保存される_バリデーションは上位層の責務", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		msgID := ulid.Make().String()

		err := repo.InsertMessage(context.Background(), &model.Message{
			ID: msgID, ConversationID: convID, Role: "user", Content: "", CreatedAt: time.Now(),
		})

		require.NoError(t, err, "NOT NULL制約は空文字を許すためRepositoryは拒否しない")
		_, _, gotContent, _, _ := queryMessageRow(t, db, msgID)
		assert.Equal(t, "", gotContent)
	})

	t.Run("W-01-07_挿入したメッセージがGetRecentMessagesで古い新しい順に読み戻せる", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		base := time.Now().Add(-time.Hour)
		ids := make([]string, 3)
		for i := 0; i < 3; i++ {
			ids[i] = ulid.Make().String()
			require.NoError(t, repo.InsertMessage(context.Background(), &model.Message{
				ID:             ids[i],
				ConversationID: convID,
				Role:           alternatingRole(i),
				Content:        "本文" + ids[i],
				CreatedAt:      base.Add(time.Duration(i) * time.Second),
			}))
		}

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, ids, messageIDsFromResult(got), "InsertMessageで書いた順（古い→新しい）で返るべき")
		assert.Equal(t, "本文"+ids[0], got[0].Content)
	})

	t.Run("W-01-08_21件挿入するとGetRecentMessagesは最古1件を落として20件返す", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		base := time.Now().Add(-time.Hour)
		ids := make([]string, 21)
		for i := 0; i < 21; i++ {
			ids[i] = ulid.Make().String()
			require.NoError(t, repo.InsertMessage(context.Background(), &model.Message{
				ID:             ids[i],
				ConversationID: convID,
				Role:           alternatingRole(i),
				Content:        "本文" + ids[i],
				CreatedAt:      base.Add(time.Duration(i) * time.Second),
			}))
		}

		got, err := repo.GetRecentMessages(context.Background(), convID)

		require.NoError(t, err)
		require.Len(t, got, 20)
		assert.Equal(t, ids[1:], messageIDsFromResult(got), "最古の1件（ids[0]）のみが落ちるべき")
	})

	t.Run("W-01-09_存在しないconversation_idは外部キー違反でエラー", func(t *testing.T) {
		err := repo.InsertMessage(context.Background(), &model.Message{
			ID:             ulid.Make().String(),
			ConversationID: ulid.Make().String(),
			Role:           "user",
			Content:        "孤児メッセージ",
			CreatedAt:      time.Now(),
		})

		assert.Error(t, err)
	})

	t.Run("W-01-10_role許容外はCHECK制約違反でエラー", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())

		err := repo.InsertMessage(context.Background(), &model.Message{
			ID: ulid.Make().String(), ConversationID: convID, Role: "bot", Content: "不正role", CreatedAt: time.Now(),
		})

		assert.Error(t, err)
	})

	t.Run("W-01-11_emotion7種外はCHECK制約違反でエラー", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		emotion := "ハッピー"

		err := repo.InsertMessage(context.Background(), &model.Message{
			ID: ulid.Make().String(), ConversationID: convID, Role: "assistant",
			Content: "不正emotion", Emotion: &emotion, CreatedAt: time.Now(),
		})

		assert.Error(t, err)
	})

	t.Run("W-01-12_同一IDの二重挿入は主キー重複でエラー", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		msgID := ulid.Make().String()
		msg := &model.Message{
			ID: msgID, ConversationID: convID, Role: "user", Content: "1回目", CreatedAt: time.Now(),
		}
		require.NoError(t, repo.InsertMessage(context.Background(), msg))

		err := repo.InsertMessage(context.Background(), msg)

		assert.Error(t, err)
	})

	t.Run("W-01-13_会話削除でCASCADEによりメッセージも消える", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		msgID := ulid.Make().String()
		require.NoError(t, repo.InsertMessage(context.Background(), &model.Message{
			ID: msgID, ConversationID: convID, Role: "user", Content: "消える発話", CreatedAt: time.Now(),
		}))
		require.Equal(t, 1, countRows(t, db, "messages", "id = $1", msgID))

		_, err := db.Exec(`DELETE FROM conversations WHERE id = $1`, convID)

		require.NoError(t, err)
		assert.Equal(t, 0, countRows(t, db, "messages", "id = $1", msgID))
	})
}
