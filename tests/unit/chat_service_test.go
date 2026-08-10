// service.ChatService の単体テスト。
// 対応: tasks/instructions_zundamon_wave_b2.md §2.3 (W-07)、docs/04_implementation/05_sse_protocol_spec.md §3.2/§3.3
package unit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"zuncha/internal/llm"
	"zuncha/internal/model"
	"zuncha/internal/service"
	"zuncha/internal/sse"
)

const (
	chatConvID    = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	chatRequestID = "01JREQUEST0000000000000000"
	chatUserText  = "こんにちはずんだもん"
)

// chatFixture は ChatService とその依存モックをまとめて保持する。
type chatFixture struct {
	svc      *service.ChatService
	msgRepo  *mockMessageRepository
	convRepo *mockConversationRepository
	llmC     *mockLLMClient
	parser   *mockResponseParser
	tts      *mockTTSClient
	hub      *sse.Hub

	mu    sync.Mutex
	order []string
	// prompts は LLM へ渡されたプロンプトを記録する（履歴の組み立て結果の検証用）。
	prompts []string
	now     time.Time
	ids     int
}

func (f *chatFixture) record(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.order = append(f.order, name)
}

func (f *chatFixture) callOrder() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.order...)
}

func (f *chatFixture) recordedPrompts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.prompts...)
}

// newChatFixture は「正常に一巡する」状態のモックを組んだ ChatService を返す。
// 個別テストは必要な期待値だけ上書きする。
func newChatFixture(t *testing.T, history []model.Message) *chatFixture {
	t.Helper()
	f := &chatFixture{
		msgRepo:  new(mockMessageRepository),
		convRepo: new(mockConversationRepository),
		llmC:     new(mockLLMClient),
		parser:   new(mockResponseParser),
		tts:      new(mockTTSClient),
		hub:      sse.NewHub(),
		now:      time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
	}

	f.msgRepo.On("InsertMessage", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { f.record("InsertMessage") }).Return(nil)
	f.msgRepo.On("GetRecentMessages", mock.Anything, chatConvID).
		Run(func(mock.Arguments) { f.record("GetRecentMessages") }).Return(history, nil)
	f.convRepo.On("SetFirstText", mock.Anything, chatConvID, mock.Anything).
		Run(func(mock.Arguments) { f.record("SetFirstText") }).Return(nil)
	f.llmC.On("GenerateResponse", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			f.mu.Lock()
			f.prompts = append(f.prompts, args.String(1))
			f.mu.Unlock()
			f.record("GenerateResponse")
		}).Return([]byte(`{}`), nil)
	f.parser.On("Parse", mock.Anything).
		Return(&llm.LLMResponse{Text: "こんにちはなのだ。", Emotion: "喜び"}, nil)
	// 既定は TTS 失敗（audio_url をスキップして done へ進む経路）。W-09 で引数が
	// (ctx, text, conversationID, messageID) の4つになった。
	f.tts.On("Synthesize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", errors.New("tts未実装"))

	streamer := service.NewResponseStreamer(f.llmC, f.parser, f.tts, sse.NewSentenceChunker())
	f.svc = service.NewChatService(
		f.msgRepo, f.convRepo, streamer, f.hub,
		func() string { f.ids++; return "01JGENERATEDID00000000000" + string(rune('A'+f.ids)) },
		func() time.Time { return f.now },
	)
	return f
}

