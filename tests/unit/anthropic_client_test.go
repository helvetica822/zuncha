// 対応仕様: docs/04_implementation/04_realtime_wiring_design.md 決-1/決-1a、
// tasks/instructions_zundamon_wave_c1.md §5.2（W-08 Claude API クライアント）
//
// 本テストは httptest のフェイクサーバのみで駆動し、実APIへは一切接続しない。
package unit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/anthropic"
)

const (
	// 仕様上の確定値（指示書 §2）。実装側の定数とは独立に、テスト側で仕様を再掲する。
	wantModel     = "claude-opus-5"
	wantMaxTokens = 16000
	// リトライ既定値2に対し WithMaxRetries(1) を明示している。したがって429時の総リクエスト数は2。
	wantRequestsOn429 = 2

	fakeAPIKey = "sk-ant-test-DUMMY-KEY-DO-NOT-USE"
)

// newFakeAPI は Claude API を模した httptest サーバを立て、そこへ向いた Client を返す。
func newFakeAPI(t *testing.T, handler http.HandlerFunc) (*anthropic.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := anthropic.NewClient(fakeAPIKey, anthropic.WithBaseURL(srv.URL+"/"))
	require.NoError(t, err)
	return c, srv
}

// writeMessage は Messages API の 200 応答（Message オブジェクト）を書き出す。
func writeMessage(t *testing.T, w http.ResponseWriter, stopReason string, content []map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"id":          "msg_fake_0001",
		"type":        "message",
		"role":        "assistant",
		"model":       wantModel,
		"stop_reason": stopReason,
		"content":     content,
		"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
	}))
}

func textBlock(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func writeAPIError(t *testing.T, w http.ResponseWriter, status int, errType string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]any{"type": errType, "message": "fake error"},
	}))
}

// --- 正常系 -----------------------------------------------------------------

func TestGenerateResponse_単一TextBlockのJSONをそのままバイト列で返す(t *testing.T) {
	const want = `{"text":"こんにちはなのだ","emotion":"喜び"}`

	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeMessage(t, w, "end_turn", []map[string]any{textBlock(want)})
	})

	got, err := c.GenerateResponse(context.Background(), "こんにちは")
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

func TestGenerateResponse_リクエストボディが確定仕様どおりである(t *testing.T) {
	var body map[string]any

	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, &body))
		writeMessage(t, w, "end_turn", []map[string]any{textBlock(`{"text":"のだ","emotion":"喜び"}`)})
	})

	_, err := c.GenerateResponse(context.Background(), "テスト発話")
	require.NoError(t, err)

	// モデルIDは日付サフィックスなしの素の文字列。
	assert.Equal(t, wantModel, body["model"])
	// MaxTokens は thinking と応答テキストの合計上限なので絞らない。
	assert.Equal(t, float64(wantMaxTokens), body["max_tokens"])

	// system にペルソナ指示が入っていること（プロンプトキャッシュ対象）。
	systemRaw, err := json.Marshal(body["system"])
	require.NoError(t, err)
	system := string(systemRaw)
	assert.Contains(t, system, "ずんだもん")
	assert.Contains(t, system, "emotion")
	assert.Contains(t, system, "ephemeral", "システムプロンプトに CacheControl が付いていること")

	// 温度系パラメータは Opus 5 では 400 になるため、キー自体が存在してはならない。
	for _, key := range []string{"temperature", "top_p", "top_k"} {
		_, ok := body[key]
		assert.Falsef(t, ok, "%s キーは送信してはならない", key)
	}

	// thinking は無効化しない（<thinking> タグ漏洩の既知失敗モードを避ける）。
	_, hasThinking := body["thinking"]
	assert.False(t, hasThinking, "thinking を明示指定してはならない（既定の adaptive のまま）")

	// output_config.effort は low（音声会話の待ち時間を抑える）。
	outputConfig, ok := body["output_config"].(map[string]any)
	require.True(t, ok, "output_config が送信されていること")
	assert.Equal(t, "low", outputConfig["effort"])
}

func TestGenerateResponse_複数TextBlockは連結される(t *testing.T) {
	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeMessage(t, w, "end_turn", []map[string]any{
			textBlock(`{"text":"こんにちは`),
			textBlock(`なのだ","emotion":"喜び"}`),
		})
	})

	got, err := c.GenerateResponse(context.Background(), "こんにちは")
	require.NoError(t, err)
	assert.Equal(t, `{"text":"こんにちはなのだ","emotion":"喜び"}`, string(got))
}

