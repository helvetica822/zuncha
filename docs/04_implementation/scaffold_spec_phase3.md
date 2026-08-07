# Phase3 実装仕様 (T-12 LLMパース / T-15 ResponseStreamer)

- 作成者: 四国めたん (テックリード)
- 対象: `internal/llm/parser.go`(T-12)、`internal/service/response_streamer.go`(T-15)。#24 のstubを本体へ置換する。
- 根拠: `tests/unit/parse_llm_response_test.go`(25ケース)、`tests/unit/response_streamer_test.go`(15ケース)を精読して抽出した行動契約。

---

## T-12: internal/llm/parser.go

エラー3分類の切り分けが肝。**構文(ErrSyntax)** = JSONとして不正/空/Markdown/`null`等でobjectにならない。**スキーマ(ErrSchema)** = objectだが必須キー(text/emotion)欠落。**値(ErrValue)** = キーはあるが型不正(text:null、emotion:数値、null等の非文字列)。emotionの7種外・空文字列は**エラーにせず「困惑」フォールバック**。

```go
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"

	"zuncha/internal/validation"
)

const fallbackEmotion = "困惑"

// ParseLLMResponse は LLM の生JSONをパースする。
// 構文エラー→ErrSyntax、必須キー欠落→ErrSchema、型不正→ErrValue。
// emotionが7種外/空文字列のときは「困惑」にフォールバックする。
func ParseLLMResponse(body []byte) (*LLMResponse, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSyntax, err)
	}
	if raw == nil {
		// body が JSON null 等でオブジェクトにならないケース
		return nil, fmt.Errorf("%w: JSONオブジェクトではありません", ErrSchema)
	}

	textRaw, ok := raw["text"]
	if !ok {
		return nil, fmt.Errorf("%w: textキーがありません", ErrSchema)
	}
	emotionRaw, ok := raw["emotion"]
	if !ok {
		return nil, fmt.Errorf("%w: emotionキーがありません", ErrSchema)
	}

	// text: null は値エラー（json.Unmarshalはnull→""で無エラーのため明示判定が必要）
	if isJSONNull(textRaw) {
		return nil, fmt.Errorf("%w: textがnullです", ErrValue)
	}
	var text string
	if err := json.Unmarshal(textRaw, &text); err != nil {
		return nil, fmt.Errorf("%w: textが文字列ではありません", ErrValue)
	}

	if isJSONNull(emotionRaw) {
		return nil, fmt.Errorf("%w: emotionがnullです", ErrValue)
	}
	var emotion string
	if err := json.Unmarshal(emotionRaw, &emotion); err != nil {
		return nil, fmt.Errorf("%w: emotionが文字列ではありません", ErrValue)
	}

	// 7種外・空文字列は「困惑」へフォールバック（validation を再利用）。
	if validation.ValidateEmotion(&emotion) != nil {
		emotion = fallbackEmotion
	}

	return &LLMResponse{Text: text, Emotion: emotion}, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
```

**ケース対応の根拠**:
- TC-01/09/10/21/23: 正常(extra無視/text空OK/長大/マルチバイト)。TC-02〜08: 7種emotion保持。
- TC-11/17/20: 構文不正/空body/Markdownブロック → json.Unmarshalが失敗 → ErrSyntax。
- TC-18(`null`): raw==nil → ErrSchema(Error判定のみ要求されるので分類は任意)。
- TC-12/13/19: textまたはemotionキー欠落/空オブジェクト → ErrSchema。
- TC-14/22: emotion "普通"/"" → ValidateEmotionがエラー → 困惑フォールバック(成功)。
- TC-15: emotion 数値 → Unmarshal失敗 → ErrValue。TC-16: text null → isJSONNull → ErrValue。
- TC-24/25: ErrSyntax/ErrSchema/ErrValueは#24で独立errors.New。`%w`ラップで`errors.Is`が各センチネルにのみ真。

---

## T-15: internal/service/response_streamer.go

送出順: **SendEmotion → SendTextChunk×N → (TTS成功時のみ)SendAudioURL → SendDone**。
- **上流失敗(LLM生成/Parse)・防御的emotion検証失敗・EventSink書込失敗** は致命的 → `SendError`へ切替え、後続を送出せず`error`を返して中断。
- **TTS(Synthesize)失敗のみ非致命** → SendAudioURLをスキップし、SendDoneで正常終了(errは返さない、SendErrorも呼ばない)。

```go
package service

import (
	"context"
	"fmt"

	"zuncha/internal/llm"
	"zuncha/internal/sse"
	"zuncha/internal/tts"
	"zuncha/internal/validation"
)

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
```

**ケース対応の根拠**:
- TC-01/03/09/10/14: 正常送出順・引数・SendDone1回。TC-02/04/05: チャンク数(1/3/100)保持しdoneより前。
- TC-06/13: LLM失敗 → Parse呼ばず SendErrorのみ、他イベント0。
- TC-07/11(TTS失敗)/15: Synthesize失敗 → SendAudioURLスキップ・SendDone送出・SendError無し・err無し。
- TC-08: emotion不正(モック注入) → SendEmotion前に中断、SendError1、error。
- TC-12: SendTextChunk(c2)書込失敗 → SendError、c3送出せず、audio/done無し、SendTextChunk2回。
- TC-15: 全フロー(正常/TTS失敗/LLM失敗)の最終イベントがSendDoneまたはSendError。

**注意**: `Synthesize`は**全文`resp.Text`**を渡す(チャンク単位ではない。TC-01/03の`Synthesize(_, "こんにちはなのだ"/"こんにちは")`が根拠)。チャンクはSSE表示用の分割。

---

## 完了検証(両タスク共通)
`export PATH="$HOME/.local/bin:$PATH"` 後:
1. `gofmt -l internal` 差分なし、`go vet ./internal/...` 成功。
2. `go test ./tests/unit/ -run 'TestParseLLMResponse|TestResponseStreamer' -v` → 全緑。
3. `go test ./tests/unit/... 2>&1 | tail -5` → FAIL数がさらに減少。
in_progress維持で検証ログ付き報告(そら復帰後レビュー→completed)。
