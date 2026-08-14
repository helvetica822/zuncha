// 対応仕様: docs/03_unit_test/11_test_specification.md 4.2（観点3-2、TC-3-2-01〜15）
package unit

import (
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"zuncha/internal/llm"
	"zuncha/internal/service"
	"zuncha/internal/sse"
	"zuncha/internal/tts"
)

type mockLLMClient struct{ mock.Mock }

func (m *mockLLMClient) GenerateResponse(ctx context.Context, prompt string) ([]byte, error) {
	args := m.Called(ctx, prompt)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

type mockResponseParser struct{ mock.Mock }

func (m *mockResponseParser) Parse(body []byte) (*llm.LLMResponse, error) {
	args := m.Called(body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*llm.LLMResponse), args.Error(1)
}

type mockTTSClient struct{ mock.Mock }

func (m *mockTTSClient) Synthesize(ctx context.Context, text, conversationID, messageID string) (string, error) {
	args := m.Called(ctx, text, conversationID, messageID)
	return args.String(0), args.Error(1)
}

type mockTextChunker struct{ mock.Mock }

func (m *mockTextChunker) Chunk(text string) []string {
	args := m.Called(text)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]string)
}

type mockEventSink struct{ mock.Mock }

func (m *mockEventSink) SendEmotion(label string) error {
	args := m.Called(label)
	return args.Error(0)
}

func (m *mockEventSink) SendTextChunk(chunk string) error {
	args := m.Called(chunk)
	return args.Error(0)
}

func (m *mockEventSink) SendAudioURL(url string) error {
	args := m.Called(url)
	return args.Error(0)
}

func (m *mockEventSink) SendDone() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockEventSink) SendError(message string) error {
	args := m.Called(message)
	return args.Error(0)
}

var _ llm.LLMClient = (*mockLLMClient)(nil)
var _ llm.ResponseParser = (*mockResponseParser)(nil)
var _ tts.TTSClient = (*mockTTSClient)(nil)
var _ sse.TextChunker = (*mockTextChunker)(nil)
var _ sse.EventSink = (*mockEventSink)(nil)

// wantGenerateErrMessage は SSE の error イベントで利用者に見せる文言（仕様書§2.2）。
// internal/service 側の errMsgGenerateResponse と同値。ここはブラウザのトーストに
// そのまま出る文字列なので、内部エラー（"llm generate: ..." 等）が混ざってはならない。
const wantGenerateErrMessage = "応答の生成に失敗しました"

// StreamResponse へ渡す会話ID・assistantメッセージID（W-09 で追加された引数）。
// audio_files への登録に必要なため TTS へ素通しされる（D-4 訂正1）。
const (
	streamerConvID    = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	streamerMessageID = "01JASSISTANTMESSAGE0000000"
)

// 内部エラー文字列の断片。error イベントのメッセージに現れてはいけない。
var internalErrFragments = []string{
	"llm generate:", "parse:", "invalid emotion:", "send emotion:",
	"send text chunk:", "send audio url:", "send done:",
	"llm unavailable", "client disconnected",
}

func callOrderOf(sink *mockEventSink) []string {
	names := make([]string, len(sink.Calls))
	for i, c := range sink.Calls {
		names[i] = c.Method
	}
	return names
}

func indexOf(names []string, target string) int {
	for i, n := range names {
		if n == target {
			return i
		}
	}
	return -1
}

