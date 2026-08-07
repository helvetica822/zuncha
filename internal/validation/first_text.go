package validation

const firstTextMaxRunes = 20

// TruncateFirstText は s の先頭20ルーン（コードポイント単位）を返す。trim・サニタイズしない。
func TruncateFirstText(s string) string {
	runes := []rune(s)
	if len(runes) > firstTextMaxRunes {
		return string(runes[:firstTextMaxRunes])
	}
	return s
}
