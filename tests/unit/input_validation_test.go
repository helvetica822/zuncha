// 対応仕様: docs/03_unit_test/04_test_specification.md 4.4（観点1-4、TC-1-4-01〜14）
package unit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"zuncha/internal/validation"
)

const zeroWidthSpace = "\u200B" // ZERO WIDTH SPACE（標準trimでは除去されない）
const fullWidthSpace = "\u3000" // IDEOGRAPHIC SPACE（全角スペース）

func TestIsValidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"TC-1-4-01_通常テキストはtrueを返す", "こんにちは", true},
		{"TC-1-4-02_前後空白があっても中身があればtrueを返す", " こんにちは ", true},
		{"TC-1-4-03_空文字列はfalseを返す", "", false},
		{"TC-1-4-04_半角スペースのみはfalseを返す", "   ", false},
		{"TC-1-4-05_タブ改行のみはfalseを返す", "\t\n", false},
		{"TC-1-4-06_全角スペースのみはfalseを返す", strings.Repeat(fullWidthSpace, 3), false},
		{"TC-1-4-07_ゼロ幅スペースのみはtrueを返す", zeroWidthSpace, true},
		{"TC-1-4-08_ゼロ幅と半角スペース混在はtrueを返す", " " + zeroWidthSpace + " ", true},
		{"TC-1-4-09_1文字の非空白文字はtrueを返す", "あ", true},
		{"TC-1-4-10_1文字の空白文字はfalseを返す", " ", false},
		{"TC-1-4-11_空白に挟まれた非空白はtrueを返す", " a ", true},
		{"TC-1-4-12_極端に長い空白のみはfalseを返す", strings.Repeat(" ", 1000), false},
		{"TC-1-4-13_絵文字のみはtrueを返す", "😀", true},
		{"TC-1-4-14_ゼロ幅スペースと実文字混在はtrueを返す", "こんにちは" + zeroWidthSpace, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validation.IsValidInput(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
