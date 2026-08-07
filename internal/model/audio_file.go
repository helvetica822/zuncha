package model

import "time"

// AudioFile は TTS 生成音声ファイルの一時管理レコード。
type AudioFile struct {
	ID             string
	ConversationID string
	MessageID      string
	FilePath       string
	CreatedAt      time.Time
	FetchedAt      *time.Time
}
