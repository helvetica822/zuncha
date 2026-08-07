// Package handler は zuncha API の HTTP ハンドラを提供する。
//
// SSE ハンドラを main.go に置くと httptest で単体検証できないため、
// ハンドラ層をパッケージとして切り出している。
package handler

import (
	"net/http"

	"zuncha/internal/repository"
	"zuncha/internal/service"
	"zuncha/internal/sse"
)

// Handler はハンドラが依存するサービスを保持する。
type Handler struct {
	createConv *service.CreateConversationService
	fetchAudio *service.FetchAudioService
	chat       *service.ChatService
	convRepo   repository.ConversationRepository
	hub        *sse.Hub
}

func NewHandler(
	createConv *service.CreateConversationService,
	fetchAudio *service.FetchAudioService,
	chat *service.ChatService,
	convRepo repository.ConversationRepository,
	hub *sse.Hub,
) *Handler {
	return &Handler{
		createConv: createConv,
		fetchAudio: fetchAudio,
		chat:       chat,
		convRepo:   convRepo,
		hub:        hub,
	}
}

// Routes は全エンドポイントを登録した ServeMux を返す。
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /conversations", h.CreateConversation)
	mux.HandleFunc("GET /audio/{id}", h.GetAudio)
	mux.HandleFunc("GET /conversations/{id}/events", h.Events)
	mux.HandleFunc("POST /conversations/{id}/messages", h.PostMessage)
	return mux
}
