package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// loadConfig の VOICEVOX_BASE_URL の解決（未設定時のデフォルト補完）を固定する。
// この値は voicevox.NewClient へ「渡すだけ」の設定値で、壊れても起動は成功し、
// TTS 失敗は非致命なので「文字は出るが一生無音」のまま気づけない
// （lessons.md 2026-08-10「外部ライブラリに渡すだけの設定値」判定基準1に該当）。
func TestLoadConfig(t *testing.T) {
	t.Run("VOICEVOX_BASE_URL未設定時はデフォルトURLが使われる", func(t *testing.T) {
		// t.Setenv でテスト終了後の復元を登録してから Unsetenv し、
		// 「空文字列」ではなく「真に未設定」の状態を作る。
		t.Setenv(envVoicevoxBaseURL, "placeholder")
		require.NoError(t, os.Unsetenv(envVoicevoxBaseURL))

		cfg := loadConfig()

		assert.Equal(t, defaultVoicevoxBaseURL, cfg.voicevoxBaseURL)
		// 定数側が書き換わったときに気づけるよう、期待値そのものも直接固定する。
		assert.Equal(t, "http://localhost:50021", cfg.voicevoxBaseURL)
	})

	t.Run("VOICEVOX_BASE_URLが空文字列でもデフォルトURLが使われる", func(t *testing.T) {
		t.Setenv(envVoicevoxBaseURL, "")

		cfg := loadConfig()

		assert.Equal(t, "http://localhost:50021", cfg.voicevoxBaseURL)
	})

	t.Run("VOICEVOX_BASE_URL設定時はその値が優先される", func(t *testing.T) {
		// W-11 の Compose ではサービス名で上書きする想定（main.go のコメント）。
		t.Setenv(envVoicevoxBaseURL, "http://voicevox:50021")

		cfg := loadConfig()

		assert.Equal(t, "http://voicevox:50021", cfg.voicevoxBaseURL)
		assert.NotEqual(t, defaultVoicevoxBaseURL, cfg.voicevoxBaseURL)
	})
}
