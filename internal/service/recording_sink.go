package service

import (
	"context"
	"log"
	"strings"
	"time"

	"zuncha/internal/model"
	"zuncha/internal/repository"
	"zuncha/internal/sse"
)

// メッセージの role。DB の CHECK 制約と同じ値（validation 側の非公開定数とは別に持つ）。
const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

// assistantSaveTimeout は assistant 応答の保存にかける上限。
// done の送出を待たせすぎないよう短く取る。
const assistantSaveTimeout = 5 * time.Second

// recordingSink は EventSink のデコレータ。配信を素通しさせつつ応答を蓄積し、
// done の直前に assistant メッセージを永続化する。
//
// StreamResponse は単一 goroutine から順番にメソッドを呼ぶため、蓄積の排他は不要。
type recordingSink struct {
	inner          sse.EventSink
	repo           repository.MessageRepository
	conversationID string
	// messageID は呼び出し側（ChatService）が事前採番した assistant メッセージのID。
	// TTS合成(audio_files登録)が SendDone より先行するため、ここで採番すると
	// audio_files.message_id と一致させられない（D-4 訂正2）。
	messageID string
	now       func() time.Time

	emotion *string
	text    strings.Builder
}

// NewRecordingSink は inner をラップした EventSink を返す。
// now は関数として注入する（時刻に依存させない既存方針と一貫させ、テストで決定化できる）。
// messageID は採番済みの値を受け取る（採番元は ChatService）。
func NewRecordingSink(
	inner sse.EventSink,
	repo repository.MessageRepository,
	conversationID string,
	messageID string,
	now func() time.Time,
) sse.EventSink {
	return &recordingSink{
		inner:          inner,
		repo:           repo,
		conversationID: conversationID,
		messageID:      messageID,
		now:            now,
	}
}

// SendEmotion は label を保持してから委譲する。
func (s *recordingSink) SendEmotion(label string) error {
	l := label
	s.emotion = &l
	return s.inner.SendEmotion(label)
}

// SendTextChunk はチャンクを蓄積してから委譲する。
func (s *recordingSink) SendTextChunk(chunk string) error {
	s.text.WriteString(chunk)
	return s.inner.SendTextChunk(chunk)
}

func (s *recordingSink) SendAudioURL(url string) error {
	return s.inner.SendAudioURL(url)
}

// SendDone は assistant メッセージを保存してから done を送る。
// 順序を逆にすると「画面には出たがDBに無い」状態が起こる（仕様書§3.2）。
//
// 保存に失敗してもログのみでエラーを返さず done は送る。応答は既にユーザーの画面へ
// 届いており、ここでエラートーストを出すと混乱させるだけで、履歴の欠落は
// 次回の文脈が減るだけで致命的ではない。
func (s *recordingSink) SendDone() error {
	msg := &model.Message{
		ID:             s.messageID,
		ConversationID: s.conversationID,
		Role:           roleAssistant,
		Content:        s.text.String(),
		Emotion:        s.emotion,
		CreatedAt:      s.now(),
	}
	// EventSink のメソッドは ctx を取らないため、保存用の ctx をここで作る。
	// リクエストのキャンセルから切り離して応答を確実に残す一方、Background 単体だと
	// DBがハングしたとき SendDone が無制限に待つので短いタイムアウトを被せる。
	ctx, cancel := context.WithTimeout(context.Background(), assistantSaveTimeout)
	defer cancel()

	if err := s.repo.InsertMessage(ctx, msg); err != nil {
		log.Printf("assistant メッセージの保存に失敗（配信は継続）: conversation_id=%s: %v",
			s.conversationID, err)
	}
	return s.inner.SendDone()
}

// SendError は委譲のみ。失敗した応答は履歴に残さない。
func (s *recordingSink) SendError(message string) error {
	return s.inner.SendError(message)
}

var _ sse.EventSink = (*recordingSink)(nil)
