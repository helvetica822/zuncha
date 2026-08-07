package tts

import "context"

// TTSClient はテキストから音声を合成し URL を返す。
type TTSClient interface {
	Synthesize(ctx context.Context, text string) (string, error)
}
