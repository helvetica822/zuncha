package llm

import (
	"bytes"
	"encoding/json"
	"fmt"

	"zuncha/internal/validation"
)

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

	text, err := parseRequiredString(textRaw, "text")
	if err != nil {
		return nil, err
	}
	emotion, err := parseRequiredString(emotionRaw, "emotion")
	if err != nil {
		return nil, err
	}

	// 7種外・空文字列は既定値へフォールバック（validation を真実の源泉として参照）。
	if validation.ValidateEmotion(&emotion) != nil {
		emotion = validation.FallbackEmotion
	}

	return &LLMResponse{Text: text, Emotion: emotion}, nil
}

// parseRequiredString は必須の文字列フィールドを取り出す。
// null は値エラー（json.Unmarshalはnull→""で無エラーのため明示判定が必要）、
// 非文字列も値エラーとして field 名付きで返す。
func parseRequiredString(raw json.RawMessage, field string) (string, error) {
	if isJSONNull(raw) {
		return "", fmt.Errorf("%w: %sがnullです", ErrValue, field)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("%w: %sが文字列ではありません", ErrValue, field)
	}
	return s, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
