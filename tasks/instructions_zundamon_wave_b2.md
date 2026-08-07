# 実装指示書 — Wave B-2: ハンドラ配線とオーケストレーション (W-06 / W-07)

- 指示者: 四国めたん (テックリード) / 2026-08-05
- 実装担当: ずんだもん
- **実装契約: `docs/04_implementation/05_sse_protocol_spec.md`**（特に §2 / §3 / §4）
- 設計方針: `docs/04_implementation/04_realtime_wiring_design.md` D-1 / **D-5（2026-08-05 改訂）**
- 前提: Wave A (W-01/W-01b/W-02) 完了済 / **Wave B-1 (W-03/W-04/W-05) の完了後に着手**

> Wave B-1 の `HTTPConn` / `Hub` / `Fanout` / `SentenceChunker` が揃っていないと配線できません。B-1 を先に仕上げて報告してください。

---

## 0-0. 【着手の前提条件】モックスタブを `m.Called()` 形式へ差し替える

Wave A で追加した2つのスタブは `mock.Called()` を通していないため、**testify の `m.Calls` に記録が残りません**。

```go
// 現状（Wave A）— 呼び出しが記録されない
func (m *mockConversationRepository) SetFirstText(ctx context.Context, conversationID, text string) error {
	return nil
}
```

`AssertNumberOfCalls` / `AssertCalled` / `.Return` でのエラー注入は「0件記録」で**大声で落ちる**ため偽陽性になりませんが、**`AssertNotCalled` と `mock.InOrder`／`.NotBefore` の順序検証だけは静かに通ってしまいます**(そらがレビューで testify v1.9.0 の `mock.go:532` を読んで特定)。

**そして W-07 の仕様「最初のユーザー発話のみ記録する」の最も自然なテストは「2回目は `SetFirstText` を呼ばない」= `AssertNotCalled` であり、まさに偽陽性の当たり所です。**

したがって **W-06/W-07 の実装より先に**、`tests/unit/create_conversation_service_test.go` と `tests/unit/fetch_audio_service_test.go` の2スタブを他のモックメソッドと同じ形式へ差し替えてください。

```go
func (m *mockConversationRepository) SetFirstText(ctx context.Context, conversationID, text string) error {
	args := m.Called(ctx, conversationID, text)
	return args.Error(0)
}
```

- `m.Called()` を通すと**引数のマッチャ設定が無い呼び出しでパニックする**ため、既存テストが `SetFirstText`/`InsertRecord` を呼んでいないことの証明にもなります。もし既存テストが落ちたら「実は呼ばれていた」ことの発見なので、**手を止めて私に報告**してください。
- 既存のテストケース・期待値は引き続き1行も変更しないこと。

---

## 0. このフェーズで最も重要な方針転換（先に読むこと）

設計書 D-5 の当初案「`ResponseStreamer.StreamResponse` の引数を会話コンテキストへ拡張する」は**撤回しました**。既存の 3-2 テスト15件を壊すだけで得るものが無いためです。

**`internal/service/response_streamer.go` と `internal/llm/*.go` の既存コードは一切変更しないこと。** 代わりに周辺へ3つの部品を足します。

```
ChatService (W-07)        保存 → 履歴取得 → BuildPrompt → sink構築 → StreamResponse
  ├─ BuildPrompt          純粋関数。履歴 → プロンプト文字列
  ├─ RecordingSink        EventSink デコレータ。応答を蓄積し done 直前に永続化
  └─ Fanout (B-1済)       EventSink 実装。request_id を注入して Hub へ
handler (W-06)            HTTPリクエストの検証 → ChatService 呼び出し
```

---

## 1. W-06: ハンドラ (`internal/handler/`)

### 1.1 パッケージを新設し、既存2ハンドラも移設する

`internal/handler` を新設し、以下を置く:

| ファイル | 内容 |
|---|---|
| `handler.go` | `Handler` 構造体（依存を保持）+ `NewHandler(...)` + `Routes() *http.ServeMux` |
| `conversation.go` | `POST /conversations`（**main.go から移設**） |
| `audio.go` | `GET /audio/{id}`（**main.go から移設**） |
| `events.go` | `GET /conversations/{id}/events`（新規） |
| `messages.go` | `POST /conversations/{id}/messages`（新規） |
| `respond.go` | `respondJSON` / `respondError`（**main.go から移設**） |

**移設を含める判断の根拠（調査済み）**: `cmd/api/main_test.go` は `parseAllowedOrigins` のみを検証しており、ハンドラには触っていません。`tests/integration/cors_test.go`・`graceful_shutdown_test.go` は独自の mux を組んでいるため無影響です。**両方とも実測で確認済み**なので、移設で壊れるテストはありません。

