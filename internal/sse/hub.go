package sse

import "sync"

// registration は Hub に登録された1接続を表す。
// ポインタ自体を識別子として使うことで、(1) 同一の Conn 値を複数回登録しても
// 登録ごとに独立して解除でき、(2) Conn の実装が比較不能な型でも map キーにできる。
type registration struct {
	conn Conn
}

// Hub は会話IDごとの接続群を保持し、ブロードキャストする。
// 10人・単一インスタンス前提のため外部ストア（Redis等）は使わない。
type Hub struct {
	mu    sync.RWMutex
	conns map[string]map[*registration]struct{}
}

// NewHub は Hub を生成する（引数なし）。
func NewHub() *Hub {
	return &Hub{conns: make(map[string]map[*registration]struct{})}
}

// Register は接続を登録し、解除用の関数を返す（handler が defer で呼ぶ）。
// 返された関数は何度呼んでも安全（handler の defer と Hub 側の失敗時解除で
// 二重解除が普通に起きるため）。
func (h *Hub) Register(conversationID string, c Conn) (unregister func()) {
	reg := &registration{conn: c}

	h.mu.Lock()
	if h.conns[conversationID] == nil {
		h.conns[conversationID] = make(map[*registration]struct{})
	}
	h.conns[conversationID][reg] = struct{}{}
	h.mu.Unlock()

	return func() { h.unregister(conversationID, reg) }
}

// Broadcast は当該会話の全接続へ1フレームを送る。
// エラーは返さない（1接続の失敗で応答生成全体を落とすのは誤り）。
// 書き込みに失敗した接続はその場で解除する（死んだ接続を残すとメモリと配信コストが積む）。
// 登録0件の会話へのブロードキャストは何もしない（正常系）。
func (h *Hub) Broadcast(conversationID string, name string, data any) {
	// 書き込み中にロックを保持しない。WriteEvent の失敗時に解除（書き込みロック）が
	// 必要になり、読み取りロックを持ったままでは昇格できずデッドロックするため、
	// 配信先をスナップショットしてからロック外で書き込む。
	h.mu.RLock()
	targets := make([]*registration, 0, len(h.conns[conversationID]))
	for reg := range h.conns[conversationID] {
		targets = append(targets, reg)
	}
	h.mu.RUnlock()

	for _, reg := range targets {
		if err := reg.conn.WriteEvent(name, data); err != nil {
			h.unregister(conversationID, reg)
		}
	}
}

// ConnCount は当該会話に登録されている接続数を返す。
// ハンドラが切断時に解除していることを観測可能にするために公開する。
func (h *Hub) ConnCount(conversationID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns[conversationID])
}

// unregister は登録を取り除く。存在しない登録に対しては何もしない（冪等）。
func (h *Hub) unregister(conversationID string, reg *registration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	regs, ok := h.conns[conversationID]
	if !ok {
		return
	}
	delete(regs, reg)
	// 会話ごとの map を空のまま残すとメモリが積むため、最後の接続が抜けたら消す。
	if len(regs) == 0 {
		delete(h.conns, conversationID)
	}
}
