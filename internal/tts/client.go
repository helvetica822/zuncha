package tts

import "context"

// TTSClient はテキストから音声を合成し URL を返す。
//
// conversationID/messageID は合成結果を audio_files へ登録する際の紐付けに使う。
// これらを引数で受けるのは、TTS 実装が「保存先レコードのキー」を自前で決められないため
// （docs/04_implementation/04_realtime_wiring_design.md D-4 訂正1）。
// 引数が増えても実装差し替え可能性（NF-MAINT-01）は損なわれない。
type TTSClient interface {
	Synthesize(ctx context.Context, text, conversationID, messageID string) (string, error)
}
