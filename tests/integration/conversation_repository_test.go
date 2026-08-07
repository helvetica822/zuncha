// I/O層テスト（実DB接続）。対応仕様: docs/03_unit_test/08_test_specification.md 4.1 I/O層（観点2-1、TC-2-1-08〜15）
package integration

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

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
		_, _ = db.Exec("TRUNCATE conversations, messages, audio_files CASCADE")
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
		_, _ = db.Exec("TRUNCATE conversations, messages, audio_files CASCADE")
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
		_, _ = db.Exec("TRUNCATE conversations, messages, audio_files CASCADE")
		now := time.Now()
		convID := ulidLike(6)
		insertConversation(t, db, convID, startedAtForExpiry(now.Add(-time.Hour)))

		deleted, err := repo.GC(context.Background(), now)

		require.NoError(t, err)
		assert.Equal(t, int64(1), deleted)
	})

	t.Run("TC-2-1-13_期限切れ1000件が全件削除される", func(t *testing.T) {
		_, _ = db.Exec("TRUNCATE conversations, messages, audio_files CASCADE")
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

func TestConversationRepository_SetFirstText(t *testing.T) {
	db := setupTestDB(t)
	repo := postgres.NewConversationRepository(db)

	t.Run("W-01b-01_first_textがNULLの会話に値が保存される", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())

		err := repo.SetFirstText(context.Background(), convID, "こんにちはずんだもん")

		require.NoError(t, err)
		_, _, firstText := queryConversationRow(t, db, convID)
		require.True(t, firstText.Valid)
		assert.Equal(t, "こんにちはずんだもん", firstText.String)
	})

	t.Run("W-01b-02_2回目の呼び出しでは上書きされない", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		require.NoError(t, repo.SetFirstText(context.Background(), convID, "最初の発話"))

		err := repo.SetFirstText(context.Background(), convID, "2回目の発話")

		require.NoError(t, err, "2回目は「何もしない」が正しいのでエラーにしない")
		_, _, firstText := queryConversationRow(t, db, convID)
		assert.Equal(t, "最初の発話", firstText.String,
			"first_textは最初のユーザー発話のみを残すため上書きされてはならない")
	})

	t.Run("W-01b-03_存在しない会話IDでもエラーにならない_0件更新は冪等", func(t *testing.T) {
		err := repo.SetFirstText(context.Background(), ulid.Make().String(), "宛先なし")

		assert.NoError(t, err)
	})

	t.Run("W-01b-04_空文字列は保存され以降は上書きされない", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())

		require.NoError(t, repo.SetFirstText(context.Background(), convID, ""))
		_, _, afterFirst := queryConversationRow(t, db, convID)
		require.True(t, afterFirst.Valid, "空文字列はNULLではなく値として保存される")
		assert.Equal(t, "", afterFirst.String)

		require.NoError(t, repo.SetFirstText(context.Background(), convID, "後から来た発話"))

		_, _, afterSecond := queryConversationRow(t, db, convID)
		assert.Equal(t, "", afterSecond.String,
			"空文字列でもIS NULLではなくなるため、2回目は上書きされない（境界）")
	})

	t.Run("W-01b-05_ちょうど20文字は保存される", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		text := strings.Repeat("あ", 20)

		err := repo.SetFirstText(context.Background(), convID, text)

		require.NoError(t, err)
		_, _, firstText := queryConversationRow(t, db, convID)
		assert.Equal(t, 20, utf8.RuneCountInString(firstText.String))
		assert.Equal(t, text, firstText.String)
	})

	t.Run("W-01b-06_21文字はVARCHAR20の制約でエラーになる", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())

		err := repo.SetFirstText(context.Background(), convID, strings.Repeat("あ", 21))

		assert.Error(t, err,
			"Repositoryは黙って切り詰めない。切り詰めは呼び出し側のTruncateFirstTextの責務であり、"+
				"漏れをDB制約で早期に発火させる")
		// そらの指摘（要修正）: この「DB制約で発火する」は全称ではない。
		// VARCHAR(n) は SQL 仕様上、超過分が半角スペースのみの場合はエラーにせず
		// n 文字へ黙って切り詰める（実測: 'あ'×20 + ' ' は成功し length=20）。
		// 全角スペース・改行・通常文字の超過はエラーになる（実測で確認）。
		// つまり呼び出し側の TruncateFirstText 漏れのうち、
		// 「21文字目以降が半角スペースだけ」のケースだけは DB では検出できない。
		_, _, firstText := queryConversationRow(t, db, convID)
		assert.False(t, firstText.Valid, "エラー時はfirst_textが未設定のまま残るべき")
	})

	t.Run("W-01b-07_絵文字混在20文字がコードポイント単位で保存される", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		text := strings.Repeat("あ", 10) + strings.Repeat("😀", 10) // 20コードポイント/40バイト超

		err := repo.SetFirstText(context.Background(), convID, text)

		require.NoError(t, err, "VARCHAR(20)はバイト数ではなく文字数で数えるため20コードポイントは収まる")
		_, _, firstText := queryConversationRow(t, db, convID)
		assert.Equal(t, text, firstText.String)
		assert.Equal(t, 20, utf8.RuneCountInString(firstText.String))
	})
}

func TestConversationRepository_Exists(t *testing.T) {
	// W-06 のハンドラが「会話が存在しなければ404」を判定するために必要なメソッド。
	// GetRecentMessages では「存在するが発話0件」と「存在しない」を区別できないため、
	// 専用の存在確認を設ける（指示書に明記が無かったため、めたんへ報告済みの追加）。
	db := setupTestDB(t)
	repo := postgres.NewConversationRepository(db)

	t.Run("W-06-E1_存在する会話はtrueを返す", func(t *testing.T) {
		convID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())

		got, err := repo.Exists(context.Background(), convID)

		require.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("W-06-E2_存在しない会話はfalseを返しエラーにしない", func(t *testing.T) {
		got, err := repo.Exists(context.Background(), ulid.Make().String())

		require.NoError(t, err, "見つからないことは異常ではない")
		assert.False(t, got)
	})

	t.Run("W-06-E3_ULID形式でない文字列でもfalseを返しエラーにしない", func(t *testing.T) {
		got, err := repo.Exists(context.Background(), "not-a-ulid")

		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("W-06-E4_空文字列でもfalseを返す", func(t *testing.T) {
		got, err := repo.Exists(context.Background(), "")

		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("W-06-E5_GCで削除された会話はfalseになる", func(t *testing.T) {
		now := time.Now()
		convID := ulid.Make().String()
		insertConversation(t, db, convID, startedAtForExpiry(now.Add(-time.Hour)))
		exists, err := repo.Exists(context.Background(), convID)
		require.NoError(t, err)
		require.True(t, exists)

		_, err = repo.GC(context.Background(), now)
		require.NoError(t, err)

		got, err := repo.Exists(context.Background(), convID)
		require.NoError(t, err)
		assert.False(t, got)
	})
}
