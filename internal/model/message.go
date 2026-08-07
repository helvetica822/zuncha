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
