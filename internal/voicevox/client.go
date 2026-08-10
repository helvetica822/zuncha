// Package voicevox は VOICEVOX ENGINE の HTTP API を tts.TTSClient として提供する。
//
// 直接依存を本パッケージへ閉じ込め、差し替え可能に保つ（F-TTS-02/03、NF-MAINT-01、C-02）。
// 合成した WAV はファイルストアへ保存し、audio_files へ登録して `/audio/{ulid}` を返す
// （docs/04_implementation/04_realtime_wiring_design.md D-4）。
package voicevox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"zuncha/internal/model"
	"zuncha/internal/tts"
)

// speakerID は合成に使う VOICEVOX のスタイルID。
//
// 根拠: VOICEVOX ENGINE の `GET /speakers` が返す話者一覧のうち、
// 「ずんだもん」の styles に含まれる「ノーマル」の id が 3（既定エンジンの標準割り当て。
// あまあま=1 / セクシー=5 / ツンツン=7 / ささやき=22 と同じ並びの一部）。
// 本アプリのキャラクターはずんだもんで、平常の読み上げに使うため「ノーマル」を選ぶ。
// 起動先のエンジンで確認する場合は `curl -s :50021/speakers | jq '.[] | select(.name=="ずんだもん") | .styles'`。
const speakerID = 3

// VOICEVOX API のエンドポイントとパラメータ名。
const (
	pathAudioQuery = "/audio_query"
	pathSynthesis  = "/synthesis"
	paramText      = "text"
	paramSpeaker   = "speaker"
)

const (
	// requestTimeout は audio_query / synthesis の1リクエストあたりの上限。
	// ハンドラ側60秒のうち LLM(30秒)・DB保存の余地を残す配分。
	requestTimeout = 20 * time.Second
	// defaultAudioDir は WAV の保存先。FetchAudio が取得時に削除する一時領域。
	// W-11(Docker Compose) でボリュームを割り当てる際は WithAudioDir で上書きする。
	defaultAudioDir = "/tmp/zuncha/audio"
	// audioFileExt / audioURLPrefix は VOICEVOX が返す形式と SSE audio_url の仕様（F-RT-01）。
	audioFileExt   = ".wav"
	audioURLPrefix = "/audio/"

	contentTypeJSON = "application/json"
)

// AudioRepository は音声レコードの登録のみを必要とする（消費側で定義）。
// repository.AudioRepository 全体に依存させると、本パッケージが使わない
// 取得・削除の契約変更にまで巻き込まれる。
type AudioRepository interface {
	InsertRecord(ctx context.Context, audio *model.AudioFile) error
}

// FileWriter は WAV の書き込みのみを必要とする（消費側で定義）。
// 読み取り側の service.FileStore とは意図的に分けている（localfs.FileStore.Write のコメント参照）。
type FileWriter interface {
	Write(path string, data []byte) error
}

// Client は VOICEVOX 経由の tts.TTSClient 実装。
type Client struct {
	baseURL    string
	httpClient *http.Client
	repo       AudioRepository
	files      FileWriter
	newID      func() string
	now        func() time.Time
	audioDir   string
}

var _ tts.TTSClient = (*Client)(nil)

// Option は Client 生成時のオプション。
type Option func(*Client)

// WithAudioDir は WAV の保存先ディレクトリを差し替える。
func WithAudioDir(dir string) Option {
	return func(c *Client) { c.audioDir = dir }
}

// NewClient は VOICEVOX クライアントを生成する。
//
// baseURL は環境変数の読み出しを cmd/api の loadConfig に集約する既存方針に合わせ、
// 引数で受け取る（本パッケージ内で os.Getenv を読まない）。
// newID/now は関数注入（乱数・時刻に依存させない既存方針と一貫させ、テストで決定化できる）。
func NewClient(
	baseURL string,
	repo AudioRepository,
	files FileWriter,
	newID func() string,
	now func() time.Time,
	opts ...Option,
) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("voicevox: ベースURLが空です")
	}

	c := &Client{
		// 末尾スラッシュを落としてから固定パスを連結する（"//audio_query" を避ける）。
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: requestTimeout},
		repo:       repo,
		files:      files,
		newID:      newID,
		now:        now,
		audioDir:   defaultAudioDir,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Synthesize は text を合成し、SSE で配信する `/audio/{ulid}` を返す。
