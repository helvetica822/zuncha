package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseAllowedOrigins の境界（空・空要素・前後空白）を検証する。
// 未設定は os.Getenv が "" を返すため、空文字列ケースで同時にカバーされる。
func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"空文字列は空スライス", "", []string{}},
		{"空白のみは空スライス", "   ", []string{}},
		{"カンマのみは空スライス", ",,", []string{}},
		{"単一要素はそのまま", "https://a.example.com", []string{"https://a.example.com"}},
		{"空要素は無視される", "a,,b", []string{"a", "b"}},
		{"前後空白はtrimされる", " a , b ", []string{"a", "b"}},
		{"空白と空要素の混在", " a , , b , ", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAllowedOrigins(tt.raw)
			assert.Equal(t, tt.want, got)
			assert.Len(t, got, len(tt.want))
		})
	}
}
