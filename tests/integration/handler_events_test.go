// GET /conversations/{id}/events の結合テスト。
// 対応: tasks/instructions_zundamon_wave_b2.md §1.2/§1.4、docs/04_implementation/05_sse_protocol_spec.md §2
package integration

import (
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerEvents(t *testing.T) {
	t.Run("W-06-V1_不正なULIDは400", func(t *testing.T) {
		f := newHandlerFixture(t)

		resp, err := f.client.Get(f.server.URL + "/conversations/not-a-ulid/events")

		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("W-06-V2_存在しない会話は404", func(t *testing.T) {
		f := newHandlerFixture(t)

		resp, err := f.client.Get(f.server.URL + "/conversations/" + ulid.Make().String() + "/events")

		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode,
			"存在しない会話へ延々と接続させない")
	})

	t.Run("W-06-V3_正常接続でSSEヘッダ4種が設定される", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)

		c := f.connectEvents(t, convID)

		assert.Equal(t, http.StatusOK, c.resp.StatusCode)
		assert.Equal(t, "text/event-stream", c.resp.Header.Get("Content-Type"))
		assert.Equal(t, "no-cache", c.resp.Header.Get("Cache-Control"))
		assert.Equal(t, "no", c.resp.Header.Get("X-Accel-Buffering"))
	})

	t.Run("W-06-V4_接続直後にretry3000が流れてくる", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		c := f.connectEvents(t, convID)

		lines := c.readFrame(t)

		require.Len(t, lines, 1)
		assert.Equal(t, "retry: 3000", lines[0], "再接続間隔のサーバー指示（F-RT-02）")
	})

	t.Run("W-06-V5_Broadcastしたイベントをクライアントが受信できる", func(t *testing.T) {
		// 接続からブロードキャストまでのエンドツーエンド疎通証明。
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		c := f.connectEvents(t, convID)
		require.Equal(t, []string{"retry: 3000"}, c.readFrame(t))
		waitConnCount(t, f.hub, convID, 1)

		f.hub.Broadcast(convID, "emotion", map[string]string{"request_id": "01JREQ", "label": "喜び"})

		name, data := c.readEvent(t)
		assert.Equal(t, "emotion", name)
		assert.JSONEq(t, `{"request_id":"01JREQ","label":"喜び"}`, data)
	})

	t.Run("W-06-V6_同一会話の2接続に同じイベントが届く", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		first := f.connectEvents(t, convID)
		second := f.connectEvents(t, convID)
		require.Equal(t, []string{"retry: 3000"}, first.readFrame(t))
		require.Equal(t, []string{"retry: 3000"}, second.readFrame(t))
		waitConnCount(t, f.hub, convID, 2)

		f.hub.Broadcast(convID, "done", map[string]string{"request_id": "01JREQ"})

		for i, c := range []*sseClient{first, second} {
			name, data := c.readEvent(t)
			assert.Equal(t, "done", name, "接続%dに届いていない", i)
			assert.JSONEq(t, `{"request_id":"01JREQ"}`, data)
		}
	})

	t.Run("W-06-V7_クライアント切断でHubから解除される", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		c := f.connectEvents(t, convID)
		require.Equal(t, []string{"retry: 3000"}, c.readFrame(t))
		waitConnCount(t, f.hub, convID, 1)

		c.close()

		// defer unregister() が無いと接続が Hub に残り続け、メモリと配信コストが積む。
		waitConnCount(t, f.hub, convID, 0)
	})

	t.Run("W-06-V8_1本切断してももう1本には届き続ける", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		gone := f.connectEvents(t, convID)
		alive := f.connectEvents(t, convID)
		require.Equal(t, []string{"retry: 3000"}, gone.readFrame(t))
		require.Equal(t, []string{"retry: 3000"}, alive.readFrame(t))
		waitConnCount(t, f.hub, convID, 2)

		gone.close()
		waitConnCount(t, f.hub, convID, 1)
		f.hub.Broadcast(convID, "done", map[string]string{"request_id": "01JREQ"})

		name, _ := alive.readEvent(t)
		assert.Equal(t, "done", name)
	})
}
