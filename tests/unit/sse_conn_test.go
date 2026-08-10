// sse.HTTPConn の単体テスト。
// 対応: tasks/instructions_zundamon_wave_b1.md §1 (W-03)、docs/04_implementation/05_sse_protocol_spec.md §2/§4.3
package unit

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/sse"
)

// retryPreamble は接続確立直後に必ず送出される再接続間隔の指示（F-RT-02）。
const retryPreamble = "retry: 3000\n\n"

// syncResponseWriter は書き込みを mutex で保護した ResponseWriter。
// HTTPConn の書き込み goroutine と テスト goroutine が同じバッファに触るため、
// -race をクリーンに保つには保護が必要（保護しないとテスト側の欠陥で競合が出る）。
type syncResponseWriter struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	header   http.Header
	writes   int
	flushes  int
	status   int
	writeErr error
	written  chan struct{}
}

func newSyncResponseWriter() *syncResponseWriter {
	return &syncResponseWriter{
		header:  make(http.Header),
		written: make(chan struct{}, 512),
	}
}

func (w *syncResponseWriter) Header() http.Header { return w.header }

func (w *syncResponseWriter) WriteHeader(status int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
}

func (w *syncResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	n, err := w.buf.Write(p)
	w.writes++
	select {
	case w.written <- struct{}{}:
	default:
	}
	return n, err
}

func (w *syncResponseWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushes++
}

func (w *syncResponseWriter) body() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *syncResponseWriter) flushCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushes
}

// waitWrites は累計 total 件の書き込みが完了するまで待つ（ポーリングではなく通知待ち）。
func (w *syncResponseWriter) waitWrites(t *testing.T, total int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		w.mu.Lock()
		n := w.writes
		w.mu.Unlock()
		if n >= total {
			return
		}
		select {
		case <-w.written:
		case <-deadline:
			t.Fatalf("書き込みが累計%d件に達しなかった（現在%d件、出力=%q）", total, n, w.body())
		}
	}
}

// notFlusherWriter は http.Flusher を実装しない ResponseWriter。
type notFlusherWriter struct{ header http.Header }

func (w *notFlusherWriter) Header() http.Header         { return w.header }
func (w *notFlusherWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *notFlusherWriter) WriteHeader(int)             {}

// テスト用ペイロード。json.Marshal のフィールド順を固定し、仕様書§2.2の並びと一致させる。
type testEmotionPayload struct {
	RequestID string `json:"request_id"`
	Label     string `json:"label"`
}

type testTextPayload struct {
	RequestID string `json:"request_id"`
	Chunk     string `json:"chunk"`
}

// startConn は HTTPConn を生成し書き込み goroutine を起動する。
func startConn(t *testing.T, w http.ResponseWriter) *sse.HTTPConn {
	t.Helper()
	conn, err := sse.NewHTTPConn(w)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go conn.Run(ctx)
	return conn
}

// writeEventNonBlocking は WriteEvent を別 goroutine で実行し、戻らなければテストを失敗させる。
// 「満杯でもブロックしない」契約が壊れたときにテストバイナリがハングせず、
// クリーンな失敗として現れるようにするためのガード（ハングはCIで原因究明が高コスト）。
func writeEventNonBlocking(t *testing.T, conn *sse.HTTPConn, name string, data any) error {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- conn.WriteEvent(name, data) }()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatalf("WriteEvent(%s)がブロックした（ノンブロッキング送信の契約違反）", name)
		return nil
	}
}

// waitDone は conn.Done() が閉じるのを待つ。閉じなければテストを失敗させる。
// 素の `<-conn.Done()` だと Done を閉じ忘れた実装でテストバイナリがハングし、
// CI で原因究明が高コストになるため、常にこのガードを通す。
func waitDone(t *testing.T, conn *sse.HTTPConn) {
	t.Helper()
	select {
	case <-conn.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Run終了後もDone()が閉じていない（死んだ接続が解除されない）")
	}
}

