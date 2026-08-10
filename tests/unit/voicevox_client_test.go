// 対応仕様: docs/04_implementation/04_realtime_wiring_design.md D-4(2026-08-10訂正版)、
// tasks/instructions_zundamon_wave_w09.md §2.1/§3（W-09 VOICEVOX TTSクライアント）
//
// 本テストは httptest のフェイクVOICEVOXのみで駆動し、実VOICEVOX(既定 :50021)へは一切接続しない。
package unit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"zuncha/internal/model"
	"zuncha/internal/tts"
	"zuncha/internal/voicevox"
)

const (
	// wantSpeakerID は VOICEVOX ENGINE の /speakers が返す styles[].id のうち
	// 「ずんだもん / ノーマル」に対応する値。実装側の定数とは独立にテスト側で仕様を再掲する。
	wantSpeakerID = "3"

	vvConvID      = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	vvMessageID   = "01JASSISTANTMESSAGE0000000"
	vvAudioID     = "01JAUDIOFILEID000000000000"
	vvText        = "こんにちはなのだ。"
	vvAudioDir    = "/tmp/zuncha-test-audio"
	vvQueryJSON   = `{"accent_phrases":[],"speedScale":1.0,"kana":"コンニチワナノダ"}`
	vvSynthesized = "RIFF\x00\x00\x00\x00WAVEfake"
)

var vvNow = time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)

// recordedRequest はフェイクVOICEVOXが受けたリクエストの観測結果。
type recordedRequest struct {
	TextParam    string
	SpeakerParam string
	Body         string
	ContentType  string
}

// fakeVoicevox は audio_query / synthesis の2エンドポイントを模す。
type fakeVoicevox struct {
	mu sync.Mutex

	// 応答の制御（0 なら 200 として扱う）。
	audioQueryStatus int
	synthesisStatus  int
	queryJSON        string
	wav              string

	audioQueries []recordedRequest
	syntheses    []recordedRequest
}

func newFakeVoicevox() *fakeVoicevox {
	return &fakeVoicevox{queryJSON: vvQueryJSON, wav: vvSynthesized}
}

func (f *fakeVoicevox) record(dst *[]recordedRequest, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	*dst = append(*dst, recordedRequest{
		TextParam:    r.URL.Query().Get("text"),
		SpeakerParam: r.URL.Query().Get("speaker"),
		Body:         string(body),
		ContentType:  r.Header.Get("Content-Type"),
	})
}