func TestResponseStreamer(t *testing.T) {
	t.Run("TC-3-2-01_正常フローはemotion_text_audio_url_doneの順で送出される", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", []byte("raw")).Return(&llm.LLMResponse{Text: "こんにちはなのだ", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "こんにちはなのだ").Return([]string{"こんにちはなのだ"})
		ttsClient.On("Synthesize", mock.Anything, "こんにちはなのだ", mock.Anything, mock.Anything).Return("/audio/01ARZ3NDEKTSV4RRFFQ69G5FAV", nil)
		sink.On("SendEmotion", "喜び").Return(nil)
		sink.On("SendTextChunk", "こんにちはなのだ").Return(nil)
		sink.On("SendAudioURL", "/audio/01ARZ3NDEKTSV4RRFFQ69G5FAV").Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err)
		assert.Equal(t, []string{"SendEmotion", "SendTextChunk", "SendAudioURL", "SendDone"}, callOrderOf(sink))
	})

	t.Run("TC-3-2-02_3チャンクすべてdoneより前に送出される", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return([]string{"c1", "c2", "c3"})
		ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendAudioURL", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err)
		names := callOrderOf(sink)
		doneIndex := indexOf(names, "SendDone")
		chunkCount := 0
		for i, n := range names {
			if n == "SendTextChunk" {
				chunkCount++
				assert.Less(t, i, doneIndex, "textチャンクはdoneより前に送出されること")
			}
		}
		assert.Equal(t, 3, chunkCount)
	})

	t.Run("TC-3-2-03_各イベントの引数が仕様通りである", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "こんにちは", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "こんにちは").Return([]string{"こんにちは"})
		ttsClient.On("Synthesize", mock.Anything, "こんにちは", mock.Anything, mock.Anything).Return("/audio/01ARZ3NDEKTSV4RRFFQ69G5FAV", nil)
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendAudioURL", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err)
		sink.AssertCalled(t, "SendEmotion", "喜び")
		sink.AssertCalled(t, "SendTextChunk", "こんにちは")
		sink.AssertCalled(t, "SendAudioURL", "/audio/01ARZ3NDEKTSV4RRFFQ69G5FAV")
	})

	t.Run("TC-3-2-04_1チャンクのみでもtext_doneの順で送出される", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return([]string{"text"})
		ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendAudioURL", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err)
		names := callOrderOf(sink)
		assert.Less(t, indexOf(names, "SendTextChunk"), indexOf(names, "SendDone"))
	})

	t.Run("TC-3-2-05_100チャンクでも順序が入れ替わらない", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		wantChunks := make([]string, 100)
		for i := range wantChunks {
			wantChunks[i] = string(rune('a' + i%26))
		}

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return(wantChunks)
		ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
		sink.On("SendEmotion", mock.Anything).Return(nil)
		for _, c := range wantChunks {
			sink.On("SendTextChunk", c).Return(nil)
		}
		sink.On("SendAudioURL", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err)
		var gotChunks []string
		for _, c := range sink.Calls {
			if c.Method == "SendTextChunk" {
				gotChunks = append(gotChunks, c.Arguments.String(0))
			}
		}
		assert.Equal(t, wantChunks, gotChunks)
	})

	t.Run("TC-3-2-06_LLM呼び出し失敗時はerrorのみ送出される", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return(nil, errors.New("llm unavailable"))
		sink.On("SendError", wantGenerateErrMessage).Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		assert.Error(t, err)
		sink.AssertNumberOfCalls(t, "SendError", 1)
		sink.AssertNotCalled(t, "SendEmotion", mock.Anything)
		sink.AssertNotCalled(t, "SendTextChunk", mock.Anything)
		parser.AssertNotCalled(t, "Parse", mock.Anything)
	})

	t.Run("TC-3-2-07_TTS失敗時はaudio_urlなしでdoneが送出される", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return([]string{"text"})
		ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("voicevox unavailable"))
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err, "TTS失敗時もStreamResponse自体はerrorを返さない（doneで完了扱いにするため）")
		sink.AssertNotCalled(t, "SendAudioURL", mock.Anything)
		sink.AssertNumberOfCalls(t, "SendDone", 1)
		sink.AssertNotCalled(t, "SendError", mock.Anything)
	})

	t.Run("TC-3-2-08_防御的チェックでemotion不正はエラーとして扱われる", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		// 本来ParseLLMResponse（3-1）のフォールバックにより起こり得ないが、
		// 防御的チェックの動作を確認するためParserモックで直接不正値を注入する
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "存在しない感情ラベル"}, nil)
		sink.On("SendError", wantGenerateErrMessage).Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		assert.Error(t, err)
		sink.AssertNotCalled(t, "SendEmotion", mock.Anything)
		sink.AssertNumberOfCalls(t, "SendError", 1)
	})

	t.Run("TC-3-2-09_emotionはtextより必ず先に送出される", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return([]string{"text"})
		ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendAudioURL", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err)
		names := callOrderOf(sink)
		assert.Less(t, indexOf(names, "SendEmotion"), indexOf(names, "SendTextChunk"))
	})

	t.Run("TC-3-2-10_全textチャンク送出後でなければaudio_urlは送出されない", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return([]string{"c1", "c2", "c3"})
		ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendAudioURL", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err)
		names := callOrderOf(sink)
		audioIndex := indexOf(names, "SendAudioURL")
		for i, n := range names {
			if n == "SendTextChunk" {
				assert.Less(t, i, audioIndex, "全textチャンクはaudio_urlより前に送出されること")
			}
		}
	})

	t.Run("TC-3-2-12_errorは送出中でも任意のタイミングで割り込める", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return([]string{"c1", "c2", "c3"})
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", "c1").Return(nil)
		// 2番目のチャンク送出中にシンク側で書き込みエラーが発生した状況を模擬する
		sink.On("SendTextChunk", "c2").Return(errors.New("client disconnected"))
		sink.On("SendError", wantGenerateErrMessage).Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		assert.Error(t, err)
		sink.AssertNumberOfCalls(t, "SendTextChunk", 2)
		sink.AssertNotCalled(t, "SendTextChunk", "c3")
		sink.AssertNumberOfCalls(t, "SendError", 1)
		sink.AssertNotCalled(t, "SendAudioURL", mock.Anything)
		sink.AssertNotCalled(t, "SendDone")
	})

	t.Run("TC-3-2-13_error送出後は後続イベントを送出しない", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return(nil, errors.New("llm unavailable"))
		sink.On("SendError", wantGenerateErrMessage).Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		_ = streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		sink.AssertNumberOfCalls(t, "SendError", 1)
		sink.AssertNumberOfCalls(t, "SendEmotion", 0)
		sink.AssertNumberOfCalls(t, "SendTextChunk", 0)
		sink.AssertNumberOfCalls(t, "SendAudioURL", 0)
		sink.AssertNumberOfCalls(t, "SendDone", 0)
	})

	t.Run("TC-3-2-14_doneはちょうど1回だけ送出される", func(t *testing.T) {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return([]string{"text"})
		ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendAudioURL", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

		require.NoError(t, err)
		sink.AssertNumberOfCalls(t, "SendDone", 1)
	})
}