- **移設は「移動のみ」**。ロジックの変更・改善は一切しないこと（差分を読めなくしないため）。
- `main.go` に残すのは `loadConfig` / `parseAllowedOrigins` / `main`（DI配線）/ 定数のみ。
- SSEハンドラを `main.go` に置くと `httptest` で単体検証できません。ここを分けるのが移設の目的です。

### 1.2 `GET /conversations/{id}/events`

```
1. {id} を validation.IsValidULID で検証 → 不正なら 400
2. 会話の存在確認 → 無ければ 404
3. NewHTTPConn(w) → 失敗なら 500
4. unregister := hub.Register(id, conn); defer unregister()
5. go conn.Run(r.Context())  ※ または Run を同 goroutine でブロックさせる
6. <-r.Context().Done() でハンドラを維持（戻るとレスポンスが閉じる）
```

| 項目 | 決定 | 理由 |
|---|---|---|
| ハンドラの生存 | `r.Context().Done()` まで**戻らない** | ハンドラが return するとHTTPレスポンスが閉じ、SSE接続が切れる |
| 会話の存在確認 | **する**（404を返す） | 存在しない会話へ延々と接続させない |
| `defer unregister()` | **必須** | 無いと接続が Hub に残り続け、メモリと配信コストが積む |

### 1.3 `POST /conversations/{id}/messages`

仕様書 §3.1 の表のとおり。**ステータスコードと本文をそのまま実装**してください。

| 状況 | ステータス |
|---|---|
| 受理 | `202` + `{"request_id":"..."}` |
| `id` / `request_id` が不正なULID | `400` |
| `text` が trim 後に空（`validation.IsValidInput`） | `400` |
| **`text` が不正なUTF-8**（`utf8.ValidString` が false） | `400` |
| 会話が存在しない | `404` |
| 同一 `request_id` の再送 | `202`（何もしない = §3.3） |

> **⚠ 2026-08-05 訂正**: 下記の理由は**前提が誤っていた**。`encoding/json` の `Decode` は不正UTF-8を **U+FFFD へ置換する**ため、JSON body 経路では `req.Text` に不正UTF-8は残らず、**このガードは到達不能**(実測で確認)。DBの encoding エラーも起きない。ガードは多層防御として残すが、**テストは「400になる」ではなく「202で受理され U+FFFD 置換された状態で保存され INSERT が壊れない」を検証する実態に合わせた内容**とすること。詳細は設計書「A-2 の訂正」。実際に効くのは STT結果の経路(生バイトを扱う場合)のみ。

**不正UTF-8を入口で弾く理由（そらのレビュー申し送り②b への対応・上記訂正を踏まえて読むこと）**: `validation.TruncateFirstText` は20ルーン以下の入力を**そのまま返す**ため、不正UTF-8がサニタイズされずDBへ届き `INSERT` が encoding エラーで落ちます(21ルーン以上なら `[]rune` 変換で U+FFFD に置換されるという**非対称**があり、短い入力ほど危険という直感に反する挙動)。`TruncateFirstText` の契約(trim/サニタイズしない)は既存11件のテストで固定されているので**変更しません**。代わりに**ハンドラ入口の1箇所で弾く**のが最も安く、`messages.content` と `conversations.first_text` の両方を同時に守れます (NF-SEC-01「ユーザー入力は全てバリデーション」)。同じ検査を `POST .../stt` のSTT結果にも適用してください(Whisper の出力も外部入力です)。

**最重要の罠 — ここを間違えると応答生成が即死します**:

```go
// ❌ これは動かない。202 を返した時点で r.Context() がキャンセルされ、応答生成が中断する
go s.chat.HandleUserMessage(r.Context(), ...)

// ✅ 親のキャンセルを切り離し、独自のタイムアウトを被せる
ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), responseTimeout)
go func() { defer cancel(); _ = s.chat.HandleUserMessage(ctx, ...) }()
```

`responseTimeout` は **60秒**の定数として定義すること。

### 1.4 テスト（`tests/integration/handler_*_test.go`）

`httptest.NewServer` / `httptest.NewRecorder` で駆動。DBは既存 `setupTestDB` を流用。

**`/events`**:
- [ ] 不正ULID → `400` / 存在しない会話 → `404`。
- [ ] 正常接続 → ヘッダ4種が設定され、`retry: 3000` が流れてくる。
- [ ] 接続中に `hub.Broadcast` → クライアントが受信できる（**エンドツーエンドの疎通証明**）。
- [ ] クライアント切断 → **Hub から解除される**（解除の観測方法: 切断後に Broadcast しても登録数が0、または2本目の接続だけに届く形で検証する）。

