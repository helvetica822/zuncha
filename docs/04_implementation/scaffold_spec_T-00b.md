# T-00b 契約スキャフォールド 実装仕様

- 作成者: 四国めたん (テックリード)
- 目的: `tests/unit/` の Go テスト(単一パッケージ `unit`)が**コンパイル可能**になるよう、`internal/*` の全参照シンボル(型・インターフェース・関数シグネチャ・センチネルエラー)を stub body 付きで用意する。これにより `go test ./tests/unit/...` が「build failed」ではなく**実行され、大半 FAIL のフル RED ベースライン**が得られる。
- 根拠: 全11 unitテストファイルのモック定義・構造体リテラル・関数呼び出しから抽出した契約(サブエージェント棚卸し)。

## 重要方針(必読)

1. **stub body はゼロ値返却。`panic()` は使わない**。panic は `go test` プロセス全体をクラッシュさせ、フル RED ベースラインが得られない。ゼロ値ならアサーション失敗で綺麗に FAIL する。
2. **ポインタ返却関数は `nil` でなく空構造体ポインタ(`&T{}`)を返す**。テストが戻り値を dereference する箇所(`got.Text` 等)での nil パニックを避けるため。
3. インターフェースはメソッド宣言のみ(body 不要、テスト側のモックが実装する)。
4. 既存の各パッケージ `doc.go`(package コメント)は残し、新規 `.go` にはパッケージコメントを重複させない。
5. **範囲外**: `internal/postgres`・`internal/localfs`・`MessageRepository` は unit テスト未参照。#24 では作らない(integration 対応の T-07/T-10/T-11 で追加)。
6. #24 は結果的に **T-06(model 定義)を内包**する(model は本物の struct 定義)。

## 技術判断(確定事項)

- **R-2 解決**: `FileStore` インターフェースは `var _` 束縛が無い暗黙 I/F。Go の「消費側で定義」慣習に従い **`internal/service` に定義**する。`localfs.NewFileStore()`(T-11)がこれを満たす型を返し、DI で注入する。
- **GC 署名**: モック定義より `GC(ctx, now) (int64, error)`(削除件数 + error)。計画書の「error のみ」を訂正。

---

## 実装するファイル一覧と内容

### internal/model/conversation.go
```go
package model

import "time"

// Conversation は会話セッションを表すドメインモデル。
type Conversation struct {
	ID        string
	StartedAt time.Time
	ExpiresAt time.Time
	FirstText *string
}
```

### internal/model/message.go
```go
package model

import "time"

// Message は会話内の1発話を表す。emotion は assistant のみ設定され nullable。
type Message struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	Emotion        *string
	CreatedAt      time.Time
}
```

### internal/model/audio_file.go
```go
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
```
> 注: unit テストの構造体リテラルは**名前付きフィールド**なので、余剰フィールド追加は破壊しない(DB 設計に整合)。

### internal/validation/ulid.go
```go
package validation

// IsValidULID は s が有効な ULID 形式(26文字 Crockford Base32)か判定する。
func IsValidULID(s string) bool {
	return false // T-01 で実装
}
```

### internal/validation/first_text.go
```go
package validation

// TruncateFirstText は s の先頭20ルーンを返す。
func TruncateFirstText(s string) string {
	return "" // T-02 で実装
}
```

### internal/validation/role_emotion.go
```go
package validation

// ValidateRole は role が許容値か検証する。
func ValidateRole(role string) error {
	return nil // T-03 で実装
}

// ValidateEmotion は emotion(nullable)が許容値か検証する。
func ValidateEmotion(emotion *string) error {
	return nil // T-03 で実装
}

// ValidateRoleEmotionConsistency は role と emotion の整合を検証する。
func ValidateRoleEmotionConsistency(role string, emotion *string) error {
	return nil // T-03 で実装
}
```

### internal/validation/input.go
```go
package validation

// IsValidInput は s が trim 後に非空か判定する。
func IsValidInput(s string) bool {
	return false // T-04 で実装
}
```

### internal/gc/expiration.go
```go
package gc

import "time"

// IsExpired は expiresAt が now より前(期限切れ)か判定する。
func IsExpired(expiresAt time.Time, now time.Time) bool {
	return false // T-05 で実装
}
```

### internal/repository/reverse.go
```go
package repository

import "zuncha/internal/model"

// ReverseMessages は messages を逆順にした新スライスを返す(空でも非 nil)。
func ReverseMessages(messages []model.Message) []model.Message {
	return nil // T-07 で実装(空でも非 nil を返すこと)
}
```

### internal/repository/repository.go
```go
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
}

// AudioRepository は音声ファイルレコードの永続化を抽象化する。
type AudioRepository interface {
	GetByULID(ctx context.Context, ulid string) (*model.AudioFile, error)
	UpdateFetchedAt(ctx context.Context, ulid string, fetchedAt time.Time) error
	DeleteRecord(ctx context.Context, ulid string) error
}

// ErrAudioNotFound は音声レコードが存在しない場合のセンチネルエラー。
var ErrAudioNotFound = errors.New("repository: audio file not found")
```

### internal/service/filestore.go
```go
package service

// FileStore は音声ファイルの読み取り・削除を抽象化する(消費側で定義)。
type FileStore interface {
	Read(path string) ([]byte, error)
	Delete(path string) error
}
```

