package service

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"

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

// CreateConversation は GC を実行後（失敗は握り潰す）、新規 ULID の会話を作成する。
func (s *CreateConversationService) CreateConversation(ctx context.Context) (*model.Conversation, error) {
	now := time.Now()
	// GC は付随処理。失敗しても会話作成は継続する（TC-2-1-04）。
	_, _ = s.repo.GC(ctx, now)

	conv := &model.Conversation{ID: ulid.Make().String()}
	if err := s.repo.InsertConversation(ctx, conv); err != nil {
		return nil, err
	}
	return conv, nil
}