// frameAfterPreamble は retry プリアンブルを除いた本体フレームを返す。
func frameAfterPreamble(t *testing.T, body string) string {
	t.Helper()
	require.True(t, strings.HasPrefix(body, retryPreamble),
		"出力は retry プリアンブルで始まるべきだが %q だった", body)
	return strings.TrimPrefix(body, retryPreamble)
}

func TestNewHTTPConn(t *testing.T) {
	t.Run("W-03-01_SSE用レスポンスヘッダ4つが値まで正しく設定される", func(t *testing.T) {
		rec := httptest.NewRecorder()

		_, err := sse.NewHTTPConn(rec)

		require.NoError(t, err)
		assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
		assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))
		assert.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"),
			"プロキシのバッファリングで配信が固まるのを防ぐため最初から付ける")
	})

	t.Run("W-03-02_接続確立直後にretry3000が1回送出される", func(t *testing.T) {
		w := newSyncResponseWriter()

		_, err := sse.NewHTTPConn(w)

		require.NoError(t, err)
		assert.Equal(t, retryPreamble, w.body())
		assert.Equal(t, 1, strings.Count(w.body(), "retry:"), "retryは1回だけ")
		assert.GreaterOrEqual(t, w.flushCount(), 1, "プリアンブルもFlushされるべき")
	})

	t.Run("W-03-03_httpFlusher非対応のResponseWriterはエラーになる", func(t *testing.T) {
		conn, err := sse.NewHTTPConn(&notFlusherWriter{header: make(http.Header)})

		require.Error(t, err, "Flushできない環境でSSEは成立しないので起動時に落とす")
		assert.Nil(t, conn)
	})

	t.Run("W-03-04_idフィールドは振らない_再送非対応の誤解を避ける", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn := startConn(t, w)

		require.NoError(t, conn.WriteEvent("done", map[string]string{"request_id": "01J"}))
		w.waitWrites(t, 2)

		assert.NotContains(t, w.body(), "id: ", "id:フィールドを振ってはならない（仕様書§2.3）")
	})
}

