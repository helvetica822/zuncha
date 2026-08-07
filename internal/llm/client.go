package llm

import "context"

// LLMClient は LLM への問い合わせを抽象化する。
type LLMClient interface {
	GenerateResponse(ctx context.Context, prompt string) ([]byte, error)
}

// ResponseParser は LLM 応答のパースを抽象化する。
type ResponseParser interface {
	Parse(body []byte) (*LLMResponse, error)
}
