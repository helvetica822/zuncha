package handler

import (
	"errors"
	"net/http"

	"zuncha/internal/repository"
)

// GetAudio は GET /audio/{id} を処理する。
func (h *Handler) GetAudio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := h.fetchAudio.FetchAudio(r.Context(), id)
	if errors.Is(err, repository.ErrAudioNotFound) {
		respondError(w, http.StatusNotFound, "音声が見つかりません")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "音声の取得に失敗しました")
		return
	}
	w.Header().Set("Content-Type", contentTypeWAV)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
