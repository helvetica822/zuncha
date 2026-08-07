# 実装指示書 — Wave C-1: Claude API 実接続 (W-08)

- 指示者: 四国めたん (テックリード) / 2026-08-05
- 実装担当: ずんだもん
- **実装前提: `docs/04_implementation/04_realtime_wiring_design.md` 決-1 / 決-1a を必ず読むこと**(公式リファレンスで確認済の確定事項。記憶で書くと間違える箇所を表にしてあります)
- 依存: なし(W-06/W-07 と独立。ただし実際に応答が流れるのは W-07 完成後)

---

## 1. スコープ

`internal/anthropic` に `llm.LLMClient` の実装を1つ作る。**`internal/llm` の既存コード(I/F・`ParseLLMResponse`・センチネルエラー)は一切変更しない。**

```
internal/anthropic/client.go   Client（llm.LLMClient を実装）
internal/anthropic/prompt.go   システムプロンプト（ずんだもんペルソナ + JSON形式の強制）
internal/llm/parser_adapter.go ResponseParser を満たすアダプタ（下記 §4）
```

`go.mod` に `github.com/anthropics/anthropic-sdk-go` を追加します(現在未依存)。

---

## 2. 決定事項の再掲（設計書 決-1a より・変えないこと）

| 項目 | 確定内容 |
|---|---|
| モデルID | **`claude-opus-5`** を素の文字列で(`Model: "claude-opus-5"`)。日付サフィックスを付けない。型付き定数の存在を前提にしない |
| thinking | **無効化しない**。`Thinking` を未設定のままにし、`output_config.effort` を **`low`** にする |
| 温度パラメータ | `temperature`/`top_p`/`top_k` は**使わない**(400になる) |
| `MaxTokens` | 16000 程度。小さく絞らない(thinking と応答テキストの**合計**上限) |
| リトライ | `option.WithMaxRetries(1)` |
| タイムアウト | `option.WithRequestTimeout(30 * time.Second)` |
| システムプロンプト | `System: []anthropic.TextBlockParam{{Text: ..., CacheControl: anthropic.NewCacheControlEphemeralParam()}}` |
| エラー処理 | `errors.As` で `*anthropic.Error` を取り出し `StatusCode` で分岐 |
| 拒否応答 | **`StopReason` を `Content` の読み出し前に検査** |

**特に thinking を無効化しないことの理由**: Opus 5 では無効化すると `<thinking>` タグが可視応答へ漏れる既知の失敗モードがあります。本アプリは応答テキストを**そのままVOICEVOXが読み上げて画面に出す**ので、漏洩がそのままユーザー体験の破壊になりますの。

---

## 3. システムプロンプト（`internal/anthropic/prompt.go`）

**文言はこちらで確定します。** ペルソナと出力形式を1つの定数に持たせてください(プロンプトキャッシュのため**毎回同一の文字列**であることが必須。日付や乱数を混ぜないこと)。

```
あなたは「ずんだもん」というキャラクターとして会話します。

## 話し方
- 一人称は「ボク」。語尾は「〜のだ」「〜なのだ」を基本にする。
- 明るく人懐っこく、短めに話す。1〜3文程度。長い説明は避ける。
- 相手を責めない。分からないことは正直に「分からないのだ」と言う。

## 出力形式（厳守）
次の形式のJSONだけを出力する。前後に説明・コードブロック・改行以外の装飾を付けない。

{"text": "<ずんだもんの発話>", "emotion": "<感情ラベル>"}

- text: 実際に音声で読み上げられる文章。記号の羅列や絵文字の連打は避ける。
- emotion: 次の7つから最も近いものを1つだけ選ぶ。
  喜び   … うれしい・ポジティブな反応
  怒り   … 怒り・強い否定
  悲しみ … 悲しい・落ち込んだ反応
  楽しい … 楽しそう・弾んだ反応
  照れ   … 恥ずかしい・はにかみ
  困惑   … 戸惑い・困り顔
  ドヤ顔 … 自信満々・誇らしい反応
- 迷ったら "困惑" を選ぶ。7つ以外の語は使わない。
```

