// 対応仕様: docs/03_unit_test/04_test_specification.md 4.3（観点1-3、TC-1-3-R/E/C）
package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"zuncha/internal/validation"
)

func strPtr(s string) *string {
	return &s
}

func TestValidateRole(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		wantErr bool
	}{
		{"TC-1-3-R01_userはvalidでnilを返す", "user", false},
		{"TC-1-3-R02_assistantはvalidでnilを返す", "assistant", false},
		{"TC-1-3-R03_CHECK制約外のroleはエラーを返す", "admin", true},
		{"TC-1-3-R04_空文字列roleはエラーを返す", "", true},
		{"TC-1-3-R05_大文字小文字違いUserはエラーを返す", "User", true},
		{"TC-1-3-R06_大文字小文字違いASSISTANTはエラーを返す", "ASSISTANT", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateRole(tt.role)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEmotion(t *testing.T) {
	tests := []struct {
		name    string
		emotion *string
		wantErr bool
	}{
		{"TC-1-3-E01_emotionがnilはvalidでnilを返す", nil, false},
		{"TC-1-3-E02_喜びはvalidでnilを返す", strPtr("喜び"), false},
		{"TC-1-3-E03_怒りはvalidでnilを返す", strPtr("怒り"), false},
		{"TC-1-3-E04_悲しみはvalidでnilを返す", strPtr("悲しみ"), false},
		{"TC-1-3-E05_楽しいはvalidでnilを返す", strPtr("楽しい"), false},
		{"TC-1-3-E06_照れはvalidでnilを返す", strPtr("照れ"), false},
		{"TC-1-3-E07_困惑はvalidでnilを返す", strPtr("困惑"), false},
		{"TC-1-3-E08_ドヤ顔はvalidでnilを返す", strPtr("ドヤ顔"), false},
		{"TC-1-3-E09_7種にない値はエラーを返す", strPtr("普通"), true},
		{"TC-1-3-E10_ローマ字表記はエラーを返す", strPtr("happy"), true},
		{"TC-1-3-E11_前後空白付きはエラーを返す", strPtr(" 喜び "), true},
		{"TC-1-3-E12_空文字列はエラーを返す", strPtr(""), true},
		{"TC-1-3-E13_部分一致1文字はエラーを返す", strPtr("喜"), true},
		{"TC-1-3-E14_前方一致はエラーを返す", strPtr("喜びだ"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateEmotion(tt.emotion)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRoleEmotionConsistency(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		emotion *string
		wantErr bool
	}{
		{"TC-1-3-C01_userとnilの組み合わせはvalid", "user", nil, false},
		{"TC-1-3-C02_assistantと喜びの組み合わせはvalid", "assistant", strPtr("喜び"), false},
		{"TC-1-3-C03_assistantとnilの組み合わせもvalid", "assistant", nil, false},
		{"TC-1-3-C04_userと喜びの組み合わせは矛盾でエラーを返す", "user", strPtr("喜び"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateRoleEmotionConsistency(tt.role, tt.emotion)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
