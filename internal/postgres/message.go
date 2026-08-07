package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"zuncha/internal/model"
	"zuncha/internal/repository"
)

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// recentMessagesLimit は GetRecentMessages が返す直近メッセージの最大件数。
const recentMessagesLimit = 20

// GetRecentMessages は直近20件を古い→新しい順で返す（0件でも非 nil）。
func (r *MessageRepository) GetRecentMessages(ctx context.Context, conversationID string) ([]model.Message, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, conversation_id, role, content, emotion, created_at
		 FROM messages
		 WHERE conversation_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2`, conversationID, recentMessagesLimit)
	if err != nil {
		return nil, fmt.Errorf("get recent messages: %w", err)
	}
	defer rows.Close()

	messages := make([]model.Message, 0)
	for rows.Next() {
		var m model.Message
		var emotion sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &emotion, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if emotion.Valid {
			m.Emotion = &emotion.String
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}
	// クエリは新しい→古い(created_at DESC, id DESC)で取得。古い→新しい順に反転する。
	return repository.ReverseMessages(messages), nil
}

// InsertMessage はメッセージを1件保存する。ID採番と role/emotion の検証は呼び出し側の責務。
// CreatedAt がゼロ値の場合は DB 側の NOW() に委ねる（ゼロ値のまま保存すると
// GetRecentMessages の created_at 並びが壊れるため）。
func (r *MessageRepository) InsertMessage(ctx context.Context, msg *model.Message) error {
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO messages (id, conversation_id, role, content, emotion, created_at)
		 VALUES ($1, $2, $3, $4, $5, COALESCE($6::timestamptz, NOW()))`,
		msg.ID, msg.ConversationID, msg.Role, msg.Content, msg.Emotion, nullTimeOrNow(msg.CreatedAt),
	); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

var _ repository.MessageRepository = (*MessageRepository)(nil)