### internal/service/create_conversation.go
```go
package service

import (
	"context"

	"zuncha/internal/model"
	"zuncha/internal/repository"
)

// CreateConversationService は会話作成ユースケースを担う。
type CreateConversationService struct {
	repo repository.ConversationRepository
}

func NewCreateConversationService(repo repository.ConversationRepository) *CreateConversationService {
	return &CreateConversationService{repo: repo}
}

// CreateConversation は GC 実行後に新規会話を作成する。
func (s *CreateConversationService) CreateConversation(ctx context.Context) (*model.Conversation, error) {
	return &model.Conversation{}, nil // T-08 で実装
}
```

### internal/service/fetch_audio.go
```go
package service

import (
	"context"

	"zuncha/internal/repository"
)

// FetchAudioService は音声取得〜削除ユースケースを担う。
type FetchAudioService struct {
	repo  repository.AudioRepository
	files FileStore
}

func NewFetchAudioService(repo repository.AudioRepository, files FileStore) *FetchAudioService {
	return &FetchAudioService{repo: repo, files: files}
}

// FetchAudio は ULID の音声を読み取り、取得済みマーク後にファイル・レコードを削除する。
func (s *FetchAudioService) FetchAudio(ctx context.Context, ulid string) ([]byte, error) {
	return nil, nil // T-09 で実装
}
```

### internal/service/response_streamer.go
```go
package service

import (
	"context"

	"zuncha/internal/llm"
	"zuncha/internal/sse"
	"zuncha/internal/tts"
)

// ResponseStreamer は LLM 生成〜SSE 配信のオーケストレーションを担う。
type ResponseStreamer struct {
	llmClient llm.LLMClient
	parser    llm.ResponseParser
	ttsClient tts.TTSClient
	chunker   sse.TextChunker
}

func NewResponseStreamer(
	llmClient llm.LLMClient,
	parser llm.ResponseParser,
	ttsClient tts.TTSClient,
	chunker sse.TextChunker,
) *ResponseStreamer {
	return &ResponseStreamer{
		llmClient: llmClient,
		parser:    parser,
		ttsClient: ttsClient,
		chunker:   chunker,
	}
}

// StreamResponse は prompt に対する応答を生成し sink へ配信する。
func (s *ResponseStreamer) StreamResponse(ctx context.Context, sink sse.EventSink, prompt string) error {
	return nil // T-15 で実装
}
```

### internal/llm/response.go
```go
package llm

// LLMResponse は LLM 応答のパース結果。
type LLMResponse struct {
	Text    string
	Emotion string
}
```

### internal/llm/parser.go
```go
package llm

// ParseLLMResponse は LLM の生 JSON をパースして LLMResponse を返す。
func ParseLLMResponse(body []byte) (*LLMResponse, error) {
	return &LLMResponse{}, nil // T-12 で実装
}
```

### internal/llm/client.go
```go
package llm

import "context"

// LLMClient は LLM への問い合わせを抽象化する。
type LLMClient interface {
	GenerateResponse(ctx context.Context, prompt string) ([]byte, error)
}

// ResponseParser は LLM 応答のパースを抽象化する。
type ResponseParser interface {
	Parse(body []byte) (*LLMResponse, error)
}
```

### internal/llm/errors.go
```go
package llm

import "errors"

// LLM 応答パースのセンチネルエラー(errors.Is で相互に区別可能)。
var (
	ErrSyntax = errors.New("llm: syntax error")
	ErrSchema = errors.New("llm: schema error")
	ErrValue  = errors.New("llm: value error")
)
```
> 注: T-12 実装時にエラー階層(ラップ)を導入するが、スキャフォールドでは3つを独立インスタンスとして定義すれば `errors.Is` の区別は成立する。

### internal/tts/client.go
```go
package tts

import "context"

// TTSClient はテキストから音声を合成し URL を返す。
type TTSClient interface {
	Synthesize(ctx context.Context, text string) (string, error)
}
```

### internal/sse/sse.go
```go
package sse

// TextChunker はテキストを配信単位に分割する。
type TextChunker interface {
	Chunk(text string) []string
}

// EventSink は SSE イベントの送出を抽象化する。
type EventSink interface {
	SendEmotion(label string) error
	SendTextChunk(chunk string) error
	SendAudioURL(url string) error
	SendDone() error
	SendError(message string) error
}
```

### internal/stt/result.go
```go
package stt

// STTResult は音声認識の結果。
type STTResult struct {
	Text       string
	Confidence float64
}
```

### internal/stt/judgment.go
```go
package stt

import "time"

// IsRecognitionFailed は認識失敗(空・空白・低信頼度)か判定する。
func IsRecognitionFailed(result STTResult) bool {
	return false // T-13 で実装
}

// IsTimedOut は無音開始から threshold 以上経過したか判定する。
func IsTimedOut(silenceStart time.Time, now time.Time, threshold time.Duration) bool {
	return false // T-13 で実装
}
```

---

## 完了検証(必須・ログ報告)

`export PATH="$HOME/.local/bin:$PATH"` 後:

1. `gofmt -l internal` → 差分なし。
2. `go build ./...` → 成功。
3. `go vet ./internal/... ./cmd/...` → 成功。
4. `go test ./tests/unit/... 2>&1 | tail -20` → **「build failed」が消え、テストが実行され FAIL/PASS のサマリーが出る**こと(大半 FAIL のフル RED ベースライン確立)。パニックでプロセスが落ちていないこと。
5. FE には一切影響を与えない(このタスクは Go のみ)。

完了報告時はタスクを completed にせず `in_progress` のまま、上記ログ付きで報告すること(そらレビュー✅後に completed)。