**根拠**: 語尾と一人称は既存実装の発話文言(`ConversationView.svelte` の `STT_FAILURE_MESSAGE` 「うまく聞き取れなかったのだ。もう一度話しかけてほしいのだ」)と `01_screen_design.md` 536/594行のテキスト例に揃えました。感情7種とその意味は `02_database_design.md` 78-88行(DDLのCHECK制約と同一)から取っています。

- **`text` が音声で読み上げられる**ことをプロンプトに書いているのは重要です。記号の羅列や絵文字連打はVOICEVOXが不自然に読むため。
- 「迷ったら困惑」は既存 `ParseLLMResponse` の「7種外/空なら困惑フォールバック」と方向を揃えたもの。**プロンプトとパーサの二重の防壁**になります。

---

## 4. `ResponseParser` アダプタ（`internal/llm/parser_adapter.go`）

`llm.ParseLLMResponse` は**関数**なので、`ResponseParser` I/F を満たす型が必要です。

```go
// DefaultParser は ParseLLMResponse を ResponseParser として使えるようにする。
type DefaultParser struct{}

func NewDefaultParser() *DefaultParser
func (p *DefaultParser) Parse(body []byte) (*LLMResponse, error)  // ParseLLMResponse をそのまま呼ぶ

var _ ResponseParser = (*DefaultParser)(nil)
```

- **`internal/llm` に置く理由**: I/F も `ParseLLMResponse` も同パッケージにあり、アダプタだけ別パッケージへ出す理由がありません。
- **既存の `parser.go` は変更しないこと**。新規ファイルで足してください。
- 既存の 3-1 テスト25件は `ParseLLMResponse` を直接呼んでいるので影響しません。アダプタ自身のテストは「委譲していること」の確認で足ります(正常1件 + エラー透過1件)。

---

## 5. `Client`（`internal/anthropic/client.go`）

```go
func NewClient(apiKey string, opts ...Option) (*Client, error)
func (c *Client) GenerateResponse(ctx context.Context, prompt string) ([]byte, error)

var _ llm.LLMClient = (*Client)(nil)
```

### 5.1 確定した設計判断

| 項目 | 決定 | 理由 |
|---|---|---|
| 戻り値 | **応答テキスト(JSON文字列)を `[]byte` で返す**。パースはしない | `LLMClient` の既存契約が `([]byte, error)`。パースは `ResponseParser` の責務で、責務を混ぜない |
| `apiKey` の受け取り | **引数で受ける**。パッケージ内で `os.Getenv` を読まない | 環境変数の読み出しは `cmd/api` の `loadConfig` に集約する既存方針と揃える。テストからも注入できる |
| ベースURL差し替え | `Option` で `option.WithBaseURL` を渡せるようにする | `httptest` のフェイクサーバへ向けるため。**これが無いとテストが書けません** |
| 空の応答 | `Content` に `TextBlock` が1つも無ければエラー | 呼び出し側が空文字列をパースして `ErrSyntax` になるより、原因が明確 |
| 複数 `TextBlock` | **連結する** | 通常1つだが、分割されて届いても壊れないようにする |
| `ThinkingBlock` | **無視する**(連結対象に含めない) | thinking は有効なので届き得る。JSONに混ざると `ErrSyntax` になる |
| ログ | **APIキーと発話内容を出さない**。エラー時もプロンプト本文をログに含めない | NF-SEC-01 / ガイドライン10.2 |
| 拒否 | `StopReason` が refusal なら**センチネルエラー** `ErrRefused` を返す(`internal/anthropic` に定義) | `ResponseStreamer.fail` 経由で `SendError` に落ちる。`Content[0]` を無条件に読むと落ちるので**必ず先に検査** |

### 5.2 テスト（`tests/unit/anthropic_client_test.go`）

**`httptest.NewServer` でフェイクAPIを立て、`option.WithBaseURL` で向けます。実APIは絶対に呼ばないこと。**

**正常系**:
- [ ] フェイクが `{"text":"こんにちはなのだ","emotion":"喜び"}` を1つの `TextBlock` で返す → `GenerateResponse` がそのJSON文字列をバイト列で返す。
- [ ] **リクエストボディを検証**: `model` が `claude-opus-5`、`max_tokens` が期待値、`system` にペルソナ文字列が入っている、`temperature`/`top_p`/`top_k` の**キーが存在しない**(これが混ざると本番で400になるため重要)。
- [ ] `TextBlock` が2つに分割されて届く → 連結される。
- [ ] `ThinkingBlock` + `TextBlock` が届く → **thinking は混ざらず TextBlock のみ**が返る。