func TestResponseStreamer_TTSへ会話IDとメッセージIDを渡す(t *testing.T) {
	// W-09: audio_files への登録には conversation_id・message_id が必要で、
	// Synthesize(ctx, text) には渡す手段が無かった（D-4 訂正1）。
	// StreamResponse が受けた値をそのまま素通しすることを固定する。
	llmClient := new(mockLLMClient)
	parser := new(mockResponseParser)
	ttsClient := new(mockTTSClient)
	chunker := new(mockTextChunker)
	sink := new(mockEventSink)

	llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
	parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "こんにちはなのだ", Emotion: "喜び"}, nil)
	chunker.On("Chunk", "こんにちはなのだ").Return([]string{"こんにちは", "なのだ"})
	ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("/audio/x", nil)
	sink.On("SendEmotion", mock.Anything).Return(nil)
	sink.On("SendTextChunk", mock.Anything).Return(nil)
	sink.On("SendAudioURL", mock.Anything).Return(nil)
	sink.On("SendDone").Return(nil)

	streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
	err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

	require.NoError(t, err)
	ttsClient.AssertNumberOfCalls(t, "Synthesize", 1)
	// 読み上げ対象はチャンクではなく応答全文（申し送り B1-1 の固定）。
	ttsClient.AssertCalled(t, "Synthesize", mock.Anything, "こんにちはなのだ",
		streamerConvID, streamerMessageID)
}

func TestResponseStreamer_TTS失敗はログに記録される(t *testing.T) {
	// そら指摘③: TTS 失敗は非致命（audio_url をスキップして done）だが、ttsErr が
	// 握りつぶされていたため VOICEVOX の停止や URL 設定ミスが完全に無症状になっていた
	// （「文字は出るが一生無音」）。運用時に原因へ辿り着ける記録を残すことを固定する。
	//
	// 発話内容（resp.Text）はログに載せない（NF-SEC。既存の anthropic クライアントと同方針）。
	const secretText = "誰にも知られたくない秘密の発話内容なのだ"

	captureLog := func(t *testing.T) *strings.Builder {
		t.Helper()
		var buf strings.Builder
		origOut := log.Writer()
		origFlags := log.Flags()
		log.SetOutput(&buf)
		log.SetFlags(0)
		t.Cleanup(func() {
			log.SetOutput(origOut)
			log.SetFlags(origFlags)
		})
		return &buf
	}

	// TTS の成否だけを変えた同一フローを流す。
	runFlow := func(t *testing.T, ttsErr error) *strings.Builder {
		t.Helper()
		buf := captureLog(t)

		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: secretText, Emotion: "喜び"}, nil)
		chunker.On("Chunk", secretText).Return([]string{secretText})
		if ttsErr != nil {
			ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", ttsErr)
		} else {
			ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
			sink.On("SendAudioURL", mock.Anything).Return(nil)
		}
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		require.NoError(t, streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID))
		return buf
	}

	t.Run("失敗理由と会話IDがログに残る", func(t *testing.T) {
		logged := runFlow(t, errors.New("voicevox unavailable")).String()

		require.NotEmpty(t, logged, "TTS失敗が無記録のままでは運用時に原因へ辿り着けない")
		assert.Contains(t, logged, "voicevox unavailable", "失敗理由（原因エラー）が残ること")
		assert.Contains(t, logged, streamerConvID, "どの会話が無音になったか特定できること")
	})

	t.Run("発話内容はログに出さない", func(t *testing.T) {
		logged := runFlow(t, errors.New("voicevox unavailable")).String()

		assert.NotContains(t, logged, secretText, "発話内容がログに出てはならない（NF-SEC）")
		assert.NotContains(t, logged, "秘密の発話内容")
	})

	t.Run("TTS成功時は何もログに出さない", func(t *testing.T) {
		logged := runFlow(t, nil).String()

		assert.Empty(t, logged, "正常時のログはノイズになるため出さない")
	})
}

