package handler

import (
	"log"
	"net/http"

	"zuncha/internal/sse"
	"zuncha/internal/validation"
)

// Events は GET /conversations/{id}/events を処理する（SSE 常設チャネル）。
//
// このハンドラは r.Context() がキャンセルされるまで戻らない。ハンドラが return すると
// HTTP レスポンスが閉じて SSE 接続が切れるため、Run を同 goroutine でブロックさせる
// （これにより ResponseWriter へ書く goroutine が1本という不変条件も保たれる）。
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validation.IsValidULID(id) {
		respondError(w, http.StatusBadRequest, "会話IDの形式が不正です")
		return
	}

	exists, err := h.convRepo.Exists(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "会話の確認に失敗しました")
		return
	}
	if !exists {
		// 存在しない会話へ延々と接続させない。
		respondError(w, http.StatusNotFound, "会話が見つかりません")
		return
	}

	// ヘッダの書き出しはここから始まるため、エラー応答は必ずこの前に済ませる。
	conn, err := sse.NewHTTPConn(w)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "ストリーミングに対応していません")
		return
	}

	// 解除を忘れると接続が Hub に残り続け、メモリと配信コストが積む。
	unregister := h.hub.Register(id, conn)
	defer unregister()

	log.Printf("SSE接続を開始: conversation_id=%s", id)

	// Run を同 goroutine でブロック呼び出しする。
	// ctx キャンセルでも書き込み失敗でも Run は戻り、戻った直後に defer unregister() が
	// 走るため、half-open 接続でハンドラが永久ブロックすることはない。
	// `go conn.Run(ctx)` + `select { <-r.Context().Done(); <-conn.Done() }` にすると、
	// ctx 側で起きたときに Run がまだ ResponseWriter を触っている最中にハンドラが
	// return し得る（ハンドラ return 後の ResponseWriter 使用は禁止）。
	conn.Run(r.Context())
}
