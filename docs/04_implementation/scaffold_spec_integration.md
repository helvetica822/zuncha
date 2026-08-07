# Integration実装仕様 (T-10 PostgreSQL Repository / T-11 localfs FileStore)

- 作成者: 四国めたん (テックリード)
- 対象: `internal/postgres/*.go`(T-10, 新規)、`internal/localfs/filestore.go`(T-11)、および `internal/repository` への `MessageRepository` I/F 追加。
- 前提: `ZUNCHA_TEST_DATABASE_URL`(`source scripts/test_env.sh`)設定下で `go test ./tests/integration/... -v` を実行。migrationは適用済み(T-DB、生成列修正版)。
- 根拠: `tests/integration/conversation_repository_test.go`(TC-2-1-08〜15)、`message_repository_test.go`(TC-2-2-05〜15)、`audio_fetch_test.go`(TC-2-3-09〜13)、および helpers_test.go の INSERT列。

---

## 0. repository への MessageRepository I/F 追加

`internal/repository/repository.go` に追記(unitでは未参照だったため#24未定義)。**limit引数なし**(内部20固定)。

```go
// MessageRepository は会話メッセージの取得を抽象化する。
type MessageRepository interface {
	GetRecentMessages(ctx context.Context, conversationID string) ([]model.Message, error)
}
```

---

## T-11: internal/localfs/filestore.go

```go
package localfs

import "os"

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

// Delete はパスのファイルを削除する。存在しなければエラー（os.Remove の性質）。
func (f *FileStore) Delete(path string) error {
	return os.Remove(path)
}

// Write は 2026-08-05 に Wave A (W-02) で追加。親ディレクトリを自動作成し、既存は上書きする。
// service.FileStore インターフェースには含めない（読み取り側の契約を変えないため）。
func (f *FileStore) Write(path string, data []byte) error
```
**契約根拠**: `service.FileStore`(Read/Delete)を満たす。※**2026-08-05: 構造体に `Write` を追加済みだが `service.FileStore` I/F は Read/Delete のまま不変**(書き込みは W-09 の TTS 側が消費側で I/F を定義する)。Read不在→エラー(TC-2-3-11: Readで失敗しUpdate/Delete未実行→fetched_at NULLのまま残る)。Delete不在→エラー(TC-2-3-12の並行"1回のみ成功"は、物理ファイルos.Removeが片方でENOENTエラーになる性質に依拠。DeleteRecordは冪等ゆえ差別化点はファイル削除)。

---

## T-10: internal/postgres/ (3ファイル新規)

各Repositoryは `db *sql.DB` を保持するコンストラクタパターン。`import "github.com/lib/pq"` はテスト側でblank import済みゆえ実装では不要(database/sqlのみ)。各ファイル末尾で `var _ repository.Xxx = (*Xxx)(nil)` を宣言しI/F適合を保証。

### internal/postgres/conversation.go
```go
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
	return res.RowsAffected()
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

var _ repository.ConversationRepository = (*ConversationRepository)(nil)
```
**根拠**: GCは`< $1`(now注入)でTC-2-1-11(ちょうど一致は非削除)、CASCADEでTC-2-1-09、件数はRowsAffected()でTC-2-1-12/13。InsertはDEFAULT NOW()/生成列に委ねTC-2-1-10(started_at≈now、expires_at=+30d、first_text NULL)、PK重複でTC-2-1-14エラー。first_text(*string nil)→NULL。

### internal/postgres/message.go
```go
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

// GetRecentMessages は直近20件を古い→新しい順で返す（0件でも非 nil）。
func (r *MessageRepository) GetRecentMessages(ctx context.Context, conversationID string) ([]model.Message, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, conversation_id, role, content, emotion, created_at
		 FROM messages
		 WHERE conversation_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT 20`, conversationID)
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

var _ repository.MessageRepository = (*MessageRepository)(nil)
```
**根拠**: `ORDER BY created_at DESC, id DESC LIMIT 20`で直近20件を取得し、`ReverseMessages`(T-07)で古→新へ反転。同一created_atは`id DESC`→反転で`id ASC`=生成順でTC-2-2-13安定。TC-2-2-06/12は末尾20件(古い1件除外)、TC-2-2-08/09は空(ReverseMessagesが非nil空返却)。emotionはNullStringで*stringへ。

### internal/postgres/audio.go
```go
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

var _ repository.AudioRepository = (*AudioRepository)(nil)
```
**根拠**: 不存在→ErrAudioNotFound(TC-2-3-10/12の2回目404、unit TC-2-3-02のerrors.Is)。fetched_atはNullTimeで*timeへ(TC-2-3-13の非NULL異常データも読める)。Update/Deleteは0件でもエラーにしない冪等契約(unit TC-2-3-08)。並行時の差別化はlocalfs.Delete(ファイル)側。

---

## 完了検証(必須・ログ報告)
`export PATH="$HOME/.local/bin:$PATH" && source scripts/test_env.sh` 後:
1. `gofmt -l internal` 差分なし、`go vet ./internal/...` 成功。
2. `go test ./tests/integration/... -v 2>&1 | tail -40` → **t.Skipされず実行**され、TC-2-1-08〜15・TC-2-2-05〜15・TC-2-3-09〜13が全緑。
3. `go test ./tests/unit/... 2>&1 | tail -5` → unit側に退行がないこと。
in_progress維持で検証ログ付き報告(そら復帰後レビュー→completed)。

> 注: これでバックエンドは unit + integration 全緑となり、GREEN phase のバックエンド実装が完了する。残るは cmd/api の配線(テスト対象外)のみ。
