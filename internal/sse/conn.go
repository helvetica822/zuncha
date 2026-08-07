package sse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Conn は SSE 1接続への書き込みを抽象化する。
type Conn interface {
	// WriteEvent は event 名と JSON 化可能な data を1フレームとして送出する。
	WriteEvent(name string, data any) error
}

// ErrSinkOverflow は送信バッファが満杯（遅い/死んだクライアント）であることを示す。
var ErrSinkOverflow = errors.New("sse: sink overflow")

// ErrConnClosed は接続の書き込みループが既に終了していることを示す。
// これを返すことで Broadcast の既存の失敗解除パスがそのまま効き、
// 死んだ接続の解除に新しい経路を作らずに済む。
var ErrConnClosed = errors.New("sse: connection closed")

const (
	// sinkBufferSize は1接続あたりの送信バッファ段数。満杯時はブロックせず
	// ErrSinkOverflow を返し、遅い1クライアントで他の配信を止めない（NF-SCALE-01）。
	sinkBufferSize = 64
	// defaultHeartbeatInterval は無通信で切断されるのを防ぐハートビート間隔（仕様書§2.3）。
	defaultHeartbeatInterval = 15 * time.Second
	// retryMillis は再接続間隔のサーバー指示（F-RT-02）。
	retryMillis = 3000
)

// heartbeatFrame は SSE コメント行。イベントとしては解釈されず接続維持だけに使う。
var heartbeatFrame = []byte(": ping\n\n")

// HTTPConn は http.ResponseWriter への SSE 書き込みを担う。
//
// http.ResponseWriter は並行書き込み安全ではない。1本の接続には複数リクエストの
// 応答イベントとハートビートが別 goroutine から到来するため、書き込みを
// バッファ付きチャネル + 書き込み専用 goroutine 1本（Run）に集約する。
// 書き手が1本しかないので mutex は不要になる。
type HTTPConn struct {
	w                 http.ResponseWriter
	flusher           http.Flusher
	frames            chan []byte
	heartbeatInterval time.Duration

	// done は Run の終了を伝える。ctx キャンセルでも書き込み失敗でも閉じる。
	done      chan struct{}
	closeOnce sync.Once
}

// HTTPConnOption は HTTPConn の生成時オプション。
type HTTPConnOption func(*HTTPConn)

// WithHeartbeatInterval はハートビート間隔を上書きする（既定 defaultHeartbeatInterval）。
// 既定の15秒は実時間で待つとテストが遅く不安定になるため、検証時に短い間隔を注入する口。
// 非正の値は無視して既定を保つ（time.NewTicker が非正の間隔でパニックするため）。
func WithHeartbeatInterval(d time.Duration) HTTPConnOption {
	return func(c *HTTPConn) {
		if d > 0 {
			c.heartbeatInterval = d
		}
	}
}

// NewHTTPConn は SSE 用ヘッダを設定し、retry 指示を送出した接続を返す。
// w が http.Flusher を実装しない場合はエラー（Flush できない環境でSSEは成立しないため、
// 無言で動かないより起動時に落とす）。
//
// retry の書き出しは Run 起動前・単一 goroutine の段階で行うため、
// 単一ライタの前提は崩れない（かつ送信バッファを1段も消費しない）。
//
// opts は可変長なので NewHTTPConn(w) の呼び出しは無改変で動く。
func NewHTTPConn(w http.ResponseWriter, opts ...HTTPConnOption) (*HTTPConn, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("sse: response writer %T does not implement http.Flusher", w)
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// 将来リバースプロキシを挟んだときにバッファリングで配信が固まるのを防ぐ。
	h.Set("X-Accel-Buffering", "no")

	c := &HTTPConn{
		w:                 w,
		flusher:           flusher,
		frames:            make(chan []byte, sinkBufferSize),
		heartbeatInterval: defaultHeartbeatInterval,
		done:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}

	if _, err := c.w.Write([]byte(fmt.Sprintf("retry: %d\n\n", retryMillis))); err != nil {
		return nil, fmt.Errorf("sse: write retry preamble: %w", err)
	}
	c.flusher.Flush()
	return c, nil
}

// Done は Run が終了したときに閉じられるチャネルを返す。
// ctx キャンセル・書き込み失敗のいずれでも閉じる。
// ハンドラはこれを待つことで、half-open 接続で永久ブロックするのを避けられる。
func (c *HTTPConn) Done() <-chan struct{} {
	return c.done
}

// closeDone は done を閉じる（多重呼び出し安全）。
func (c *HTTPConn) closeDone() {
	c.closeOnce.Do(func() { close(c.done) })
}

// WriteEvent はフレームを送信バッファへ積むだけで、実際の書き込みは Run が行う。
// バッファが満杯なら ErrSinkOverflow を返す（ブロックしない）。
// Run が既に終了していれば ErrConnClosed を返す（終了後も成功を返し続けると、
// 死んだ接続が Hub に残ったままバッファ64件分の遅延が生じる）。
func (c *HTTPConn) WriteEvent(name string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse: marshal %s event: %w", name, err)
	}

	// json.Marshal の出力は文字列中の改行を \n へエスケープするため1行が保証される。
	// 整形して複数行にすると data 行が割れて SSE パースが壊れる。
	frame := make([]byte, 0, len("event: \ndata: \n\n")+len(name)+len(payload))
	frame = append(frame, "event: "...)
	frame = append(frame, name...)
	frame = append(frame, "\ndata: "...)
	frame = append(frame, payload...)
	frame = append(frame, "\n\n"...)

	// 終了判定を先に単独で見る（frames への送信と同じ select に混ぜると、
	// 両方が同時に ready のとき Go がランダムに選ぶため挙動が非決定的になる）。
	select {
	case <-c.done:
		return ErrConnClosed
	default:
	}

	select {
	case c.frames <- frame:
		return nil
	default:
		return ErrSinkOverflow
	}
}

// Run は ResponseWriter へ書く唯一の goroutine の本体。
// ctx のキャンセルまたは書き込み失敗で戻り、戻る際に Done() のチャネルを閉じる。
//
// 【重要】http.ResponseWriter はハンドラが return した後は使用禁止のため、
// **ハンドラは Run が戻るまで return してはならない**。
// 最も安全なのは、ハンドラが同 goroutine で Run をブロック呼び出しすること
// （こうすれば「Run 実行中にハンドラが return する」状態が構造的に作れない）。
// 別 goroutine で起動する場合は、必ず Done() を待ってから return すること。
func (c *HTTPConn) Run(ctx context.Context) {
	defer c.closeDone()

	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-c.frames:
			if !c.write(frame) {
				return
			}
		case <-ticker.C:
			if !c.write(heartbeatFrame) {
				return
			}
		}
	}
}

// write は1フレームを書き出して Flush する。書き込みを続けられるかを返す。
func (c *HTTPConn) write(frame []byte) bool {
	if _, err := c.w.Write(frame); err != nil {
		return false
	}
	c.flusher.Flush()
	return true
}

var _ Conn = (*HTTPConn)(nil)
