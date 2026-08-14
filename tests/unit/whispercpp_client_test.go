// whisper-server (HTTP) クライアントの単体テスト。
// 対応仕様: docs/04_implementation/04_realtime_wiring_design.md D-3 / D-3a(2026-08-14決定)、
// tasks/instructions_zundamon_wave_w10.md §2
//
// 本テストは httptest のフェイク whisper-server のみで駆動し、実 whisper-server へは一切接続しない。
package unit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/whispercpp"
)

const (
	// wcWAV は変換済み音声の代役。中身は解釈されないので固定バイト列でよい。
	wcWAV = "RIFF\x24\x00\x00\x00WAVEfake-pcm-payload"
	// wcText は whisper-server がトップレベル "text" で返す転写結果。
	wcText = "こんにちはなのだ"
	// wantResponseFormat は D-3a の確定事項。json だと信頼度が取れないため verbose_json。
	wantResponseFormat = "verbose_json"
	// wantInferencePath は whisper-server の推論エンドポイント。
	wantInferencePath = "/inference"
	// wantFileField は whisper-server の README / 組み込みヘルプの curl 例が示すフィールド名。
	wantFileField = "file"
)

// recordedInference はフェイク whisper-server が受け取ったリクエストの観測結果。
type recordedInference struct {
	Method         string
	Path           string
	ContentType    string
	ResponseFormat string
	FileFound      bool
	FileName       string
	FileBody       string
}

// fakeWhisperServer は whisper-server の /inference を模す。
type fakeWhisperServer struct {
	mu sync.Mutex

	status   int // 0 なら 200 として扱う
	body     string
	delay    time.Duration
	requests []recordedInference
}

func newFakeWhisperServer(body string) *fakeWhisperServer {
	return &fakeWhisperServer{body: body}
}

func (f *fakeWhisperServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := recordedInference{
		Method:      r.Method,
		Path:        r.URL.Path,
		ContentType: r.Header.Get("Content-Type"),
	}
	// multipart として解釈できたときだけ中身を記録する（解釈できない = 契約違反も観測対象）。
	if err := r.ParseMultipartForm(1 << 20); err == nil {
		rec.ResponseFormat = r.FormValue("response_format")
		if file, header, err := r.FormFile(wantFileField); err == nil {
			defer file.Close()
			rec.FileFound = true
			rec.FileName = header.Filename
			buf := new(strings.Builder)
			_, _ = io.Copy(buf, file)
			rec.FileBody = buf.String()
		}
	}

	f.mu.Lock()
	f.requests = append(f.requests, rec)
	status, body, delay := f.status, f.body, f.delay
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (f *fakeWhisperServer) recorded() []recordedInference {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedInference(nil), f.requests...)
}

// verboseJSON は D-3a に載っている whisper-server の verbose_json 応答を組み立てる。
func verboseJSON(text string, noSpeechProbs ...float64) string {
	segments := make([]string, 0, len(noSpeechProbs))
	for i, p := range noSpeechProbs {
		segments = append(segments, fmt.Sprintf(
			`{"id":%d,"text":"seg","start":0.0,"end":1.2,"no_speech_prob":%v,"avg_logprob":-0.3}`, i, p))
	}
	return fmt.Sprintf(
		`{"task":"transcribe","language":"japanese","duration":2.5,"text":%q,"segments":[%s]}`,
		text, strings.Join(segments, ","))
}

// newWhisperClient はフェイクサーバへ向けたクライアントを組む。
func newWhisperClient(t *testing.T, fake *fakeWhisperServer, opts ...whispercpp.Option) *whispercpp.Client {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	c, err := whispercpp.NewClient(server.URL, opts...)
	require.NoError(t, err)
	return c
}

