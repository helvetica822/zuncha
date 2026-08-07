package handler

import "net/http"

// CreateConversation は POST /conversations を処理する。
func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	conv, err := h.createConv.CreateConversation(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "会話の作成に失敗しました")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"id": conv.ID})
}