func TestChatService_HandleUserMessage(t *testing.T) {
	t.Run("W-07-C1_保存_first_text_履歴取得_LLMの順に呼ばれる", func(t *testing.T) {
		f := newChatFixture(t, nil)

		err := f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText)

		require.NoError(t, err)
		order := f.callOrder()
		require.GreaterOrEqual(t, len(order), 4)
		assert.Equal(t, "InsertMessage", order[0], "ユーザー発話の保存が最初")
		assert.Equal(t, "SetFirstText", order[1])
		assert.Equal(t, "GetRecentMessages", order[2],
			"保存より先に履歴を取ると自分の発話が文脈から抜ける")
		assert.Equal(t, "GenerateResponse", order[3])
	})

	t.Run("W-07-C2_ユーザー発話がuser_roleで保存される", func(t *testing.T) {
		f := newChatFixture(t, nil)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		msg := insertedMessage(t, f.msgRepo)
		assert.Equal(t, "user", msg.Role)
		assert.Equal(t, chatUserText, msg.Content)
		assert.Equal(t, chatConvID, msg.ConversationID)
		assert.Equal(t, f.now, msg.CreatedAt)
		assert.NotEmpty(t, msg.ID, "IDは注入した newID で採番される")
	})

	t.Run("W-07-C3_履歴に自分の発話が含まれてプロンプトになる", func(t *testing.T) {
		// ChatService が先に保存するので、GetRecentMessages の戻りに当該発話が含まれる。
		history := []model.Message{
			{Role: "user", Content: "前の発話"},
			{Role: "assistant", Content: "前の応答なのだ"},
			{Role: "user", Content: chatUserText},
		}
		f := newChatFixture(t, history)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		prompts := f.recordedPrompts()
		require.Len(t, prompts, 1)
		assert.Equal(t,
			"user: 前の発話\nassistant: 前の応答なのだ\nuser: "+chatUserText,
			prompts[0])
		assert.Contains(t, prompts[0], chatUserText, "自分の発話が文脈に含まれること")
	})

	t.Run("W-07-C4_SetFirstTextは20ルーンに切り詰めて渡される", func(t *testing.T) {
		f := newChatFixture(t, nil)
		long := "あいうえおかきくけこさしすせそたちつてとなにぬねの" // 25ルーン

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, long))

		f.convRepo.AssertCalled(t, "SetFirstText", mock.Anything, chatConvID,
			"あいうえおかきくけこさしすせそたちつてと")
	})

	t.Run("W-07-C5_SetFirstTextが失敗しても処理は続行する", func(t *testing.T) {
		f := newChatFixture(t, nil)
		f.convRepo.ExpectedCalls = nil
		f.convRepo.On("SetFirstText", mock.Anything, mock.Anything, mock.Anything).
			Run(func(mock.Arguments) { f.record("SetFirstText") }).Return(errors.New("db down"))

		err := f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText)

		require.NoError(t, err, "first_text は付随処理なので会話本体を止めない")
		f.llmC.AssertNumberOfCalls(t, "GenerateResponse", 1)
	})

	t.Run("W-07-C6_ユーザー発話の保存が失敗したら中断してエラーを返す", func(t *testing.T) {
		f := newChatFixture(t, nil)
		f.msgRepo.ExpectedCalls = nil
		f.msgRepo.On("InsertMessage", mock.Anything, mock.Anything).Return(errors.New("db down"))
		f.msgRepo.On("GetRecentMessages", mock.Anything, mock.Anything).Return(nil, nil)

		err := f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText)

		require.Error(t, err, "発話を失ったまま応答してはならない")
		f.llmC.AssertNotCalled(t, "GenerateResponse", mock.Anything, mock.Anything)
	})

	t.Run("W-07-C7_同一request_idの2回目はLLMが呼ばれない", func(t *testing.T) {
		f := newChatFixture(t, nil)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))
		err := f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText)

		require.NoError(t, err, "再送は何もせず成功扱い（冪等）")
		f.llmC.AssertNumberOfCalls(t, "GenerateResponse", 1)
		// 1回目の user + assistant の2件のみ（2回目は何も保存しない）
		f.msgRepo.AssertNumberOfCalls(t, "InsertMessage", 2)
	})

	t.Run("W-07-C8_別のrequest_idなら処理される", func(t *testing.T) {
		f := newChatFixture(t, nil)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))
		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID,
			"01JREQUEST0000000000000001", chatUserText))

		f.llmC.AssertNumberOfCalls(t, "GenerateResponse", 2)
	})

	t.Run("W-07-C9_5分経過後の同一request_idは再度処理される", func(t *testing.T) {
		f := newChatFixture(t, nil)
		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		f.now = f.now.Add(5*time.Minute + time.Second)
		err := f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText)

		require.NoError(t, err)
		// 保持期間を過ぎた request_id は再処理される
		f.llmC.AssertNumberOfCalls(t, "GenerateResponse", 2)
	})

	t.Run("W-07-C10_5分未満の同一request_idは処理されない", func(t *testing.T) {
		f := newChatFixture(t, nil)
		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		f.now = f.now.Add(5*time.Minute - time.Second)
		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		f.llmC.AssertNumberOfCalls(t, "GenerateResponse", 1)
	})

	t.Run("W-07-C11_応答が登録済み接続へSSEイベントとして流れる", func(t *testing.T) {
		f := newChatFixture(t, nil)
		conn := newRecordingConn()
		f.hub.Register(chatConvID, conn)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		names := make([]string, 0, len(conn.recorded()))
		for _, e := range conn.recorded() {
			names = append(names, e.Name)
		}
		assert.Equal(t, []string{"emotion", "text", "done"}, names,
			"TTS失敗時は audio_url をスキップして done へ進む")
		assert.Equal(t,
			map[string]string{"request_id": chatRequestID, "label": "喜び"},
			conn.recorded()[0].Data)
	})

	t.Run("W-07-C12_assistant応答が保存される", func(t *testing.T) {
		f := newChatFixture(t, nil)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		var assistant *model.Message
		for _, call := range f.msgRepo.Calls {
			if call.Method != "InsertMessage" {
				continue
			}
			if msg := call.Arguments.Get(1).(*model.Message); msg.Role == "assistant" {
				assistant = msg
			}
		}
		require.NotNil(t, assistant, "RecordingSink 経由で assistant 応答が保存されるべき")
		assert.Equal(t, "こんにちはなのだ。", assistant.Content)
		require.NotNil(t, assistant.Emotion)
		assert.Equal(t, "喜び", *assistant.Emotion)
	})

	t.Run("W-09-C19_事前生成したassistantMessageIDがTTSと保存の両方へ伝播する", func(t *testing.T) {
		// D-4 訂正: audio_files への登録は assistant メッセージ保存（SendDone）より先に
		// 行われるため、両者が同じIDを共有できるよう ChatService が事前に採番する。
		// ここが食い違うと audio_files.message_id が存在しないメッセージを指す。
		f := newChatFixture(t, nil)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		var userID, assistantID string
		for _, call := range f.msgRepo.Calls {
			if call.Method != "InsertMessage" {
				continue
			}
			msg := call.Arguments.Get(1).(*model.Message)
			switch msg.Role {
			case "user":
				userID = msg.ID
			case "assistant":
				assistantID = msg.ID
			}
		}
		require.NotEmpty(t, assistantID)
		f.tts.AssertNumberOfCalls(t, "Synthesize", 1)
		f.tts.AssertCalled(t, "Synthesize", mock.Anything, mock.Anything, chatConvID, assistantID)
		assert.NotEqual(t, userID, assistantID,
			"ユーザー発話と同じIDを使い回すと messages の主キーが衝突する")
	})

	t.Run("W-09-C20_TTSへ渡す会話IDはリクエストの会話IDである", func(t *testing.T) {
		f := newChatFixture(t, nil)
		other := "01BRZ3NDEKTSV4RRFFQ69G5FAV"
		f.msgRepo.On("GetRecentMessages", mock.Anything, other).Return([]model.Message(nil), nil)
		f.convRepo.On("SetFirstText", mock.Anything, other, mock.Anything).Return(nil)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), other, chatRequestID, chatUserText))

		f.tts.AssertCalled(t, "Synthesize", mock.Anything, mock.Anything, other, mock.Anything)
	})

	t.Run("W-07-C13_10並列で同一request_idを投げてもLLMは1回だけ", func(t *testing.T) {
		f := newChatFixture(t, nil)
		const parallelism = 10
		var wg sync.WaitGroup
		errs := make([]error, parallelism)

		for i := 0; i < parallelism; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				errs[idx] = f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText)
			}(i)
		}
		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}
		// 二重送信の第二防衛線は並列でも1回に絞れること
		f.llmC.AssertNumberOfCalls(t, "GenerateResponse", 1)
	})
}

