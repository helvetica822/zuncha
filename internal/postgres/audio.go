package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"zuncha/internal/model"
	"zuncha/internal/repository"
)

type AudioRepository struct {
	db *sql.DB
}

func NewAudioRepository(db *sql.DB) *AudioRepository {
	return &AudioRepository{db: db}
}

// GetByULID は音声レコードを返す。存在しなければ repository.ErrAudioNotFound。
func (r *AudioRepository) GetByULID(ctx context.Context, ulid string) (*model.AudioFile, error) {
	var a model.AudioFile
	var fetchedAt sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, conversation_id, message_id, file_path, created_at, fetched_at
		 FROM audio_files WHERE id = $1`, ulid).
		Scan(&a.ID, &a.ConversationID, &a.MessageID, &a.FilePath, &a.CreatedAt, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repository.ErrAudioNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get audio by ulid: %w", err)
	}
	if fetchedAt.Valid {
		a.FetchedAt = &fetchedAt.Time
	}
	return &a, nil
}

// UpdateFetchedAt は取得日時を記録する（0件でもエラーにしない=冪等）。
func (r *AudioRepository) UpdateFetchedAt(ctx context.Context, ulid string, fetchedAt time.Time) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE audio_files SET fetched_at = $2 WHERE id = $1`, ulid, fetchedAt,
	); err != nil {
		return fmt.Errorf("update fetched_at: %w", err)
	}
	return nil
}

// DeleteRecord はレコードを削除する（0件でもエラーにしない=冪等）。
func (r *AudioRepository) DeleteRecord(ctx context.Context, ulid string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM audio_files WHERE id = $1`, ulid,
	); err != nil {
		return fmt.Errorf("delete audio record: %w", err)
	}
	return nil
}

// InsertRecord は音声レコードを1件保存する。
// fetched_at は列に含めない（未取得 = NULL が INSERT 時の正しい初期状態のため、
// audio.FetchedAt が非nilで渡ってきても無視する）。
// CreatedAt がゼロ値の場合は DB 側の NOW() に委ねる。
func (r *AudioRepository) InsertRecord(ctx context.Context, audio *model.AudioFile) error {
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO audio_files (id, conversation_id, message_id, file_path, created_at)
		 VALUES ($1, $2, $3, $4, COALESCE($5::timestamptz, NOW()))`,
		audio.ID, audio.ConversationID, audio.MessageID, audio.FilePath, nullTimeOrNow(audio.CreatedAt),
	); err != nil {
		return fmt.Errorf("insert audio record: %w", err)
	}
	return nil
}

var _ repository.AudioRepository = (*AudioRepository)(nil)
