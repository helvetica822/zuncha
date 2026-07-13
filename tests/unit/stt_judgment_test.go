// 対応仕様: docs/03_unit_test/11_test_specification.md 4.3（観点3-3、TC-3-3-R01〜07、T01〜05）
package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"zuncha/internal/stt"
)

func TestIsRecognitionFailed(t *testing.T) {
	// 信頼度閾値0.5は7章P決定による暫定値であり、将来調整される可能性がある（TC-3-3-R05〜R07）。
	tests := []struct {
		name   string
		result stt.STTResult
		want   bool
	}{
		{"TC-3-3-R01_閾値以上の信頼度でfalseを返す", stt.STTResult{Text: "こんにちは", Confidence: 0.8}, false},
		{"TC-3-3-R02_空文字列はtrueを返す", stt.STTResult{Text: "", Confidence: 0.8}, true},
		{"TC-3-3-R03_閾値未満の信頼度はtrueを返す", stt.STTResult{Text: "こんにちは", Confidence: 0.3}, true},
		{"TC-3-3-R04_空白のみのテキストはtrueを返す", stt.STTResult{Text: "   ", Confidence: 0.8}, true},
		{"TC-3-3-R05_信頼度がちょうど閾値でfalseを返す", stt.STTResult{Text: "こんにちは", Confidence: 0.5}, false},
		{"TC-3-3-R06_閾値のわずかに下でtrueを返す", stt.STTResult{Text: "こんにちは", Confidence: 0.49}, true},
		{"TC-3-3-R07_閾値のわずかに上でfalseを返す", stt.STTResult{Text: "こんにちは", Confidence: 0.51}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stt.IsRecognitionFailed(tt.result)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsTimedOut(t *testing.T) {
	silenceStart := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	const threshold = 8 * time.Second

	tests := []struct {
		name    string
		elapsed time.Duration
		want    bool
	}{
		{"TC-3-3-T01_5秒経過はfalseを返す", 5 * time.Second, false},
		{"TC-3-3-T02_9秒経過はtrueを返す", 9 * time.Second, true},
		{"TC-3-3-T03_ちょうど8秒でtrueを返す", 8 * time.Second, true},
		{"TC-3-3-T04_7点9秒でfalseを返す", 7900 * time.Millisecond, false},
		{"TC-3-3-T05_8点1秒でtrueを返す", 8100 * time.Millisecond, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := silenceStart.Add(tt.elapsed)

			got := stt.IsTimedOut(silenceStart, now, threshold)

			assert.Equal(t, tt.want, got)
		})
	}
}
