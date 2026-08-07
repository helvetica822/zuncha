package localfs

import (
	"fmt"
	"os"
	"path/filepath"
)

// dirPerm/filePerm は生成する音声ディレクトリ・ファイルのパーミッション。
const (
	dirPerm  os.FileMode = 0o755
	filePerm os.FileMode = 0o644
)

// FileStore はローカルファイルシステム上の音声ファイルを読み書きする。
type FileStore struct{}

// NewFileStore は FileStore を生成する（引数なし）。
func NewFileStore() *FileStore {
	return &FileStore{}
}

// Read はパスのファイル内容を返す。存在しなければエラー。
func (f *FileStore) Read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Write はパスへファイルを書き込む。親ディレクトリが無ければ作成する。既存ファイルは上書きする。
// service.FileStore インターフェースには含めない（読み取り側の契約を変えないため）。
// 書き込みを使う TTS 側で、消費側が必要な I/F を定義する。
func (f *FileStore) Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create audio dir %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("write audio file %s: %w", path, err)
	}
	return nil
}

// Delete はパスのファイルを削除する。存在しなければエラー（os.Remove の性質）。
func (f *FileStore) Delete(path string) error {
	return os.Remove(path)
}
