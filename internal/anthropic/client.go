// Package anthropic は Claude API を llm.LLMClient として提供する。
//
// internal/llm の I/F・ParseLLMResponse・センチネルエラーには依存させず（差し替え
// 可能性の担保）、本パッケージは「プロンプトを渡して応答テキスト(JSON文字列)を得る」
// ことだけに責務を絞る。パースは llm.ResponseParser の責務。
package anthropic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"zuncha/internal/llm"
)

// 確定仕様（docs/04_implementation/04_realtime_wiring_design.md 決-1a）。
const (
	// model は日付サフィックスを付けない素の文字列で渡す。
	model = "claude-opus-5"
	// maxTokens は thinking と応答テキストの「合計」上限。絞ると応答が途中で切れる。
	maxTokens = 16000
	// maxRetries は既定2だと「タイムアウト×3」の実時間を消費し、ハンドラ側60秒の
	// 予算をLLMだけで食い潰すため1に絞る。
	maxRetries = 1
	// requestTimeout はハンドラ側60秒のうちTTS・DB保存の余地を残す配分。
	requestTimeout = 30 * time.Second
)

// ErrRefused は Claude の安全性分類器が応答を拒否したことを表すセンチネルエラー。
// HTTP 200 + stop_reason: "refusal" で返るため、APIエラーとしては検出できない。
var ErrRefused = errors.New("anthropic: モデルが応答を拒否しました")

// textBlockType は Content 内のテキストブロックを示す type 値。
// thinking ブロックは JSON に混ざると ErrSyntax の原因になるため連結対象に含めない。
const textBlockType = "text"

// Client は Claude API 経由の llm.LLMClient 実装。
type Client struct {
	api sdk.Client
}

var _ llm.LLMClient = (*Client)(nil)

// Option は Client 生成時のオプション。
type Option func(*clientOptions)

type clientOptions struct {
	requestOptions []option.RequestOption
}

// WithBaseURL は API のベースURLを差し替える。
// テストで httptest のフェイクサーバへ向けるために用いる。
func WithBaseURL(baseURL string) Option {
	return func(o *clientOptions) {
		o.requestOptions = append(o.requestOptions, option.WithBaseURL(baseURL))
	}
}

// NewClient は Claude API クライアントを生成する。
//
// apiKey は引数で受け取り、本パッケージ内で os.Getenv を読まない。環境変数の読み出しは
// cmd/api の loadConfig に集約する既存方針に揃え、テストからも注入可能にするため。
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("anthropic: APIキーが空です")
	}

	var co clientOptions
	for _, opt := range opts {
		opt(&co)
	}

	requestOptions := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithMaxRetries(maxRetries),
		option.WithRequestTimeout(requestTimeout),
	}
	// 呼び出し側のオプションを後ろに置き、ベースURLの差し替えを効かせる。
	requestOptions = append(requestOptions, co.requestOptions...)

	return &Client{api: sdk.NewClient(requestOptions...)}, nil
}

// GenerateResponse はプロンプトを Claude へ送り、応答テキスト(JSON文字列)を返す。
//
// パースは行わない（llm.ResponseParser の責務）。エラー時もプロンプト本文・APIキーを
// メッセージに含めない（NF-SEC-01）。
func (c *Client) GenerateResponse(ctx context.Context, prompt string) ([]byte, error) {
	msg, err := c.api.Messages.New(ctx, sdk.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		// システムプロンプトは毎回不変＝キャッシュ対象。会話履歴は Messages 側（揮発）に置く。
		System: []sdk.TextBlockParam{{
			Text:         SystemPrompt,
			CacheControl: sdk.NewCacheControlEphemeralParam(),
		}},
		// Thinking は未設定のまま（既定の adaptive）。Opus 5 で無効化すると <thinking>
		// タグが可視応答へ漏れ、そのままVOICEVOXが読み上げてしまう。
		// temperature/top_p/top_k は Opus 5 では 400 になるため一切設定しない。
		OutputConfig: sdk.OutputConfigParam{
			Effort: sdk.OutputConfigEffortLow,
			Format: sdk.JSONOutputFormatParam{Schema: responseSchema},
		},
		Messages: []sdk.MessageParam{
			sdk.NewUserMessage(sdk.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		// %w で包み、呼び出し側が errors.As(*anthropic.Error) / errors.Is(context.Canceled)
		// で分岐できるようにする。プロンプト本文は含めない。
		return nil, fmt.Errorf("anthropic: メッセージ生成に失敗しました: %w", err)
	}

	// 拒否時は content が空または部分的。Content を読む前に必ず検査する。
	if msg.StopReason == sdk.StopReasonRefusal {
		return nil, ErrRefused
	}

	var b strings.Builder
	for _, block := range msg.Content {
		if block.Type == textBlockType {
			b.WriteString(block.Text)
		}
	}
	if b.Len() == 0 {
		return nil, errors.New("anthropic: 応答にテキストブロックが含まれていません")
	}
	return []byte(b.String()), nil
}
