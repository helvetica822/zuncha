package repository

import "zuncha/internal/model"

// ReverseMessages は messages を逆順にした新しいスライスを返す（空入力でも非 nil）。
func ReverseMessages(messages []model.Message) []model.Message {
	reversed := make([]model.Message, len(messages))
	for i, m := range messages {
		reversed[len(messages)-1-i] = m
	}
	return reversed
}
