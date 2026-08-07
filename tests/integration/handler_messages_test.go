// POST /conversations/{id}/messages の結合テスト。
// 対応: tasks/instructions_zundamon_wave_b2.md §1.3/§1.4、docs/04_implementation/05_sse_protocol_spec.md §3
package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRequestID = "01JREQ0123456789ABCDEFGHJK"

func TestHandlerPostMessage_ステータスコード(t *testing.T) {
	f := newHandlerFixture(t)
	convID := f.seedConversation(t)

	tests := []struct {
		name       string
		convID     string
		body       string
		wantStatus int
	}{
		{
			name:       "正常な発話は202",
			convID:     convID,
			body:       fmt.Sprintf(`{"request_id":%q,"text":"こんにちは"}`, testRequestID),
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "会話IDが不正なULIDなら400",
			convID:     "not-a-ulid",
			body:       fmt.Sprintf(`{"request_id":%q,"text":"こんにちは"}`, testRequestID),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "request_idが不正なULIDなら400",
			convID:     convID,
			body:       `{"request_id":"short","text":"こんにちは"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "request_idが空なら400",
			convID:     convID,
			body:       `{"request_id":"","text":"こんにちは"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "textが空なら400",
			convID:     convID,
			body:       fmt.Sprintf(`{"request_id":%q,"text":""}`, ulid.Make().String()),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "textが空白のみなら400",
			convID:     convID,
			body:       fmt.Sprintf(`{"request_id":%q,"text":"   \t\n"}`, ulid.Make().String()),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "textが全角スペースのみなら400",
			convID:     convID,
			body:       fmt.Sprintf(`{"request_id":%q,"text":"　　"}`, ulid.Make().String()),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "JSONが壊れていたら400",
			convID:     convID,
			body:       `{"request_id":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "存在しない会話なら404",
			convID:     ulid.Make().String(),
			body:       fmt.Sprintf(`{"request_id":%q,"text":"こんにちは"}`, ulid.Make().String()),
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := f.postMessage(t, tt.convID, tt.body)

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

func TestHandlerPostMessage(t *testing.T) {
	t.Run("W-06-M1_202のレスポンス本文はrequest_idを返す", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)

		resp := f.postMessage(t, convID,
			fmt.Sprintf(`{"request_id":%q,"text":"こんにちは"}`, testRequestID))

		require.Equal(t, http.StatusAccepted, resp.StatusCode)
		var got map[string]string
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		assert.Equal(t, map[string]string{"request_id": testRequestID}, got)
	})

	t.Run("W-06-M2_不正なUTF-8はjsonデコーダがU_FFFDへ置換しDBは壊れない", func(t *testing.T) {
		// 【実測に基づく重要な注記・めたんへ報告済み】
		// 指示書§1.3 は「不正UTF-8を utf8.ValidString で 400 に弾く」としているが、
		// encoding/json の Decode は文字列値の不正UTF-8バイトと孤立サロゲートを
		// U+FFFD(ef bf bd) へ置換するため、req.Text に不正UTF-8が残ることは無い。
		// 実測: {"text":"\xff\xfe"} → bytes=ef bf bd ef bf bd / ValidString=true
		//       {"text":"\ud800"}  → bytes=ef bf bd          / ValidString=true
		// したがって JSON 本文経路では 400 にならず、DBの encoding エラーも起きない。
		// ハンドラの utf8.ValidString ガードは多層防御として残してあるが、この経路からは
		// 到達不能であり、実際に効くのは JSON を通さない経路（§1.3 末尾のSTT結果）である。
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		body := fmt.Sprintf("{\"request_id\":%q,\"text\":\"\xff\xfe\"}", testRequestID)

		resp := f.postMessage(t, convID, body)

		require.Equal(t, http.StatusAccepted, resp.StatusCode,
			"json デコーダが置換済みなので不正UTF-8としては検出されない")
		waitAssistantMessage(t, f.db, convID)
		assert.Equal(t, 1,
			countRows(t, f.db, "messages", "conversation_id = $1 AND role = 'user' AND content = $2",
				convID, "��"),
			"U+FFFD へ置換された状態で保存され、INSERT は encoding エラーにならない")
		_, _, firstText := queryConversationRow(t, f.db, convID)
		require.True(t, firstText.Valid)
		assert.Equal(t, "��", firstText.String)
	})

	t.Run("W-06-M3_同一request_idを2回POSTしてもLLM呼び出しは1回", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		body := fmt.Sprintf(`{"request_id":%q,"text":"こんにちは"}`, testRequestID)

		first := f.postMessage(t, convID, body)
		require.Equal(t, http.StatusAccepted, first.StatusCode)
		waitAssistantMessage(t, f.db, convID)

		second := f.postMessage(t, convID, body)

		assert.Equal(t, http.StatusAccepted, second.StatusCode, "再送も202で受理する（冪等）")
		// 2回目が処理されないことを確認するための猶予。
		time.Sleep(200 * time.Millisecond)
		assert.Equal(t, 1, f.llmC.callCount(), "二重送信の第二防衛線（仕様書§3.3）")
		assert.Equal(t, 1, countRows(t, f.db, "messages", "conversation_id = $1 AND role = 'user'", convID))
	})

	t.Run("W-06-M4_202を返した後も応答生成が完走する", func(t *testing.T) {
		// context.WithoutCancel が効いていることの証明。
		// r.Context() をそのまま渡すと 202 返却時点でキャンセルされ応答生成が即死する。
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		release := make(chan struct{})
		f.llmC.mu.Lock()
		f.llmC.release = release
		f.llmC.mu.Unlock()

		resp := f.postMessage(t, convID,
			fmt.Sprintf(`{"request_id":%q,"text":"こんにちは"}`, testRequestID))
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
		require.NoError(t, resp.Body.Close()) // HTTPレスポンス完了 = r.Context() はキャンセル済み

		close(release) // ここでLLMを進ませる

		content := waitAssistantMessage(t, f.db, convID)
		assert.Equal(t, "こんにちはなのだ。", content,
			"202返却後もキャンセルされずに応答生成が完走すること")
		assert.Empty(t, f.llmC.contextErrors(),
			"LLM呼び出しがctxキャンセルで打ち切られていないこと")
	})

	t.Run("W-06-M5_応答イベントがevents側へemotion_text_doneの順で流れる", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		c := f.connectEvents(t, convID)
		require.Equal(t, []string{"retry: 3000"}, c.readFrame(t))
		waitConnCount(t, f.hub, convID, 1)

		resp := f.postMessage(t, convID,
			fmt.Sprintf(`{"request_id":%q,"text":"こんにちは"}`, testRequestID))
		require.Equal(t, http.StatusAccepted, resp.StatusCode)

		names := make([]string, 0, 3)
		for i := 0; i < 3; i++ {
			name, data := c.readEvent(t)
			names = append(names, name)
			assert.Contains(t, data, testRequestID, "全イベントにrequest_idが入る（仕様書§2.2）")
		}
		assert.Equal(t, []string{"emotion", "text", "done"}, names,
			"TTS未実装なので audio_url はスキップされ done へ進む")
	})

	t.Run("W-06-M6_ユーザー発話が保存されfirst_textにも記録される", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)

		resp := f.postMessage(t, convID,
			fmt.Sprintf(`{"request_id":%q,"text":"こんにちはずんだもん"}`, testRequestID))
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
		waitAssistantMessage(t, f.db, convID)

		assert.Equal(t, 1,
			countRows(t, f.db, "messages", "conversation_id = $1 AND role = 'user' AND content = $2",
				convID, "こんにちはずんだもん"))
		_, _, firstText := queryConversationRow(t, f.db, convID)
		require.True(t, firstText.Valid)
		assert.Equal(t, "こんにちはずんだもん", firstText.String)
	})

	t.Run("W-06-M7_プロンプトに自分の発話が含まれる", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)

		resp := f.postMessage(t, convID,
			fmt.Sprintf(`{"request_id":%q,"text":"きょうはいい天気なのだ"}`, testRequestID))
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
		waitAssistantMessage(t, f.db, convID)

		prompts := f.llmC.recordedPrompts()
		require.Len(t, prompts, 1)
		assert.Equal(t, "user: きょうはいい天気なのだ", prompts[0],
			"保存 → 履歴取得の順なので自分の発話が文脈に入る")
	})
}
