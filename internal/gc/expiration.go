package gc

import "time"

// IsExpired は expiresAt が now より前（期限切れ）か判定する。等号は非対称（一致は false）。
func IsExpired(expiresAt time.Time, now time.Time) bool {
	return expiresAt.Before(now)
}