func TestWhisperCppClient_confidenceの算出(t *testing.T) {
	tests := []struct {
		name           string
		noSpeechProbs  []float64
		wantConfidence float64
	}{
		{
			name:           "segmentsが1件ならconfidenceは1-no_speech_prob",
			noSpeechProbs:  []float64{0.1},
			wantConfidence: 0.9,
		},
		{
			name: "segmentsが複数件なら最大のno_speech_probから算出する（最も低い信頼度を採用）",
			// 最小(0.02)なら0.98、平均(0.24)なら0.76 になる。最大(0.6)を採ることを固定する。
			noSpeechProbs:  []float64{0.02, 0.6, 0.1},
			wantConfidence: 0.4,
		},
		{
			name:           "最大値が末尾にあっても採用される",
			noSpeechProbs:  []float64{0.6, 0.02},
			wantConfidence: 0.4,
		},
		{
			name:           "no_speech_probが0なら信頼度は1",
			noSpeechProbs:  []float64{0.0},
			wantConfidence: 1.0,
		},
		{
			name:           "no_speech_probが1なら信頼度は0",
			noSpeechProbs:  []float64{1.0},
			wantConfidence: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeWhisperServer(verboseJSON(wcText, tt.noSpeechProbs...))
			c := newWhisperClient(t, fake)

			got, err := c.Transcribe(context.Background(), []byte(wcWAV))

			require.NoError(t, err)
			assert.InDelta(t, tt.wantConfidence, got.Confidence, 1e-9)
			assert.Equal(t, wcText, got.Text)
		})
	}
}

func TestWhisperCppClient_segmentsが空ならconfidenceは0(t *testing.T) {
	// text が非空でも segments が無ければ 0（D-3a: 無音のみ等は認識失敗として扱う）。
	fake := newFakeWhisperServer(verboseJSON(wcText))
	c := newWhisperClient(t, fake)

	got, err := c.Transcribe(context.Background(), []byte(wcWAV))

	require.NoError(t, err)
	assert.Equal(t, 0.0, got.Confidence)
	assert.Equal(t, wcText, got.Text)
}

func TestWhisperCppClient_segmentsキー自体が無くてもconfidenceは0(t *testing.T) {
	fake := newFakeWhisperServer(`{"task":"transcribe","text":"あ"}`)
	c := newWhisperClient(t, fake)

	got, err := c.Transcribe(context.Background(), []byte(wcWAV))

	require.NoError(t, err)
	assert.Equal(t, 0.0, got.Confidence)
	assert.Equal(t, "あ", got.Text)
}

func TestWhisperCppClient_textはトップレベルの値をそのまま返す(t *testing.T) {
	// segments[].text を結合し直さないこと（D-3a）。
	body := `{"text":"全体の転写テキスト","segments":[` +
		`{"id":0,"text":"部分1","no_speech_prob":0.1},` +
		`{"id":1,"text":"部分2","no_speech_prob":0.2}]}`
	fake := newFakeWhisperServer(body)
	c := newWhisperClient(t, fake)

	got, err := c.Transcribe(context.Background(), []byte(wcWAV))

	require.NoError(t, err)
	assert.Equal(t, "全体の転写テキスト", got.Text)
	assert.NotEqual(t, "部分1部分2", got.Text)
	assert.InDelta(t, 0.8, got.Confidence, 1e-9)
}

func TestWhisperCppClient_リクエストの中身(t *testing.T) {
	// 「外部サービスへ渡すだけの値」は送信ボディそのものを検証する
	// （tasks/lessons.md 2026-08-10）。ここが壊れると whisper-server は
	// 既定の json 形式で応答し、confidence が常に 0 の「無言のバグ」になる。
	fake := newFakeWhisperServer(verboseJSON(wcText, 0.1))
	c := newWhisperClient(t, fake)

	_, err := c.Transcribe(context.Background(), []byte(wcWAV))
	require.NoError(t, err)

	reqs := fake.recorded()
	require.Len(t, reqs, 1)
	got := reqs[0]

	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, wantInferencePath, got.Path)
	assert.True(t, strings.HasPrefix(got.ContentType, "multipart/form-data"),
		"Content-Type が multipart/form-data ではない: %q", got.ContentType)
	assert.Equal(t, wantResponseFormat, got.ResponseFormat)
	assert.True(t, got.FileFound, "file フィールドが見つからない")
	assert.Equal(t, wcWAV, got.FileBody, "WAVバイト列がそのまま送られていない")
	assert.NotEmpty(t, got.FileName, "file パートにファイル名が無い（whisper-server はファイルとして受け取る）")
}

