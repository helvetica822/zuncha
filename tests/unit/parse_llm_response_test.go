// 対応仕様: docs/03_unit_test/11_test_specification.md 4.1（観点3-1、TC-3-1-01〜25）
package unit

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/llm"
)

func TestParseLLMResponse(t *testing.T) {
	longText := strings.Repeat("あ", 10000)

	tests := []struct {
		name        string
		body        []byte
		wantErr     error // nilなら成功を期待
		wantText    string
		wantEmotion string
	}{
		{"TC-3-1-01_仕様通りのJSONを正しくパースする", []byte(`{"text": "こんにちは", "emotion": "喜び"}`), nil, "こんにちは", "喜び"},
		{"TC-3-1-09_想定外フィールドは無視される", []byte(`{"text": "こんにちは", "emotion": "喜び", "extra": "unknown"}`), nil, "こんにちは", "喜び"},
		{"TC-3-1-10_textが空文字列でもパースは成功する", []byte(`{"text": "", "emotion": "喜び"}`), nil, "", "喜び"},
		{"TC-3-1-11_構文不正はErrSyntaxを返す", []byte(`{"text": "こんにちは", "emotion": "喜び"`), llm.ErrSyntax, "", ""},
		{"TC-3-1-12_textキー欠落はErrSchemaを返す", []byte(`{"emotion": "喜び"}`), llm.ErrSchema, "", ""},
		{"TC-3-1-13_emotionキー欠落はErrSchemaを返す", []byte(`{"text": "こんにちは"}`), llm.ErrSchema, "", ""},
		{"TC-3-1-14_emotionが7種外なら困惑にフォールバックする", []byte(`{"text": "こんにちは", "emotion": "普通"}`), nil, "こんにちは", "困惑"},
		{"TC-3-1-15_emotionが数値型はErrValueを返す", []byte(`{"text": "こんにちは", "emotion": 123}`), llm.ErrValue, "", ""},
		{"TC-3-1-16_textがnullはErrValueを返す", []byte(`{"text": null, "emotion": "喜び"}`), llm.ErrValue, "", ""},
		{"TC-3-1-17_空文字列ボディは構文エラーを返す", []byte(``), llm.ErrSyntax, "", ""},
		{"TC-3-1-19_空オブジェクトはErrSchemaを返す", []byte(`{}`), llm.ErrSchema, "", ""},
		{"TC-3-1-20_Markdownコードブロックは構文エラーを返す", []byte("```json\n{\"text\": \"こんにちは\", \"emotion\": \"喜び\"}\n```"), llm.ErrSyntax, "", ""},
		{"TC-3-1-21_長大なtextでもパースが完了する", []byte(`{"text": "` + longText + `", "emotion": "喜び"}`), nil, longText, "喜び"},
		{"TC-3-1-22_emotionが空文字列でも困惑にフォールバックする", []byte(`{"text": "こんにちは", "emotion": ""}`), nil, "こんにちは", "困惑"},
		{"TC-3-1-23_マルチバイト文字が正しくデコードされる", []byte(`{"text": "ずんだもんなのだ", "emotion": "喜び"}`), nil, "ずんだもんなのだ", "喜び"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := llm.ParseLLMResponse(tt.body)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantText, got.Text)
			assert.Equal(t, tt.wantEmotion, got.Emotion)
		})
	}

	t.Run("TC-3-1-18_null文字列はエラーを返す", func(t *testing.T) {
		_, err := llm.ParseLLMResponse([]byte(`null`))
		assert.Error(t, err)
	})
}

func TestParseLLMResponse_emotion7種(t *testing.T) {
	emotions := []string{"喜び", "怒り", "悲しみ", "楽しい", "照れ", "困惑", "ドヤ顔"}
	labels := []string{"02", "03", "04", "05", "06", "07", "08"}

	for i, emotion := range emotions {
		emotion := emotion
		t.Run("TC-3-1-"+labels[i]+"_emotionが"+emotion+"で成功する", func(t *testing.T) {
			body := []byte(`{"text": "こんにちは", "emotion": "` + emotion + `"}`)

			got, err := llm.ParseLLMResponse(body)

			require.NoError(t, err)
			assert.Equal(t, emotion, got.Emotion)
		})
	}
}

func TestParseLLMResponse_構文エラーとスキーマ検証エラーを区別できる(t *testing.T) {
	// TC-3-1-24
	_, syntaxErr := llm.ParseLLMResponse([]byte(`{"text": "こんにちは", "emotion": "喜び"`))
	_, schemaErr := llm.ParseLLMResponse([]byte(`{"emotion": "喜び"}`))

	assert.True(t, errors.Is(syntaxErr, llm.ErrSyntax))
	assert.False(t, errors.Is(syntaxErr, llm.ErrSchema))
	assert.True(t, errors.Is(schemaErr, llm.ErrSchema))
	assert.False(t, errors.Is(schemaErr, llm.ErrSyntax))
}

func TestParseLLMResponse_スキーマ検証エラーと値検証エラーを区別できる(t *testing.T) {
	// TC-3-1-25
	_, schemaErr := llm.ParseLLMResponse([]byte(`{"text": "こんにちは"}`))
	_, valueErr := llm.ParseLLMResponse([]byte(`{"text": "こんにちは", "emotion": 123}`))

	assert.True(t, errors.Is(schemaErr, llm.ErrSchema))
	assert.False(t, errors.Is(schemaErr, llm.ErrValue))
	assert.True(t, errors.Is(valueErr, llm.ErrValue))
	assert.False(t, errors.Is(valueErr, llm.ErrSchema))
}