func (f *fakeVoicevox) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/audio_query":
		f.record(&f.audioQueries, r)
		if f.audioQueryStatus != 0 {
			w.WriteHeader(f.audioQueryStatus)
			_, _ = w.Write([]byte(`{"detail":"fake audio_query error"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(f.queryJSON))
	case "/synthesis":
		f.record(&f.syntheses, r)
		if f.synthesisStatus != 0 {
			w.WriteHeader(f.synthesisStatus)
			_, _ = w.Write([]byte(`{"detail":"fake synthesis error"}`))
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write([]byte(f.wav))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeVoicevox) recordedAudioQueries() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.audioQueries...)
}

func (f *fakeVoicevox) recordedSyntheses() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.syntheses...)
}

// mockFileWriter は WAV 書き込み側の I/F（voicevox 側で消費側定義）のモック。
type mockFileWriter struct{ mock.Mock }

func (m *mockFileWriter) Write(path string, data []byte) error {
	args := m.Called(path, data)
	return args.Error(0)
}

// vvFixture は Client と観測用モックをまとめる。
type vvFixture struct {
	client *voicevox.Client
	fake   *fakeVoicevox
	repo   *mockAudioRepository
	files  *mockFileWriter
	ids    []string
	idx    int
}

// newVVFixture は「正常に一巡する」状態のフェイク・モックを組んだ Client を返す。
// ids は newID が返す値の並び（尽きたら最後の値を返す）。
func newVVFixture(t *testing.T, ids ...string) *vvFixture {
	t.Helper()
	if len(ids) == 0 {
		ids = []string{vvAudioID}
	}
	f := &vvFixture{
		fake:  newFakeVoicevox(),
		repo:  new(mockAudioRepository),
		files: new(mockFileWriter),
		ids:   ids,
	}
	srv := httptest.NewServer(f.fake)
	t.Cleanup(srv.Close)

	f.repo.On("InsertRecord", mock.Anything, mock.Anything).Return(nil)
	f.files.On("Write", mock.Anything, mock.Anything).Return(nil)

	c, err := voicevox.NewClient(
		srv.URL,
		f.repo,
		f.files,
		func() string {
			id := f.ids[min(f.idx, len(f.ids)-1)]
			f.idx++
			return id
		},
		func() time.Time { return vvNow },
		voicevox.WithAudioDir(vvAudioDir),
	)
	require.NoError(t, err)
	f.client = c
	return f
}

func (f *vvFixture) recordedAudioQueries() []recordedRequest { return f.fake.recordedAudioQueries() }

func (f *vvFixture) recordedSyntheses() []recordedRequest { return f.fake.recordedSyntheses() }

// insertedAudio は InsertRecord に渡された *model.AudioFile を取り出す。
func insertedAudio(t *testing.T, repo *mockAudioRepository) *model.AudioFile {
	t.Helper()
	for _, call := range repo.Calls {
		if call.Method == "InsertRecord" {
			audio, ok := call.Arguments.Get(1).(*model.AudioFile)
			require.True(t, ok, "InsertRecord の第2引数が *model.AudioFile ではない")
			return audio
		}
	}
	t.Fatal("InsertRecord が呼ばれていない")
	return nil
}

// writtenFile は Write に渡された (path, data) を取り出す。
func writtenFile(t *testing.T, files *mockFileWriter) (string, []byte) {
	t.Helper()
	for _, call := range files.Calls {
		if call.Method == "Write" {
			return call.Arguments.String(0), call.Arguments.Get(1).([]byte)
		}
	}
	t.Fatal("Write が呼ばれていない")
	return "", nil
}

// --- I/F 適合 ---------------------------------------------------------------

var _ tts.TTSClient = (*voicevox.Client)(nil)

// --- 正常系 -----------------------------------------------------------------

func TestVoicevoxSynthesize_正常系(t *testing.T) {
	t.Run("W-09-V1_audio_queryとsynthesisを経てaudioURLを返す", func(t *testing.T) {
		f := newVVFixture(t)

		url, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		assert.Equal(t, "/audio/"+vvAudioID, url, "SSE audio_url は /audio/{ulid} 形式（F-RT-01）")
		assert.Len(t, f.recordedAudioQueries(), 1)
		assert.Len(t, f.recordedSyntheses(), 1)
	})

	t.Run("W-09-V2_audio_queryにtextとspeakerIDが乗る", func(t *testing.T) {
		f := newVVFixture(t)

		_, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		got := f.recordedAudioQueries()[0]
		assert.Equal(t, vvText, got.TextParam)
		assert.Equal(t, wantSpeakerID, got.SpeakerParam, "ずんだもん(ノーマル)のスタイルIDであること")
	})

	t.Run("W-09-V3_synthesisにも同じspeakerIDが乗る", func(t *testing.T) {
		f := newVVFixture(t)

		_, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		assert.Equal(t, wantSpeakerID, f.recordedSyntheses()[0].SpeakerParam,
			"audio_query と別のspeakerだと韻律と声が食い違う")
	})

	t.Run("W-09-V4_synthesisのbodyはaudio_queryの応答をそのまま渡す", func(t *testing.T) {
		f := newVVFixture(t)
		f.fake.queryJSON = `{"accent_phrases":[{"moras":[]}],"speedScale":1.25,"kana":"テスト"}`

		_, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		got := f.recordedSyntheses()[0]
		assert.JSONEq(t, f.fake.queryJSON, got.Body,
			"クエリを作り直すと audio_query の韻律結果が捨てられる")
		assert.Equal(t, f.fake.queryJSON, got.Body, "バイト列としてもそのまま渡すこと")
		assert.Equal(t, "application/json", got.ContentType)
	})

	t.Run("W-09-V5_WAVがULIDベースのパスへ書き込まれる", func(t *testing.T) {
		f := newVVFixture(t)

		_, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		path, data := writtenFile(t, f.files)
		assert.Equal(t, filepath.Join(vvAudioDir, vvAudioID+".wav"), path)
		assert.Equal(t, []byte(vvSynthesized), data, "synthesis の応答バイト列がそのまま保存されること")
	})

	t.Run("W-09-V6_audio_filesレコードが渡した会話ID_メッセージIDで登録される", func(t *testing.T) {
		f := newVVFixture(t)

		url, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		audio := insertedAudio(t, f.repo)
		assert.Equal(t, vvAudioID, audio.ID)
		assert.Equal(t, "/audio/"+audio.ID, url, "戻り値のULIDとレコードのIDが一致すること")
		assert.Equal(t, vvConvID, audio.ConversationID)
		assert.Equal(t, vvMessageID, audio.MessageID)
		assert.Equal(t, filepath.Join(vvAudioDir, vvAudioID+".wav"), audio.FilePath)
		assert.Equal(t, vvNow, audio.CreatedAt)
		assert.Nil(t, audio.FetchedAt, "未取得=NULL が INSERT 時の正しい初期状態")
	})

	t.Run("W-09-V7_呼び出しごとに別ULID_別パスになる", func(t *testing.T) {
		f := newVVFixture(t, "01JAUDIOFIRST0000000000000", "01JAUDIOSECOND000000000000")

		first, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)
		require.NoError(t, err)
		second, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)
		require.NoError(t, err)

		assert.Equal(t, "/audio/01JAUDIOFIRST0000000000000", first)
		assert.Equal(t, "/audio/01JAUDIOSECOND000000000000", second)
		assert.NotEqual(t, first, second, "同一テキストでもファイルは一意であること")
	})

	t.Run("W-09-V8_書き込みはINSERTより前に行われる", func(t *testing.T) {
		// 順序が逆だと「レコードはあるがファイルが無い」状態が生じ、GET /audio/{id} が
		// 500 になる（レコードだけ先に見える窓ができる）。
		f := newVVFixture(t)
		var order []string
		f.files.ExpectedCalls = nil
		f.repo.ExpectedCalls = nil
		f.files.On("Write", mock.Anything, mock.Anything).
			Run(func(mock.Arguments) { order = append(order, "Write") }).Return(nil)
		f.repo.On("InsertRecord", mock.Anything, mock.Anything).
			Run(func(mock.Arguments) { order = append(order, "InsertRecord") }).Return(nil)

		_, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		assert.Equal(t, []string{"Write", "InsertRecord"}, order)
	})
}

func TestVoicevoxSynthesize_エッジケース(t *testing.T) {
	t.Run("W-09-V9_区切り文字を含むテキストもエスケープされて欠落しない", func(t *testing.T) {
		// & や = を素朴に文字列連結でURLへ載せると、speaker が上書きされたり
		// テキストが途中で切れたりする。
		f := newVVFixture(t)
		tricky := "a&speaker=99&text=乗っ取り?#断片 プラス+記号"

		_, err := f.client.Synthesize(context.Background(), tricky, vvConvID, vvMessageID)

		require.NoError(t, err)
		got := f.recordedAudioQueries()[0]
		assert.Equal(t, tricky, got.TextParam, "テキストは1つのクエリ値として届くこと")
		assert.Equal(t, wantSpeakerID, got.SpeakerParam, "speaker がテキストで上書きされないこと")
	})

	t.Run("W-09-V10_1000文字の長文でも全文が渡る", func(t *testing.T) {
		f := newVVFixture(t)
		long := strings.Repeat("あ", 1000)

		_, err := f.client.Synthesize(context.Background(), long, vvConvID, vvMessageID)

		require.NoError(t, err)
		assert.Equal(t, long, f.recordedAudioQueries()[0].TextParam)
	})

	t.Run("W-09-V11_空テキストでもクライアント側で弾かずVOICEVOXへ渡す", func(t *testing.T) {
		// 空文字の可否判定はVOICEVOX側の責務。クライアントで独自に弾くと
		// エンジンの挙動変更に追従できなくなる。
		f := newVVFixture(t)

		_, err := f.client.Synthesize(context.Background(), "", vvConvID, vvMessageID)

		require.NoError(t, err)
		require.Len(t, f.recordedAudioQueries(), 1)
		assert.Equal(t, "", f.recordedAudioQueries()[0].TextParam)
	})

	t.Run("W-09-V12_空のWAV応答でも0バイトで保存され成功する", func(t *testing.T) {
		f := newVVFixture(t)
		f.fake.wav = ""

		url, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		assert.Equal(t, "/audio/"+vvAudioID, url)
		_, data := writtenFile(t, f.files)
		assert.Empty(t, data)
	})
}

// --- 異常系 -----------------------------------------------------------------

func TestVoicevoxSynthesize_異常系(t *testing.T) {
	t.Run("W-09-V13_audio_queryが非2xxならエラーで中断する", func(t *testing.T) {
		f := newVVFixture(t)
		f.fake.audioQueryStatus = http.StatusUnprocessableEntity

		url, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.Error(t, err)
		assert.Empty(t, url)
		assert.Empty(t, f.recordedSyntheses(), "audio_query が失敗したら synthesis を呼ばない")
		f.files.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
		f.repo.AssertNotCalled(t, "InsertRecord", mock.Anything, mock.Anything)
	})

	t.Run("W-09-V14_synthesisが非2xxならエラーで中断する", func(t *testing.T) {
		f := newVVFixture(t)
		f.fake.synthesisStatus = http.StatusInternalServerError

		url, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.Error(t, err)
		assert.Empty(t, url)
		f.files.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
		f.repo.AssertNotCalled(t, "InsertRecord", mock.Anything, mock.Anything)
	})

	t.Run("W-09-V15_エラーメッセージにステータスコードが含まれる", func(t *testing.T) {
		f := newVVFixture(t)
		f.fake.audioQueryStatus = http.StatusUnprocessableEntity

		_, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "422", "切り分けのためステータスは残す")
	})

	t.Run("W-09-V16_キャンセル済みctxはcontext_Canceledで失敗する", func(t *testing.T) {
		f := newVVFixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := f.client.Synthesize(ctx, vvText, vvConvID, vvMessageID)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, f.recordedAudioQueries(), "キャンセル済みならリクエスト自体が飛ばない")
	})

	t.Run("W-09-V17_ファイル書き込み失敗ならエラーでINSERTしない", func(t *testing.T) {
		f := newVVFixture(t)
		f.files.ExpectedCalls = nil
		f.files.On("Write", mock.Anything, mock.Anything).Return(errors.New("disk full"))

		url, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.Error(t, err)
		assert.Empty(t, url)
		f.repo.AssertNotCalled(t, "InsertRecord", mock.Anything, mock.Anything)
	})

	t.Run("W-09-V18_INSERT失敗ならエラーを返す_WAVは書き込み済みでよい", func(t *testing.T) {
		// 孤児ファイル（レコードなしのWAV）が残るが、掃除は別Waveの検討事項
		// （tasks/todo.md 申し送り）。ここでエラーを返さないと SSE audio_url が
		// 404 になる URL を配信してしまう。
		f := newVVFixture(t)
		f.repo.ExpectedCalls = nil
		f.repo.On("InsertRecord", mock.Anything, mock.Anything).Return(errors.New("db down"))

		url, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.Error(t, err)
		assert.Empty(t, url, "登録できていないULIDのURLを返してはならない")
		f.files.AssertNumberOfCalls(t, "Write", 1)
	})

	t.Run("W-09-V19_VOICEVOXへ到達できない場合はエラーになる", func(t *testing.T) {
		repo := new(mockAudioRepository)
		files := new(mockFileWriter)
		// 誰もリッスンしていないポートへ向ける（httptest サーバを即閉じて再利用）。
		srv := httptest.NewServer(newFakeVoicevox())
		unreachable := srv.URL
		srv.Close()

		c, err := voicevox.NewClient(unreachable, repo, files,
			func() string { return vvAudioID }, func() time.Time { return vvNow })
		require.NoError(t, err)

		_, err = c.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.Error(t, err)
		files.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
		repo.AssertNotCalled(t, "InsertRecord", mock.Anything, mock.Anything)
	})

	t.Run("W-09-V20_audio_queryの応答が空でもsynthesisへそのまま渡す", func(t *testing.T) {
		// 空bodyの妥当性判断はVOICEVOX側。クライアントでJSONを解釈しないことの固定。
		f := newVVFixture(t)
		f.fake.queryJSON = ""

		_, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		assert.Equal(t, "", f.recordedSyntheses()[0].Body)
	})
}

func TestVoicevoxNewClient(t *testing.T) {
	t.Run("W-09-V21_ベースURLが空ならエラーになる", func(t *testing.T) {
		_, err := voicevox.NewClient("", new(mockAudioRepository), new(mockFileWriter),
			func() string { return vvAudioID }, func() time.Time { return vvNow })

		require.Error(t, err, "実行時に初めて落ちるより生成時に落とす")
	})

	t.Run("W-09-V22_末尾スラッシュ付きのベースURLでもパスが二重にならない", func(t *testing.T) {
		fake := newFakeVoicevox()
		srv := httptest.NewServer(fake)
		t.Cleanup(srv.Close)
		repo := new(mockAudioRepository)
		repo.On("InsertRecord", mock.Anything, mock.Anything).Return(nil)
		files := new(mockFileWriter)
		files.On("Write", mock.Anything, mock.Anything).Return(nil)

		c, err := voicevox.NewClient(srv.URL+"/", repo, files,
			func() string { return vvAudioID }, func() time.Time { return vvNow })
		require.NoError(t, err)

		url, err := c.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

		require.NoError(t, err)
		assert.Equal(t, "/audio/"+vvAudioID, url)
	})
}

// TestVoicevoxQueryJSONは、クライアントがクエリJSONを解釈しない（構造に依存しない）ことを固定する。
func TestVoicevoxSynthesize_クエリJSONの構造に依存しない(t *testing.T) {
	f := newVVFixture(t)
	// 将来 VOICEVOX がフィールドを増やしても素通しできること。
	f.fake.queryJSON = `{"unknown_future_field":123,"accent_phrases":[]}`

	_, err := f.client.Synthesize(context.Background(), vvText, vvConvID, vvMessageID)

	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(f.recordedSyntheses()[0].Body), &decoded))
	assert.Equal(t, float64(123), decoded["unknown_future_field"])
}