func TestResponseStreamer_errorイベントに内部エラー文字列を出さない(t *testing.T) {
	// そら指摘②: fail が err.Error() をそのまま SendError に渡していたため、
	// "llm generate: llm unavailable" のような内部エラー文字列がブラウザのトーストに出ていた。
	// 利用者向け文言は全ステップ同一の1文に統一する（同Waveの chat.go の errMsg 定数と同じ扱い）。
	//
	// SendError の引数は mock.Anything で受けて「実際に渡された値」を観測する。
	// 固定文言で On を張ると引数違いが panic になり、何が漏れたのかが分からなくなるため。
	sendErrorArgs := func(sink *mockEventSink) []string {
		var args []string
		for _, c := range sink.Calls {
			if c.Method == "SendError" {
				args = append(args, c.Arguments.String(0))
			}
		}
		return args
	}

	// 中断が起こり得る各ステップを、そのステップで初めて失敗する形に組む。
	cases := map[string]func(*mockLLMClient, *mockResponseParser, *mockTTSClient, *mockTextChunker, *mockEventSink){
		"LLM生成の失敗": func(l *mockLLMClient, p *mockResponseParser, tc *mockTTSClient, ch *mockTextChunker, s *mockEventSink) {
			l.On("GenerateResponse", mock.Anything, mock.Anything).Return(nil, errors.New("llm unavailable"))
		},
		"パースの失敗": func(l *mockLLMClient, p *mockResponseParser, tc *mockTTSClient, ch *mockTextChunker, s *mockEventSink) {
			l.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
			p.On("Parse", mock.Anything).Return(nil, errors.New("invalid character '}' looking for beginning of value"))
		},
		"emotion不正": func(l *mockLLMClient, p *mockResponseParser, tc *mockTTSClient, ch *mockTextChunker, s *mockEventSink) {
			l.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
			p.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "存在しない感情ラベル"}, nil)
		},
		"emotion送出の失敗": func(l *mockLLMClient, p *mockResponseParser, tc *mockTTSClient, ch *mockTextChunker, s *mockEventSink) {
			l.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
			p.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
			s.On("SendEmotion", mock.Anything).Return(errors.New("client disconnected"))
		},
		"textチャンク送出の失敗": func(l *mockLLMClient, p *mockResponseParser, tc *mockTTSClient, ch *mockTextChunker, s *mockEventSink) {
			l.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
			p.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
			ch.On("Chunk", "text").Return([]string{"c1"})
			s.On("SendEmotion", mock.Anything).Return(nil)
			s.On("SendTextChunk", mock.Anything).Return(errors.New("client disconnected"))
		},
		"audio_url送出の失敗": func(l *mockLLMClient, p *mockResponseParser, tc *mockTTSClient, ch *mockTextChunker, s *mockEventSink) {
			l.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
			p.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
			ch.On("Chunk", "text").Return([]string{"c1"})
			tc.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
			s.On("SendEmotion", mock.Anything).Return(nil)
			s.On("SendTextChunk", mock.Anything).Return(nil)
			s.On("SendAudioURL", mock.Anything).Return(errors.New("client disconnected"))
		},
		"done送出の失敗": func(l *mockLLMClient, p *mockResponseParser, tc *mockTTSClient, ch *mockTextChunker, s *mockEventSink) {
			l.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
			p.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
			ch.On("Chunk", "text").Return([]string{"c1"})
			tc.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
			s.On("SendEmotion", mock.Anything).Return(nil)
			s.On("SendTextChunk", mock.Anything).Return(nil)
			s.On("SendAudioURL", mock.Anything).Return(nil)
			s.On("SendDone").Return(errors.New("client disconnected"))
		},
	}

	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			llmClient := new(mockLLMClient)
			parser := new(mockResponseParser)
			ttsClient := new(mockTTSClient)
			chunker := new(mockTextChunker)
			sink := new(mockEventSink)
			setup(llmClient, parser, ttsClient, chunker, sink)
			sink.On("SendError", mock.Anything).Return(nil)

			streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
			err := streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)

			require.Error(t, err, "戻り値には内部エラーを残す（ログ・呼び出し側用）")
			args := sendErrorArgs(sink)
			require.Len(t, args, 1)
			assert.Equal(t, wantGenerateErrMessage, args[0], "全ステップ同一の利用者向け文言であること")
			for _, fragment := range internalErrFragments {
				assert.NotContains(t, args[0], fragment,
					"内部エラー文字列がブラウザのトーストに出てはならない")
			}
		})
	}
}

