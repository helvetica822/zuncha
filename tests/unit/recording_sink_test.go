// service.RecordingSink の単体テスト。
// 対応: tasks/instructions_zundamon_wave_b2.md §2.2 (W-07)
package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"zuncha/internal/model"
	"zuncha/internal/repository"
	"zuncha/internal/service"
	"zuncha/internal/sse"
)

type mockMessageRepository struct{ mock.Mock }

func (m *mockMessageRepository) GetRecentMessages(ctx context.Context, conversationID string) ([]model.Message, error) {
	args := m.Called(ctx, conversationID)
	if msgs := args.Get(0); msgs != nil {
		return msgs.([]model.Message), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockMessageRepository) InsertMessage(ctx context.Context, msg *model.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

var _ repository.MessageRepository = (*mockMessageRepository)(nil)

const (
	sinkConvID    = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	sinkMessageID = "01JASSISTANTMESSAGE0000000"
)

var sinkNow = time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

// newTestRecordingSink は決定的な messageID/now を注入した RecordingSink を返す。
//
// W-09: messageID は「渡された値をそのまま使う」形に変わった（旧: newID func() string を
// 内部で呼ぶ）。TTS合成が assistant メッセージ保存より先行するため、audio_files と
// messages で同じIDを共有する必要があり、採番は ChatService 側へ移した（D-4 訂正2）。
func newTestRecordingSink(repo repository.MessageRepository, inner *mockEventSink) sse.EventSink {
	return service.NewRecordingSink(
		inner,
		repo,
		sinkConvID,
		sinkMessageID,
		func() time.Time { return sinkNow },
	)
}

// insertedMessage は InsertMessage に渡された model.Message を取り出す。
func insertedMessage(t *testing.T, repo *mockMessageRepository) *model.Message {
	t.Helper()
	for _, call := range repo.Calls {
		if call.Method == "InsertMessage" {
			msg, ok := call.Arguments.Get(1).(*model.Message)
			require.True(t, ok, "InsertMessage の第2引数が *model.Message ではない")
			return msg
		}
	}
	t.Fatal("InsertMessage が呼ばれていない")
	return nil
}

func TestRecordingSink(t *testing.T) {
	t.Run("W-07-R1_全イベントが同じ順序でinnerへ委譲される", func(t *testing.T) {
		repo := new(mockMessageRepository)
		repo.On("InsertMessage", mock.Anything, mock.Anything).Return(nil)
		inner := new(mockEventSink)
		var order []string
		inner.On("SendEmotion", "喜び").Run(func(mock.Arguments) { order = append(order, "SendEmotion") }).Return(nil)
		inner.On("SendTextChunk", mock.Anything).Run(func(mock.Arguments) { order = append(order, "SendTextChunk") }).Return(nil)
		inner.On("SendAudioURL", "/audio/01J").Run(func(mock.Arguments) { order = append(order, "SendAudioURL") }).Return(nil)
		inner.On("SendDone").Run(func(mock.Arguments) { order = append(order, "SendDone") }).Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendEmotion("喜び"))
		require.NoError(t, sink.SendTextChunk("ずんだもん"))
		require.NoError(t, sink.SendTextChunk("なのだ"))
		require.NoError(t, sink.SendAudioURL("/audio/01J"))
		require.NoError(t, sink.SendDone())

		assert.Equal(t,
			[]string{"SendEmotion", "SendTextChunk", "SendTextChunk", "SendAudioURL", "SendDone"},
			order)
	})

	t.Run("W-07-R2_SendDoneでassistantメッセージが全チャンク連結で保存される", func(t *testing.T) {
		repo := new(mockMessageRepository)
		repo.On("InsertMessage", mock.Anything, mock.Anything).Return(nil)
		inner := new(mockEventSink)
		inner.On("SendEmotion", "喜び").Return(nil)
		inner.On("SendTextChunk", mock.Anything).Return(nil)
		inner.On("SendDone").Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendEmotion("喜び"))
		require.NoError(t, sink.SendTextChunk("ずんだもんなのだ。"))
		require.NoError(t, sink.SendTextChunk("元気なのだ。"))
		require.NoError(t, sink.SendDone())

		repo.AssertNumberOfCalls(t, "InsertMessage", 1)
		msg := insertedMessage(t, repo)
		assert.Equal(t, sinkMessageID, msg.ID)
		assert.Equal(t, sinkConvID, msg.ConversationID)
		assert.Equal(t, "assistant", msg.Role)
		assert.Equal(t, "ずんだもんなのだ。元気なのだ。", msg.Content)
		require.NotNil(t, msg.Emotion)
		assert.Equal(t, "喜び", *msg.Emotion)
		assert.Equal(t, sinkNow, msg.CreatedAt)
	})

	t.Run("W-07-R3_チャンク0件のままdoneならcontentは空で保存される", func(t *testing.T) {
		repo := new(mockMessageRepository)
		repo.On("InsertMessage", mock.Anything, mock.Anything).Return(nil)
		inner := new(mockEventSink)
		inner.On("SendDone").Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendDone())

		msg := insertedMessage(t, repo)
		assert.Equal(t, "", msg.Content)
	})

	t.Run("W-07-R4_emotion未受信のままdoneならemotionはnil", func(t *testing.T) {
		repo := new(mockMessageRepository)
		repo.On("InsertMessage", mock.Anything, mock.Anything).Return(nil)
		inner := new(mockEventSink)
		inner.On("SendTextChunk", mock.Anything).Return(nil)
		inner.On("SendDone").Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendTextChunk("なのだ"))
		require.NoError(t, sink.SendDone())

		msg := insertedMessage(t, repo)
		assert.Nil(t, msg.Emotion, "assistant の emotion は nullable")
	})

	t.Run("W-07-R5_InsertMessage失敗でもinnerSendDoneが呼ばれnilを返す", func(t *testing.T) {
		// 判断の固定（指示書§2.2）: 保存失敗はログのみで done は送る。
		// 応答は既にユーザーの画面に届いており、ここでエラートーストを出すと混乱させるだけ。
		// 履歴の欠落は次回の文脈が減るだけで致命的でない。
		repo := new(mockMessageRepository)
		repo.On("InsertMessage", mock.Anything, mock.Anything).Return(errors.New("db down"))
		inner := new(mockEventSink)
		inner.On("SendTextChunk", mock.Anything).Return(nil)
		inner.On("SendDone").Return(nil)
		sink := newTestRecordingSink(repo, inner)
		require.NoError(t, sink.SendTextChunk("なのだ"))

		err := sink.SendDone()

		assert.NoError(t, err, "保存失敗を呼び出し元へ伝播させない")
		inner.AssertNumberOfCalls(t, "SendDone", 1)
	})

	t.Run("W-07-R6_SendErrorではInsertMessageが呼ばれない", func(t *testing.T) {
		repo := new(mockMessageRepository)
		inner := new(mockEventSink)
		inner.On("SendTextChunk", mock.Anything).Return(nil)
		inner.On("SendError", "応答の生成に失敗しました").Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendTextChunk("途中まで"))
		require.NoError(t, sink.SendError("応答の生成に失敗しました"))

		repo.AssertNotCalled(t, "InsertMessage", mock.Anything, mock.Anything)
		inner.AssertNumberOfCalls(t, "SendError", 1)
	})

	t.Run("W-07-R7_InsertMessageはinnerSendDoneより前に呼ばれる", func(t *testing.T) {
		// 順序が逆になると「画面には出たがDBに無い」状態が起こる（仕様書§3.2）。
		repo := new(mockMessageRepository)
		inner := new(mockEventSink)
		var order []string
		repo.On("InsertMessage", mock.Anything, mock.Anything).
			Run(func(mock.Arguments) { order = append(order, "InsertMessage") }).Return(nil)
		inner.On("SendDone").
			Run(func(mock.Arguments) { order = append(order, "inner.SendDone") }).Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendDone())

		assert.Equal(t, []string{"InsertMessage", "inner.SendDone"}, order)
	})

	t.Run("W-07-R8_innerのエラーはそのまま伝播する", func(t *testing.T) {
		repo := new(mockMessageRepository)
		inner := new(mockEventSink)
		wantErr := errors.New("sink overflow")
		inner.On("SendEmotion", "困惑").Return(wantErr)
		sink := newTestRecordingSink(repo, inner)

		err := sink.SendEmotion("困惑")

		assert.ErrorIs(t, err, wantErr, "ResponseStreamer の中断判定を壊さないため委譲先のエラーは隠さない")
	})

	t.Run("W-07-R9_複数チャンクの連結順序が保たれる", func(t *testing.T) {
		repo := new(mockMessageRepository)
		repo.On("InsertMessage", mock.Anything, mock.Anything).Return(nil)
		inner := new(mockEventSink)
		inner.On("SendTextChunk", mock.Anything).Return(nil)
		inner.On("SendDone").Return(nil)
		sink := newTestRecordingSink(repo, inner)

		for _, chunk := range []string{"あ", "い", "う", "え", "お"} {
			require.NoError(t, sink.SendTextChunk(chunk))
		}
		require.NoError(t, sink.SendDone())

		assert.Equal(t, "あいうえお", insertedMessage(t, repo).Content)
	})

	t.Run("W-07-R10_emotionは最後に受信した値が保存される", func(t *testing.T) {
		repo := new(mockMessageRepository)
		repo.On("InsertMessage", mock.Anything, mock.Anything).Return(nil)
		inner := new(mockEventSink)
		inner.On("SendEmotion", mock.Anything).Return(nil)
		inner.On("SendDone").Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendEmotion("喜び"))
		require.NoError(t, sink.SendEmotion("困惑"))
		require.NoError(t, sink.SendDone())

		require.NotNil(t, insertedMessage(t, repo).Emotion)
		assert.Equal(t, "困惑", *insertedMessage(t, repo).Emotion)
	})
}

