// localfs.FileStore.Write の単体テスト（実ファイルシステム上の一時ディレクトリを使用）。
// 対応: docs/04_implementation/04_realtime_wiring_design.md W-02
package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/localfs"
)

func TestFileStore_Write(t *testing.T) {
	files := localfs.NewFileStore()

	t.Run("W-02-W1_書き込んだ内容がReadでバイト列一致で読み戻せる", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audio.wav")
		want := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0xFF}

		err := files.Write(path, want)

		require.NoError(t, err)
		got, err := files.Read(path)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("W-02-W2_存在しない親ディレクトリは自動作成される", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a", "b", "c", "audio.wav")

		err := files.Write(path, []byte("nested"))

		require.NoError(t, err)
		got, err := files.Read(path)
		require.NoError(t, err)
		assert.Equal(t, []byte("nested"), got)
	})

	t.Run("W-02-W3_既存ファイルは追記ではなく上書きされる", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audio.wav")
		require.NoError(t, files.Write(path, []byte("古い長い内容")))

		err := files.Write(path, []byte("新"))

		require.NoError(t, err)
		got, err := files.Read(path)
		require.NoError(t, err)
		assert.Equal(t, []byte("新"), got, "追記されると古い内容が残ってしまう")
	})

	t.Run("W-02-W4_空バイト列は0バイトのファイルになる", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.wav")

		err := files.Write(path, []byte{})

		require.NoError(t, err)
		info, statErr := os.Stat(path)
		require.NoError(t, statErr)
		assert.Equal(t, int64(0), info.Size())
		got, err := files.Read(path)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("W-02-W5_Write_Delete_Readのライフサイクルが一巡する", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "audio.wav")
		require.NoError(t, files.Write(path, []byte("wav-bytes")))
		require.NoError(t, files.Delete(path))

		_, err := files.Read(path)

		require.Error(t, err)
		assert.True(t, os.IsNotExist(err), "削除後のReadはファイル不存在エラーになるべき")
	})
}
