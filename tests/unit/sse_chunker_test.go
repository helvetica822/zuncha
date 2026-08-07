// sse.SentenceChunker の単体テスト。
// 対応: tasks/instructions_zundamon_wave_b1.md §3 (W-05)
package unit

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/sse"
)

func TestSentenceChunker_Chunk(t *testing.T) {
	chunker := sse.NewSentenceChunker()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "句点2文で2チャンクに分かれ各々が句点を含む",
			text: "こんにちはなのだ。元気なのだ。",
			want: []string{"こんにちはなのだ。", "元気なのだ。"},
		},
		{
			name: "全角感嘆符で1チャンク",
			text: "やったのだ！",
			want: []string{"やったのだ！"},
		},
		{
			name: "全角疑問符で1チャンク",
			text: "なんでなのだ？",
			want: []string{"なんでなのだ？"},
		},
		{
			name: "半角感嘆符も区切り文字として扱う",
			text: "Yes!",
			want: []string{"Yes!"},
		},
		{
			name: "半角疑問符も区切り文字として扱う",
			text: "What?",
			want: []string{"What?"},
		},
		{
			name: "改行で分割され改行はチャンクに含まれる",
			text: "line1\nline2",
			want: []string{"line1\n", "line2"},
		},
		{
			name: "区切り文字なしの短文は1チャンク",
			text: "区切り文字のない短い文",
			want: []string{"区切り文字のない短い文"},
		},
		{
			name: "読点は区切らない",
			text: "これは、区切らないのだ",
			want: []string{"これは、区切らないのだ"},
		},
		{
			name: "連続する句点は3チャンクになる",
			text: "。。。",
			want: []string{"。", "。", "。"},
		},
		{
			name: "末尾に区切り文字が無い残余も落とさない",
			text: "あ。い",
			want: []string{"あ。", "い"},
		},
		{
			name: "空白のみはtrimされず1チャンクとして保持される",
			text: "   ",
			want: []string{"   "},
		},
		{
			name: "空入力は長さ0のスライス",
			text: "",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chunker.Chunk(tt.text)

			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("W-05-B1_60ルーンちょうどは1チャンク", func(t *testing.T) {
		text := strings.Repeat("あ", 60)

		got := chunker.Chunk(text)

		require.Len(t, got, 1)
		assert.Equal(t, text, got[0])
		assert.Equal(t, 60, utf8.RuneCountInString(got[0]))
	})

	t.Run("W-05-B2_61ルーンは60と1の2チャンクに強制分割される", func(t *testing.T) {
		text := strings.Repeat("あ", 61)

		got := chunker.Chunk(text)

		require.Len(t, got, 2)
		assert.Equal(t, 60, utf8.RuneCountInString(got[0]))
		assert.Equal(t, 1, utf8.RuneCountInString(got[1]))
		assert.Equal(t, text, got[0]+got[1], "結合すると原文に戻る（欠落なし）")
	})

	t.Run("W-05-B3_59ルーンは分割されない", func(t *testing.T) {
		text := strings.Repeat("あ", 59)

		got := chunker.Chunk(text)

		require.Len(t, got, 1)
		assert.Equal(t, text, got[0])
	})

	t.Run("W-05-B4_絵文字のみ130ルーンでもルーン境界で切れ文字化けしない", func(t *testing.T) {
		text := strings.Repeat("😀", 130)

		got := chunker.Chunk(text)

		require.Len(t, got, 3, "60+60+10")
		for i, chunk := range got {
			assert.True(t, utf8.ValidString(chunk), "チャンク%dが不正なUTF-8になっている", i)
			assert.NotContains(t, chunk, "�", "置換文字（文字化け）が混入している")
		}
		assert.Equal(t, 60, utf8.RuneCountInString(got[0]))
		assert.Equal(t, 60, utf8.RuneCountInString(got[1]))
		assert.Equal(t, 10, utf8.RuneCountInString(got[2]))
		assert.Equal(t, text, strings.Join(got, ""))
	})

	t.Run("W-05-B5_空入力はnilではなく長さ0の非nilスライス", func(t *testing.T) {
		got := chunker.Chunk("")

		assert.NotNil(t, got, "nilを返してはならない（既存ReverseMessagesと同じ流儀）")
		assert.Len(t, got, 0)
	})

	t.Run("W-05-B6_空チャンクは生成されない", func(t *testing.T) {
		got := chunker.Chunk("あ。\n。い！")

		for i, chunk := range got {
			assert.NotEmpty(t, chunk, "チャンク%dが空文字列になっている", i)
		}
		assert.Equal(t, []string{"あ。", "\n", "。", "い！"}, got)
	})

	t.Run("W-05-B7_60ルーン上限と区切り文字が混在しても原文が復元できる", func(t *testing.T) {
		text := strings.Repeat("あ", 70) + "。" + strings.Repeat("い", 30)

		got := chunker.Chunk(text)

		assert.Equal(t, text, strings.Join(got, ""), "全チャンクの結合は原文と一致すべき")
		assert.Equal(t, []string{
			strings.Repeat("あ", 60),
			strings.Repeat("あ", 10) + "。",
			strings.Repeat("い", 30),
		}, got)
	})
}

// TextChunker インターフェースを満たすことをコンパイル時に保証する。
var _ sse.TextChunker = (*sse.SentenceChunker)(nil)
