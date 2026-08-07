package llm

import (
	"strings"

	"zuncha/internal/model"
)

// roleContentSep は1発話内の role と content の区切り。
const roleContentSep = ": "

// BuildPrompt は会話履歴を LLM へ渡すプロンプト文字列へ整形する。
//
// 引数は履歴のみで userText は取らない。呼び出し側（ChatService）がユーザー発話を
// 先に保存するため、履歴の末尾に当該発話が既に含まれており、別途渡すと二重になる。
//
// システムプロンプト（ずんだもん口調・JSON形式の強制）は含めない。それはプロバイダ実装の
// 責務であり、ここに書くと差し替え可能性が壊れる。emotion も含めない（発話内容で足りる）。
// 空履歴では空文字列を返す。
func BuildPrompt(history []model.Message) string {
	lines := make([]string, 0, len(history))
	for _, m := range history {
		lines = append(lines, m.Role+roleContentSep+m.Content)
	}
	return strings.Join(lines, "\n")
}
