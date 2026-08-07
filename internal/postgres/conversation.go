package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"zuncha/internal/model"
	"zuncha/internal/repository"
)

type ConversationRepository struct {
	db *sql.DB
}

func NewConversationRepository(db *sql.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// GC は expires_at < now の会話を削除し(CASCADEで子も削除)、削除件数を返す。
func (r *ConversationRepository) GC(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM conversations WHERE expires_at < $1`, now)
	if err != nil {
		return 0, fmt.Errorf("gc conversations: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("gc rows affected: %w", err)
	}
	return n, nil
}

// InsertConversation は会話を作成する。started_at/expires_at は DB 側で生成する
// （id と first_text のみ指定。expires_at は生成列ゆえ明示しない = R-6）。
func (r *ConversationRepository) InsertConversation(ctx context.Context, conv *model.Conversation) error {
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO conversations (id, first_text) VALUES ($1, $2)`,
		conv.ID, conv.FirstText,
	); err != nil {
		return fmt.Errorf("insert conversation: %w", err)
	}
	return nil
}

// SetFirstText は first_text が未設定の場合のみ text を記録する。
// 「SELECT で空か確認してから UPDATE」だと 10 並列で競合するため、
// `AND first_text IS NULL` を付けた1文で原子的に条件付き更新する。
// 0件更新はエラーにしない（2回目以降は「何もしない」のが正しい = 冪等）。
// 20文字への切り詰めは呼び出し側（validation.TruncateFirstText）の責務。
func (r *ConversationRepository) SetFirstText(ctx context.Context, conversationID, text string) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE conversations SET first_text = $2 WHERE id = $1 AND first_text IS NULL`,
		conversationID, text,
	); err != nil {
		return fmt.Errorf("set first text: %w", err)
	}
	return nil
}

// Exists は会話の存在を返す。ハンドラが 404 を判定するために使う。
// 見つからないことは異常ではないので (false, nil) を返す。
func (r *ConversationRepository) Exists(ctx context.Context, conversationID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM conversations WHERE id = $1)`, conversationID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("conversation exists: %w", err)
	}
	return exists, nil
}

var _ repository.ConversationRepository = (*ConversationRepository)(nil)