//
// 手順: audio_query（韻律クエリ生成）→ synthesis（WAV生成）→ ファイル保存 → audio_files 登録。
// ファイル保存を登録より先に行うのは、「レコードはあるがファイルが無い」状態
// （GET /audio/{id} が 500 になる窓）を作らないため。逆順の孤児ファイルは
// 実害が小さく、掃除は別Waveの検討事項とする。
//
// messageID に対応する messages 行はこの時点でまだ存在しない（保存は SendDone のタイミング）。
// audio_files.message_id は FK を持たないためこれで INSERT できる（D-4 訂正2）。
// 存在チェックは意図的に行わない。
func (c *Client) Synthesize(ctx context.Context, text, conversationID, messageID string) (string, error) {
	query, err := c.audioQuery(ctx, text)
	if err != nil {
		return "", err
	}

	wav, err := c.synthesis(ctx, query)
	if err != nil {
		return "", err
	}

	id := c.newID()
	path := filepath.Join(c.audioDir, id+audioFileExt)
	if err := c.files.Write(path, wav); err != nil {
		return "", fmt.Errorf("voicevox: WAVの保存に失敗しました: %w", err)
	}

	if err := c.repo.InsertRecord(ctx, &model.AudioFile{
		ID:             id,
		ConversationID: conversationID,
		MessageID:      messageID,
		FilePath:       path,
		CreatedAt:      c.now(),
	}); err != nil {
		// 登録できていないULIDのURLを返すと、フロントの GET /audio/{id} が 404 になる。
		return "", fmt.Errorf("voicevox: 音声レコードの登録に失敗しました: %w", err)
	}

	return audioURLPrefix + id, nil
}

// audioQuery は POST /audio_query?text=...&speaker=... の応答（クエリJSON）をそのまま返す。
// クライアント側で JSON を解釈しない（エンジンのフィールド追加に追従不要にするため）。
func (c *Client) audioQuery(ctx context.Context, text string) ([]byte, error) {
	params := url.Values{}
	params.Set(paramText, text)
	params.Set(paramSpeaker, strconv.Itoa(speakerID))

	body, err := c.post(ctx, pathAudioQuery+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("voicevox: audio_query に失敗しました: %w", err)
	}
	return body, nil
}

// synthesis は audio_query の応答をボディに載せて POST /synthesis?speaker=... を呼び、WAVを返す。
// query を作り直すと audio_query が算出した韻律が捨てられるため、必ず受け取ったバイト列を渡す。
func (c *Client) synthesis(ctx context.Context, query []byte) ([]byte, error) {
	params := url.Values{}
	params.Set(paramSpeaker, strconv.Itoa(speakerID))

	body, err := c.post(ctx, pathSynthesis+"?"+params.Encode(), query)
	if err != nil {
		return nil, fmt.Errorf("voicevox: synthesis に失敗しました: %w", err)
	}
	return body, nil
}

// post は VOICEVOX へ POST し、2xx の応答ボディを返す。非2xxはステータス付きのエラーにする。
func (c *Client) post(ctx context.Context, pathWithQuery string, body []byte) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+pathWithQuery, reader)
	if err != nil {
		return nil, fmt.Errorf("リクエストの生成に失敗: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// *url.Error は Unwrap を持つため、包み直さずに返すだけで
		// errors.Is(err, context.Canceled) が成立する。
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("応答の読み取りに失敗: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// 切り分けのためステータスは残す。応答本文は載せない（発話内容が混ざり得るため NF-SEC）。
		return nil, fmt.Errorf("予期しないステータス %d", resp.StatusCode)
	}
	return respBody, nil
}
