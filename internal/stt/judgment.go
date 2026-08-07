package stt

import (
	"time"

	"zuncha/internal/validation"
)

// STTConfidenceThreshold は認識成功とみなす信頼度の下限（7章P暫定値）。
const STTConfidenceThreshold = 0.5

// IsRecognitionFailed は認識失敗（空/空白のみ/信頼度不足）か判定する。
// テキストの空判定は validation.IsValidInput を再利用する。
func IsRecognitionFailed(result STTResult) bool {
	if !validation.IsValidInput(result.Text) {
		return true
	}
	return result.Confidence < STTConfidenceThreshold
}

// IsTimedOut は無音開始から threshold 以上経過したか判定する（等号は経過扱い=true）。
func IsTimedOut(silenceStart time.Time, now time.Time, threshold time.Duration) bool {
	return now.Sub(silenceStart) >= threshold
}
