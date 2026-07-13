// 対応仕様: docs/03_unit_test/04_test_specification.md 4.1（観点1-1、TC-1-1-01〜21）
package unit

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"

	"zuncha/internal/validation"
)

func TestIsValidULID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"TC-1-1-01_有効な26文字ULIDでtrueを返す", "01ARZ3NDEKTSV4RRFFQ69G5FAV", true},
		{"TC-1-1-03_25文字1文字不足でfalseを返す", "01ARZ3NDEKTSV4RRFFQ69G5FA", false},
		{"TC-1-1-04_27文字1文字超過でfalseを返す", "01ARZ3NDEKTSV4RRFFQ69G5FAVX", false},
		{"TC-1-1-05_除外文字Iを含む場合falseを返す", "01ARZ3NDEKTSV4RRFFQ6IG5FAV", false},
		{"TC-1-1-06_除外文字Lを含む場合falseを返す", "01ARZ3NDEKTSV4RRFFQ6LG5FAV", false},
		{"TC-1-1-07_除外文字Oを含む場合falseを返す", "01ARZ3NDEKTSV4RRFFQ6OG5FAV", false},
		{"TC-1-1-08_除外文字Uを含む場合falseを返す", "01ARZ3NDEKTSV4RRFFQ6UG5FAV", false},
		{"TC-1-1-09_空文字列でfalseを返す", "", false},
		{"TC-1-1-10_UUID形式でfalseを返す", "550e8400-e29b-41d4-a716-446655440000", false},
		{"TC-1-1-11_隣接許容文字Hでtrueを返す", "01ARZ3NDEKTSV4RRFFQ6HG5FAV", true},
		{"TC-1-1-12_隣接許容文字Jでtrueを返す", "01ARZ3NDEKTSV4RRFFQ6JG5FAV", true},
		{"TC-1-1-13_隣接許容文字Kでtrueを返す", "01ARZ3NDEKTSV4RRFFQ6KG5FAV", true},
		{"TC-1-1-14_隣接許容文字Mでtrueを返す", "01ARZ3NDEKTSV4RRFFQ6MG5FAV", true},
		{"TC-1-1-15_隣接許容文字Tでtrueを返す", "01ARZ3NDEKTSV4RRFFQ6TG5FAV", true},
		{"TC-1-1-16_隣接許容文字Vでtrueを返す", "01ARZ3NDEKTSV4RRFFQ6VG5FAV", true},
		{"TC-1-1-17_先頭文字が値域外でも文字種桁数を満たせばtrueを返す", "ZZZZZZZZZZZZZZZZZZZZZZZZZZ", true},
		{"TC-1-1-18_前後空白付きULIDはfalseを返す", " 01ARZ3NDEKTSV4RRFFQ69G5FAV ", false},
		{"TC-1-1-19_小文字ULIDはfalseを返す", "01arz3ndektsv4rrffq69g5fav", false},
		{"TC-1-1-20_全角文字混入でfalseを返す", "01ARZ3NDEKTSV4RRFFQ6ＡG5FAV", false},
		{"TC-1-1-21_絵文字混入でfalseを返す", "01ARZ3NDEKTSV4RRFFQ6\U0001F600G5FAV", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validation.IsValidULID(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidULID_生成ULIDの往復整合性(t *testing.T) {
	// TC-1-1-02: 実際の採番関数が生成したULIDを本バリデータに通すと必ずtrueになる
	for i := 0; i < 100; i++ {
		id := ulid.Make()
		assert.True(t, validation.IsValidULID(id.String()))
	}
}
