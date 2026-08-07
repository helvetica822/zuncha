// I/O層テスト（実DB接続・実ファイルシステム）。対応仕様: docs/03_unit_test/08_test_specification.md 4.3 I/O層（観点2-3、TC-2-3-09〜13）
package integration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/localfs"
	"zuncha/internal/model"
	"zuncha/internal/postgres"
	"zuncha/internal/service"
)

func writeTestAudioFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "audio.wav")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestFetchAudio_IO(t *testing.T) {
	db := setupTestDB(t)
	repo := postgres.NewAudioRepository(db)
	files := localfs.NewFileStore()
	svc := service.NewFetchAudioService(repo, files)

	t.Run("TC-2-3-09_処理完了後にファイルとレコードが削除される", func(t *testing.T) {
		convID := ulidLike(20)
		msgID := ulidLike(21)
		audioID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		insertMessage(t, db, msgID, convID, "assistant", "おまたせなのだ", nil, time.Now())
		filePath := writeTestAudioFile(t, "wav-bytes")
		insertAudioFile(t, db, audioID, convID, msgID, filePath, nil)

		data, err := svc.FetchAudio(context.Background(), audioID)

		require.NoError(t, err)
		assert.Equal(t, []byte("wav-bytes"), data)
		_, statErr := os.Stat(filePath)
		assert.True(t, os.IsNotExist(statErr), "物理ファイルが削除されていること")
		assert.Equal(t, 0, countRows(t, db, "audio_files", "id = $1", audioID))
	})

	t.Run("TC-2-3-10_削除済みULIDへの再アクセスは404相当", func(t *testing.T) {
		convID := ulidLike(22)
		msgID := ulidLike(23)
		audioID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		insertMessage(t, db, msgID, convID, "assistant", "おまたせなのだ", nil, time.Now())
		filePath := writeTestAudioFile(t, "wav-bytes")
		insertAudioFile(t, db, audioID, convID, msgID, filePath, nil)

		_, err := svc.FetchAudio(context.Background(), audioID)
		require.NoError(t, err)

		_, err = svc.FetchAudio(context.Background(), audioID)

		assert.Error(t, err)
	})

	t.Run("TC-2-3-11_ファイル不存在時は更新も削除も実行しない", func(t *testing.T) {
		convID := ulidLike(24)
		msgID := ulidLike(25)
		audioID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		insertMessage(t, db, msgID, convID, "assistant", "おまたせなのだ", nil, time.Now())
		missingPath := filepath.Join(t.TempDir(), "does-not-exist.wav")
		insertAudioFile(t, db, audioID, convID, msgID, missingPath, nil)

		_, err := svc.FetchAudio(context.Background(), audioID)

		assert.Error(t, err)
		assert.Equal(t, 1, countRows(t, db, "audio_files", "id = $1 AND fetched_at IS NULL", audioID))
	})

	t.Run("TC-2-3-12_同時リクエストは2回目が404相当になる", func(t *testing.T) {
		convID := ulidLike(26)
		msgID := ulidLike(27)
		audioID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		insertMessage(t, db, msgID, convID, "assistant", "おまたせなのだ", nil, time.Now())
		filePath := writeTestAudioFile(t, "wav-bytes")
		insertAudioFile(t, db, audioID, convID, msgID, filePath, nil)

		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				_, err := svc.FetchAudio(context.Background(), audioID)
				errs[idx] = err
			}(i)
		}
		wg.Wait()

		successCount := 0
		for _, err := range errs {
			if err == nil {
				successCount++
			}
		}
		// 確認事項J：排他制御（明示的なロック等）は実装しない（未定義動作）。
		// ここで期待する「1回のみ成功」は、アプリケーションレベルの排他制御によってではなく、
		// レコード削除SQLの原子性（同一行に対するDELETEは片方のみが実際の削除行を得る）という
		// Postgres自体の性質に依拠した結果にすぎない。タイミングやコネクション数によっては
		// 実際には未定義動作の範囲内で異なる結果になり得る点を明記しておく。
		assert.Equal(t, 1, successCount, "同時リクエストのうち1回のみが成功すること（アプリの排他制御ではなくDELETEの原子性による）")
		assert.Equal(t, 0, countRows(t, db, "audio_files", "id = $1", audioID))
	})

	t.Run("TC-2-3-13_fetched_at非NULLの異常データでも一貫した挙動を示す", func(t *testing.T) {
		convID := ulidLike(28)
		msgID := ulidLike(29)
		audioID := ulid.Make().String()
		insertConversation(t, db, convID, time.Now())
		insertMessage(t, db, msgID, convID, "assistant", "おまたせなのだ", nil, time.Now())
		filePath := writeTestAudioFile(t, "wav-bytes")
		alreadyFetched := time.Now().Add(-time.Minute)
		insertAudioFile(t, db, audioID, convID, msgID, filePath, &alreadyFetched)

		data, err := svc.FetchAudio(context.Background(), audioID)

		require.NoError(t, err, "fetched_atが既に非NULLでも、レコードと実ファイルが存在する限り正常に処理を完走する")
		assert.Equal(t, []byte("wav-bytes"), data)
		assert.Equal(t, 0, countRows(t, db, "audio_files", "id = $1", audioID))
	})
}