**`/messages`**:
- [ ] 各ステータスコードを表駆動で検証（正常202・不正ULID 400・空text 400・空白のみtext 400・会話なし404）。
- [ ] **同一 `request_id` を2回POST → LLM呼び出しが1回だけ**（フェイクLLMの呼び出し回数で検証）。§3.3 の第二防衛線。
- [ ] **202 を返した後も応答生成が完走する**（`context.WithoutCancel` が効いている証明）。これを**必ず書くこと** — 罠の再発防止テストです。
- [ ] 応答イベントが `/events` 側へ流れる（`emotion` → `text` → `done` の順序）。

---

## 2. W-07: `BuildPrompt` / `RecordingSink` / `ChatService`

### 2.1 `BuildPrompt`（`internal/llm/prompt.go`）

```go
func BuildPrompt(history []model.Message) string
```

| 項目 | 決定 | 理由 |
|---|---|---|
| 引数 | 履歴のみ。**`userText` を取らない** | `ChatService` がユーザー発話を**先に保存**するため、履歴末尾に当該発話が既に含まれる。別途渡すと二重になる |
| システムプロンプト | **含めない** | ずんだもん口調・JSON形式の強制は W-08 のプロバイダ実装（`internal/anthropic`）の責務。ここに書くと差し替え可能性が壊れる |
| 整形 | `role: content` を改行区切りで並べる。emotion は**含めない** | LLMへの入力は発話内容のみで足りる |
| 空履歴 | 空文字列を返す | |

**テスト**: 空履歴 / 1件 / 20件 / user・assistant混在の順序保持 / 改行を含む content / 絵文字。**具体的な文字列で**検証すること。

### 2.2 `RecordingSink`（`internal/service/recording_sink.go`）

```go
func NewRecordingSink(
	inner sse.EventSink,
	repo repository.MessageRepository,
	conversationID string,
	newID func() string,
	now func() time.Time,
) sse.EventSink
```

| メソッド | 挙動 |
|---|---|
| `SendEmotion(label)` | label を保持 → `inner` へ委譲 |
| `SendTextChunk(chunk)` | `strings.Builder` へ追記 → `inner` へ委譲 |
| `SendAudioURL(url)` | 委譲のみ |
| `SendDone()` | **`InsertMessage`（assistant・蓄積テキスト・保持した emotion）を先に実行** → その後 `inner.SendDone()` |
| `SendError(msg)` | 委譲のみ（**保存しない**） |

| 項目 | 決定 | 理由 |
|---|---|---|
| 保存のタイミング | `SendDone` の**直前** | 「画面には出たがDBに無い」を防ぐ（仕様書 §3.2） |
| 保存失敗時 | **ログのみ。`SendDone` は送る**（エラーを返さない） | 応答は既にユーザーの画面に届いている。ここでエラートーストを出すと混乱させるだけ。履歴の欠落は次回の文脈が減るだけで致命的でない。**この判断はテストのコメントにも書いて固定すること** |
| `newID` / `now` の注入 | 関数として注入 | 既存方針（乱数・時刻に依存させない）と一貫。テストで決定化できる |
| 保存に使う ctx | **`context.WithTimeout(context.Background(), 5*time.Second)`**（2026-08-05 確定） | 署名に ctx を足さない（§2.2 の契約を維持）。`Background` 単体だとDBハング時に `SendDone` が無制限に待つため、短いタイムアウトを被せる。リクエストのキャンセルから独立して保存できる利点はそのまま維持される |
| emotion が未受信のまま done | emotion は `nil` で保存（`*string`） | assistant の emotion は nullable |

**テスト（unit・モックRepository）**:
- [ ] 全イベントが `inner` へ**同じ順序で**委譲される。
- [ ] `SendDone` で `InsertMessage` が1回呼ばれ、**content が全チャンクの連結**・`role='assistant'`・`emotion` が受信した label と一致。
- [ ] チャンクが0件のまま `done` → content 空で保存される（境界）。
- [ ] emotion 未受信のまま `done` → `emotion` が nil。
- [ ] **`InsertMessage` が失敗しても `inner.SendDone()` が呼ばれ、`SendDone` は nil を返す**（上表の判断の固定）。
- [ ] `SendError` では `InsertMessage` が呼ばれない。
- [ ] `InsertMessage` の呼び出しが `inner.SendDone()` **より前**であること（順序検証）。

### 2.3 `ChatService`（`internal/service/chat.go`）

