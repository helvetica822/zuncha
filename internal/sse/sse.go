package sse

// TextChunker はテキストを配信単位に分割する。
type TextChunker interface {
	Chunk(text string) []string
}

// EventSink は SSE イベントの送出を抽象化する。
type EventSink interface {
	SendEmotion(label string) error
	SendTextChunk(chunk string) error
	SendAudioURL(url string) error
	SendDone() error
	SendError(message string) error
}
