package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
	"unicode/utf8"

	"zuncha/internal/validation"
)

// responseTimeout は応答生成（LLM→TTS→配信）全体の上限。
const responseTimeout = 60 * time.Second

// postMessageRequest は POST /conversations/{id}/messages のリクエスト本文。
// request_id はクライアント側が ULID で採番する（送信前に相関キーを持てるため）。
type postMessageRequest struct {
	RequestID string `json:"request_id"`
	Text      string `json:"text"`
}

// PostMessage は POST /conversations/{id}/messages を処理する。
// 応答生成は別 goroutine で SSE へ流し、202 を即返す。
func (h *Handler) PostMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validation.IsValidULID(id) {
		respondError(w, http.StatusBadRequest, "会話IDの形式が不正です")
		return
	}

	var req postMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "リクエスト本文の形式が不正です")
		return
	}
	if !validation.IsValidULID(req.RequestID) {
		respondError(w, http.StatusBadRequest, "request_idの形式が不正です")
		return
	}
	// 不正UTF-8はここで弾く。validation.TruncateFirstText は20ルーン以下の入力を
	// そのまま返すため、通すと messages.content と conversations.first_text の
	// INSERT が encoding エラーで落ちる（21ルーン以上なら []rune 変換で U+FFFD に
	// 置換されるという非対称があり、短い入力ほど危険）。
	if !utf8.ValidString(req.Text) {
		respondError(w, http.StatusBadRequest, "入力に不正な文字が含まれています")
		return
	}
	if !validation.IsValidInput(req.Text) {
		respondError(w, http.StatusBadRequest, "発話内容を入力してください")
		return
	}

	exists, err := h.convRepo.Exists(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "会話の確認に失敗しました")
		return
	}
	if !exists {
		respondError(w, http.StatusNotFound, "会話が見つかりません")
		return
	}

	// r.Context() をそのまま渡すと 202 を返した時点でキャンセルされ、応答生成が即死する。
	// 親のキャンセルを切り離し、独自のタイムアウトを被せる。
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), responseTimeout)
	go func() {
		defer cancel()
		if err := h.chat.HandleUserMessage(ctx, id, req.RequestID, req.Text); err != nil {
			log.Printf("応答生成に失敗: conversation_id=%s request_id=%s: %v", id, req.RequestID, err)
		}
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{"request_id": req.RequestID})
}