func TestChatService_中断時のSendError(t *testing.T) {
	// めたんの判断（申し送り sending 固着への対応）:
	// 中断してエラーを返すだけだと、handler は `_ = HandleUserMessage(...)` なので
	// フロントには done も error も届かず inputState が sending のまま固着する。
	// フロントは既に INV-3b「SSE error を受けたら sending/transcribing から editable へ復帰」
	// を持っているので、バックエンドが SendError を1発送れば既存機構で解決する。
	eventNames := func(conn *recordingConn) []string {
		names := make([]string, 0, len(conn.recorded()))
		for _, e := range conn.recorded() {
			names = append(names, e.Name)
		}
		return names
	}

	t.Run("W-07-C14_ユーザー発話の保存失敗時にerrorイベントが接続へ届く", func(t *testing.T) {
		f := newChatFixture(t, nil)
		f.msgRepo.ExpectedCalls = nil
		f.msgRepo.On("InsertMessage", mock.Anything, mock.Anything).Return(errors.New("db down"))
		f.msgRepo.On("GetRecentMessages", mock.Anything, mock.Anything).Return(nil, nil)
		conn := newRecordingConn()
		f.hub.Register(chatConvID, conn)

		err := f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText)

		require.Error(t, err)
		// require で件数を確定させてから添字アクセスする。assert のまま添字を取ると
		// イベント0件のときに index out of range で panic し、テスト関数全体が死んで
		// 後続のサブテスト（C15以降）が実行されなくなる。
		require.Equal(t, []string{"error"}, eventNames(conn),
			"中断時もSSE errorを送らないとフロントがsendingのまま固着する（INV-3b）")
		data, ok := conn.recorded()[0].Data.(map[string]string)
		require.True(t, ok)
		assert.Equal(t, chatRequestID, data["request_id"], "相関キーが入っていること")
		assert.NotEmpty(t, data["message"])
	})

	t.Run("W-07-C15_履歴取得失敗時にerrorイベントが接続へ届く", func(t *testing.T) {
		f := newChatFixture(t, nil)
		f.msgRepo.ExpectedCalls = nil
		f.msgRepo.On("InsertMessage", mock.Anything, mock.Anything).Return(nil)
		f.msgRepo.On("GetRecentMessages", mock.Anything, mock.Anything).
			Return(nil, errors.New("db down"))
		conn := newRecordingConn()
		f.hub.Register(chatConvID, conn)

		err := f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText)

		require.Error(t, err)
		assert.Equal(t, []string{"error"}, eventNames(conn))
		f.llmC.AssertNotCalled(t, "GenerateResponse", mock.Anything, mock.Anything)
	})

	t.Run("W-07-C16_正常系ではerrorイベントは流れない", func(t *testing.T) {
		f := newChatFixture(t, nil)
		conn := newRecordingConn()
		f.hub.Register(chatConvID, conn)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		assert.Equal(t, []string{"emotion", "text", "done"}, eventNames(conn))
		assert.NotContains(t, eventNames(conn), "error")
	})

	t.Run("W-07-C17_重複request_idの2回目ではerrorも流れない", func(t *testing.T) {
		// 再送は「何もしないのが正しい」。ここで error を送ると、1回目の正常応答を
		// 受け取ったフロントに余計なトーストが出る。
		f := newChatFixture(t, nil)
		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))
		conn := newRecordingConn()
		f.hub.Register(chatConvID, conn)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		assert.Empty(t, eventNames(conn))
	})

	t.Run("W-07-C18_SetFirstText失敗では続行するのでerrorは流れない", func(t *testing.T) {
		f := newChatFixture(t, nil)
		f.convRepo.ExpectedCalls = nil
		f.convRepo.On("SetFirstText", mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("db down"))
		conn := newRecordingConn()
		f.hub.Register(chatConvID, conn)

		require.NoError(t, f.svc.HandleUserMessage(context.Background(), chatConvID, chatRequestID, chatUserText))

		assert.Equal(t, []string{"emotion", "text", "done"}, eventNames(conn),
			"付随処理の失敗でユーザーにエラーを見せる必要はない")
	})
}
