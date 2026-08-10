// 対応仕様: tasks/instructions_zundamon_wave_c1.md §4（ResponseParser アダプタ）
//
// 本テストは「ParseLLMResponse へ委譲していること」の確認に絞る。
// パース挙動そのものは parse_llm_response_test.go の25件が固定している。
package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/llm"
)

func TestDefaultParser_ParseはParseLLMResponseへ委譲する(t *testing.T) {
	p := llm.NewDefaultParser()

	got, err := p.Parse([]byte(`{"text":"こんにちはなのだ","emotion":"喜び"}`))
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "こんにちはなのだ", got.Text)
	assert.Equal(t, "喜び", got.Emotion)
}

func TestDefaultParser_Parseはセンチネルエラーをそのまま透過する(t *testing.T) {
	p := llm.NewDefaultParser()

	got, err := p.Parse([]byte(`これはJSONではない`))
	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorIs(t, err, llm.ErrSyntax)
}
