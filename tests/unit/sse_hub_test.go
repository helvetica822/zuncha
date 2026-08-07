// sse.Hub の単体テスト。
// 対応: tasks/instructions_zundamon_wave_b1.md §2 (W-04)、docs/04_implementation/05_sse_protocol_spec.md §4.2
package unit

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/sse"
)

type recordedEvent struct {
	Name string
	Data any
}

// recordingConn は受信イベントを記録する Conn。Broadcast は複数 goroutine から
// 呼ばれ得るため記録を mutex で保護する。
type recordingConn struct {
	mu     sync.Mutex
	events []recordedEvent
	err    error
}

func newRecordingConn() *recordingConn {
	return &recordingConn{}
}

func newFailingConn(err error) *recordingConn {
	return &recordingConn{err: err}
}

func (c *recordingConn) WriteEvent(name string, data any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, recordedEvent{Name: name, Data: data})
	return c.err
}

func (c *recordingConn) recorded() []recordedEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]recordedEvent(nil), c.events...)
}

func (c *recordingConn) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

var _ sse.Conn = (*recordingConn)(nil)

func TestHub_Broadcast(t *testing.T) {
	const convID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("W-04-01_登録した接続にイベント名とペイロードがそのまま届く", func(t *testing.T) {
		hub := sse.NewHub()
		conn := newRecordingConn()
		hub.Register(convID, conn)
		payload := map[string]string{"request_id": "01JREQ", "label": "喜び"}

		hub.Broadcast(convID, "emotion", payload)

		require.Len(t, conn.recorded(), 1)
		assert.Equal(t, "emotion", conn.recorded()[0].Name)
		assert.Equal(t, payload, conn.recorded()[0].Data)
	})

	t.Run("W-04-02_同一会話の3接続すべてに届く", func(t *testing.T) {
		hub := sse.NewHub()
		conns := []*recordingConn{newRecordingConn(), newRecordingConn(), newRecordingConn()}
		for _, c := range conns {
			hub.Register(convID, c)
		}

		hub.Broadcast(convID, "text", map[string]string{"chunk": "なのだ"})

		for i, c := range conns {
			require.Len(t, c.recorded(), 1, "接続%dに届いていない（会話履歴は全ユーザー共有・C-06）", i)
			assert.Equal(t, "text", c.recorded()[0].Name)
		}
	})

	t.Run("W-04-03_別会話の接続には届かない", func(t *testing.T) {
		hub := sse.NewHub()
		mine := newRecordingConn()
		other := newRecordingConn()
		hub.Register(convID, mine)
		hub.Register("01ARZ3NDEKTSV4RRFFQ69G5FBW", other)

		hub.Broadcast(convID, "text", map[string]string{"chunk": "なのだ"})

		assert.Len(t, mine.recorded(), 1)
		assert.Empty(t, other.recorded(), "他人の会話にイベントが漏れている（重大バグ）")
	})

	t.Run("W-04-04_unregister後は届かない", func(t *testing.T) {
		hub := sse.NewHub()
		conn := newRecordingConn()
		unregister := hub.Register(convID, conn)

		unregister()
		hub.Broadcast(convID, "text", map[string]string{"chunk": "なのだ"})

		assert.Empty(t, conn.recorded())
	})

	t.Run("W-04-05_unregisterを2回呼んでもパニックしない", func(t *testing.T) {
		hub := sse.NewHub()
		conn := newRecordingConn()
		unregister := hub.Register(convID, conn)

		assert.NotPanics(t, func() {
			unregister()
			unregister()
		}, "handlerのdeferとHub側の失敗時解除で二重解除は普通に起きる")

		hub.Broadcast(convID, "text", map[string]string{"chunk": "なのだ"})
		assert.Empty(t, conn.recorded())
	})

	t.Run("W-04-06_書き込み失敗した接続は解除され同一会話の他の接続には届き続ける", func(t *testing.T) {
		hub := sse.NewHub()
		dead := newFailingConn(errors.New("sink overflow"))
		alive := newRecordingConn()
		hub.Register(convID, dead)
		hub.Register(convID, alive)

		hub.Broadcast(convID, "text", map[string]string{"chunk": "1回目"})
		hub.Broadcast(convID, "text", map[string]string{"chunk": "2回目"})

		assert.Equal(t, 1, dead.count(), "失敗した接続は1回目で解除され2回目は来ないべき")
		assert.Equal(t, 2, alive.count(), "1接続の失敗で他の接続への配信を止めてはならない")
	})

	t.Run("W-04-07_登録0件の会話へのBroadcastは何もせずパニックもしない", func(t *testing.T) {
		hub := sse.NewHub()

		assert.NotPanics(t, func() {
			hub.Broadcast("01ARZ3NDEKTSV4RRFFQ69G5FCX", "done", map[string]string{"request_id": "01J"})
		}, "誰も見ていない会話への配信は正常系")
	})

	t.Run("W-04-08_解除後に再登録すると再び届く", func(t *testing.T) {
		hub := sse.NewHub()
		conn := newRecordingConn()
		unregister := hub.Register(convID, conn)
		unregister()

		hub.Register(convID, conn)
		hub.Broadcast(convID, "text", map[string]string{"chunk": "なのだ"})

		assert.Len(t, conn.recorded(), 1, "会話のエントリを消しても再登録できるべき")
	})

	t.Run("W-04-09_同一Conn値を2回登録すると2回配信され解除は登録ごとに独立する", func(t *testing.T) {
		hub := sse.NewHub()
		conn := newRecordingConn()
		unregisterFirst := hub.Register(convID, conn)
		hub.Register(convID, conn)

		unregisterFirst()
		hub.Broadcast(convID, "text", map[string]string{"chunk": "なのだ"})

		assert.Equal(t, 1, conn.count(),
			"登録は接続ごとの識別子で管理され、1つ解除しても残りは生きているべき")
	})
}

