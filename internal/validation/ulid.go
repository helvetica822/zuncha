package validation

import "strings"

const ulidLength = 26

// Crockford Base32（I/L/O/U を除外、大文字のみ）。
const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// IsValidULID は s が26文字・全て Crockford Base32 の許容文字か判定する。
// trim・正規化はしない。全角/絵文字/小文字/前後空白は false。
func IsValidULID(s string) bool {
	if len(s) != ulidLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(crockfordBase32, s[i]) < 0 {
			return false
		}
	}
	return true
}
