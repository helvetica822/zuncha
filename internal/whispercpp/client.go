// Package whispercpp は whisper-server (HTTP) 経由の音声認識クライアントを提供する。
//
// Whisper.cpp への直接依存を本パッケージへ閉じ込め、差し替え可能に保つ
// (docs/04_implementation/04_realtime_wiring_design.md D-3 / D-3a、C-01、NF-MAINT-01)。
package whispercpp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"zuncha/internal/stt"
)

const (
	// pathInference は whisper-server の推論エンドポイント。
	pathInference = "/inference"
	// fieldFile / fieldResponseFormat は whisper-server が受け取るフォームフィールド名。
	// 根拠: whisper.cpp examples/server の組み込みヘルプが示す curl 例
	//   curl :8080/inference -F file="@<path>" -F response_format="json"
	fieldFile           = "file"
	fieldResponseFormat = "response_format"
	// responseFormatVerboseJSON は D-3a の確定事項。
	// response_format=json だと {"text": "..."} のみで信頼度相当の値が取れないため、
	// segments[].no_speech_prob が付く verbose_json を使う。
	responseFormatVerboseJSON = "verbose_json"
	// uploadFilename は file パートのファイル名。whisper-server はアップロードを
	// 「ファイル」として扱うため、名前の無いパートにはしない。中身は WAV。
	uploadFilename = "audio.wav"

	// defaultRequestTimeout は1回の推論に許す上限。
	// ハンドラ側の STT 全体予算(handler.sttTimeout = 30秒)から、multipart 受信・
	// ffmpeg 変換・会話存在チェックの余地(約5秒)を引いた配分。
	// LLM(30秒)+TTS(20秒)=ハンドラ60秒という既存の配分方針と同じ考え方。
	defaultRequestTimeout = 25 * time.Second
)

// inferenceResponse は response_format=verbose_json の応答のうち本アプリが使う部分。
// 未使用フィールド(task/language/duration/segments の詳細)は意図的に読まない
// (whisper-server 側のフィールド追加に追従不要にするため)。
type inferenceResponse struct {
	Text     string `json:"text"`
	Segments []struct {
		NoSpeechProb float64 `json:"no_speech_prob"`
	} `json:"segments"`
}

// Client は whisper-server 経由の音声認識クライアント。
type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

var _ interface {
	Transcribe(ctx context.Context, wav []byte) (stt.STTResult, error)
} = (*Client)(nil)

// Option は Client 生成時のオプション。
type Option func(*Client)

// WithTimeout は1リクエストあたりの上限を差し替える。
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// NewClient は whisper-server クライアントを生成する。
//
// baseURL は環境変数の読み出しを cmd/api の loadConfig に集約する既存方針に合わせ、
// 引数で受け取る(本パッケージ内で os.Getenv を読まない)。
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("whispercpp: ベースURLが空です")
	}

	c := &Client{
		// 末尾スラッシュを落としてから固定パスを連結する("//inference" を避ける)。
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{},
		timeout:    defaultRequestTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// Transcribe は WAV バイト列を whisper-server へ送り、認識結果を返す。
func (c *Client) Transcribe(ctx context.Context, wav []byte) (stt.STTResult, error) {
	// http.Client.Timeout ではなく ctx の期限として被せる。
	// こうすると期限超過が context.DeadlineExceeded として errors.Is で識別でき、
	// 呼び出し側のキャンセル(ハンドラのタイムアウト)と同じ扱いにできる。
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body, contentType, err := buildInferenceForm(wav)
	if err != nil {
		return stt.STTResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+pathInference, body)
	if err != nil {
		return stt.STTResult{}, fmt.Errorf("whispercpp: リクエストの生成に失敗しました: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// *url.Error は Unwrap を持つため、%w で包めば
		// errors.Is(err, context.Canceled/DeadlineExceeded) が成立する。
		return stt.STTResult{}, fmt.Errorf("whispercpp: 推論リクエストに失敗しました: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return stt.STTResult{}, fmt.Errorf("whispercpp: 応答の読み取りに失敗しました: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// 切り分けのためステータスは残す。応答本文は載せない(発話内容が混ざり得るため NF-SEC-01)。
		return stt.STTResult{}, fmt.Errorf("whispercpp: 予期しないステータス %d", resp.StatusCode)
	}

	var parsed inferenceResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// 応答本文はエラーに含めない(転写テキスト = 発話内容そのものであるため)。
		return stt.STTResult{}, fmt.Errorf("whispercpp: 応答JSONの解釈に失敗しました: %w", err)
	}

	return stt.STTResult{
		Text:       parsed.Text,
		Confidence: confidence(parsed),
	}, nil
}

// buildInferenceForm は /inference へ送る multipart/form-data 本体を組み立てる。
func buildInferenceForm(wav []byte) (*bytes.Buffer, string, error) {
	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)

	part, err := mw.CreateFormFile(fieldFile, uploadFilename)
	if err != nil {
		return nil, "", fmt.Errorf("whispercpp: フォームの組み立てに失敗しました: %w", err)
	}
	if _, err := part.Write(wav); err != nil {
		return nil, "", fmt.Errorf("whispercpp: 音声データの書き込みに失敗しました: %w", err)
	}
	if err := mw.WriteField(fieldResponseFormat, responseFormatVerboseJSON); err != nil {
		return nil, "", fmt.Errorf("whispercpp: フォームの組み立てに失敗しました: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, "", fmt.Errorf("whispercpp: フォームの終端に失敗しました: %w", err)
	}
	return body, mw.FormDataContentType(), nil
}

// confidence は verbose_json の応答から信頼度を算出する。
//
// D-3a(確定): confidence = 1 - max(segments[].no_speech_prob)。
// 最大値(=最も無音である確率が高い＝最も疑わしい区間)を全体の代表とすることで、
// 誤認識を見逃さない側へ倒す。segments が空(無音のみ等)なら 0 = 認識失敗扱い。
func confidence(resp inferenceResponse) float64 {
	if len(resp.Segments) == 0 {
		return 0
	}
	maxNoSpeechProb := resp.Segments[0].NoSpeechProb
	for _, seg := range resp.Segments[1:] {
		if seg.NoSpeechProb > maxNoSpeechProb {
			maxNoSpeechProb = seg.NoSpeechProb
		}
	}
	return 1 - maxNoSpeechProb
}