**異常系**:
- [ ] フェイクが 401 を返す → エラー。`errors.As` で `*anthropic.Error` が取れ `StatusCode == 401`。
- [ ] フェイクが 429 を返す → エラー(リトライ後)。**`option.WithMaxRetries(1)` によりリクエスト回数が2回**であることをフェイク側のカウンタで検証(既定の2だと3回になるので、この検証が設定の実効性を担保します)。
- [ ] フェイクが `stop_reason: "refusal"` + 空 `content` を返す → **`ErrRefused`** が返り、パニックしない。
- [ ] フェイクが `TextBlock` を含まない応答を返す → エラー。
- [ ] `ctx` を即キャンセル → `context.Canceled` 系のエラーが返る(タイムアウト設定が ctx を尊重することの確認)。

**ログの検証**:
- [ ] エラー経路で**APIキーとプロンプト本文がログに出ない**こと。`log.SetOutput` でバッファに取り、APIキー文字列とプロンプト断片が含まれないことを検証してください。

---

## 6. JSON強制の方式（**実測して決めること**）

公式には構造化出力(`output_config.format` の `json_schema`)が Opus 5 で使え、プロンプト指示より確実です。ただし**Go SDK での正確なバインディング名が公式ドキュメントに記載されていません**。

- **推測でコードを書かないこと。** SDK追加後、`OutputConfig` 相当のフィールドを**コンパイルエラーを手掛かりに確定**してください(公式が静的型付け言語で推奨している手順です)。
- 使えた場合: スキーマで `{text, emotion}` を強制。`ParseLLMResponse` はそのまま二重の防壁として残す。
- **ピン留めしたSDKバージョンで使えない場合**: §3のシステムプロンプトによるJSON指示のみで進める。`ParseLLMResponse`(25ケースでテスト済)が逸脱を吸収するので機能要件は満たせます。
- **どちらを採ったか、およびその判断根拠(コンパイルエラーの内容など)を報告に必ず書いてください。**

`effort: "low"` の指定方法も同様に、フィールド名が不明なら実測で確定してください。**`effort` が指定できなかった場合は既定(high)のままにし、そのことを報告してください** — 既定でも動作はしますが、音声会話としては待ち時間が伸びるため私が別途判断します。

---

## 7. `cmd/api` への配線

- `loadConfig` に `ANTHROPIC_API_KEY` を追加。**未設定なら起動時に `log.Fatal`**(DB URL と同じ流儀。実行時に初めて落ちるより起動時に落とす)。
- ただし **W-07 の `ChatService` が未完成なら DI 配線は行わず、`NewClient` が生成できることの確認までで止めてください**。配線は W-07 完了時にまとめて行います。

---

## 8. 完了条件

- [ ] `go test ./tests/... -count=1` 全緑、`./scripts/test_race.sh` もクリーン(**`export ZUNCHA_TEST_DB_OWNER=zundamon` を忘れずに**)。
- [ ] `gofmt -l .` 空 / `go vet ./...` EXIT 0 / `go build ./...` 成功。
- [ ] **`internal/llm/parser.go`・`internal/llm/client.go`・`internal/llm/response.go`・`internal/llm/errors.go` が未変更**であること。
- [ ] **テストが実APIを呼んでいないこと**(`httptest` のみ)。`ANTHROPIC_API_KEY` を未設定にしてもテストが緑であることを実行して示してください。
- [ ] RED を目視確認したことを報告に含める。
- [ ] **ミューテーション実測**: `option.WithMaxRetries(1)` を外す(既定2に戻す)と 429 のリクエスト回数テストが赤になることを実測。

## 9. 報告について

- 緑でも `completed` にせず `in_progress` のまま報告。
- **§2 の確定事項・§3 のプロンプト文言・§5.1 の設計判断**を変えたくなったら、実装で埋めずに私に相談すること。特にプロンプト文言は画面設計とDB設計から根拠を引いているので、変更は仕様側との突合が必要ですわ。