func TestHTTPConn_WriteEvent(t *testing.T) {
	t.Run("W-03-05_emotionフレームがSSE形式で出力される", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn := startConn(t, w)

		err := conn.WriteEvent("emotion", testEmotionPayload{RequestID: "01JABC", Label: "喜び"})

		require.NoError(t, err)
		w.waitWrites(t, 2)
		assert.Equal(t,
			"event: emotion\ndata: {\"request_id\":\"01JABC\",\"label\":\"喜び\"}\n\n",
			frameAfterPreamble(t, w.body()))
	})

	t.Run("W-03-06_改行を含むテキストでもdata行が1行に収まる", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn := startConn(t, w)

		err := conn.WriteEvent("text", testTextPayload{RequestID: "01JABC", Chunk: "line1\nline2"})

		require.NoError(t, err)
		w.waitWrites(t, 2)
		frame := frameAfterPreamble(t, w.body())
		assert.Equal(t,
			"event: text\ndata: {\"request_id\":\"01JABC\",\"chunk\":\"line1\\nline2\"}\n\n",
			frame, "生の改行ではなく\\nへエスケープされ、data行が2行に割れないこと")
		assert.Equal(t, 1, strings.Count(frame, "data:"), "data行は1行だけ")
		assert.Equal(t, 3, strings.Count(frame, "\n"),
			"フレームの改行はevent行末・data行末・終端空行の3つだけ")
	})

	t.Run("W-03-07_絵文字と全角を含むテキストが文字化けしない", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn := startConn(t, w)

		err := conn.WriteEvent("text", testTextPayload{RequestID: "01JABC", Chunk: "ずんだもん😀なのだ！"})

		require.NoError(t, err)
		w.waitWrites(t, 2)
		frame := frameAfterPreamble(t, w.body())
		assert.Contains(t, frame, "ずんだもん😀なのだ！")
		assert.NotContains(t, frame, "\\u", "非ASCIIがユニコードエスケープされていない")
	})

	t.Run("W-03-08_空文字列のチャンクでもフレーム整形は壊れない", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn := startConn(t, w)

		err := conn.WriteEvent("text", testTextPayload{RequestID: "01JABC", Chunk: ""})

		require.NoError(t, err)
		w.waitWrites(t, 2)
		assert.Equal(t,
			"event: text\ndata: {\"request_id\":\"01JABC\",\"chunk\":\"\"}\n\n",
			frameAfterPreamble(t, w.body()))
	})

	t.Run("W-03-09_各フレームの書き込み後にFlushされる", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn := startConn(t, w)

		require.NoError(t, conn.WriteEvent("text", testTextPayload{RequestID: "01J", Chunk: "1"}))
		require.NoError(t, conn.WriteEvent("text", testTextPayload{RequestID: "01J", Chunk: "2"}))
		w.waitWrites(t, 3)

		assert.GreaterOrEqual(t, w.flushCount(), 3, "プリアンブル+2フレームでFlushは3回以上")
	})

	t.Run("W-03-10_JSON化できないデータはエラーを返す", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn := startConn(t, w)

		err := conn.WriteEvent("text", make(chan int))

		require.Error(t, err)
		assert.False(t, errors.Is(err, sse.ErrSinkOverflow), "オーバーフローとは別のエラーであるべき")
	})

	t.Run("W-03-11_バッファ64件までは成功し65件目でErrSinkOverflowになる", func(t *testing.T) {
		w := newSyncResponseWriter()
		// Run を起動しない＝誰も読み出さないので、バッファ容量の境界を確定的に検証できる。
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)

		for i := 0; i < 64; i++ {
			require.NoError(t, writeEventNonBlocking(t, conn, "text", testTextPayload{RequestID: "01J", Chunk: "x"}),
				"%d件目は成功すべき", i+1)
		}

		overflowErr := writeEventNonBlocking(t, conn, "text", testTextPayload{RequestID: "01J", Chunk: "x"})

		require.Error(t, overflowErr, "65件目はブロックせずエラーを返すべき")
		assert.ErrorIs(t, overflowErr, sse.ErrSinkOverflow)
	})

	t.Run("W-03-12_オーバーフロー時もブロックしない", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		for i := 0; i < 64; i++ {
			require.NoError(t, writeEventNonBlocking(t, conn, "text", testTextPayload{RequestID: "01J", Chunk: "x"}))
		}

		gotErr := writeEventNonBlocking(t, conn, "text", testTextPayload{RequestID: "01J", Chunk: "x"})

		assert.ErrorIs(t, gotErr, sse.ErrSinkOverflow,
			"満杯時はブロックせずErrSinkOverflowを返すべき（遅い1クライアントで他9人が止まる）")
	})
}

