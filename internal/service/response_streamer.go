package service

import (
	"context"
	"fmt"

	"zuncha/internal/llm"
	"zuncha/internal/sse"
	"zuncha/internal/tts"
	"zuncha/internal/validation"
)

// error イベントで利用者に見せる文言（仕様書§2.2）。ブラウザのトーストにそのまま出るため、
// 内部エラー文字列（"llm generate: ..." 等）は載せない。中断理由を細分化しても利用者の
// 取れる行動は変わらないので、全ステップ同一の1文にする。
const errMsgGenerateResponse = "応答の生成に失敗しました"

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
//
// conversationID/messageID は TTS へ素通しする（audio_files への登録に必要。
// docs/04_implementation/04_realtime_wiring_design.md D-4 訂正1）。messageID は
// 呼び出し側が事前採番した assistant メッセージのID。
func (s *ResponseStreamer) StreamResponse(
	ctx context.Context,
	sink sse.EventSink,
	prompt, conversationID, messageID string,
) error {
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
	// 読み上げは応答全文（チャンクではない。申し送り B1-1）。
	if url, ttsErr := s.ttsClient.Synthesize(ctx, resp.Text, conversationID, messageID); ttsErr == nil {
		if err := sink.SendAudioURL(url); err != nil {
			return s.fail(sink, fmt.Errorf("send audio url: %w", err))
		}
	}

	if err := sink.SendDone(); err != nil {
		return s.fail(sink, fmt.Errorf("send done: %w", err))
	}
	return nil
}

// fail は利用者向け文言で SendError を送出してから err を返す（中断用ヘルパ）。
// err は呼び出し側・ログ用にそのまま返し、利用者には定数の文言だけを見せる。
func (s *ResponseStreamer) fail(sink sse.EventSink, err error) error {
	_ = sink.SendError(errMsgGenerateResponse)
	return err
}
