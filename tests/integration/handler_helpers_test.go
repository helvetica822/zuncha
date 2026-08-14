// ハンドラ結合テスト用の共通フェイクとヘルパー。
// 対応: tasks/instructions_zundamon_wave_b2.md §1.4 (W-06)
package integration

import (
	"bufio"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"zuncha/internal/handler"
	"zuncha/internal/llm"
	"zuncha/internal/localfs"
	"zuncha/internal/postgres"
	"zuncha/internal/service"
	"zuncha/internal/sse"
	"zuncha/internal/stt"
)

// fakeLLMClient は呼び出し回数とプロンプトを記録する LLMClient。
// release が非nilの場合、そこが閉じられる（またはctxがキャンセルされる）まで待つ。
// これにより「202を返した後も応答生成が完走するか」を確定的に検証できる。
type fakeLLMClient struct {
	mu      sync.Mutex
	calls   int
	prompts []string
	ctxErrs []error

	release chan struct{}
	body    []byte
}

func newFakeLLMClient() *fakeLLMClient {
	return &fakeLLMClient{body: []byte(`{"text":"こんにちはなのだ。","emotion":"喜び"}`)}
}

func (f *fakeLLMClient) GenerateResponse(ctx context.Context, prompt string) ([]byte, error) {
	f.mu.Lock()
	f.calls++
	f.prompts = append(f.prompts, prompt)
	release := f.release
	body := f.body
	f.mu.Unlock()

	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			// context.WithoutCancel を外すとここに落ちる（＝応答生成が即死する）。
			f.mu.Lock()
			f.ctxErrs = append(f.ctxErrs, ctx.Err())
			f.mu.Unlock()
			return nil, ctx.Err()
		}
	}
	return body, nil
}

func (f *fakeLLMClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeLLMClient) recordedPrompts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.prompts...)
}

func (f *fakeLLMClient) contextErrors() []error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]error(nil), f.ctxErrs...)
}

// fakeResponseParser は固定のパース結果を返す。
type fakeResponseParser struct{}

func (fakeResponseParser) Parse(body []byte) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Text: "こんにちはなのだ。", Emotion: "喜び"}, nil
}

// fakeTTSClient は常に失敗する（ハンドラ結合テストでは audio_url をスキップさせる。
// 実 VOICEVOX への依存をテストへ持ち込まない）。
// W-09 で Synthesize の引数に conversationID/messageID が加わった（D-4 訂正1）。
type fakeTTSClient struct{}

func (fakeTTSClient) Synthesize(ctx context.Context, text, conversationID, messageID string) (string, error) {
	return "", context.Canceled
}

// fakeAudioConverter は ffmpeg を呼ばずに変換結果（またはエラー）を返す。
// ffmpeg はこの開発環境に存在しないため、結合テストへ実バイナリ依存を持ち込まない。
type fakeAudioConverter struct {
	mu     sync.Mutex
	calls  int
	inputs [][]byte

	out []byte
	err error
}

func (f *fakeAudioConverter) Convert(_ context.Context, input []byte) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.inputs = append(f.inputs, append([]byte(nil), input...))
	return f.out, f.err
}

func (f *fakeAudioConverter) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeAudioConverter) recordedInputs() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.inputs...)
}

// fakeSTTClient は whisper-server を呼ばずに認識結果（またはエラー）を返す。
type fakeSTTClient struct {
	mu     sync.Mutex
	calls  int
	result stt.STTResult
	err    error
}

func (f *fakeSTTClient) Transcribe(_ context.Context, _ []byte) (stt.STTResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.result, f.err
}

func (f *fakeSTTClient) setResult(result stt.STTResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.result = result
	f.err = nil
}

func (f *fakeSTTClient) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeSTTClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// handlerFixture はハンドラ結合テストの一式を保持する。
type handlerFixture struct {
	server  *httptest.Server
	db      *sql.DB
	hub     *sse.Hub
	llmC    *fakeLLMClient
	sttConv *fakeAudioConverter
	sttC    *fakeSTTClient
	client  *http.Client
}

// newHandlerFixture は実DB・実Hub・フェイクLLMで組んだ HTTP サーバを起動する。
func newHandlerFixture(t *testing.T) *handlerFixture {
	t.Helper()
	h, db, hub, llmC, sttConv, sttC := newTestHandler(t)

	server := httptest.NewServer(h.Routes())
	t.Cleanup(server.Close)

	return &handlerFixture{
		server: server, db: db, hub: hub, llmC: llmC,
		sttConv: sttConv, sttC: sttC, client: server.Client(),
	}
}

