package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"zuncha/internal/llm"
	"zuncha/internal/model"
	"zuncha/internal/repository"
	"zuncha/internal/sse"
	"zuncha/internal/validation"
)

// requestSeenTTL は処理済み request_id を覚えておく期間。
// 飛行中 + 完了後しばらくを覆えば再送・リトライ・複数タブを弾ける（仕様書§3.3）。
const requestSeenTTL = 5 * time.Minute

// 中断時にフロントへ通知するメッセージ。
// 中断しても SSE の error を送らないと done も error も届かず、フロントの inputState が
// sending のまま固着する。フロントは INV-3b で「error を受けたら editable へ復帰」を
// 既に持っているため、1発送れば既存機構で救済される。
const (
	errMsgSaveUserMessage = "発話の保存に失敗しました。もう一度お試しください。"
	errMsgLoadHistory     = "会話履歴の取得に失敗しました。もう一度お試しください。"
)

// ChatService はユーザー発話1件の受理から SSE 配信までを組み立てる。
type ChatService struct {
	msgRepo  repository.MessageRepository
	convRepo repository.ConversationRepository
	streamer *ResponseStreamer
	hub      *sse.Hub
	newID    func() string
	now      func() time.Time

	// mu は seen を保護する。10人・単一インスタンス前提なので map + 時刻で足りる（LRU等は不要）。
	mu   sync.Mutex
	seen map[string]time.Time
}

func NewChatService(
	msgRepo repository.MessageRepository,
	convRepo repository.ConversationRepository,
	streamer *ResponseStreamer,
	hub *sse.Hub,
	newID func() string,
	now func() time.Time,
) *ChatService {
	return &ChatService{
		msgRepo:  msgRepo,
		convRepo: convRepo,
		streamer: streamer,
		hub:      hub,
		newID:    newID,
		now:      now,
		seen:     make(map[string]time.Time),
	}
}

// HandleUserMessage はユーザー発話を保存し、履歴からプロンプトを組み立てて応答を配信する。
//
// 処理順序（変えないこと）:
//  1. ユーザー発話を保存（失敗したら中断。発話を失ったまま応答してはならない）
//  2. first_text を記録（冪等なので毎回呼ぶ。失敗はログのみ）
//  3. 履歴取得 → プロンプト組み立て（1の後なので自分の発話が文脈に入る）
//  4. assistant メッセージIDの事前採番 → sink 構築（Fanout で request_id 注入 →
//     RecordingSink で応答を永続化）
//  5. 応答生成と配信
//
// 同一 request_id の2回目以降は何もせず nil を返す（二重送信の第二防衛線・仕様書§3.3）。
func (s *ChatService) HandleUserMessage(ctx context.Context, conversationID, requestID, text string) error {
	if !s.markRequest(requestID) {
		// 再送は何もしないのが正しい。ここで error を送ると、1回目の正常応答を
		// 受け取ったフロントに余計なトーストが出る。
		return nil
	}

	// 中断経路でも error を送れるよう、処理を始める前に sink を用意する。
	fanout := sse.NewFanout(s.hub, conversationID, requestID)

	userMsg := &model.Message{
		ID:             s.newID(),
		ConversationID: conversationID,
		Role:           roleUser,
		Content:        text,
		CreatedAt:      s.now(),
	}
	if err := s.msgRepo.InsertMessage(ctx, userMsg); err != nil {
		_ = fanout.SendError(errMsgSaveUserMessage)
		return fmt.Errorf("insert user message: %w", err)
	}

	// first_text は付随処理。会話本体を止める理由がないため失敗はログのみ
	// （既存 CreateConversationService の GC と同じ流儀）。
	// 20ルーンへの切り詰めは呼び出し側の責務なのでここで通す。
	if err := s.convRepo.SetFirstText(ctx, conversationID, validation.TruncateFirstText(text)); err != nil {
		log.Printf("first_text の記録に失敗（会話は継続）: conversation_id=%s: %v", conversationID, err)
	}

	history, err := s.msgRepo.GetRecentMessages(ctx, conversationID)
	if err != nil {
		_ = fanout.SendError(errMsgLoadHistory)
		return fmt.Errorf("get recent messages: %w", err)
	}
	prompt := llm.BuildPrompt(history)

	// assistant メッセージのIDはここで事前採番する。TTS合成(audio_files登録)は
	// assistant メッセージの保存(SendDone)より先に走るため、両者で同じIDを共有するには
	// RecordingSink 内での採番では間に合わない（D-4 訂正2）。
	assistantMessageID := s.newID()

	sink := NewRecordingSink(
		fanout,
		s.msgRepo,
		conversationID,
		assistantMessageID,
		s.now,
	)
	return s.streamer.StreamResponse(ctx, sink, prompt, conversationID, assistantMessageID)
}

// markRequest は requestID を処理対象として記録し、新規なら true を返す。
// 既に記録済み（保持期間内）なら false を返す。
func (s *ChatService) markRequest(requestID string) bool {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 呼び出しごとに期限切れを掃除する程度で十分。
	for id, seenAt := range s.seen {
		if now.Sub(seenAt) > requestSeenTTL {
			delete(s.seen, id)
		}
	}
	if _, exists := s.seen[requestID]; exists {
		return false
	}
	s.seen[requestID] = now
	return true
}
