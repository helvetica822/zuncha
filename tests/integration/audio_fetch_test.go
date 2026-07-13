// I/O層テスト（実DB接続・実ファイルシステム）。対応仕様: docs/03_unit_test/08_test_specification.md 4.3 I/O層（観点2-3、TC-2-3-09〜13）
package integration

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/localfs"
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
