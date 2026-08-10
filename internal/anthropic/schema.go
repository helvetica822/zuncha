package anthropic

// emotions は F-AI-02 の感情ラベル7種。
//
// 真実の源泉は docs/02_functional_design/02_database_design.md の DDL CHECK 制約
// （および internal/validation の validEmotions）。ここは構造化出力のスキーマに
// 渡すため順序が必要で、validation 側は非公開マップ（順序なし）のため再掲している。
// 追加・変更時は DDL / validation / SystemPrompt の3箇所と揃えること。
var emotions = []string{"喜び", "怒り", "悲しみ", "楽しい", "照れ", "困惑", "ドヤ顔"}

// responseSchema は構造化出力(output_config.format)で強制する {text, emotion} のスキーマ。
//
// これは1枚目の防壁で、逸脱は既存の llm.ParseLLMResponse（25ケースでテスト済み・
// 7種外は「困惑」フォールバック）が2枚目の防壁として吸収する。
var responseSchema = func() map[string]any {
	enum := make([]any, 0, len(emotions))
	for _, e := range emotions {
		enum = append(enum, e)
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":    map[string]any{"type": "string"},
			"emotion": map[string]any{"type": "string", "enum": enum},
		},
		"required":             []any{"text", "emotion"},
		"additionalProperties": false,
	}
}()
