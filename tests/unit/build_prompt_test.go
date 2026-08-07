// llm.BuildPrompt の単体テスト。
// 対応: tasks/instructions_zundamon_wave_b2.md §2.1 (W-07)
package unit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"zuncha/internal/llm"
	"zuncha/internal/model"
)

// promptMessage は BuildPrompt の入力を組み立てるヘルパー。
// emotion はプロンプトに含まれない仕様なので、意図的に値を入れて無視を検証する。
func promptMessage(role, content string, emotion *string) model.Message {
	return model.Message{Role: role, Content: content, Emotion: emotion}
}

func TestBuildPrompt(t *testing.T) {
	joy := "喜び"

	t.Run("W-07-P1_空履歴は空文字列を返す", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{})

		assert.Equal(t, "", got)
	})

	t.Run("W-07-P2_nil履歴も空文字列を返す", func(t *testing.T) {
		got := llm.BuildPrompt(nil)

		assert.Equal(t, "", got)
	})

	t.Run("W-07-P3_1件はroleとcontentをコロン区切りで並べる", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{
			promptMessage("user", "こんにちは", nil),
		})

		assert.Equal(t, "user: こんにちは", got)
	})

	t.Run("W-07-P4_user_assistant混在で履歴の順序が保持される", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{
			promptMessage("user", "こんにちは", nil),
			promptMessage("assistant", "こんにちはなのだ", &joy),
			promptMessage("user", "元気？", nil),
		})

		assert.Equal(t, "user: こんにちは\nassistant: こんにちはなのだ\nuser: 元気？", got)
	})

	t.Run("W-07-P5_emotionはプロンプトに含まれない", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{
			promptMessage("assistant", "やったのだ", &joy),
		})

		assert.Equal(t, "assistant: やったのだ", got)
		assert.NotContains(t, got, "喜び", "LLMへの入力は発話内容のみで足りる")
	})

	t.Run("W-07-P6_システムプロンプトは含まれない", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{
			promptMessage("user", "こんにちは", nil),
		})

		// 口調・JSON形式の強制は W-08 のプロバイダ実装の責務。ここに書くと差し替え可能性が壊れる。
		assert.Equal(t, "user: こんにちは", got)
		assert.NotContains(t, got, "ずんだもん")
		assert.NotContains(t, got, "JSON")
	})

	t.Run("W-07-P7_contentの改行はそのまま保持される", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{
			promptMessage("user", "1行目\n2行目", nil),
			promptMessage("assistant", "了解なのだ", nil),
		})

		assert.Equal(t, "user: 1行目\n2行目\nassistant: 了解なのだ", got)
	})

	t.Run("W-07-P8_絵文字と全角が文字化けしない", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{
			promptMessage("user", "😀ずんだもん！", nil),
		})

		assert.Equal(t, "user: 😀ずんだもん！", got)
	})

	t.Run("W-07-P9_空contentでもrole行は出力される", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{
			promptMessage("user", "", nil),
			promptMessage("assistant", "なのだ", nil),
		})

		assert.Equal(t, "user: \nassistant: なのだ", got)
	})

	t.Run("W-07-P10_20件すべてが順序どおり出力される", func(t *testing.T) {
		history := make([]model.Message, 0, 20)
		wantLines := make([]string, 0, 20)
		for i := 0; i < 20; i++ {
			role := "user"
			if i%2 == 1 {
				role = "assistant"
			}
			content := fmt.Sprintf("発話%02d", i)
			history = append(history, promptMessage(role, content, nil))
			wantLines = append(wantLines, role+": "+content)
		}

		got := llm.BuildPrompt(history)

		assert.Equal(t, strings.Join(wantLines, "\n"), got)
		assert.Equal(t, 19, strings.Count(got, "\n"), "20行なら改行は19個（末尾に余分な改行を付けない）")
	})

	t.Run("W-07-P11_末尾に余分な改行を付けない", func(t *testing.T) {
		got := llm.BuildPrompt([]model.Message{
			promptMessage("user", "こんにちは", nil),
		})

		assert.False(t, strings.HasSuffix(got, "\n"))
	})
}