func TestRecordingSink_保存用ctx(t *testing.T) {
	// 指示書§2.2 の署名には ctx が無いため、SendDone 内で保存用の ctx を自前で作る。
	// context.Background() 単体だとDBハング時に SendDone が無制限に待つので、
	// 短いタイムアウトを被せる（リクエストのキャンセルから独立して保存できる利点は維持）。
	// ctx の状態は「InsertMessage が呼ばれた瞬間」に見る必要がある。
	// SendDone を抜けた後は defer cancel() 済みなので、後から検査すると
	// 常にキャンセル済みに見えてしまう（テスト側の観測点の誤り）。
	type ctxSnapshot struct {
		hasDeadline bool
		remaining   time.Duration
		err         error
	}
	captureCtx := func(repo *mockMessageRepository, snap *ctxSnapshot) {
		repo.On("InsertMessage", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				ctx := args.Get(0).(context.Context)
				deadline, ok := ctx.Deadline()
				snap.hasDeadline = ok
				if ok {
					snap.remaining = time.Until(deadline)
				}
				snap.err = ctx.Err()
			}).Return(nil)
	}

	t.Run("W-07-R11_InsertMessageへ渡すctxには5秒のタイムアウトが設定される", func(t *testing.T) {
		repo := new(mockMessageRepository)
		var snap ctxSnapshot
		captureCtx(repo, &snap)
		inner := new(mockEventSink)
		inner.On("SendDone").Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendDone())

		require.True(t, snap.hasDeadline, "タイムアウト無しだとDBハング時にSendDoneが無制限に待つ")
		assert.Greater(t, snap.remaining, 4*time.Second, "5秒のタイムアウトであること")
		assert.LessOrEqual(t, snap.remaining, 5*time.Second)
	})

	t.Run("W-07-R12_保存用ctxは呼び出し時点でキャンセルされていない", func(t *testing.T) {
		repo := new(mockMessageRepository)
		var snap ctxSnapshot
		captureCtx(repo, &snap)
		inner := new(mockEventSink)
		inner.On("SendDone").Return(nil)
		sink := newTestRecordingSink(repo, inner)

		require.NoError(t, sink.SendDone())

		assert.NoError(t, snap.err,
			"リクエストのキャンセルとは独立に保存できること（応答を失わないため）")
	})
}
