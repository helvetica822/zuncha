# 実装指示書 — Wave W-10: Whisper.cpp STT実装

- 指示者: 四国めたん(テックリード) / つむぎ代行 / 2026-08-14
- 実装担当: ずんだもん
- **実装前提: `docs/04_implementation/04_realtime_wiring_design.md` D-3・D-3a(2026-08-14決定)を必ず読むこと**
- 依存: なし(既存の`internal/stt`パッケージ判定ロジックはW-01〜で実装済み、そのまま使う)

---

## 1. 背景・スコープ

STT(音声認識)は「ブラウザの録音データ(webm/opus) → ffmpeg変換(16kHz mono WAV) → whisper-server(HTTP) → テキスト+信頼度」という経路。今回のWaveは**バックエンド側のみ**(`POST /conversations/{id}/stt`ハンドラ、ffmpeg変換、whisper-serverクライアント)。フロントの録音配線(`src/lib/recorder.ts`)はW-16で別途行うので**今回は触らない**。

新規実装:
```
internal/whispercpp/client.go   whisper-server HTTPクライアント
internal/audioconv/converter.go ffmpegでの音声形式変換
internal/service/speech_to_text.go  変換+認識のオーケストレーション(仮称、命名はお任せ)
internal/handler/stt.go         POST /conversations/{id}/stt ハンドラ
```

---

## 2. `internal/whispercpp/client.go`

### 2.1 確定事項(D-3aより・変えないこと)

whisper-serverの`/inference`は`response_format=json`だと`{"text": "..."}`のみで信頼度が取れない。**`response_format=verbose_json`を使う**こと。

レスポンス形式(公式ソース確認済み):
```json
{
  "task": "transcribe",
  "language": "ja",
  "duration": 2.5,
  "text": "全体の転写テキスト",
  "segments": [
    {
      "id": 0,
      "text": "...",
      "start": 0.0,
      "end": 1.2,
      "no_speech_prob": 0.05,
      "avg_logprob": -0.3,
      ...
    }
  ]
}
```

**confidence算出(確定)**: `confidence = 1 - max(segments[].no_speech_prob)`。segmentsが空(無音のみ等)なら`confidence = 0`。text はトップレベルの`"text"`フィールドをそのまま使う(segmentsを結合し直す必要はない)。

### 2.2 実装

```go
package whispercpp

// Client は whisper-server(HTTP) 経由の音声認識クライアント。
type Client struct { ... }

func NewClient(baseURL string, opts ...Option) (*Client, error)

// Transcribe は WAV バイト列を whisper-server へ送り、認識結果を返す。
func (c *Client) Transcribe(ctx context.Context, wav []byte) (stt.STTResult, error)
```

- `POST {baseURL}/inference` へ `multipart/form-data`(フィールド名`file`、`response_format=json`ではなく`response_format=verbose_json`をフォームフィールドとして付与)
- ベースURLは`Option`パターンで注入可能に(Wave C-1の`internal/anthropic`、W-09の`internal/voicevox`と同じパターン)
- リクエストタイムアウトを設定すること(値は指示書§5のハンドラ全体タイムアウトとの兼ね合いで決める。**推測せず、ハンドラ側の設計と合わせて考えること**)
- 空文字列・不正なJSON応答などの異常系エラー処理

### 2.3 テスト(`tests/unit/whispercpp_client_test.go`)

`httptest.NewServer`のみ。実whisper-serverは絶対に呼ばないこと。

**正常系**:
- [ ] `verbose_json`のレスポンス(segments1件、no_speech_prob=0.1)→ `confidence=0.9`、`text`がそのまま返る
- [ ] segments複数件、no_speech_probが異なる → 最大値から算出したconfidenceになる(最も低い信頼度が採用されることを検証)
- [ ] segments空配列 → `confidence=0`
- [ ] リクエストに`response_format=verbose_json`のフォームフィールドと`file`(WAVバイト列)が乗っていることを検証(**リクエストボディそのものをアサートする** — `tasks/lessons.md` 2026-08-10「外部ライブラリに渡すだけの設定値」の教訓を適用すること)

**異常系**:
- [ ] whisper-serverが非2xxを返す → エラー
- [ ] レスポンスが不正なJSON → エラー
- [ ] `ctx`を即キャンセル → `context.Canceled`系のエラー
- [ ] whisper-serverへ到達できない(接続拒否) → エラー

**ミューテーション実測**: `no_speech_prob`の最大値ではなく最小値や平均を使うよう改変してテストが赤になることを確認(`mutation-test-overlay`スキル使用)。

---

## 3. `internal/audioconv/converter.go`(ffmpeg変換)

### 3.1 設計

```go
package audioconv

// Converter は音声データをffmpegで16kHz mono WAVへ変換する。
type Converter struct { ... }

func NewConverter() *Converter

// Convert は input(webm/opus等)を16kHz mono WAVへ変換して返す。
func (c *Converter) Convert(ctx context.Context, input []byte) ([]byte, error)
```

- `exec.CommandContext`でffmpegバイナリを呼ぶ。標準入力からinputを渡し(`cmd.Stdin`)、標準出力からWAVを受け取る(`cmd.Stdout`)。一時ファイルを作らずパイプ処理すること(ディスクI/Oを避ける、`localfs`のようなファイル永続化は不要な一時データのため)
- ffmpegの引数: 入力形式は自動判定(`-i pipe:0`)、出力は`-f wav -ar 16000 -ac 1 pipe:1`相当(16kHz・モノラル・WAV形式)。**正確なフラグは`ffmpeg -h`相当のドキュメントで確認し、推測で書かないこと**
- 標準エラー出力(stderr)はログ用に保持し、変換失敗時のエラーメッセージに含める(ただし音声データ自体は含めないこと)