func TestGenerateResponse_ThinkingBlockは連結対象に含めない(t *testing.T) {
	const want = `{"text":"わかったのだ","emotion":"ドヤ顔"}`

	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeMessage(t, w, "end_turn", []map[string]any{
			{"type": "thinking", "thinking": "ユーザーは挨拶している。まず…", "signature": "sig"},
			textBlock(want),
		})
	})

	got, err := c.GenerateResponse(context.Background(), "こんにちは")
	require.NoError(t, err)
	assert.Equal(t, want, string(got), "thinking の内容が混ざってはならない")
}

// --- 異常系 -----------------------------------------------------------------

func TestGenerateResponse_401はStatusCode付きのAPIエラーになる(t *testing.T) {
	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(t, w, http.StatusUnauthorized, "authentication_error")
	})

	_, err := c.GenerateResponse(context.Background(), "こんにちは")
	require.Error(t, err)

	var apiErr *sdk.Error
	require.True(t, errors.As(err, &apiErr), "errors.As で *anthropic.Error を取り出せること")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
}

func TestGenerateResponse_429はWithMaxRetries1により2回で打ち切られる(t *testing.T) {
	var calls atomic.Int32

	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		writeAPIError(t, w, http.StatusTooManyRequests, "rate_limit_error")
	})

	_, err := c.GenerateResponse(context.Background(), "こんにちは")
	require.Error(t, err)

	var apiErr *sdk.Error
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	assert.Equal(t, int32(wantRequestsOn429), calls.Load(),
		"既定のリトライ回数2のままなら3回になる。WithMaxRetries(1) の実効性を担保する検証")
}

func TestGenerateResponse_拒否応答はErrRefusedを返しパニックしない(t *testing.T) {
	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		// 拒否時は content が空または部分的。Content[0] を無条件に読む実装は落ちる。
		writeMessage(t, w, "refusal", []map[string]any{})
	})

	require.NotPanics(t, func() {
		_, err := c.GenerateResponse(context.Background(), "こんにちは")
		require.Error(t, err)
		assert.ErrorIs(t, err, anthropic.ErrRefused)
	})
}

func TestGenerateResponse_TextBlockが1つも無ければエラー(t *testing.T) {
	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeMessage(t, w, "end_turn", []map[string]any{
			{"type": "thinking", "thinking": "…", "signature": "sig"},
		})
	})

	_, err := c.GenerateResponse(context.Background(), "こんにちは")
	require.Error(t, err)
	assert.NotErrorIs(t, err, anthropic.ErrRefused)
}

func TestGenerateResponse_キャンセル済みctxはcontextCanceledを返す(t *testing.T) {
	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeMessage(t, w, "end_turn", []map[string]any{textBlock(`{"text":"のだ","emotion":"喜び"}`)})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GenerateResponse(ctx, "こんにちは")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- ログ -------------------------------------------------------------------

func TestGenerateResponse_エラー経路でAPIキーと発話内容をログに出さない(t *testing.T) {
	const secretPrompt = "user: 誰にも知られたくない秘密の発話内容なのだ"

	var buf strings.Builder
	origOut := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(origOut)
		log.SetFlags(origFlags)
	})

	c, _ := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(t, w, http.StatusInternalServerError, "api_error")
	})

	_, err := c.GenerateResponse(context.Background(), secretPrompt)
	require.Error(t, err)

	logged := buf.String()
	assert.NotContains(t, logged, fakeAPIKey, "APIキーがログに出てはならない")
	assert.NotContains(t, logged, "秘密の発話内容", "プロンプト本文がログに出てはならない")

	// 返却エラー自体にも混入させない（エラーはハンドラでログされ得るため）。
	assert.NotContains(t, err.Error(), fakeAPIKey)
	assert.NotContains(t, err.Error(), "秘密の発話内容")
}

// --- コンストラクタ ---------------------------------------------------------

func TestNewClient_空のAPIキーはエラー(t *testing.T) {
	c, err := anthropic.NewClient("")
	require.Error(t, err)
	assert.Nil(t, c)
}
