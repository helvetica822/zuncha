package service

import (
	"context"
	"fmt"

	"zuncha/internal/stt"
)

// AudioConverter は録音データを whisper-server が受け付ける形式へ変換する。
type AudioConverter interface {
	Convert(ctx context.Context, input []byte) ([]byte, error)
}

// STTClient は変換済み WAV を音声認識にかける。
type STTClient interface {
	Transcribe(ctx context.Context, wav []byte) (stt.STTResult, error)
}

// SpeechToTextService は「音声形式変換 → 音声認識」のオーケストレーションを担う。
type SpeechToTextService struct {
	converter AudioConverter
	client    STTClient
}

func NewSpeechToTextService(converter AudioConverter, client STTClient) *SpeechToTextService {
	return &SpeechToTextService{converter: converter, client: client}
}

// Transcribe は録音データを変換してから認識し、結果を返す。
//
// 認識失敗(空文字・信頼度不足)の判定はここでは行わない。
// stt.IsRecognitionFailed はハンドラ層が使い、200 {"failed":true} へ落とす
// (認識失敗はエラーではなく正常系の一部)。
func (s *SpeechToTextService) Transcribe(ctx context.Context, rawAudio []byte) (stt.STTResult, error) {
	wav, err := s.converter.Convert(ctx, rawAudio)
	if err != nil {
		return stt.STTResult{}, fmt.Errorf("音声形式の変換に失敗しました: %w", err)
	}

	result, err := s.client.Transcribe(ctx, wav)
	if err != nil {
		return stt.STTResult{}, fmt.Errorf("音声認識に失敗しました: %w", err)
	}
	return result, nil
}
