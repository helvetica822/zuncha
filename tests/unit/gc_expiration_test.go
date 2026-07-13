// 対応仕様: docs/03_unit_test/04_test_specification.md 4.5（観点1-5、TC-1-5-01〜07）
package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"zuncha/internal/gc"
)

func TestIsExpired(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		now       time.Time
		want      bool
	}{
		{"TC-1-5-01_1日前はGC対象trueを返す", now.Add(-24 * time.Hour), now, true},
		{"TC-1-5-02_1日後はGC対象外falseを返す", now.Add(24 * time.Hour), now, false},
		{"TC-1-5-03_ゼロ値はGC対象trueを返す", time.Time{}, now, true},
		{"TC-1-5-04_完全一致は等号非対称性によりfalseを返す", now, now, false},
		{"TC-1-5-05_1ナノ秒前はtrueを返す", now.Add(-1 * time.Nanosecond), now, true},
		{"TC-1-5-06_1ナノ秒後はfalseを返す", now.Add(1 * time.Nanosecond), now, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gc.IsExpired(tt.expiresAt, tt.now)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsExpired_タイムゾーン相違(t *testing.T) {
	// TC-1-5-07: expiresAtをUTCで、nowを同一絶対時刻のJSTで指定してもfalseを返す
	jst := time.FixedZone("JST", 9*60*60)
	expiresAtUTC := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	nowJST := time.Date(2026, 7, 8, 21, 0, 0, 0, jst) // UTC正午と同一絶対時刻

	got := gc.IsExpired(expiresAtUTC, nowJST)

	assert.False(t, got)
}