### 3.2 テスト戦略(重要)

**この開発環境にはffmpegがインストールされていない。** そのため:

1. `Converter`を直接使うテストとは別に、**`AudioConverter`インターフェース**(`Convert(ctx, input []byte) ([]byte, error)`)を定義し、`internal/service`側はこのインターフェース越しに依存する(モックでテスト可能にする)
2. `internal/audioconv`パッケージ自体のテスト(`tests/unit/audioconv_test.go`)は、`exec.LookPath("ffmpeg")`でffmpegの有無を確認し、**無ければ`t.Skip("ffmpegが未インストールのためスキップ")`**する(既存のDB接続テストの`t.Skip`パターンと同じ考え方)
3. skipされたテストが「緑に見えて実は何も検証していない」ことを`t.Log`で明示すること(既存のDB未設定時の警告パターンを踏襲)

**完了条件でこのSkipに関する報告を必須とする**(下記§6参照)。

---

## 4. `internal/service`: オーケストレーション

```go
package service

type SpeechToTextService struct {
	converter AudioConverter    // Convert(ctx, []byte) ([]byte, error)
	client    STTClient          // Transcribe(ctx, []byte) (stt.STTResult, error) — internal/whispercppが実装
}

func (s *SpeechToTextService) Transcribe(ctx context.Context, rawAudio []byte) (stt.STTResult, error) {
	wav, err := s.converter.Convert(ctx, rawAudio)
	if err != nil { ... }
	return s.client.Transcribe(ctx, wav)
}
```

- `AudioConverter`・`STTClient`は`internal/service`側で最小I/Fとして定義する(W-09の`internal/voicevox`が`AudioRepository`/`FileWriter`を消費側で定義したのと同じ方針)
- テストは両方ともモックで駆動(`tests/unit/speech_to_text_service_test.go`)

---

## 5. `internal/handler/stt.go`

```go
// POST /conversations/{id}/stt
func (h *Handler) HandleSTT(w http.ResponseWriter, r *http.Request)
```

- `multipart/form-data`で音声ファイルを受信(`r.ParseMultipartForm`、フィールド名は`audio`。フロントとの契約なので`01_screen_design.md`と齟齬が無いか確認すること)
- 会話IDのULID形式チェック・`convRepo.Exists`での存在チェック(`messages.go`の`PostMessage`と同じパターン)
- サービス層(`SpeechToTextService.Transcribe`)を呼ぶ
- `stt.IsRecognitionFailed(result)`で判定:
  - 失敗 → `200 {"failed": true}`(`01_screen_design.md`の仕様どおり。**400や500にしないこと** — 認識失敗はクライアントエラーではなく正常系の一部)
  - 成功 → `200 {"text": "...", "confidence": 0.xx}`
- サービス層のエラー(ffmpeg失敗・whisper-server接続失敗等) → `500`
- **タイムアウト設定**: 既存の`responseTimeout`(60秒、LLM+TTS用)とは別に、STT用の妥当な値を設定すること。同期処理なのでリクエストのr.Context()をそのまま使ってよい(202+goroutineパターンにする必要はない)。値の根拠を報告に書くこと

### テスト(`tests/integration/handler_stt_test.go`または`tests/unit`、依存関係次第で判断)

- [ ] 正常系: multipart/form-dataで音声を送り、200 + text/confidenceが返る
- [ ] 認識失敗: confidenceが閾値未満 → 200 + `{"failed": true}`
- [ ] 会話IDが不正な形式 → 400
- [ ] 存在しない会話ID → 404
- [ ] multipart形式が不正 → 400
- [ ] サービス層エラー(ffmpeg/whisper-server失敗) → 500

---

## 6. `cmd/api`への配線

- `WHISPER_SERVER_BASE_URL`環境変数を`loadConfig`に追加。W-09の`VOICEVOX_BASE_URL`と同じ判断基準(秘密情報でない・標準ポートで開発時そのまま動く)で、デフォルト値の要否を判断し、理由を報告に書くこと
- `Handler`に`SpeechToTextService`を注入する配線を追加

---

## 7. 完了条件

- [ ] `go test ./... -count=1` 全緑、`./scripts/test_race.sh`もクリーン(`export ZUNCHA_TEST_DB_OWNER=zundamon`)
- [ ] `gofmt -l .`空 / `go vet ./...` EXIT 0 / `go build ./...`成功
- [ ] `internal/llm`・`internal/anthropic`・`internal/tts`・`internal/voicevox`は無変更であること
- [ ] whisper-serverクライアントのテストが実whisper-serverを呼んでいないこと(httptestのみ)
- [ ] **ffmpeg未インストール環境でのSkip状況を報告に明記**(何件skipされたか、それらが緑に見えて実は未検証であることの注記)
- [ ] RED確認を報告に含める
- [ ] ミューテーション実測(§2.3の1点 + 既存テストへの退行がないこと)

## 8. 報告について

- 緑でも`completed`にせず`in_progress`のまま報告
- STT用タイムアウトの値の根拠、`WHISPER_SERVER_BASE_URL`未設定時の扱いの判断理由を必ず書くこと
- ffmpegの正確なコマンドライン引数は自己判断で決めず、根拠(公式ドキュメントの該当箇所など)を報告に書くこと
- D-3・D-3aの確定事項を変えたくなったら、実装で埋めずに相談すること