func TestHTTPConn_Heartbeat(t *testing.T) {
	// 既定は15秒。実時間15秒待つテストは F.I.R.S.T の Fast/Repeatable に反するため、
	// WithHeartbeatInterval で短い間隔を注入して機構そのものを検証する。
	const testInterval = 20 * time.Millisecond
	const heartbeat = ": ping\n\n"

	t.Run("W-03-15_指定間隔でハートビートが繰り返し送出される", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w, sse.WithHeartbeatInterval(testInterval))
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go conn.Run(ctx)

		// retry プリアンブル1件 + ping 2件 = 累計3件
		w.waitWrites(t, 3)

		assert.GreaterOrEqual(t, strings.Count(w.body(), heartbeat), 2,
			"1回だけでは「繰り返し送出される」ことを証明できない")
	})

	t.Run("W-03-16_ハートビートはSSEコメント行でありイベントではない", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w, sse.WithHeartbeatInterval(testInterval))
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go conn.Run(ctx)

		w.waitWrites(t, 2)

		body := w.body()
		assert.Contains(t, body, heartbeat)
		assert.NotContains(t, body, "event: ping",
			"ハートビートはコメント行であり、クライアントがイベントとして解釈してはならない")
		assert.NotContains(t, body, "data: ")
	})

	t.Run("W-03-17_ハートビートとイベントフレームが混在しても双方壊れない", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w, sse.WithHeartbeatInterval(testInterval))
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go conn.Run(ctx)
		w.waitWrites(t, 2) // 先にハートビートを1回出させる

		require.NoError(t, conn.WriteEvent("text", testTextPayload{RequestID: "01J", Chunk: "なのだ"}))
		w.waitWrites(t, 3)

		body := w.body()
		assert.Contains(t, body, heartbeat)
		assert.Contains(t, body,
			"event: text\ndata: {\"request_id\":\"01J\",\"chunk\":\"なのだ\"}\n\n",
			"ハートビートが割り込んでもイベントフレームは分断されない")
	})

	t.Run("W-03-18_既定間隔ではこの短時間にハートビートが出ない", func(t *testing.T) {
		w := newSyncResponseWriter()
		// オプション無し = 既定15秒。既存の NewHTTPConn(w) 呼び出しが無改変で動くことも兼ねて示す。
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		<-w.written // retry プリアンブルの通知を捨てる
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go conn.Run(ctx)

		select {
		case <-w.written:
			t.Fatalf("既定15秒間隔のはずが短時間で書き込みが発生した（出力=%q）", w.body())
		case <-time.After(100 * time.Millisecond):
			// 期待どおり何も出ない
		}
		assert.Equal(t, retryPreamble, w.body())
	})

	t.Run("W-03-19_非正の間隔を渡しても既定に落ちてパニックせず動き続ける", func(t *testing.T) {
		// そらのB-1レビュー要修正2: 以前は assert.NotPanics(t, func() { go conn.Run(ctx) })
		// と書いていたが、別 goroutine の panic は recover できないため、これは
		// 「go 文の実行自体がパニックしないこと」しか見ておらず何も守っていなかった。
		// 実際に守る形（同 goroutine で Run を実行し、パニックせず正常終了すること）に改めた。
		// time.NewTicker は非正の間隔でパニックするため、既定へのフォールバックが必要。
		w := newSyncResponseWriter()

		conn, err := sse.NewHTTPConn(w, sse.WithHeartbeatInterval(0))

		require.NoError(t, err)
		<-w.written
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 先にキャンセルしておけば Run は同 goroutine でも即座に戻る

		assert.NotPanics(t, func() { conn.Run(ctx) },
			"非正の間隔が time.NewTicker へ渡ると panic するので既定へフォールバックすべき")

		// 既定15秒へ落ちていること（短時間ではハートビートが出ない）。
		assert.Equal(t, retryPreamble, w.body())
	})
}

func TestHTTPConn_Run(t *testing.T) {
	t.Run("W-03-13_ctxキャンセルでRunが戻る", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			conn.Run(ctx)
			close(done)
		}()

		cancel()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("ctxキャンセルでRunが戻らなかった（goroutineリーク）")
		}
	})

	t.Run("W-03-14_書き込み失敗でRunが戻る", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		w.mu.Lock()
		w.writeErr = errors.New("client gone")
		w.mu.Unlock()
		require.NoError(t, conn.WriteEvent("text", testTextPayload{RequestID: "01J", Chunk: "x"}))

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		done := make(chan struct{})
		go func() {
			conn.Run(ctx)
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("書き込み失敗後もRunが回り続けている（死んだ接続の検出漏れ）")
		}
	})
}

