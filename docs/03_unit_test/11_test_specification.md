# 単体テスト仕様書 — フェーズ3（外部連携・イベント処理、モック多用）

| 項目 | 内容 |
|------|------|
| バージョン | 1.0 |
| 作成日 | 2026-07-08 |
| 作成者 | WhiteCUL（テスト仕様書・テストコード作成担当） |
| 入力 | `01_test_plan.md`（テスト計画）、`09_test_perspectives_phase3.md`（テスト観点、めたん作成）、`10_test_cases_phase3.md`（テストケース、ひまり作成） |
| 対象 | フェーズ3（3機能、パース層／オーケストレーション層／判定層分割を含む、計52件） |
| 次工程 | そらによるQAレビュー |

---

## 目次

1. [目的・対象](#1-目的対象)
2. [設計方針（パース層・判定層とオーケストレーション層の使い分け）](#2-設計方針パース層判定層とオーケストレーション層の使い分け)
3. [パッケージ構成・インターフェース／関数シグネチャ一覧](#3-パッケージ構成インターフェース関数シグネチャ一覧)
4. [テストケース一覧](#4-テストケース一覧)
5. [テストケース総数の確認結果](#5-テストケース総数の確認結果)
6. [テストコード配置・実行方法](#6-テストコード配置実行方法)
7. [未決事項の反映状況（Q・R・S）](#7-未決事項の反映状況qrs)

---

## 1. 目的・対象

本書は `10_test_cases_phase3.md` で設計されたフェーズ3のテストケースを、実装着手前の正式な単体テスト仕様書として清書したものである。
対象は `01_test_plan.md` フェーズ3に定義された3機能であり、STT/LLM/TTS/SSE配信という外部依存を多く含むため、`09_test_perspectives_phase3.md` の指摘に基づき「パース層・判定層（モック不要の純粋関数）」と「オーケストレーション層（モック必須）」に分離して構成する。

| 観点番号 | 機能 | 対応機能ID / 設計根拠 |
|---------|------|----------------------|
| 3-1 | LLM応答JSONパース（text/emotion同時抽出） | F-AI-02 |
| 3-2 | SSEイベント送出ロジック（emotion/text/audio_url/done/error） | F-RT-01, F-EMO-03 / 画面設計書7章 |
| 3-3 | STT失敗判定・8秒無音タイムアウト判定ロジック | F-STT-02, F-STT-03 / 画面設計書9章 |

---

## 2. 設計方針（パース層・判定層とオーケストレーション層の使い分け）

`09_test_perspectives_phase3.md` 前提・総括の指摘に基づき、以下の2階層構成を採用する。

### パース層・判定層（純粋関数、モック不要）

- 「外部から受け取った生データの解釈」のみを担当する層。DB・HTTP・ファイルI/Oを一切含まない。
- フェーズ1・2で確立した資産（`ValidateEmotion`、`IsValidInput`、Clock注入パターン）を再利用し、独自の再実装は行わない。
- 対象: `ParseLLMResponse`（3-1）、`IsRecognitionFailed` / `IsTimedOut`（3-3）

### オーケストレーション層（モック必須）

- 「解釈結果を使って次に何を呼ぶか」を担当する層。LLM/TTS呼び出し、SSEイベント送出はすべてインターフェース越しに行う。
- `LLMClient` / `ResponseParser` / `TTSClient` / `TextChunker` / `EventSink` をそれぞれ`testify/mock`でモック化し、実際のHTTP通信・実SSEストリーム（`http.ResponseWriter`）には一切依存しない。
- 対象: `ResponseStreamer.StreamResponse`（3-2）

| 層 | モック方針 | 対応するテストファイル |
|----|-----------|----------------------|
| パース層 | モック不要 | `tests/unit/parse_llm_response_test.go` |
| 判定層 | モック不要 | `tests/unit/stt_judgment_test.go` |
| オーケストレーション層 | 全依存をtestify/mockでモック化 | `tests/unit/response_streamer_test.go` |

> フェーズ3は`01_test_plan.md`5章の方針通りSTT/LLM/TTS/SSE配信をすべてモック化する対象であるため、フェーズ2のようなI/O層（実DB接続等）のテストディレクトリ（`tests/integration/`）は使用しない。

---

## 3. パッケージ構成・インターフェース／関数シグネチャ一覧

| パッケージ | 役割 |
|-----------|------|
| `internal/llm` | LLM応答パース（`ParseLLMResponse`）、`LLMClient`／`ResponseParser`インターフェース |
| `internal/tts` | `TTSClient`インターフェース |
| `internal/sse` | `EventSink`／`TextChunker`インターフェース |
| `internal/stt` | STT失敗判定・タイムアウト判定（`IsRecognitionFailed`／`IsTimedOut`） |
| `internal/service` | オーケストレーション層（`ResponseStreamer`） |

### 3.1 `internal/llm`

```go
type LLMResponse struct {
    Text    string
    Emotion string
}

// ParseLLMResponse はLLM呼び出し（HTTPクライアント）から完全に分離された純粋関数。
// emotion値が7種以外の場合はエラーにせず「困惑」にフォールバックする（7章L決定）。
// Markdownコードブロック除去は責務に含めない（7章Q決定）。
func ParseLLMResponse(body []byte) (*LLMResponse, error)

var ErrSyntax error // 構文エラー
var ErrSchema error // スキーマ検証エラー（必須フィールド欠落）
var ErrValue  error // 値検証エラー（型不一致等。emotion値の7種外はErrValue対象外＝フォールバックするため）

type LLMClient interface {
    GenerateResponse(ctx context.Context, prompt string) ([]byte, error)
}

type ResponseParser interface {
    Parse(body []byte) (*LLMResponse, error)
}
```

> `ErrSyntax` / `ErrSchema` / `ErrValue` は`errors.Is`で判定できるセンチネルエラーとし、`ParseLLMResponse`は`fmt.Errorf("...: %w", ErrSyntax)`のようにラップして返す。3種類のエラーを呼び出し元が区別できることが3-1の存在意義（`09_test_perspectives_phase3.md` 3-1例外系）。

### 3.2 `internal/tts`

```go
type TTSClient interface {
    // Synthesize は音声合成を行い、フロントが取得する一時URL（例: /audio/{ulid}）を返す。
    Synthesize(ctx context.Context, text string) (audioURL string, err error)
}
```

### 3.3 `internal/sse`

```go
type EventSink interface {
    SendEmotion(label string) error
    SendTextChunk(chunk string) error
    SendAudioURL(url string) error
    SendDone() error
    SendError(message string) error
}

type TextChunker interface {
    Chunk(text string) []string
}
```

> `EventSink`の各メソッドは、画面設計書7章のイベントペイロード仕様（`emotion`は`{"label": ...}`、`text`は`{"chunk": ...}`、`audio_url`は`{"url": ...}`）に対応する契約とする。実際のJSONシリアライズ・SSEワイヤーフォーマットへの変換は、この抽象の先にある具象実装（GREEN phaseで作成）の責務であり、オーケストレーション層のテストでは「正しい引数で正しいメソッドが呼ばれたか」を検証する。

### 3.4 `internal/stt`

```go
type STTResult struct {
    Text       string
    Confidence float64
}

const SttConfidenceThreshold = 0.5 // 暫定値（7章P決定）。将来調整前提のため名前付き定数化する

// IsRecognitionFailed はSTT結果が失敗扱いかどうかを判定する純粋関数。
// 空文字列・trim後空白のみ・信頼度が閾値未満（confidence < 0.5）の場合にtrueを返す。
// trim判定は1-4のIsValidInputを再利用する（7章R決定）。
func IsRecognitionFailed(result STTResult) bool

// IsTimedOut は無音継続時間が閾値以上かどうかを判定する純粋関数（elapsed >= threshold、7章O決定）。
// 1-5のIsExpiredと同じClock注入パターン（silenceStart, nowを引数で受け取る）を踏襲する。
func IsTimedOut(silenceStart, now time.Time, threshold time.Duration) bool
```

### 3.5 `internal/service`（オーケストレーション層）

```go
type ResponseStreamer struct { /* unexported */ }

func NewResponseStreamer(
    llmClient llm.LLMClient,
    parser llm.ResponseParser,
    ttsClient tts.TTSClient,
    chunker sse.TextChunker,
) *ResponseStreamer

func (s *ResponseStreamer) StreamResponse(ctx context.Context, sink sse.EventSink, prompt string) error
```

**内部の処理順序**（7章M・N・S決定を反映）:

```
1. body, err := llmClient.GenerateResponse(ctx, prompt)
   err != nil → sink.SendError(...); return err                         （確認事項K：LLM呼び出し失敗）
2. resp, err := parser.Parse(body)
   err != nil → sink.SendError(...); return err                         （確認事項K：パース失敗もerrorイベント）
3. err := ValidateEmotion(&resp.Emotion)  // 1-3の資産を再利用した防御的チェック
   err != nil → sink.SendError(...); return err                         （確認事項S：本来到達しない想定の防御）
4. sink.SendEmotion(resp.Emotion)
5. for _, chunk := range chunker.Chunk(resp.Text) { sink.SendTextChunk(chunk) }
6. audioURL, err := ttsClient.Synthesize(ctx, resp.Text)
   err == nil → sink.SendAudioURL(audioURL)
   err != nil → audio_urlをスキップ（確認事項N：doneのみ送出、errorにしない）
7. sink.SendDone()
```

> **EventSink書き込み失敗時の扱い**（そらのQAレビュー指摘反映）: 上記1〜7のいずれかのステップで`sink.Send*`（`SendEmotion`／`SendTextChunk`／`SendAudioURL`）がエラーを返した場合、以降のステップの実行を中断し、その時点で`sink.SendError(...)`に切り替えて呼び出し元にエラーを返す（TC-3-2-12）。これはLLM呼び出し失敗時（確認事項K）と同じ「エラー検知時は即`error`イベントへ切り替える」方針をシンク書き込み失敗にも適用したものであり、送出中の任意のタイミングで`error`が割り込める（確認事項M）という契約の一部として扱う。

`ResponseParser`をLLMClientと分離して注入可能にしているのは、TC-3-2-08（3-1のフォールバックにより本来到達しないはずの不正なemotion値を防御的チェックが検出するケース）を、実際の`ParseLLMResponse`を経由せずモックで直接再現できるようにするための設計判断である。

---

## 4. テストケース一覧

`10_test_cases_phase3.md` のGiven/When/Thenをそのまま踏襲し、対応するGoテストコード（ファイル・サブテスト名）を突き合わせる。

### 4.1 3-1. `ParseLLMResponse`（25件）— `tests/unit/parse_llm_response_test.go`

| ID | Given | Then | 対応サブテスト（`TestParseLLMResponse` 内、特記なき限り） |
|----|-------|------|------------------------------------------------------|
| TC-3-1-01 | 仕様通りのJSON | Text/Emotionを正しく抽出 | `TC-3-1-01_仕様通りのJSONを正しくパースする` |
| TC-3-1-02〜08 | emotion 7種それぞれ | 各値で成功 | `TC-3-1-0N_emotionが<ラベル>で成功する`（7件） |
| TC-3-1-09 | 想定外フィールド付き | 無視して抽出成功 | `TC-3-1-09_想定外フィールドは無視される` |
| TC-3-1-10 | textが空文字列 | パース自体は成功 | `TC-3-1-10_textが空文字列でもパースは成功する` |
| TC-3-1-11 | 構文不正なJSON | 構文エラー | `TC-3-1-11_構文不正はErrSyntaxを返す` |
| TC-3-1-12 | textキー欠落 | スキーマ検証エラー | `TC-3-1-12_textキー欠落はErrSchemaを返す` |
| TC-3-1-13 | emotionキー欠落 | スキーマ検証エラー | `TC-3-1-13_emotionキー欠落はErrSchemaを返す` |
| TC-3-1-14 | emotionが7種にない値（確認事項L） | 「困惑」にフォールバックし成功 | `TC-3-1-14_emotionが7種外なら困惑にフォールバックする` |
| TC-3-1-15 | emotionが数値型 | 値検証エラー | `TC-3-1-15_emotionが数値型はErrValueを返す` |
| TC-3-1-16 | textがnull | 値検証エラー | `TC-3-1-16_textがnullはErrValueを返す` |
| TC-3-1-17 | 空文字列ボディ | 構文エラー | `TC-3-1-17_空文字列ボディは構文エラーを返す` |
| TC-3-1-18 | `"null"`文字列 | エラー（種別問わず） | `TC-3-1-18_null文字列はエラーを返す` |
| TC-3-1-19 | 空オブジェクト`{}` | スキーマ検証エラー | `TC-3-1-19_空オブジェクトはErrSchemaを返す` |
| TC-3-1-20 | Markdownコードブロック囲み（確認事項Q） | 構文エラー（除去は責務外） | `TC-3-1-20_Markdownコードブロックは構文エラーを返す` |
| TC-3-1-21 | textが10,000文字 | パース完了 | `TC-3-1-21_長大なtextでもパースが完了する` |
| TC-3-1-22 | emotionが空文字列 | 「困惑」にフォールバック | `TC-3-1-22_emotionが空文字列でも困惑にフォールバックする` |
| TC-3-1-23 | マルチバイト文字 | 文字化けせずデコード | `TC-3-1-23_マルチバイト文字が正しくデコードされる` |
| TC-3-1-24 | 構文エラー・スキーマエラーそれぞれの入力 | `errors.Is`で種別を区別できる | `TestParseLLMResponse_構文エラーとスキーマ検証エラーを区別できる`（別関数） |
| TC-3-1-25 | スキーマエラー・値エラーそれぞれの入力 | `errors.Is`で種別を区別できる | `TestParseLLMResponse_スキーマ検証エラーと値検証エラーを区別できる`（別関数） |

### 4.2 3-2. SSEイベント送出ロジック（15件）— `tests/unit/response_streamer_test.go`

| ID | Given | Then | 対応サブテスト |
|----|-------|------|----------------|
| TC-3-2-01 | LLM・TTS両モック成功 | emotion→text→audio_url→doneの順（確認事項M） | `TC-3-2-01_正常フローはemotion_text_audio_url_doneの順で送出される` |
| TC-3-2-02 | textが3チャンク | 3チャンクすべてdoneより前 | `TC-3-2-02_3チャンクすべてdoneより前に送出される` |
| TC-3-2-03 | 正常応答フロー | 各イベントのペイロード（引数）が仕様通り | `TC-3-2-03_各イベントの引数が仕様通りである` |
| TC-3-2-04 | textが1チャンクのみ | text→doneの順 | `TC-3-2-04_1チャンクのみでもtext_doneの順で送出される` |
| TC-3-2-05 | textが100チャンク | 全チャンクが順序通り | `TC-3-2-05_100チャンクでも順序が入れ替わらない` |
| TC-3-2-06 | LLMClientがエラー（確認事項K） | text/emotionなしでerrorのみ | `TC-3-2-06_LLM呼び出し失敗時はerrorのみ送出される` |
| TC-3-2-07 | LLM成功・TTSがエラー（確認事項N） | audio_urlなしでdone送出、errorにしない | `TC-3-2-07_TTS失敗時はaudio_urlなしでdoneが送出される` |
| TC-3-2-08 | パース結果のemotionが不正（確認事項S） | エラー扱い、emotion未送出 | `TC-3-2-08_防御的チェックでemotion不正はエラーとして扱われる` |
| TC-3-2-09 | 正常応答フロー | emotionがtextより必ず先 | `TC-3-2-09_emotionはtextより必ず先に送出される` |
| TC-3-2-10 | text複数チャンクの正常フロー | 全textチャンク後でなければaudio_url未送出 | `TC-3-2-10_全textチャンク送出後でなければaudio_urlは送出されない` |
| TC-3-2-11 | 正常フロー・TTS失敗フロー | audio_url（または省略）の後でなければdoneなし | `TestResponseStreamer_doneはaudio_url相当の後でのみ送出される`（別関数） |
| TC-3-2-12 | text送出中にEventSinkのSendTextChunkが書き込み失敗 | errorが任意タイミングで割り込み可能 | `TC-3-2-12_errorは送出中でも任意のタイミングで割り込める` |
| TC-3-2-13 | errorが送出される条件 | error後は後続イベントなし | `TC-3-2-13_error送出後は後続イベントを送出しない` |
| TC-3-2-14 | 正常応答フロー | doneはちょうど1回 | `TC-3-2-14_doneはちょうど1回だけ送出される` |
| TC-3-2-15 | 正常・TTS失敗・LLM失敗の3フロー | いずれもdoneまたはerrorで終端 | `TestResponseStreamer_全フローがdoneまたはerrorで終端する`（別関数） |

### 4.3 3-3. `IsRecognitionFailed` / `IsTimedOut`（12件）— `tests/unit/stt_judgment_test.go`

#### `IsRecognitionFailed`（7件）

| ID | Given | Then | 対応サブテスト（`TestIsRecognitionFailed` 内） |
|----|-------|------|----------------------------------------------|
| TC-3-3-R01 | 正常テキスト・信頼度0.8 | `false` | `TC-3-3-R01_閾値以上の信頼度でfalseを返す` |
| TC-3-3-R02 | 空文字列 | `true` | `TC-3-3-R02_空文字列はtrueを返す` |
| TC-3-3-R03 | 正常テキスト・信頼度0.3 | `true` | `TC-3-3-R03_閾値未満の信頼度はtrueを返す` |
| TC-3-3-R04 | 空白のみ・信頼度0.8（確認事項R） | `true` | `TC-3-3-R04_空白のみのテキストはtrueを返す` |
| TC-3-3-R05 | 信頼度ちょうど0.5（確認事項P） | `false` | `TC-3-3-R05_信頼度がちょうど閾値でfalseを返す` |
| TC-3-3-R06 | 信頼度0.49 | `true` | `TC-3-3-R06_閾値のわずかに下でtrueを返す` |
| TC-3-3-R07 | 信頼度0.51 | `false` | `TC-3-3-R07_閾値のわずかに上でfalseを返す` |

#### `IsTimedOut`（5件）

| ID | Given | Then | 対応サブテスト（`TestIsTimedOut` 内） |
|----|-------|------|--------------------------------------|
| TC-3-3-T01 | 5秒経過 | `false` | `TC-3-3-T01_5秒経過はfalseを返す` |
| TC-3-3-T02 | 9秒経過 | `true` | `TC-3-3-T02_9秒経過はtrueを返す` |
| TC-3-3-T03 | ちょうど8秒経過（確認事項O） | `true` | `TC-3-3-T03_ちょうど8秒でtrueを返す` |
| TC-3-3-T04 | 7.9秒経過 | `false` | `TC-3-3-T04_7点9秒でfalseを返す` |
| TC-3-3-T05 | 8.1秒経過 | `true` | `TC-3-3-T05_8点1秒でtrueを返す` |

---

## 5. テストケース総数の確認結果

`10_test_cases_phase3.md` のサマリー「計52件（3-1: 25件、3-2: 15件、3-3: 12件）」について、全テストケースIDを機械的に突合した結果、記載通り**52件**（重複なし）であることを確認した。

| 観点 | 件数 |
|------|------|
| 3-1 ParseLLMResponse | 25件 |
| 3-2 SSEイベント送出ロジック | 15件 |
| 3-3 IsRecognitionFailed | 7件 |
| 3-3 IsTimedOut | 5件 |
| **合計** | **52件** |

---

## 6. テストコード配置・実行方法

### 配置

```
zuncha/
├── go.mod
├── internal/
│   ├── llm/          # 未実装（GREEN phaseで作成）
│   ├── tts/          # 未実装（GREEN phaseで作成）
│   ├── sse/          # 未実装（GREEN phaseで作成）
│   ├── stt/          # 未実装（GREEN phaseで作成）
│   └── service/      # 未実装（GREEN phaseで作成、ResponseStreamerを追加）
└── tests/
    └── unit/
        ├── (フェーズ1・2の既存テストファイル)
        ├── parse_llm_response_test.go   # 3-1（25件、モック不要）
        ├── response_streamer_test.go    # 3-2（15件、testify/mock使用）
        └── stt_judgment_test.go         # 3-3（12件、モック不要）
```

### 実行方法（GREEN phase以降）

```bash
go mod tidy
go test ./tests/unit/... -v
```

フェーズ3はすべて`tests/unit/`配下に配置する（`01_test_plan.md`5章の方針通り、STT/LLM/TTS/SSEはすべてモック化対象のため、フェーズ2のようなI/O層テスト・実DB接続は不要）。

### 現状（RED phase）

`internal/llm` / `internal/tts` / `internal/sse` / `internal/stt` / `internal/service`（`ResponseStreamer`）のいずれも未実装のため、上記コマンドは import エラーによりコンパイルが通らない。これはTDDのRED状態として意図した挙動である。

---

## 7. 未決事項の反映状況（Q・R・S）

`10_test_cases_phase3.md` 7章の未決事項について、つむぎの判断（本チャットでの指示：「いずれも本書の暫定対応のまま確定、変更なし」）を反映済み。

| # | 対象 | つむぎの判断 | 本書・テストコードへの反映 |
|---|------|-------------|--------------------------|
| Q | 3-1 Markdownコードブロック除去の責務 | この関数の責務に含めない（TC-3-1-20のまま変更なし） | `ParseLLMResponse`はコードブロック除去を行わず、Markdown囲みは構文エラーとして扱う設計を維持 |
| R | 3-3 STT結果が空白のみの場合の判定 | 1-4の`IsValidInput`を再利用する（TC-3-3-R04のまま変更なし） | `IsRecognitionFailed`のtrim判定はフェーズ1の`IsValidInput`を再利用する設計方針として明記 |
| S | 3-2 防御的チェックでemotionが不正だった場合の扱い | エラーとして扱う（TC-3-2-08のまま変更なし） | `ResponseStreamer.StreamResponse`内部フローのステップ3（防御的`ValidateEmotion`チェック）として明記 |

---

## 変更履歴

### 2026-07-08

初版作成。`10_test_cases_phase3.md` を正式な単体テスト仕様書として清書。テストケース件数の実数確認（52件、差異なし）を実施し記録。

### 2026-07-09（そらのQAレビュー指摘反映）

TC-3-2-12について、`10_test_cases_phase3.md`のGiven記述（text送出中にLLMClientが後続エラーを返す）が3.5節のアーキテクチャ（LLM呼び出しはtext送出開始前に一度だけ完了する同期呼び出し）上原理的に起こり得ないとの指摘を受け、実際のテストコードが検証しているシナリオ（EventSinkのSendTextChunkが書き込み失敗を返す）に4.2節の記述を統一。あわせて3.5節の内部処理順序に「EventSinkのSend\*メソッドがエラーを返した場合はSendErrorに切り替えて処理を中断する」旨を明記した。

*記録: WhiteCUL*