func TestHub_10並列実行(t *testing.T) {
	// W-04-10: 既存 TestConversationRepository_10並列実行 と同じ流儀。
	// 登録・解除・配信を同時に走らせ、-race で競合が出ないことを実証する。
	hub := sse.NewHub()
	const parallelism = 10
	const iterations = 50

	var wg sync.WaitGroup
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			convID := fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5F%02d", idx)
			for n := 0; n < iterations; n++ {
				conn := newRecordingConn()
				unregister := hub.Register(convID, conn)
				hub.Broadcast(convID, "text", map[string]string{"chunk": "なのだ"})
				unregister()
				unregister() // 二重解除も並列下で安全であること
			}
		}(i)
	}

	// 同一会話へ複数 goroutine から同時に登録・配信するケースも混ぜる。
	const sharedConvID = "01ARZ3NDEKTSV4RRFFQ69G5FSH"
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn := newRecordingConn()
			unregister := hub.Register(sharedConvID, conn)
			defer unregister()
			for n := 0; n < iterations; n++ {
				hub.Broadcast(sharedConvID, "text", map[string]string{"chunk": "なのだ"})
			}
		}()
	}
	wg.Wait()

	// 全解除後は配信先が残っていないこと（リーク検知）。
	after := newRecordingConn()
	hub.Register(sharedConvID, after)
	hub.Broadcast(sharedConvID, "done", map[string]string{"request_id": "01J"})
	assert.Equal(t, 1, after.count(), "解除済みの接続が残留していないこと")
}

func TestHub_ConnCount(t *testing.T) {
	// W-06 のハンドラテストで「切断後に Hub から解除されたこと」を観測するために必要
	// （指示書§1.4 の「切断後に Broadcast しても登録数が0」を検証可能にする）。
	const convID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

	t.Run("W-04-11_登録も無い会話は0を返す", func(t *testing.T) {
		hub := sse.NewHub()

		assert.Equal(t, 0, hub.ConnCount(convID))
	})

	t.Run("W-04-12_登録数がそのまま返る", func(t *testing.T) {
		hub := sse.NewHub()
		hub.Register(convID, newRecordingConn())
		hub.Register(convID, newRecordingConn())

		assert.Equal(t, 2, hub.ConnCount(convID))
	})

	t.Run("W-04-13_解除すると減り全解除で0になる", func(t *testing.T) {
		hub := sse.NewHub()
		first := hub.Register(convID, newRecordingConn())
		second := hub.Register(convID, newRecordingConn())

		first()
		assert.Equal(t, 1, hub.ConnCount(convID))

		second()
		assert.Equal(t, 0, hub.ConnCount(convID))
	})

	t.Run("W-04-14_会話ごとに独立して数える", func(t *testing.T) {
		hub := sse.NewHub()
		other := "01ARZ3NDEKTSV4RRFFQ69G5FBW"
		hub.Register(convID, newRecordingConn())
		hub.Register(other, newRecordingConn())
		hub.Register(other, newRecordingConn())

		assert.Equal(t, 1, hub.ConnCount(convID))
		assert.Equal(t, 2, hub.ConnCount(other))
	})

	t.Run("W-04-15_書き込み失敗で解除された接続は数から外れる", func(t *testing.T) {
		hub := sse.NewHub()
		hub.Register(convID, newFailingConn(errors.New("client gone")))
		hub.Register(convID, newRecordingConn())
		require.Equal(t, 2, hub.ConnCount(convID))

		hub.Broadcast(convID, "text", map[string]string{"chunk": "なのだ"})

		assert.Equal(t, 1, hub.ConnCount(convID))
	})
}
