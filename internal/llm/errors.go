package llm

import "errors"

// LLM 応答パースのセンチネルエラー(errors.Is で相互に区別可能)。
var (
	ErrSyntax = errors.New("llm: syntax error")
	ErrSchema = errors.New("llm: schema error")
	ErrValue  = errors.New("llm: value error")
)
