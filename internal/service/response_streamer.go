package service

import (
	"context"
	"fmt"

	"zuncha/internal/llm"
	"zuncha/internal/sse"
	"zuncha/internal/tts"
	"zuncha/internal/validation"
)

// ResponseStreamer は LLM 生成〜SSE 配信のオーケストレーションを担う。
type ResponseStreamer struct {
	llmClient llm.LLMClient
	parser    llm.ResponseParser
	ttsClient tts.TTSClient
	chunker   sse.TextChunker
}

func NewResponseStreamer(
	llmClient llm.LLMClient,
	parser llm.ResponseParser,
	ttsClient tts.TTSClient,
	chunker sse.TextChunker,
) *ResponseStreamer {
	return &ResponseStreamer{llmClient: llmClient, parser: parser, ttsClient: ttsClient, chunker: chunker}
}

// StreamResponse は prompt への応答を生成し sink へ順に配信する。
// 各致命ステップの失敗は SendError に切替えて中断する。TTS失敗のみスキップして続行する。
func (s *ResponseStreamer) StreamResponse(ctx context.Context, sink sse.EventSink, prompt string) error {
	raw, err := s.llmClient.GenerateResponse(ctx, prompt)
	if err != nil {
		return s.fail(sink, fmt.Errorf("llm generate: %w", err))
	}

	resp, err := s.parser.Parse(raw)
	if err != nil {
		return s.fail(sink, fmt.Errorf("parse: %w", err))
	}

	// 防御的 emotion 検証（不正なら SendEmotion せず中断）。
	if err := validation.ValidateEmotion(&resp.Emotion); err != nil {
		return s.fail(sink, fmt.Errorf("invalid emotion: %w", err))
	}

	if err := sink.SendEmotion(resp.Emotion); err != nil {
		return s.fail(sink, fmt.Errorf("send emotion: %w", err))
	}

	for _, chunk := range s.chunker.Chunk(resp.Text) {
		if err := sink.SendTextChunk(chunk); err != nil {
			return s.fail(sink, fmt.Errorf("send text chunk: %w", err))
		}
	}

	// TTS は失敗しても致命的にしない（audio_url をスキップして done へ）。
	if url, ttsErr := s.ttsClient.Synthesize(ctx, resp.Text); ttsErr == nil {
		if err := sink.SendAudioURL(url); err != nil {
			return s.fail(sink, fmt.Errorf("send audio url: %w", err))
		}
	}

	if err := sink.SendDone(); err != nil {
		return s.fail(sink, fmt.Errorf("send done: %w", err))
	}
	return nil
}

// fail は SendError を送出してから err を返す（中断用ヘルパ）。
func (s *ResponseStreamer) fail(sink sse.EventSink, err error) error {
	_ = sink.SendError(err.Error())
	return err
}