// queryAudioRow は保存された音声レコード行を全カラム取得する。
func queryAudioRow(t *testing.T, db *sql.DB, id string) (conversationID, messageID, filePath string, createdAt time.Time, fetchedAt sql.NullTime) {
	t.Helper()
	err := db.QueryRow(
		`SELECT conversation_id, message_id, file_path, created_at, fetched_at FROM audio_files WHERE id = $1`, id,
	).Scan(&conversationID, &messageID, &filePath, &createdAt, &fetchedAt)
	require.NoError(t, err)
	return
}

// seedConversationAndMessage は audio_files の外部キー先（会話とassistantメッセージ）を用意する。
func seedConversationAndMessage(t *testing.T, db *sql.DB) (convID, msgID string) {
	t.Helper()
	convID = ulid.Make().String()
	msgID = ulid.Make().String()
	insertConversation(t, db, convID, time.Now())
	insertMessage(t, db, msgID, convID, "assistant", "おまたせなのだ", nil, time.Now())
	return convID, msgID
}

func TestAudioRepository_InsertRecord(t *testing.T) {
	db := setupTestDB(t)
	repo := postgres.NewAudioRepository(db)

	t.Run("W-02-01_挿入したレコードがGetByULIDで全フィールド一致で取得できFetchedAtはnil", func(t *testing.T) {
		convID, msgID := seedConversationAndMessage(t, db)
		audioID := ulid.Make().String()
		createdAt := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
		want := &model.AudioFile{
			ID:             audioID,
			ConversationID: convID,
			MessageID:      msgID,
			FilePath:       "/var/tmp/zuncha/" + audioID + ".wav",
			CreatedAt:      createdAt,
		}

		err := repo.InsertRecord(context.Background(), want)

		require.NoError(t, err)
		got, err := repo.GetByULID(context.Background(), audioID)
		require.NoError(t, err)
		assert.Equal(t, audioID, got.ID)
		assert.Equal(t, convID, got.ConversationID)
		assert.Equal(t, msgID, got.MessageID)
		assert.Equal(t, want.FilePath, got.FilePath)
		assert.True(t, createdAt.Equal(got.CreatedAt), "明示したCreatedAtがそのまま保存されるべき")
		assert.Nil(t, got.FetchedAt, "未取得状態はfetched_at = NULL")
	})

	t.Run("W-02-02_CreatedAtゼロ値はDB側のNOWで補完される", func(t *testing.T) {
		convID, msgID := seedConversationAndMessage(t, db)
		audioID := ulid.Make().String()
		// 判定基準はDB側の時計のみで揃える（ホストとDBコンテナのクロック差を排除）。
		before := dbNow(t, db)

		err := repo.InsertRecord(context.Background(), &model.AudioFile{
			ID: audioID, ConversationID: convID, MessageID: msgID, FilePath: "/var/tmp/a.wav",
		})
		after := dbNow(t, db)

		require.NoError(t, err)
		_, _, _, createdAt, _ := queryAudioRow(t, db, audioID)
		assert.False(t, createdAt.IsZero())
		assert.WithinRange(t, createdAt, before, after,
			"created_atはNOW()で補完され、INSERT前後にDBから取得した時刻の間に入るべき")
	})

	t.Run("W-02-03_FetchedAtに値を渡しても保存後はNULLになる", func(t *testing.T) {
		convID, msgID := seedConversationAndMessage(t, db)
		audioID := ulid.Make().String()
		fetched := time.Now().Add(-time.Hour)

		err := repo.InsertRecord(context.Background(), &model.AudioFile{
			ID: audioID, ConversationID: convID, MessageID: msgID,
			FilePath: "/var/tmp/a.wav", CreatedAt: time.Now(), FetchedAt: &fetched,
		})

		require.NoError(t, err)
		_, _, _, _, fetchedAt := queryAudioRow(t, db, audioID)
		assert.False(t, fetchedAt.Valid,
			"INSERT時の正しい初期状態は未取得=NULLなので、渡されたFetchedAtは無視される")
	})

	t.Run("W-02-04_InsertRecordしたレコードをFetchAudioが取得して消せる", func(t *testing.T) {
		convID, msgID := seedConversationAndMessage(t, db)
		audioID := ulid.Make().String()
		filePath := writeTestAudioFile(t, "生成されたwavバイト列")
		require.NoError(t, repo.InsertRecord(context.Background(), &model.AudioFile{
			ID: audioID, ConversationID: convID, MessageID: msgID, FilePath: filePath, CreatedAt: time.Now(),
		}))
		svc := service.NewFetchAudioService(repo, localfs.NewFileStore())

		data, err := svc.FetchAudio(context.Background(), audioID)

		require.NoError(t, err, "書き込み側(InsertRecord)と読み取り側(FetchAudio)が噛み合うこと")
		assert.Equal(t, []byte("生成されたwavバイト列"), data)
		assert.Equal(t, 0, countRows(t, db, "audio_files", "id = $1", audioID))
		_, statErr := os.Stat(filePath)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("W-02-05_存在しないmessage_idは外部キー違反でエラー", func(t *testing.T) {
		convID, _ := seedConversationAndMessage(t, db)

		err := repo.InsertRecord(context.Background(), &model.AudioFile{
			ID: ulid.Make().String(), ConversationID: convID, MessageID: ulid.Make().String(),
			FilePath: "/var/tmp/a.wav", CreatedAt: time.Now(),
		})

		assert.Error(t, err)
	})

	t.Run("W-02-06_存在しないconversation_idは外部キー違反でエラー", func(t *testing.T) {
		_, msgID := seedConversationAndMessage(t, db)

		err := repo.InsertRecord(context.Background(), &model.AudioFile{
			ID: ulid.Make().String(), ConversationID: ulid.Make().String(), MessageID: msgID,
			FilePath: "/var/tmp/a.wav", CreatedAt: time.Now(),
		})

		assert.Error(t, err)
	})

	t.Run("W-02-07_同一IDの二重挿入は主キー重複でエラー", func(t *testing.T) {
		convID, msgID := seedConversationAndMessage(t, db)
		audio := &model.AudioFile{
			ID: ulid.Make().String(), ConversationID: convID, MessageID: msgID,
			FilePath: "/var/tmp/a.wav", CreatedAt: time.Now(),
		}
		require.NoError(t, repo.InsertRecord(context.Background(), audio))

		err := repo.InsertRecord(context.Background(), audio)

		assert.Error(t, err)
	})

	t.Run("W-02-08_メッセージ削除でCASCADEにより音声レコードも消える", func(t *testing.T) {
		convID, msgID := seedConversationAndMessage(t, db)
		audioID := ulid.Make().String()
		require.NoError(t, repo.InsertRecord(context.Background(), &model.AudioFile{
			ID: audioID, ConversationID: convID, MessageID: msgID,
			FilePath: "/var/tmp/a.wav", CreatedAt: time.Now(),
		}))
		require.Equal(t, 1, countRows(t, db, "audio_files", "id = $1", audioID))

		_, err := db.Exec(`DELETE FROM messages WHERE id = $1`, msgID)

		require.NoError(t, err)
		assert.Equal(t, 0, countRows(t, db, "audio_files", "id = $1", audioID))
	})
}
