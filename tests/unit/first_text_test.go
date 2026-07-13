// 対応仕様: docs/03_unit_test/04_test_specification.md 4.2（観点1-2、TC-1-2-01〜11）
package unit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"zuncha/internal/validation"
)

func TestTruncateFirstText(t *testing.T) {
	family := "👨‍👩‍👧‍👦" // ZWJ結合の家族絵文字（👨, ZWJ, 👩, ZWJ, 👧, ZWJ, 👦の7ルーン）
	familyFiller := strings.Repeat("あ", 17)
	familyInput := familyFiller + family
	familyExpectedRunes := append([]rune(familyFiller), []rune(family)[:3]...)
	familyExpected := string(familyExpectedRunes)

	mixedInput := "あ1😀い2う3え4お5か6き7く8け9こ0さ1"
	mixedExpected := "あ1😀い2う3え4お5か6き7く8け9こ"

	controlInput := "あ\nい\tう" + strings.Repeat("え", 18)
	controlExpected := "あ\nい\tう" + strings.Repeat("え", 15)

	spacedInput := " " + strings.Repeat("あ", 25) + " "
	spacedExpected := " " + strings.Repeat("あ", 19)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"TC-1-2-01_20文字以下はそのまま返す", "こんにちは", "こんにちは"},
		{"TC-1-2-02_ちょうど20文字はカットなしで返す", strings.Repeat("あ", 20), strings.Repeat("あ", 20)},
		{"TC-1-2-03_21文字は先頭20文字にカットされる", strings.Repeat("あ", 21), strings.Repeat("あ", 20)},
		{"TC-1-2-04_マルチバイト混在でも文字境界を壊さない", mixedInput, mixedExpected},
		{"TC-1-2-05_空文字列は空文字列を返す", "", ""},
		{"TC-1-2-06_結合絵文字はコードポイント単位でカットされる", familyInput, familyExpected},
		{"TC-1-2-07_19文字はカットなしで返す", strings.Repeat("あ", 19), strings.Repeat("あ", 19)},
		{"TC-1-2-08_1文字は無変換で返す", "あ", "あ"},
		{"TC-1-2-09_1000文字は20文字にカットされる", strings.Repeat("あ", 1000), strings.Repeat("あ", 20)},
		{"TC-1-2-10_制御文字を含んでもサニタイズせずカットする", controlInput, controlExpected},
		{"TC-1-2-11_前後空白をtrimせずカットする", spacedInput, spacedExpected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validation.TruncateFirstText(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
