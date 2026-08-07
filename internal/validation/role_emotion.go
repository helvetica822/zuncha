package validation

import (
	"errors"
	"fmt"
)

const (
	roleUser      = "user"
	roleAssistant = "assistant"
)

// FallbackEmotion は emotion が7種外/空のときに代替する既定の感情ラベル。
// この値は必ず validEmotions に含まれる（下のマップキーで再利用している）。
const FallbackEmotion = "困惑"

var validEmotions = map[string]struct{}{
	"喜び": {}, "怒り": {}, "悲しみ": {}, "楽しい": {},
	"照れ": {}, FallbackEmotion: {}, "ドヤ顔": {},
}

// ValidateRole は role が 'user'/'assistant'（大文字小文字厳密一致）か検証する。
func ValidateRole(role string) error {
	if role == roleUser || role == roleAssistant {
		return nil
	}
	return fmt.Errorf("invalid role: %q", role)
}

// ValidateEmotion は emotion（nil可）が7種いずれかに完全一致するか検証する。
func ValidateEmotion(emotion *string) error {
	if emotion == nil {
		return nil
	}
	if _, ok := validEmotions[*emotion]; ok {
		return nil
	}
	return fmt.Errorf("invalid emotion: %q", *emotion)
}

// ValidateRoleEmotionConsistency は user に emotion が付く矛盾を検出する。
func ValidateRoleEmotionConsistency(role string, emotion *string) error {
	if role == roleUser && emotion != nil {
		return errors.New("user role must not have emotion")
	}
	return nil
}