// newTestHandler は実DB・実Hub・フェイクLLMでハンドラ一式を組む（サーバの起動方法は呼び出し側に委ねる）。
// グレースフル停止のテストは httptest ではなく実リスナ上で httpserver.Run を回す必要があるため、
// ハンドラの組み立てだけを切り出している。
func newTestHandler(t *testing.T) (
	*handler.Handler, *sql.DB, *sse.Hub, *fakeLLMClient, *fakeAudioConverter, *fakeSTTClient,
) {
	t.Helper()
	db := setupTestDB(t)

	convRepo := postgres.NewConversationRepository(db)
	msgRepo := postgres.NewMessageRepository(db)
	audioRepo := postgres.NewAudioRepository(db)
	hub := sse.NewHub()
	llmC := newFakeLLMClient()

	streamer := service.NewResponseStreamer(
		llmC, fakeResponseParser{}, fakeTTSClient{}, sse.NewSentenceChunker(),
	)
	chat := service.NewChatService(
		msgRepo, convRepo, streamer, hub,
		func() string { return ulid.Make().String() },
		time.Now,
	)
	sttConv := &fakeAudioConverter{out: []byte("RIFF-converted-wav")}
	sttC := &fakeSTTClient{result: stt.STTResult{Text: "こんにちはなのだ", Confidence: 0.9}}

	h := handler.NewHandler(
		service.NewCreateConversationService(convRepo),
		service.NewFetchAudioService(audioRepo, localfs.NewFileStore()),
		chat,
		service.NewSpeechToTextService(sttConv, sttC),
		convRepo, hub,
	)

	return h, db, hub, llmC, sttConv, sttC
}

// seedConversation は会話を1件作って IDを返す。
func (f *handlerFixture) seedConversation(t *testing.T) string {
	t.Helper()
	convID := ulid.Make().String()
	insertConversation(t, f.db, convID, time.Now())
	return convID
}

// postMessage は POST /conversations/{id}/messages を実行する。
func (f *handlerFixture) postMessage(t *testing.T, convID, body string) *http.Response {
	t.Helper()
	resp, err := f.client.Post(
		f.server.URL+"/conversations/"+convID+"/messages",
		"application/json",
		strings.NewReader(body),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// sseClient は /events への接続と受信行の読み取りを担う。
type sseClient struct {
	resp   *http.Response
	reader *bufio.Reader
	cancel context.CancelFunc
}

// connectEvents は /events へ接続し、レスポンスヘッダ受信までを終えた状態で返す。
func (f *handlerFixture) connectEvents(t *testing.T, convID string) *sseClient {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		f.server.URL+"/conversations/"+convID+"/events", nil)
	require.NoError(t, err)

	resp, err := f.client.Do(req)
	if err != nil {
		cancel()
		require.NoError(t, err)
	}
	c := &sseClient{resp: resp, reader: bufio.NewReader(resp.Body), cancel: cancel}
	t.Cleanup(c.close)
	return c
}

func (c *sseClient) close() {
	c.cancel()
	_ = c.resp.Body.Close()
}

// readFrame は空行までを1フレームとして読み、行のスライスを返す。
// コメント行（": ping"）は読み飛ばす。
func (c *sseClient) readFrame(t *testing.T) []string {
	t.Helper()
	lines := make([]string, 0, 2)
	for {
		line, err := c.reader.ReadString('\n')
		require.NoError(t, err, "SSEフレームの読み取りに失敗（受信済み=%v）", lines)
		line = strings.TrimRight(line, "\n")
		if line == "" {
			if len(lines) == 0 {
				continue // 直前のフレーム終端の空行
			}
			return lines
		}
		if strings.HasPrefix(line, ":") {
			continue // ハートビート等のコメント行
		}
		lines = append(lines, line)
	}
}

// readEvent は次の event/data フレームを (name, data) で返す。
func (c *sseClient) readEvent(t *testing.T) (name, data string) {
	t.Helper()
	lines := c.readFrame(t)
	require.Len(t, lines, 2, "event行とdata行の2行を期待した: %v", lines)
	return strings.TrimPrefix(lines[0], "event: "), strings.TrimPrefix(lines[1], "data: ")
}

// waitAssistantMessage は assistant メッセージが保存されるまで待って content を返す。
// 応答生成は別 goroutine で走るため、完了はDBの状態で観測する。
func waitAssistantMessage(t *testing.T, db *sql.DB, convID string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var content string
		err := db.QueryRow(
			`SELECT content FROM messages WHERE conversation_id = $1 AND role = 'assistant' LIMIT 1`,
			convID,
		).Scan(&content)
		if err == nil {
			return content
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("assistantメッセージが保存されなかった: conversation_id=%s", convID)
	return ""
}

// waitConnCount は Hub の登録数が want になるまで待つ。
func waitConnCount(t *testing.T, hub *sse.Hub, convID string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ConnCount(convID) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, want, hub.ConnCount(convID), "Hubの登録数が期待値に到達しなかった")
}
