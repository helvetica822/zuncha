package validation

import "strings"

// IsValidInput は s が標準 trim 後に非空か判定する。
// 半角/タブ/改行/全角スペース(U+3000)は trim 対象、ゼロ幅スペース(U+200B)・絵文字は非空扱い。
func IsValidInput(s string) bool {
	return strings.TrimSpace(s) != ""
}
