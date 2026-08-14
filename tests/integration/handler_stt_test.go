// POST /conversations/{id}/stt の結合テスト。
// 対応: docs/02_functional_design/01_screen_design.md §7.3、
// docs/04_implementation/04_realtime_wiring_design.md D-3、
// tasks/instructions_zundamon_wave_w10.md §5
//
// ffmpeg / whisper-server はどちらもフェイクに差し替えており、実バイナリ・実サーバへは接続しない。
package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/stt"
)

// sttAudioField はフロントとの契約（multipart のフィールド名）。
const sttAudioField = "audio"

// sttResponse は POST .../stt のレスポンス本文。
// failed は認識失敗時のみ true になる（成功時は省略される）。
type sttResponse struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Failed     bool    `json:"failed"`
	Error      string  `json:"error"`
}

// postSTT は音声を multipart/form-data で送る。
func (f *handlerFixture) postSTT(t *testing.T, convID, fieldName string, audio []byte) *http.Response {
	t.Helper()
	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile(fieldName, "recording.webm")
	require.NoError(t, err)
	_, err = part.Write(audio)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	return f.postSTTRaw(t, convID, mw.FormDataContentType(), body.Bytes())
}

// postSTTRaw は Content-Type とボディを指定してそのまま送る（不正 multipart の検証用）。
func (f *handlerFixture) postSTTRaw(t *testing.T, convID, contentType string, body []byte) *http.Response {
	t.Helper()
	resp, err := f.client.Post(
		f.server.URL+"/conversations/"+convID+"/stt",
		contentType,
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func decodeSTT(t *testing.T, resp *http.Response) sttResponse {
	t.Helper()
	var got sttResponse
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &got), "レスポンスがJSONではない: %s", raw)
	return got
}

func TestHandlerSTT_正常系はtextとconfidenceを返す(t *testing.T) {
	f := newHandlerFixture(t)
	convID := f.seedConversation(t)
	f.sttC.setResult(stt.STTResult{Text: "こんにちはなのだ", Confidence: 0.87})

	resp := f.postSTT(t, convID, sttAudioField, []byte("webm-opus-bytes"))

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	got := decodeSTT(t, resp)
	assert.Equal(t, "こんにちはなのだ", got.Text)
	assert.InDelta(t, 0.87, got.Confidence, 1e-9)
	assert.False(t, got.Failed, "成功時に failed が立っている")
}

func TestHandlerSTT_受け取った音声がそのままサービス層へ渡る(t *testing.T) {
	f := newHandlerFixture(t)
	convID := f.seedConversation(t)
	audio := []byte("\x1a\x45\xdf\xa3webm-container-bytes")

	resp := f.postSTT(t, convID, sttAudioField, audio)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	inputs := f.sttConv.recordedInputs()
	require.Len(t, inputs, 1)
	assert.Equal(t, audio, inputs[0], "アップロードされた音声が欠損・改変されている")
}

func TestHandlerSTT_認識失敗は200でfailedを返す(t *testing.T) {
	// 認識失敗はクライアントエラーではなく正常系の一部。400/500 にしない
	// （01_screen_design.md 9-1: フロントは応答バブルで回復動線を出す）。
	tests := []struct {
		name   string
		result stt.STTResult
	}{
		{
			name:   "信頼度が閾値未満",
			result: stt.STTResult{Text: "たぶんこう", Confidence: stt.STTConfidenceThreshold - 0.01},
		},
		{
			name:   "テキストが空",
			result: stt.STTResult{Text: "", Confidence: 0.99},
		},
		{
			name:   "テキストが空白のみ",
			result: stt.STTResult{Text: "   \t\n", Confidence: 0.99},
		},
		{
			name:   "テキストが全角スペースのみ",
			result: stt.STTResult{Text: "　　", Confidence: 0.99},
		},
		{
			name:   "無音（segments空）相当で信頼度0",
			result: stt.STTResult{Text: "", Confidence: 0.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newHandlerFixture(t)
			convID := f.seedConversation(t)
			f.sttC.setResult(tt.result)

			resp := f.postSTT(t, convID, sttAudioField, []byte("webm-opus-bytes"))

			require.Equal(t, http.StatusOK, resp.StatusCode)
			got := decodeSTT(t, resp)
			assert.True(t, got.Failed, "failed:true が返っていない")
			assert.Empty(t, got.Text, "失敗時に認識テキストを返している")
		})
	}
}