func TestWhisperCppClient_ベースURLの末尾スラッシュを重複させない(t *testing.T) {
	fake := newFakeWhisperServer(verboseJSON(wcText, 0.1))
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	c, err := whispercpp.NewClient(server.URL + "/")
	require.NoError(t, err)

	_, err = c.Transcribe(context.Background(), []byte(wcWAV))
	require.NoError(t, err)

	reqs := fake.recorded()
	require.Len(t, reqs, 1)
	assert.Equal(t, wantInferencePath, reqs[0].Path)
}

func TestWhisperCppClient_ベースURLが空ならエラー(t *testing.T) {
	c, err := whispercpp.NewClient("")

	require.Error(t, err)
	assert.Nil(t, c)
}

func TestWhisperCppClient_異常系(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "500応答はエラー", status: http.StatusInternalServerError, body: `{"error":"boom"}`},
		{name: "400応答はエラー", status: http.StatusBadRequest, body: `{"error":"bad"}`},
		{name: "404応答はエラー", status: http.StatusNotFound, body: ``},
		{name: "壊れたJSONはエラー", status: http.StatusOK, body: `{"text":`},
		{name: "空応答はエラー", status: http.StatusOK, body: ``},
		{name: "JSONでない応答はエラー", status: http.StatusOK, body: `not json at all`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeWhisperServer(tt.body)
			fake.status = tt.status
			c := newWhisperClient(t, fake)

			got, err := c.Transcribe(context.Background(), []byte(wcWAV))

			require.Error(t, err)
			assert.Equal(t, "", got.Text)
			assert.Equal(t, 0.0, got.Confidence)
		})
	}
}

func TestWhisperCppClient_異常系のエラーに音声データを載せない(t *testing.T) {
	// 発話内容はログ・エラー文字列へ出さない（NF-SEC-01 の趣旨）。
	fake := newFakeWhisperServer(`{"error":"boom"}`)
	fake.status = http.StatusInternalServerError
	c := newWhisperClient(t, fake)

	_, err := c.Transcribe(context.Background(), []byte(wcWAV))

	require.Error(t, err)
	assert.NotContains(t, err.Error(), wcWAV)
}

func TestWhisperCppClient_キャンセル済みctxはcontextCanceled(t *testing.T) {
	fake := newFakeWhisperServer(verboseJSON(wcText, 0.1))
	c := newWhisperClient(t, fake)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Transcribe(ctx, []byte(wcWAV))

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "context.Canceled ではない: %v", err)
}

func TestWhisperCppClient_到達できないサーバはエラー(t *testing.T) {
	fake := newFakeWhisperServer(verboseJSON(wcText, 0.1))
	server := httptest.NewServer(fake)
	url := server.URL
	server.Close() // 接続拒否させる

	c, err := whispercpp.NewClient(url)
	require.NoError(t, err)

	_, err = c.Transcribe(context.Background(), []byte(wcWAV))

	require.Error(t, err)
}

func TestWhisperCppClient_タイムアウトを超えるとエラー(t *testing.T) {
	fake := newFakeWhisperServer(verboseJSON(wcText, 0.1))
	fake.delay = 500 * time.Millisecond
	c := newWhisperClient(t, fake, whispercpp.WithTimeout(20*time.Millisecond))

	_, err := c.Transcribe(context.Background(), []byte(wcWAV))

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "DeadlineExceeded ではない: %v", err)
}
