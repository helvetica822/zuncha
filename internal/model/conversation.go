package model

import "time"

// Conversation は会話セッションを表すドメインモデル。
type Conversation struct {
	ID        string
	StartedAt time.Time
	ExpiresAt time.Time
	FirstText *string
}
