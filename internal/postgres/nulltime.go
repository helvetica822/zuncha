package postgres

import (
	"database/sql"
	"time"
)

// nullTimeOrNow はゼロ値の時刻を NULL として扱う sql.NullTime を返す。
// INSERT 時に COALESCE($n::timestamptz, NOW()) と組み合わせることで、
// 「呼び出し側が時刻を指定しなければ DB の NOW() に委ねる」を Go 側の分岐なしに1クエリで実現する。
// ゼロ値をそのまま渡すと 0001-01-01 が保存され、created_at 順の並びが壊れる。
// なお ::timestamptz の明示キャストは必須（無いとプレースホルダの型を推論できずエラーになる）。
func nullTimeOrNow(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: !t.IsZero()}
}