func TestResponseStreamer_doneはaudio_url相当の後でのみ送出される(t *testing.T) {
	// TC-3-2-11
	newFlow := func(ttsErr error) *mockEventSink {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
		parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
		chunker.On("Chunk", "text").Return([]string{"text"})
		if ttsErr != nil {
			ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", ttsErr)
		} else {
			ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
			sink.On("SendAudioURL", mock.Anything).Return(nil)
		}
		sink.On("SendEmotion", mock.Anything).Return(nil)
		sink.On("SendTextChunk", mock.Anything).Return(nil)
		sink.On("SendDone").Return(nil)

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		_ = streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)
		return sink
	}

	t.Run("正常フロー", func(t *testing.T) {
		sink := newFlow(nil)
		names := callOrderOf(sink)
		audioIndex := indexOf(names, "SendAudioURL")
		doneIndex := indexOf(names, "SendDone")
		require.NotEqual(t, -1, audioIndex)
		require.NotEqual(t, -1, doneIndex)
		assert.Less(t, audioIndex, doneIndex)
	})

	t.Run("TTS失敗フロー", func(t *testing.T) {
		sink := newFlow(errors.New("voicevox unavailable"))
		names := callOrderOf(sink)
		assert.Equal(t, -1, indexOf(names, "SendAudioURL"), "TTS失敗時はaudio_url自体が送出されない")
		assert.NotEqual(t, -1, indexOf(names, "SendDone"))
	})
}

func TestResponseStreamer_全フローがdoneまたはerrorで終端する(t *testing.T) {
	// TC-3-2-15
	buildFlow := func(llmErr, ttsErr error) *mockEventSink {
		llmClient := new(mockLLMClient)
		parser := new(mockResponseParser)
		ttsClient := new(mockTTSClient)
		chunker := new(mockTextChunker)
		sink := new(mockEventSink)

		if llmErr != nil {
			llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return(nil, llmErr)
			sink.On("SendError", wantGenerateErrMessage).Return(nil)
		} else {
			llmClient.On("GenerateResponse", mock.Anything, mock.Anything).Return([]byte("raw"), nil)
			parser.On("Parse", mock.Anything).Return(&llm.LLMResponse{Text: "text", Emotion: "喜び"}, nil)
			chunker.On("Chunk", "text").Return([]string{"text"})
			sink.On("SendEmotion", mock.Anything).Return(nil)
			sink.On("SendTextChunk", mock.Anything).Return(nil)
			sink.On("SendDone").Return(nil)
			if ttsErr != nil {
				ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", ttsErr)
			} else {
				ttsClient.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("/audio/x", nil)
				sink.On("SendAudioURL", mock.Anything).Return(nil)
			}
		}

		streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, chunker)
		_ = streamer.StreamResponse(context.Background(), sink, "prompt", streamerConvID, streamerMessageID)
		return sink
	}

	flows := map[string]*mockEventSink{
		"正常フロー":    buildFlow(nil, nil),
		"TTS失敗フロー": buildFlow(nil, errors.New("voicevox unavailable")),
		"LLM失敗フロー": buildFlow(errors.New("llm unavailable"), nil),
	}

	for name, sink := range flows {
		t.Run(name, func(t *testing.T) {
			names := callOrderOf(sink)
			require.NotEmpty(t, names)
			last := names[len(names)-1]
			assert.Contains(t, []string{"SendDone", "SendError"}, last, "最終イベントはdoneまたはerrorであること")
		})
	}
}