func TestHandlerSTT_閾値ちょうどは成功扱い(t *testing.T) {
	// stt.IsRecognitionFailed は Confidence < 閾値 を失敗とする（等号は成功）。
	f := newHandlerFixture(t)
	convID := f.seedConversation(t)
	f.sttC.setResult(stt.STTResult{Text: "ぎりぎり", Confidence: stt.STTConfidenceThreshold})

	resp := f.postSTT(t, convID, sttAudioField, []byte("webm-opus-bytes"))

	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := decodeSTT(t, resp)
	assert.False(t, got.Failed)
	assert.Equal(t, "ぎりぎり", got.Text)
}

func TestHandlerSTT_会話IDが不正な形式なら400(t *testing.T) {
	f := newHandlerFixture(t)

	resp := f.postSTT(t, "not-a-ulid", sttAudioField, []byte("webm-opus-bytes"))

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, 0, f.sttC.callCount(), "IDが不正なのに認識を実行している")
}

func TestHandlerSTT_存在しない会話なら404(t *testing.T) {
	f := newHandlerFixture(t)

	resp := f.postSTT(t, ulid.Make().String(), sttAudioField, []byte("webm-opus-bytes"))

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, 0, f.sttC.callCount(), "存在しない会話なのに認識を実行している")
}

func TestHandlerSTT_multipartが不正なら400(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
	}{
		{
			name:        "multipartではないJSONボディ",
			contentType: "application/json",
			body:        []byte(`{"audio":"..."}`),
		},
		{
			name:        "境界文字列が壊れている",
			contentType: "multipart/form-data; boundary=xyz",
			body:        []byte("--not-the-boundary\r\ngarbage"),
		},
		{
			name:        "ボディが空",
			contentType: "multipart/form-data; boundary=xyz",
			body:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newHandlerFixture(t)
			convID := f.seedConversation(t)

			resp := f.postSTTRaw(t, convID, tt.contentType, tt.body)

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Equal(t, 0, f.sttC.callCount())
		})
	}
}

func TestHandlerSTT_audioフィールドが無ければ400(t *testing.T) {
	f := newHandlerFixture(t)
	convID := f.seedConversation(t)

	resp := f.postSTT(t, convID, "wrong_field_name", []byte("webm-opus-bytes"))

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, 0, f.sttC.callCount())
}

func TestHandlerSTT_音声が空なら400(t *testing.T) {
	f := newHandlerFixture(t)
	convID := f.seedConversation(t)

	resp := f.postSTT(t, convID, sttAudioField, nil)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, 0, f.sttC.callCount(), "空の音声を変換・認識へ流している")
}

func TestHandlerSTT_サービス層のエラーは500(t *testing.T) {
	t.Run("ffmpeg変換の失敗", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		f.sttConv.setError(errors.New("ffmpegの実行に失敗"))

		resp := f.postSTT(t, convID, sttAudioField, []byte("webm-opus-bytes"))

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, 0, f.sttC.callCount())
	})

	t.Run("whisper-serverの失敗", func(t *testing.T) {
		f := newHandlerFixture(t)
		convID := f.seedConversation(t)
		f.sttC.setError(errors.New("whisper-serverへ到達できません"))

		resp := f.postSTT(t, convID, sttAudioField, []byte("webm-opus-bytes"))

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		got := decodeSTT(t, resp)
		assert.NotEmpty(t, got.Error)
		assert.False(t, got.Failed, "サーバ障害を認識失敗として返している")
	})
}

func TestHandlerSTT_500の本文に内部エラーの詳細を出さない(t *testing.T) {
	f := newHandlerFixture(t)
	convID := f.seedConversation(t)
	f.sttC.setError(errors.New("dial tcp 10.0.0.1:8080: connect: connection refused"))

	resp := f.postSTT(t, convID, sttAudioField, []byte("webm-opus-bytes"))

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	got := decodeSTT(t, resp)
	assert.NotContains(t, got.Error, "10.0.0.1", "内部の接続先をクライアントへ漏らしている")
}
