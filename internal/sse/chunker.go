package sse

// maxChunkRunes は区切り文字が現れない場合に強制分割するルーン数の上限。
// バイト単位だと日本語・絵文字が途中で割れて文字化けするためルーン単位で数える。
const maxChunkRunes = 60

// sentenceDelimiters は文の区切りとみなす文字。
// 読点「、」は含めない（粒度が細かすぎて表示がちらつくため）。
var sentenceDelimiters = map[rune]bool{
	'。':  true,
	'！':  true,
	'？':  true,
	'!':  true,
	'?':  true,
	'\n': true,
}

// SentenceChunker は応答テキストを文単位の配信チャンクへ分割する。
type SentenceChunker struct{}

// NewSentenceChunker は SentenceChunker を生成する（引数なし）。
func NewSentenceChunker() *SentenceChunker {
	return &SentenceChunker{}
}

// Chunk は text を配信単位へ分割する。
// 区切り文字の直後で切り、区切り文字はチャンクに含める。区切り文字が現れなくても
// maxChunkRunes ルーンで強制分割し、末尾の残余も1チャンクとして返す。
// 表示用テキストなので trim はしない（空白のみのチャンクも保持する）。
// 空入力では長さ0の非 nil スライスを返す。
func (c *SentenceChunker) Chunk(text string) []string {
	chunks := make([]string, 0)
	current := make([]rune, 0, maxChunkRunes)

	for _, r := range text {
		current = append(current, r)
		if sentenceDelimiters[r] || len(current) >= maxChunkRunes {
			chunks = append(chunks, string(current))
			current = current[:0]
		}
	}
	if len(current) > 0 {
		chunks = append(chunks, string(current))
	}
	return chunks
}

var _ TextChunker = (*SentenceChunker)(nil)
