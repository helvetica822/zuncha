package service

// FileStore は音声ファイルの読み取り・削除を抽象化する(消費側で定義)。
type FileStore interface {
	Read(path string) ([]byte, error)
	Delete(path string) error
}
