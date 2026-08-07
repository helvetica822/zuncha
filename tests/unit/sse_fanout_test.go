// sse.Fanout（EventSink 実装）の単体テスト。
// 対応: tasks/instructions_zundamon_wave_b1.md §1.4 (W-03)、docs/04_implementation/05_sse_protocol_spec.md §2.2/§4.2
package unit

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/sse"
)

const (
	fanoutConvID    = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	fanoutOtherConv = "01ARZ3NDEKTSV4RRFFQ69G5FBW"
	fanoutRequestID = "01JREQUEST0000000000000000"
)

func TestFanout_イベント種別ごとのペイロード(t *testing.T) {
	tests := []struct {
		name     string
		send     func(sink sse.EventSink) error
		wantName string
		wantData map[string]string
	}{
		{
			name:     "SendEmotionはemotionイベントとlabelを送る",
			send:     func(sink sse.EventSink) error { return sink.SendEmotion("喜び") },
			wantName: "emotion",
			wantData: map[string]string{"request_id": fanoutRequestID, "label": "喜び"},
		},
		{
			name:     "SendTextChunkはtextイベントとchunkを送る",
			send:     func(sink sse.EventSink) error { return sink.SendTextChunk("ずんだもんなのだ。") },
			wantName: "text",
			wantData: map[string]string{"request_id": fanoutRequestID, "chunk": "ずんだもんなのだ。"},
		},
		{
			name:     "SendAudioURLはaudio_urlイベントとurlを送る",
			send:     func(sink sse.EventSink) error { return sink.SendAudioURL("/audio/01JAUDIO") },
			wantName: "audio_url",
			wantData: map[string]string{"request_id": fanoutRequestID, "url": "/audio/01JAUDIO"},
		},
		{
			name:     "SendDoneはdoneイベントとrequest_idのみを送る",
			send:     func(sink sse.EventSink) error { return sink.SendDone() },
			wantName: "done",
			wantData: map[string]string{"request_id": fanoutRequestID},
		},
		{
			name:     "SendErrorはerrorイベントとmessageを送る",
			send:     func(sink sse.EventSink) error { return sink.SendError("応答の生成に失敗しました") },
			wantName: "error",
			wantData: map[string]string{"request_id": fanoutRequestID, "message": "応答の生成に失敗しました"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := sse.NewHub()
			conn := newRecordingConn()
			hub.Register(fanoutConvID, conn)
			sink := sse.NewFanout(hub, fanoutConvID, fanoutRequestID)

			err := tt.send(sink)

			require.NoError(t, err)
			require.Len(t, conn.recorded(), 1)
			assert.Equal(t, tt.wantName, conn.recorded()[0].Name)
			assert.Equal(t, tt.wantData, conn.recorded()[0].Data,
				"request_id は全イベントに注入されるべき（仕様書§2.2）")
		})
	}
}

func TestFanout(t *testing.T) {
	t.Run("W-03-F1_接続が書き込みに失敗してもnilを返す", func(t *testing.T) {
		hub := sse.NewHub()
		hub.Register(fanoutConvID, newFailingConn(sse.ErrSinkOverflow))
		sink := sse.NewFanout(hub, fanoutConvID, fanoutRequestID)

		assert.NoError(t, sink.SendEmotion("喜び"),
			"1接続に届かなかったことでResponseStreamer全体を中断させてはならない")
		assert.NoError(t, sink.SendTextChunk("なのだ"))
		assert.NoError(t, sink.SendAudioURL("/audio/01J"))
		assert.NoError(t, sink.SendError("失敗"))
		assert.NoError(t, sink.SendDone())
	})

	t.Run("W-03-F2_接続が0件でもnilを返す", func(t *testing.T) {
		hub := sse.NewHub()
		sink := sse.NewFanout(hub, fanoutConvID, fanoutRequestID)

		assert.NoError(t, sink.SendDone(), "誰も見ていない会話への配信は正常系")
	})

	t.Run("W-03-F3_他の会話の接続には流れない", func(t *testing.T) {
		hub := sse.NewHub()
		mine := newRecordingConn()
		other := newRecordingConn()
		hub.Register(fanoutConvID, mine)
		hub.Register(fanoutOtherConv, other)
		sink := sse.NewFanout(hub, fanoutConvID, fanoutRequestID)

		require.NoError(t, sink.SendTextChunk("なのだ"))

		assert.Len(t, mine.recorded(), 1)
		assert.Empty(t, other.recorded(), "会話をまたいでイベントが漏れている")
	})

	t.Run("W-03-F4_同一会話の別リクエストは異なるrequest_idで区別される", func(t *testing.T) {
		hub := sse.NewHub()
		conn := newRecordingConn()
		hub.Register(fanoutConvID, conn)
		first := sse.NewFanout(hub, fanoutConvID, "01JREQFIRST000000000000000")
		second := sse.NewFanout(hub, fanoutConvID, "01JREQSECOND00000000000000")

		require.NoError(t, first.SendDone())
		require.NoError(t, second.SendDone())

		events := conn.recorded()
		require.Len(t, events, 2)
		assert.Equal(t, map[string]string{"request_id": "01JREQFIRST000000000000000"}, events[0].Data)
		assert.Equal(t, map[string]string{"request_id": "01JREQSECOND00000000000000"}, events[1].Data)
	})

	t.Run("W-03-F5_同一会話の3接続すべてに同じペイロードが届く", func(t *testing.T) {
		hub := sse.NewHub()
		conns := []*recordingConn{newRecordingConn(), newRecordingConn(), newRecordingConn()}
		for _, c := range conns {
			hub.Register(fanoutConvID, c)
		}
		sink := sse.NewFanout(hub, fanoutConvID, fanoutRequestID)

		require.NoError(t, sink.SendEmotion("困惑"))

		for i, c := range conns {
			require.Len(t, c.recorded(), 1, "接続%dに届いていない", i)
			assert.Equal(t,
				map[string]string{"request_id": fanoutRequestID, "label": "困惑"},
				c.recorded()[0].Data)
		}
	})

	t.Run("W-03-F6_空文字列のrequest_idでも接続レベルエラーとして送出できる", func(t *testing.T) {
		// 仕様書§2.2: どのリクエストにも属さない障害では request_id を空文字列にする。
		hub := sse.NewHub()
		conn := newRecordingConn()
		hub.Register(fanoutConvID, conn)
		sink := sse.NewFanout(hub, fanoutConvID, "")

		require.NoError(t, sink.SendError("接続が切断されました"))

		require.Len(t, conn.recorded(), 1)
		assert.Equal(t,
			map[string]string{"request_id": "", "message": "接続が切断されました"},
			conn.recorded()[0].Data)
	})

	t.Run("W-03-F7_書き込み失敗した接続は解除され健全な接続には届き続ける", func(t *testing.T) {
		hub := sse.NewHub()
		dead := newFailingConn(errors.New("client gone"))
		alive := newRecordingConn()
		hub.Register(fanoutConvID, dead)
		hub.Register(fanoutConvID, alive)
		sink := sse.NewFanout(hub, fanoutConvID, fanoutRequestID)

		require.NoError(t, sink.SendTextChunk("1回目"))
		require.NoError(t, sink.SendTextChunk("2回目"))

		assert.Equal(t, 1, dead.count())
		assert.Equal(t, 2, alive.count())
	})
}
