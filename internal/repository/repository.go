package repository

import (
	"context"
	"errors"
	"time"

	"zuncha/internal/model"
)

// ConversationRepository は会話の永続化を抽象化する。
type ConversationRepository interface {
	// GC は expires_at < now の会話を削除し、削除件数を返す。
	GC(ctx context.Context, now time.Time) (int64, error)
	InsertConversation(ctx context.Context, conv *model.Conversation) error
	// SetFirstText は first_text が未設定の場合のみ text を記録する（最初のユーザー発話のみを残す）。
	SetFirstText(ctx context.Context, conversationID, text string) error
	// Exists は会話が存在するかを返す。存在しないことは異常ではないのでエラーにしない。
	// GetRecentMessages では「存在するが発話0件」と「存在しない」を区別できないため別途必要。
	Exists(ctx context.Context, conversationID string) (bool, error)
}

// AudioRepository は音声ファイルレコードの永続化を抽象化する。
type AudioRepository interface {
	GetByULID(ctx context.Context, ulid string) (*model.AudioFile, error)
	UpdateFetchedAt(ctx context.Context, ulid string, fetchedAt time.Time) error
	DeleteRecord(ctx context.Context, ulid string) error
	InsertRecord(ctx context.Context, audio *model.AudioFile) error
}

// MessageRepository は会話メッセージの永続化を抽象化する。
type MessageRepository interface {
	GetRecentMessages(ctx context.Context, conversationID string) ([]model.Message, error)
	InsertMessage(ctx context.Context, msg *model.Message) error
}

// ErrAudioNotFound は音声レコードが存在しない場合のセンチネルエラー。
var ErrAudioNotFound = errors.New("repository: audio file not found")
