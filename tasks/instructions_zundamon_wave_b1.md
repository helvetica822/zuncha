# 実装指示書 — Wave B-1: SSE 基盤 (W-03 / W-04 / W-05)

- 指示者: 四国めたん (テックリード) / 2026-08-05
- 実装担当: ずんだもん
- **実装契約: `docs/04_implementation/05_sse_protocol_spec.md`** — 本指示書と併せて必ず読むこと。仕様の正は同書。
- 規約: `.claude/rules/golang-coding-guideline.md`、`.claude/rules/tdd-comprehensive.md`

> Wave A (W-01/W-01b/W-02) の完了報告を待たずに着手可。**依存はゼロ**ですわ。
> W-06 (ハンドラ配線) と W-07 (プロンプト組み立て) は Wave A に依存するため、別途指示します。

---

## 0. 全体像

この3タスクで「SSEでイベントを流す土台」を作る。上位のハンドラやLLMには**まだ触らない**。

```
HTTPConn (W-03)   1接続への書き込み・整形・Flush・ハートビート
Hub      (W-04)   会話ID → 接続群の登録/解除/ブロードキャスト
Fanout   (W-03)   EventSink を実装し request_id を注入して Hub へ流す
SentenceChunker (W-05)  応答テキストを配信単位に分割
```

**なぜ3層に分けるのか**は仕様書 §4.1 に書いてあります。既存の `sse.EventSink` I/F は `request_id` を引数に持たず、しかも1接続は複数リクエストのイベントを流し、1会話にはN本の接続がある — この3つの多重度を1つの型で兼ねると必ず破綻します。**`internal/sse/sse.go` の既存 I/F (`EventSink`/`TextChunker`) は変更しないこと。**

---

## 1. W-03: `HTTPConn` と `Fanout` (`internal/sse/conn.go`, `internal/sse/fanout.go`)

### 1.1 `Conn` インターフェースと `HTTPConn`

```go
// Conn は SSE 1接続への書き込みを抽象化する。
type Conn interface {
	WriteEvent(name string, data any) error
}

// ErrSinkOverflow は送信バッファが満杯（遅い/死んだクライアント）であることを示す。
var ErrSinkOverflow = errors.New("sse: sink overflow")

func NewHTTPConn(w http.ResponseWriter) (*HTTPConn, error)
func (c *HTTPConn) WriteEvent(name string, data any) error
func (c *HTTPConn) Run(ctx context.Context)  // 書き込み goroutine（呼び出し側が go で起動）
```

### 1.2 確定した設計判断 (変えないこと)

| 項目 | 決定 | 理由 |
|---|---|---|
| 並行性 | **バッファ付きチャネル (容量64) + 書き込み専用 goroutine 1本** | `http.ResponseWriter` は並行書き込み安全でない。複数リクエストの応答とハートビートが別 goroutine から来るため、素朴に書くとデータ競合。**単一ライタなら mutex 自体が不要になる** |
| バッファ満杯時 | `ErrSinkOverflow` を返す (**ブロックしない**) | 遅い1クライアントで他9人の配信を止めない (NF-SCALE-01) |
| `http.Flusher` 非対応の `w` | `NewHTTPConn` が**エラーを返す** | Flush できない環境でSSEは成立しない。無言で動かないより起動時に落とす |
| ヘッダ | 仕様書 §2.1 の4つを設定。`X-Accel-Buffering: no` も**最初から付ける** | 後からプロキシ環境で配信が固まる原因を探すのは高コスト |
| `retry: 3000` | 接続確立直後に1回送出 | F-RT-02 (再接続間隔のサーバー指示) |
| ハートビート | `Run` の中で `time.Ticker` から**SSEコメント行 `: ping`** を直接書き出す (既定15秒)。**間隔はテストから差し替え可能にする** (仕様書§4.4) | 守るべき不変条件は「`ResponseWriter` へ書く goroutine が1本」であること。**`Run` 自身から送信チャネルへ送ると自己デッドロックする** (`Run` が唯一の読み手のため)。※当初「同じチャネルを通し」と書いていたのは危険を招く曖昧な記述で、2026-08-05 に訂正 |
| 終了 | `ctx.Done()` で `Run` を抜ける | goroutine リーク防止 |
| `id:` フィールド | **振らない** | 再送に対応しないため。振ると「再送できる」誤解を生む (仕様書 §2.3) |

