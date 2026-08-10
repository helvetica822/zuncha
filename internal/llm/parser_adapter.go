package llm

// DefaultParser は関数である ParseLLMResponse を ResponseParser I/F として使えるようにする
// アダプタ。I/F も ParseLLMResponse も本パッケージにあるため、アダプタも同居させる。
type DefaultParser struct{}

var _ ResponseParser = (*DefaultParser)(nil)

// NewDefaultParser は DefaultParser を生成する。
func NewDefaultParser() *DefaultParser {
	return &DefaultParser{}
}

// Parse は ParseLLMResponse へそのまま委譲する（センチネルエラーも透過する）。
func (p *DefaultParser) Parse(body []byte) (*LLMResponse, error) {
	return ParseLLMResponse(body)
}