```go
func NewChatService(
	msgRepo repository.MessageRepository,
	convRepo repository.ConversationRepository,
	streamer *ResponseStreamer,
	hub *sse.Hub,
	newID func() string,
	now func() time.Time,
) *ChatService

func (s *ChatService) HandleUserMessage(ctx context.Context, conversationID, requestID, text string) error
```

**処理順序（この順序を変えないこと）**:

1. `InsertMessage`（role=`user`, content=text, ID=`newID()`, CreatedAt=`now()`）
2. `SetFirstText(conversationID, validation.TruncateFirstText(text))` — 冪等なので毎回呼ぶ。**エラーは握り潰してログのみ**（付随処理で、会話本体を止める理由がない。既存 `CreateConversationService` の GC と同じ流儀）
3. `GetRecentMessages(conversationID)` → `llm.BuildPrompt(history)`
4. `sink := NewRecordingSink(sse.NewFanout(hub, conversationID, requestID), msgRepo, conversationID, newID, now)`
5. `s.streamer.StreamResponse(ctx, sink, prompt)`

**【2026-08-05 追加要件】中断時は `SendError` を送ってからエラーを返す**

`InsertMessage`(user) 失敗や `GetRecentMessages` 失敗で中断すると、**フロントには `done` も `error` も届かず `inputState` が `'sending'` のまま固着します**(ずんだもんの申し送り)。ハンドラは `_ = HandleUserMessage(...)` でログのみなので、誰も復帰させられません。

**対応**: `HandleUserMessage` の**冒頭で `Fanout` を作り**、中断する各経路で `sink.SendError(...)` を送ってからエラーを返すこと。

- **なぜフロント側にタイムアウトを足さないのか**: フロントは既に **INV-3b「SSE `error` を受けたら `'sending'`/`'transcribing'` から `'editable'` へ復帰する」** を持っています(`02_parent_container_design.md` §5)。バックエンドが `SendError` を1発送れば**既存機構でそのまま解決**します。フロントにタイムアウトを新設するのは機構を増やすだけですわ。
- `RecordingSink` でのラップは LLM 呼び出し直前で構いません(`RecordingSink` は `done` 時の保存だけを担うため)。
- **テスト**: `InsertMessage`(user) を失敗させたとき、`SendError` が1回送られてからエラーが返ることを検証。**ミューテーションで `SendError` を削ると赤になること**も実測してください。

**重複 `request_id` の防御（仕様書 §3.3・第二防衛線）**: `ChatService` が保持する。

- `sync.Mutex` + `map[string]time.Time`。既に存在すれば**何もせず nil を返す**。
- 保持期間は5分。呼び出しごとに古いエントリを掃除する程度で十分（10人利用なのでLRU等は不要）。
- **なぜ Hub でなく ChatService か**: 「同じ発話を二重に処理しない」は配信の問題ではなくビジネスルールです。

**テスト（unit・フェイクLLM/モックRepository）**:
- [ ] 上記1〜5の**呼び出し順序**を検証（特に「保存 → 履歴取得」の順。逆にすると自分の発話が文脈から抜ける）。
- [ ] 履歴に**自分の発話が含まれている**ことを `BuildPrompt` への入力で確認。
- [ ] `SetFirstText` が失敗しても処理が続行する。
- [ ] `InsertMessage`（user）が失敗したら**中断してエラーを返す**（発話を失ったまま応答してはならない）。
- [ ] **同一 `request_id` の2回目は LLM が呼ばれない**・1回目とは別 `request_id` なら呼ばれる。
- [ ] 5分経過後の同一 `request_id` は再度処理される（時刻は `now` 注入で決定化）。

---

## 3. 完了条件

- [ ] `go test ./tests/... -count=1` 全緑、**`-race` もクリーン**、**integration が SKIP されていないこと**をログで確認。
- [ ] `gofmt -l .` 空 / `go vet ./...` EXIT 0 / `go build ./...` 成功。
- [ ] **`internal/service/response_streamer.go`・`internal/llm/*.go`・`internal/sse/sse.go` が未変更**であること（`git diff` で示せる状態）。
- [ ] 既存テストの変更が**モックへのメソッド追加のみ**であること。
- [ ] 各タスクで RED を目視確認したことを報告に含める。
- [ ] **ミューテーション実測**: 「`context.WithoutCancel` を `r.Context()` に戻すと §1.4 の完走テストが赤になる」ことを実測して報告してください。ここは踏みやすく、かつテストで守れる箇所です。

## 4. 報告について

- 緑でも `completed` にせず `in_progress` のまま報告すること。
- **§1.1 の移設範囲・§2.2 の「保存失敗時はログのみ」・§2.3 の処理順序**を変えたくなったら、実装で埋めずに私に相談してくださいまし。