### 1.3 フレーム整形

```
event: <name>\ndata: <1行JSON>\n\n
```

- `json.Marshal` の出力をそのまま使う (文字列中の改行は `\n` にエスケープされるため**1行が保証される**)。整形して複数行にすると SSE が壊れます。
- 各フレームの書き込み後に `Flush()`。

### 1.4 `Fanout` (`EventSink` の実装)

```go
func NewFanout(h *Hub, conversationID, requestID string) EventSink
```

- `SendEmotion(label)` → `h.Broadcast(convID, "emotion", map[string]string{"request_id": rid, "label": label})`
- 同様に `SendTextChunk` → `text`/`chunk`、`SendAudioURL` → `audio_url`/`url`、`SendDone` → `done` (`request_id` のみ)、`SendError` → `error`/`message`。
- **ペイロード組み立てはここ1箇所に集約**。Hub にイベント種別を知らせない。
- 各メソッドは **`nil` を返す**。理由: `Broadcast` は個別接続の失敗を吸収する設計であり、「1接続に届かなかった」ことで `ResponseStreamer` の処理全体を中断させてはならない (中断すると他の接続にも応答が届かなくなる)。`ResponseStreamer` の既存テスト15件が期待する「EventSink 書き込み失敗で SendError に切替え中断」という挙動は、**モック sink での契約であり、実装側の Fanout では失敗を返さない**という違いを理解しておくこと。判断に迷ったら私に聞いてください。

### 1.5 テスト (`tests/unit/sse_conn_test.go` 等)

`httptest.NewRecorder()` は `http.Flusher` を実装しているので使えます。

**正常系** — 出力バイト列を**具体的な文字列で**検証:
- [ ] `WriteEvent("emotion", ...)` の出力が `event: emotion\ndata: {"label":"喜び",...}\n\n` の形になる (フィールド順は `json.Marshal` の構造体順に依存するので、構造体を定義して順序を固定するか `JSONEq` 相当で比較する)。
- [ ] 接続確立直後の出力に `retry: 3000` が含まれる。
- [ ] レスポンスヘッダ4つがすべて設定されている (値まで検証)。

**エッジケース**:
- [ ] **改行を含むテキスト** (`"line1\nline2"`) を送出 → 出力が**1行の data 行**になる (`data:` が2行に割れていない)。これが壊れると SSE パースが崩れる最重要ケース。
- [ ] 絵文字・全角を含むテキスト → 文字化けしない。
- [ ] 空文字列のチャンク → フレーム自体は出る (整形が壊れない)。
- [ ] バッファ64を超える連続書き込み → `ErrSinkOverflow` が返る (境界: 64件目までは成功)。
- [ ] `ctx` キャンセル → `Run` が戻る (goroutine が終了する)。

**異常系**:
- [ ] `http.Flusher` を実装しない `ResponseWriter` → `NewHTTPConn` がエラー。

---

## 2. W-04: `Hub` (`internal/sse/hub.go`)

```go
func NewHub() *Hub
func (h *Hub) Register(conversationID string, c Conn) (unregister func())
func (h *Hub) Broadcast(conversationID string, name string, data any)
```

### 2.1 確定した設計判断

| 項目 | 決定 | 理由 |
|---|---|---|
| 排他 | `sync.RWMutex`。`Broadcast` は読み取りロック、Register/解除は書き込みロック | 10人・単一インスタンス前提では十分。外部ストア (Redis等) は**入れない** |
| `Broadcast` の戻り値 | **なし (エラーを返さない)** | 1接続の失敗で応答生成全体を落とすのは誤り |
| 失敗した接続 | その場で解除する | 死んだ接続を残すとメモリと配信コストが積む |
| 同一会話への複数接続 | **許可し全員へ配信** | 会話履歴は全ユーザー共有 (C-06) |
| `unregister` の冪等性 | 2回呼んでも安全にする | handler が `defer` で呼ぶうえ、失敗時に Hub 側でも解除するため**二重解除が普通に起きる** |
| 登録0件の会話へ Broadcast | 何もしない (エラーにしない) | 誰も見ていない会話への配信は正常系 |

