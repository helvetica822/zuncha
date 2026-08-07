package sse

// requestIDKey は全 SSE イベントに注入する相関キーのフィールド名（仕様書§2.2）。
// これが無いと遅延 done を旧リクエストのものとして弾けない。
const requestIDKey = "request_id"

// fanout は1リクエスト分の EventSink。requestID を注入して Hub へ流す。
//
// EventSink のメソッドは request_id を引数に持たない（既存 I/F・変更しない）一方、
// request_id はイベント単位で決まる。この差を吸収するのが fanout の役目であり、
// ペイロードの組み立てをここ1箇所に集約することで Hub はイベント種別を知らずに済む。
type fanout struct {
	hub            *Hub
	conversationID string
	requestID      string
}

// NewFanout は EventSink を実装し、requestID を全イベントに注入して Hub へ流す。
func NewFanout(h *Hub, conversationID, requestID string) EventSink {
	return &fanout{hub: h, conversationID: conversationID, requestID: requestID}
}

func (f *fanout) SendEmotion(label string) error {
	return f.broadcast("emotion", "label", label)
}

func (f *fanout) SendTextChunk(chunk string) error {
	return f.broadcast("text", "chunk", chunk)
}

func (f *fanout) SendAudioURL(url string) error {
	return f.broadcast("audio_url", "url", url)
}

func (f *fanout) SendDone() error {
	return f.broadcast("done", "", "")
}

func (f *fanout) SendError(message string) error {
	return f.broadcast("error", "message", message)
}

// broadcast は request_id を注入したペイロードを Hub へ渡す。
// field が空文字列の場合は request_id のみのペイロードにする（done 用）。
//
// 常に nil を返す。Broadcast は個別接続の失敗を吸収する設計であり、
// 「1接続に届かなかった」ことで ResponseStreamer の処理全体を中断させると、
// 健全な他の接続にも応答が届かなくなるため。
func (f *fanout) broadcast(name, field, value string) error {
	data := map[string]string{requestIDKey: f.requestID}
	if field != "" {
		data[field] = value
	}
	f.hub.Broadcast(f.conversationID, name, data)
	return nil
}

var _ EventSink = (*fanout)(nil)