func TestHTTPConn_Done(t *testing.T) {
	// そらのB-1レビュー要修正1: Run の終了が誰にも通知されないと、
	// 書き込み失敗した接続が Hub に残り続け、WriteEvent も64件まで成功を返し続ける。
	t.Run("W-03-20_Run開始前はDoneは閉じていない", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)

		select {
		case <-conn.Done():
			t.Fatal("Runを起動していないのにDoneが閉じている")
		default:
		}
	})

	t.Run("W-03-21_ctxキャンセルでDoneが閉じる", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		go conn.Run(ctx)

		cancel()

		select {
		case <-conn.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("ctxキャンセルでDoneが閉じなかった")
		}
	})

	t.Run("W-03-22_書き込み失敗でDoneが閉じる", func(t *testing.T) {
		// 仕様書§2.4「書き込み失敗が起きた sink も即座に解除する」の起点。
		// §2.3のハートビートの存在理由「死んだ接続を書き込み失敗で検出する」も
		// これが無いと検出しても解除に繋がらない。
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		w.mu.Lock()
		w.writeErr = errors.New("client gone")
		w.mu.Unlock()
		require.NoError(t, conn.WriteEvent("text", testTextPayload{RequestID: "01J", Chunk: "x"}))
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		go conn.Run(ctx)

		select {
		case <-conn.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("書き込み失敗でDoneが閉じなかった（死んだ接続が解除されない）")
		}
	})

	t.Run("W-03-23_Done後のWriteEventはErrConnClosedを返す", func(t *testing.T) {
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		go conn.Run(ctx)
		cancel()
		waitDone(t, conn)

		gotErr := writeEventNonBlocking(t, conn, "text", testTextPayload{RequestID: "01J", Chunk: "x"})

		require.Error(t, gotErr, "終了後も64件まで成功を返してはならない")
		assert.ErrorIs(t, gotErr, sse.ErrConnClosed)
		assert.NotErrorIs(t, gotErr, sse.ErrSinkOverflow, "満杯とは別のエラーであるべき")
	})

	t.Run("W-03-24_Done後は何度WriteEventしてもErrConnClosed", func(t *testing.T) {
		// Broadcast の既存の「失敗した接続を解除する」経路に毎回乗せるため、
		// 1回目だけでなく常にエラーを返す必要がある。
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		go conn.Run(ctx)
		cancel()
		waitDone(t, conn)

		// 20回呼ぶのは仕様書§2.4.1のミューテーション要件。二段構えの select を1つに戻すと
		// 1回あたり約50%で成功が返るため、3回では見逃し確率が 0.5³≈12.5%（そらの実測: 30回中1回すり抜け）
		// と無視できない。20回なら 1−0.5²⁰≈99.9999% で単一実行でも必ず赤くなる。
		for i := 0; i < 20; i++ {
			gotErr := writeEventNonBlocking(t, conn, "text", testTextPayload{RequestID: "01J", Chunk: "x"})
			assert.ErrorIs(t, gotErr, sse.ErrConnClosed, "%d回目", i+1)
		}
	})

	t.Run("W-03-25_Doneは複数回受信できる", func(t *testing.T) {
		// ハンドラと Hub の双方が待てるよう、閉じたチャネルとして振る舞うこと。
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		go conn.Run(ctx)
		cancel()

		for i := 0; i < 3; i++ {
			select {
			case <-conn.Done():
			case <-time.After(2 * time.Second):
				t.Fatalf("%d回目の受信で閉じたチャネルとして振る舞っていない", i+1)
			}
		}
	})

	t.Run("W-03-26_Hubに登録した接続はDone後のBroadcastで解除される", func(t *testing.T) {
		// 防壁2: WriteEvent が ErrConnClosed を返すことで、Broadcast の
		// 既存の失敗解除パスがそのまま効く（新しい解除経路を作らずに済む）。
		w := newSyncResponseWriter()
		conn, err := sse.NewHTTPConn(w)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		go conn.Run(ctx)
		cancel()
		waitDone(t, conn)

		hub := sse.NewHub()
		const convID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		hub.Register(convID, conn)
		require.Equal(t, 1, hub.ConnCount(convID))

		hub.Broadcast(convID, "done", map[string]string{"request_id": "01J"})

		assert.Equal(t, 0, hub.ConnCount(convID), "終了済み接続はBroadcastで解除されるべき")
	})
}

func TestHTTPConn_Run二重呼び出し(t *testing.T) {
	// W-03-27: closeDone の多重呼び出し安全性。Run を2回呼ぶのは想定外の使い方だが、
	// close(c.done) を素で書くと2回目で panic する。サーバのハンドラ内での panic は
	// no-op より遥かに重いので、sync.Once による冪等性をテストで固定しておく。
	// （ミューテーション: closeOnce を外すとこのテストが panic で落ちる）
	w := newSyncResponseWriter()
	conn, err := sse.NewHTTPConn(w)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	conn.Run(ctx)

	assert.NotPanics(t, func() { conn.Run(ctx) },
		"closeDone は多重呼び出し安全であるべき（close済みチャネルの再closeはpanic）")
	waitDone(t, conn)
}