### 2.2 テスト (`tests/unit/sse_hub_test.go`)

- [ ] 登録した接続にイベントが届く (name と data を具体的に検証)。
- [ ] **同一会話に3接続 → 全員に届く**。
- [ ] **別会話の接続には届かない** (会話の分離。漏れたら他人の会話が見える重大バグ)。
- [ ] `unregister` 後は届かない。
- [ ] `unregister` を2回呼んでもパニックしない。
- [ ] `WriteEvent` がエラーを返す接続 → **その接続は解除され、同じ会話の他の接続には届く**。
- [ ] 登録0件の会話へ Broadcast → パニックしない。
- [ ] **10並列で登録/解除/配信を同時実行 → 競合しない**。既存 `TestConversationRepository_10並列実行` と同じ流儀で書くこと。
- [ ] **`go test -race` で実行し、競合検出器がクリーンであること**。この構造の正しさは `-race` でしか実証できません。報告に `-race` 付きの実行結果を必ず添えてください。

---

## 3. W-05: `SentenceChunker` (`internal/sse/chunker.go`)

既存 `TextChunker` I/F (`Chunk(text string) []string`) の実装。

### 3.1 分割規則 (確定)

| 規則 | 内容 |
|---|---|
| 区切り文字 | `。` `！` `？` `!` `?` `\n` |
| 区切り文字の扱い | **直後で切り、区切り文字はチャンクに含める** (「ずんだもんなのだ。」で1チャンク) |
| `、` | **区切らない** (粒度が細かすぎて表示がちらつく) |
| 上限 | 区切り文字が来なくても **60ルーンで強制分割** (ルーン単位。バイト単位だと日本語・絵文字が壊れる) |
| 末尾の残余 | 区切り文字が無くても1チャンクとして返す |
| 空チャンク | **生成しない** (空文字列は結果に含めない)。空白のみのチャンクは保持する |
| trim | **しない** (表示用テキストなので原文を保つ) |
| 空入力 | **長さ0の非nilスライス** を返す (`[]string{}`)。既存 `ReverseMessages` と同じ「空でも非nil」の流儀 |

### 3.2 テスト (`tests/unit/sse_chunker_test.go`) — 表駆動で

- [ ] `"こんにちはなのだ。元気なのだ。"` → 2チャンク、各々が句点を含む。
- [ ] `"やったのだ！"` / `"なんでなのだ？"` / 半角 `!` `?` → それぞれ1チャンク。
- [ ] 改行区切り → 分割される。
- [ ] 区切り文字なしの短文 → 1チャンク。
- [ ] **60ルーンちょうど** → 1チャンク / **61ルーン** → 2チャンク (境界値)。
- [ ] 60ルーン超の**絵文字・全角のみ**の文字列 → ルーン境界で切れており文字化けしない。
- [ ] `"。。。"` → 3チャンク (連続区切り)。
- [ ] 空文字列 `""` → **長さ0の非nilスライス**。`nil` ではないことを明示的に検証。
- [ ] 空白のみ `"   "` → 1チャンク (trim されず保持される)。
- [ ] 末尾に区切り文字が無い場合の残余が落ちない (`"あ。い"` → `["あ。", "い"]`)。

---

## 4. 完了条件

- [ ] `go test ./tests/... -v` 全緑、かつ **`go test -race ./tests/...` もクリーン**。
- [ ] `gofmt -l .` 空出力 / `go vet ./...` EXIT 0 / `go build ./...` 成功。
- [ ] `internal/sse/sse.go` の既存 I/F (`EventSink`/`TextChunker`) が**未変更**であること。
- [ ] 各タスクで **RED を目視確認したこと**を報告に含める。

## 5. 報告について

- 緑でも**タスクを `completed` にしない**。`in_progress` のまま私に報告。完了判定はつむぎ(PM)の最終ゲート。
- **§1.2 / §1.4 / §2.1 / §3.1 の「確定した設計判断」を変えたくなったら、実装で埋めずに私に相談**すること。特に §1.4 の「`Fanout` は `nil` を返す」は既存 `ResponseStreamer` テストの期待と噛み合わない見え方をするので、疑問に思ったら必ず聞いてくださいまし。
