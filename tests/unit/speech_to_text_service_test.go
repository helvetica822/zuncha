// SpeechToTextService（音声形式変換 → 音声認識のオーケストレーション）の単体テスト。
// 対応仕様: tasks/instructions_zundamon_wave_w10.md §4
package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/service"
	"zuncha/internal/stt"
)

// fakeAudioConverter は変換入力を記録し、指定の結果を返す。
type fakeAudioConverter struct {
	calls  int
	inputs [][]byte
	ctxs   []context.Context

	out []byte
	err error
}

func (f *fakeAudioConverter) Convert(ctx context.Context, input []byte) ([]byte, error) {
	f.calls++
	f.inputs = append(f.inputs, input)
	f.ctxs = append(f.ctxs, ctx)
	return f.out, f.err
}

// fakeSTTClient は認識入力を記録し、指定の結果を返す。
type fakeSTTClient struct {
	calls  int
	inputs [][]byte
	ctxs   []context.Context

	result stt.STTResult
	err    error
}

func (f *fakeSTTClient) Transcribe(ctx context.Context, wav []byte) (stt.STTResult, error) {
	f.calls++
	f.inputs = append(f.inputs, wav)
	f.ctxs = append(f.ctxs, ctx)
	return f.result, f.err
}

func TestSpeechToTextService_変換結果を認識クライアントへ渡す(t *testing.T) {
	raw := []byte("webm-opus-bytes")
	wav := []byte("RIFF-converted")
	conv := &fakeAudioConverter{out: wav}
	client := &fakeSTTClient{result: stt.STTResult{Text: "こんにちはなのだ", Confidence: 0.87}}
	s := service.NewSpeechToTextService(conv, client)

	got, err := s.Transcribe(context.Background(), raw)

	require.NoError(t, err)
	assert.Equal(t, "こんにちはなのだ", got.Text)
	assert.InDelta(t, 0.87, got.Confidence, 1e-9)

	require.Equal(t, 1, conv.calls)
	assert.Equal(t, raw, conv.inputs[0], "録音データがそのまま変換器へ渡っていない")
	require.Equal(t, 1, client.calls)
	assert.Equal(t, wav, client.inputs[0], "変換後のWAVが認識クライアントへ渡っていない")
	assert.NotEqual(t, raw, client.inputs[0], "変換を経ずに生データを送っている")
}

func TestSpeechToTextService_信頼度が低い結果もそのまま返す(t *testing.T) {
	// 認識失敗の判定（stt.IsRecognitionFailed）はハンドラ層の責務。
	// サービス層で握り潰すと 200 {failed:true} を返せなくなる。
	conv := &fakeAudioConverter{out: []byte("wav")}
	client := &fakeSTTClient{result: stt.STTResult{Text: "", Confidence: 0.0}}
	s := service.NewSpeechToTextService(conv, client)

	got, err := s.Transcribe(context.Background(), []byte("raw"))

	require.NoError(t, err)
	assert.Equal(t, "", got.Text)
	assert.Equal(t, 0.0, got.Confidence)
}

func TestSpeechToTextService_変換失敗なら認識を呼ばない(t *testing.T) {
	convErr := errors.New("ffmpeg失敗")
	conv := &fakeAudioConverter{err: convErr}
	client := &fakeSTTClient{result: stt.STTResult{Text: "呼ばれてはいけない", Confidence: 1.0}}
	s := service.NewSpeechToTextService(conv, client)

	got, err := s.Transcribe(context.Background(), []byte("raw"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, convErr), "元のエラーが %%w でラップされていない: %v", err)
	assert.Equal(t, 0, client.calls, "変換に失敗したのに認識クライアントを呼んでいる")
	assert.Equal(t, stt.STTResult{}, got)
}

func TestSpeechToTextService_認識失敗はエラーを伝播する(t *testing.T) {
	clientErr := errors.New("whisper-server到達不能")
	conv := &fakeAudioConverter{out: []byte("wav")}
	client := &fakeSTTClient{err: clientErr}
	s := service.NewSpeechToTextService(conv, client)

	got, err := s.Transcribe(context.Background(), []byte("raw"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, clientErr), "元のエラーが %%w でラップされていない: %v", err)
	assert.Equal(t, stt.STTResult{}, got)
}

func TestSpeechToTextService_ctxを両方へ伝播する(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")
	conv := &fakeAudioConverter{out: []byte("wav")}
	client := &fakeSTTClient{result: stt.STTResult{Text: "text", Confidence: 0.9}}
	s := service.NewSpeechToTextService(conv, client)

	_, err := s.Transcribe(ctx, []byte("raw"))

	require.NoError(t, err)
	require.Len(t, conv.ctxs, 1)
	require.Len(t, client.ctxs, 1)
	assert.Equal(t, "marker", conv.ctxs[0].Value(ctxKey{}))
	assert.Equal(t, "marker", client.ctxs[0].Value(ctxKey{}))
}
