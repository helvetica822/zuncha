# SSE ワイヤプロトコル仕様 (Wave B/D の実装契約)

- 作成: 四国めたん (テックリード) / 2026-08-05
- 位置づけ: `04_realtime_wiring_design.md` D-1/D-2 の**実装レベルの確定仕様**。バックエンド (W-03〜W-07) とフロント (W-12〜W-15) は**本書を唯一の契約**として実装する。
- 原典: `docs/02_functional_design/01_screen_design.md` §7 (イベント名・ペイロード)、F-RT-01/F-RT-02、NF-SCALE-01/02

> **機能設計書との差分は解消済み** (2026-08-05・ユーザー承認を得て `01_screen_design.md` §6/§7 を更新)。詳細は §6。

---

## 1. エンドポイント一覧

| メソッド | パス | 役割 |
|---|---|---|
| `GET` | `/conversations/{id}/events` | SSE 常設チャネル。会話画面の表示中ずっと繋いでおく |
| `POST` | `/conversations/{id}/messages` | ユーザー発話の送信。応答は SSE 側へ流れる |
| `POST` | `/conversations/{id}/stt` | 録音音声 → テキスト (同期・SSE非経由。D-3) |
| `GET` | `/audio/{ulid}` | 音声取得 (既存・実装済み) |

`{id}` は会話ID (ULID)。**全エンドポイントで `validation.IsValidULID` を通し、不正なら 400** を返す (NF-SEC-01)。会話が存在しなければ 404。

---

## 2. `GET /conversations/{id}/events`

### 2.1 レスポンスヘッダ

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

`X-Accel-Buffering: no` は将来リバースプロキシを挟んだときにバッファリングで配信が固まるのを防ぐため**最初から付ける** (後から原因究明するのは高コスト)。

### 2.2 イベント形式

`01_screen_design.md` §7 のイベント名・ペイロードを踏襲し、**全イベントに `request_id` を追加**する。

```
event: emotion
data: {"request_id":"01J...","label":"喜び"}

event: text
data: {"request_id":"01J...","chunk":"ずんだもんだよ"}

event: audio_url
data: {"request_id":"01J...","url":"/audio/01J..."}

event: done
data: {"request_id":"01J..."}

event: error
data: {"request_id":"01J...","message":"応答の生成に失敗しました"}
```

- `data` は**必ず1行のJSON**にする (改行を含めない)。SSE は `data:` の複数行を改行結合するため、JSONを整形して複数行にすると壊れる。
- **`done` にもペイロードを持たせる**のが機能設計書 §7 (「—」) との差分。相関キーが無いと申し送り2 (遅延 `done` の誤完了) が解けないため必須。
- 接続レベルのエラー (どのリクエストにも属さない障害) では `request_id` を空文字列にする。フロントは「`request_id` が空 or 自分の最新と不一致の `error`」でも**トーストは出す** (ユーザーに知らせる価値がある) が、`inputState` は触らない。

### 2.3 接続維持と再接続

| 項目 | 決定 | 理由 |
|---|---|---|
| ハートビート | **15秒ごとに SSE コメント行 `: ping`** を送出 | 無通信で切られるのを防ぎ、死んだ接続を書き込み失敗で検出する |
| `retry:` | 接続確立直後に **`retry: 3000`** を1回送る | 再接続間隔をサーバー側から指示 (F-RT-02) |
| `Last-Event-ID` | **使わない。再送しない** | 飛行中のストリームはインメモリのみ (D-1)。再接続時の取りこぼしは再送信で回復する運用とする。**イベントに `id:` を振らない**ことで「再送できるように見える」誤解を避ける |
| 同一会話への複数接続 | **許可し、全接続へブロードキャスト** | C-06 (会話履歴は全ユーザー共有) と整合。Hub は会話ID → sink の**複数**を保持する |

### 2.4 切断の扱い

- クライアント切断は `r.Context().Done()` で検知し、**Hub から sink を必ず解除**する (goroutine とメモリのリーク防止)。
- 書き込み失敗 (`http.Flusher` 経由で error) が起きた sink も即座に解除する。
- **遅い/詰まったクライアントで配信全体をブロックさせない**: sink ごとにバッファ付きチャネル (容量は 64 程度) を持ち、**満杯なら当該 sink のイベントを捨てて解除**する。1クライアントの詰まりが他の9人を止める設計は NF-SCALE-01 の前提を壊す。

#### 2.4.1 `Run` 終了の通知が必須 (2026-08-05 追加・そらのレビュー指摘)

**当初の実装には穴があった**: `Run` が書き込み失敗で抜けても、それを誰にも通知しなかった。結果:

1. 解除されるのは `WriteEvent` が `ErrSinkOverflow` を返す経路 (バッファ満杯) **のみ**で、実際の `w.Write` 失敗では解除されない → 本節の「即座に解除する」が未達。
2. §2.3 が掲げるハートビートの存在理由「**死んだ接続を書き込み失敗で検出する**」が機能しない (検出しても解除に繋がらない)。
3. `Run` 終了後も `WriteEvent` が**バッファ段数(64)まで成功を返し続ける** → 死んだ接続への配信が最大64イベント続く。
4. 最も重い: ハンドラを `go conn.Run(ctx)` + `<-r.Context().Done()` と書くと、書き込み失敗で `Run` が抜けても **ctx はキャンセルされない** (Go の net/http はハンドラが return してから接続を閉じる)。**ハンドラが永久にブロックし、登録も goroutine も解放されない**。half-open 接続 (クライアント機のクラッシュ・回線断) で顕在化する — ハートビートが守るべきケースそのもの。

**確定する対策 (二重の防壁)**:

```go
// Done は Run が終了したときに閉じられるチャネルを返す。
// ctx キャンセル・書き込み失敗のいずれでも閉じる。
func (c *HTTPConn) Done() <-chan struct{}
```

| 防壁 | 内容 | 効果 |
|---|---|---|
| 1. ハンドラ側 | **`conn.Run(r.Context())` を同 goroutine でブロック呼び出しする**(下記「防壁1の正しい形」) | `defer unregister()` がそのまま解除を担う。**Hub に新しい通知機構を足さずに済む** |
| 2. `WriteEvent` 側 | `Run` 終了後は `ErrConnClosed` を返す (**下記の二段構え**で判定) | `Broadcast` が既存の失敗解除パスで即座に解除する。上記3の「64件遅延」が消える |

##### 防壁2 は「二段構え」でなければ確率的に穴が残る (2026-08-05 訂正・そらの指摘)

当初の本節は「select に `case <-c.done:` を追加」と書いていたが、**これは誤りだった**。Go の `select` は複数の case が同時に ready なとき**一様ランダムに1つを選ぶ**。`c.done` が閉じていて、かつ `c.frames` に空きがある状態では両方が ready なので、**約半分の確率で `nil`(成功)を返してしまう**。防壁2が「たまに効かない」機構になり、上記3の64件遅延が確率的に復活する。

**確実に `ErrConnClosed` を返すには、先に `done` だけを単独で非ブロッキング判定する**:

```go
select {
case <-c.done:
    return ErrConnClosed
default:
}

select {
case c.frames <- frame:
    return nil
default:
    return ErrSinkOverflow
}
```

##### `Run` は1接続につき1度だけ呼ぶ (二重クローズ panic の防止)

`Run` の終了時に `close(c.done)` するため、**`Run` が2回呼ばれると二重クローズで panic する**。次のいずれかで塞ぐ:

- **`sync.Once` で閉じる (推奨)** — 呼び出し側のミスを実装側で吸収できる
- doc comment に「`Run` は1接続につき1度だけ呼ぶこと」を契約として明記する

**テストで多重起動のケースを1件持つこと**。この panic は本番で初めて踏むと原因究明が高コストになる。

##### 防壁1の正しい形: `Run` を同 goroutine でブロック呼び出しする (2026-08-05 訂正)

当初の本節は防壁1を `select { case <-r.Context().Done(): case <-conn.Done(): }` と書いていたが、**これは同じ節の「ハンドラは `Run` が戻るまで return してはならない」と両立しない**。そらとずんだもんが独立に同じ矛盾を指摘した。

**なぜ両立しないか**: この select は `go conn.Run(ctx)` を前提とする。`r.Context().Done()` が先に発火した場合(=クライアント切断という**最も普通のケース**)、select は即座に返るが `Run` はまだ動いており、両者の間に順序保証がない。**ハンドラが return して `ResponseWriter` が無効化された後に `Run` が `c.w.Write` を呼び得る**。keep-alive でコネクションが次のリクエストへ再利用されると、**別リクエストのレスポンスにフレームが混入する**という最も厄介な壊れ方をする。

**採用する形 — 同 goroutine でのブロック呼び出し**:

```go
unregister := hub.Register(id, conn)
defer unregister()
conn.Run(r.Context())   // ctx キャンセルでも書き込み失敗でも戻る
```

- `Run` は ctx キャンセルでも書き込み失敗でも戻るので、**「どちらの条件でもハンドラを抜けて解除する」という防壁1の目的を満たす**。
- **「`Run` 実行中にハンドラが return する」状態が構造的に作れない**。規律ではなく構造で use-after-return を防げる。
- goroutine が1本減るので、リークの余地もその分消える。

**`go conn.Run(ctx)` を使う場合の必須知識** (将来別の理由で別 goroutine 化するとき用): 待ち受けは **`<-conn.Done()` だけ**にすること。`case <-r.Context().Done():` を**足すと保証が壊れる** — `Run` の終了を追い越すためである。

##### 二段構えでも残る狭いレース (実害なし・誤診防止のため明記)

`done` の判定を通過した直後に `Run` が終了した場合、そのフレームは `frames` に積まれたまま誰も読まず、`WriteEvent` は `nil` を返す。これは原理的に避けられない。ただし**取りこぼしは1フレームで収束し、次回以降は確実に `ErrConnClosed` になる**ため実害はない。将来テストが不安定に見えたときに「二段構えが効いていない」と誤診しないよう記録しておく。

##### 防壁2 のミューテーション要件: 「複数回実行」ではなく「1テスト内で複数回呼ぶ」

確率的な穴は**複数回実行しても確実には赤にならない**(1回50%なら10回実行で約0.1%の見逃し)。**閉鎖後に `WriteEvent` を20回呼び、全てが `ErrConnClosed` であることを1テスト内で検証する**こと。こうすれば1つの select に戻すミューテーションは 1−0.5²⁰ ≈ 99.9999% の確率で**単一実行でも**赤になる。実行回数に頼らず決定的に近づけられる。

- **コールバック(onClose)方式を採らない理由**: コールバックは `Run` の goroutine から呼ばれるため、そこで Hub の解除(書き込みロック取得)を行うと呼び出し元次第でデッドロックの考慮が生まれる。`Done()` なら**既存の解除経路(`defer unregister()`)をそのまま使える**ので、新しい経路を作らない。Go の慣習 (`context.Context.Done()`) とも一致する。
- **W-06 の前提条件**: **`ResponseWriter` はハンドラ return 後は使用禁止**なので、「**ハンドラは `Run` が戻るまで return してはならない**」。同 goroutine でのブロック呼び出し(上記「防壁1の正しい形」)なら、この条件は構造的に満たされる。

---

## 3. `POST /conversations/{id}/messages`

### 3.1 リクエスト / レスポンス

```
POST /conversations/{id}/messages
Content-Type: application/json

{"request_id":"01J...","text":"こんにちは"}
```

```
202 Accepted
{"request_id":"01J..."}
```

| 状況 | ステータス | 本文 |
|---|---|---|
| 受理 | `202 Accepted` | `{"request_id":"..."}` |
| `id` / `request_id` が不正なULID | `400` | `{"error":"..."}` |
| `text` が trim 後に空 | `400` | `{"error":"..."}` (`validation.IsValidInput` を使う) |
| 会話が存在しない | `404` | `{"error":"..."}` |
| 同一 `request_id` の再送 | `202` | **何もせず受理** (§3.3) |

### 3.2 確定した設計判断

| 項目 | 決定 | 理由 |
|---|---|---|
| `request_id` の採番 | **クライアント側 (フロント) が ULID で採番** | 送信**前**に相関キーを持てる。サーバー採番だと 202 レスポンス到着より先に届いた `done` を相関できない (実際に起こり得る: SSEは別コネクション) |
| 応答が返るタイミング | `202` を**即返す**。応答生成は別 goroutine で SSE へ | HTTPレスポンスを応答生成の完了まで待たせると、SSEで配信する意味がなくなる |
| goroutine の ctx | `context.WithoutCancel(r.Context())` に**全体タイムアウト60秒**を被せる | `r.Context()` をそのまま使うと 202 返却時点でキャンセルされ、応答生成が即死する (**踏みやすい罠**) |
| ユーザー発話の保存 | `InsertMessage` (W-01) → `SetFirstText` (W-01b、`TruncateFirstText` 経由) を**LLM呼び出しの前**に行う | 保存を後回しにすると、LLM失敗時にユーザー発話が消える |
| 応答の保存 | `done` 送出**前**に assistant メッセージを `InsertMessage` | 保存してから完了通知する。順序を逆にすると「画面には出たがDBに無い」状態が起こる |

### 3.3 二重送信の最終防衛線 (申し送り1 と対になる)

飛行中および直近に処理した `request_id` を Hub 側で保持し、**同一 `request_id` の2回目以降は何もせず 202 を返す** (冪等)。

- フロント側のガード (W-15) が第一防衛線、これが第二防衛線。**両方入れる**。フロントのガードは同一フレームの二連打を、サーバー側は再送・リトライ・複数タブを防ぐ。
- 保持期間は「飛行中 + 完了後5分」程度。10人利用なので LRU 等は不要、単純な map + 時刻で足りる。

---

## 4. 接続・Hub・Fanout の三層構造

### 4.1 なぜ3層に分けるのか

`sse.EventSink` のメソッドは `SendEmotion(label)` / `SendTextChunk(chunk)` / … と**引数に `request_id` を持たない** (既存 I/F・変更しない)。一方 `request_id` は**イベント単位**で決まり、1本の接続は**複数リクエストのイベントを流す**。さらに Hub は1会話あたり**N本の接続**へ配信する。

この3つの軸 (接続 / リクエスト / 配信先の多重度) を1つの型で兼ねると破綻するため、責務を分ける:

| 層 | 型 | 責務 | 多重度 |
|---|---|---|---|
| 接続 | `Conn` (`HTTPConn`) | SSEフレームの整形・書き込み・Flush・ハートビート | 1接続に1つ |
| 配信 | `Hub` | 会話ID → 接続群の登録/解除/ブロードキャスト | プロセスに1つ |
| リクエスト | `Fanout` | `EventSink` を実装し、`request_id` を注入して Hub へ渡す | 1リクエストに1つ |

### 4.2 契約

```go
// Conn は SSE 1接続への書き込みを抽象化する。
type Conn interface {
	// WriteEvent は event 名と JSON 化可能な data を1フレームとして送出する。
	WriteEvent(name string, data any) error
}

func NewHub() *Hub
// Register は接続を登録し、解除用の関数を返す（handler が defer で呼ぶ）。
func (h *Hub) Register(conversationID string, c Conn) (unregister func())
// Broadcast は当該会話の全接続へ1フレームを送る。失敗した接続は解除し、他へは配信を続ける。
func (h *Hub) Broadcast(conversationID string, name string, data any)

// NewFanout は EventSink を実装し、requestID を全イベントに注入して Hub へ流す。
func NewFanout(h *Hub, conversationID, requestID string) EventSink
```

- **`Broadcast(convID, name, data)` の形にした理由** (当初案 `func(EventSink) error` を渡す形から変更): `request_id` の注入は**1リクエストにつき1箇所** (`Fanout`) で完結させたい。クロージャを渡す形だと、注入責務が呼び出し側に散り、イベント種別ごとに同じ `request_id` の詰め込みを書くことになる。`Fanout` がペイロードを組み立て、Hub は「名前とデータを全接続へ書く」だけにすれば、`EventSink` のメソッド分岐は `Fanout` の中だけに収まる。
- `Hub` は `sync.RWMutex` で保護。**並行テスト必須** (既存 `TestConversationRepository_10並列実行` と同じ流儀で、10並列の登録/解除/配信が壊れないことを検証)。
- `Broadcast` はエラーを**返さない**。1接続の失敗で応答生成全体を落とすのは誤り (他の接続には届けるべき)。失敗した接続はその場で解除する。

### 4.3 単一ライタ方式 (`HTTPConn` の並行性)

`http.ResponseWriter` は**並行書き込み安全ではない**。1本の接続には「複数リクエストの応答イベント」と「15秒ハートビート」が別 goroutine から到来するため、素朴に書くとデータ競合になる。

**採用する構造**: 接続ごとに**バッファ付きチャネル (容量64) + 書き込み専用 goroutine 1本**。

- `WriteEvent` はチャネルへの**ノンブロッキング送信**のみを行う。ミューテックスは不要 (書き手が1本しかないため)。
- **チャネル満杯 = 遅い/死んだクライアント**。`ErrSinkOverflow` を返し、Hub が当該接続を解除する。1クライアントの詰まりで他の9人の配信を止めない (NF-SCALE-01 の前提)。
- **ハートビートは書き込み goroutine (`Run`) の中で `time.Ticker` から直接書き出す**。守るべき不変条件は「**`ResponseWriter` へ書く goroutine が常に1本**」であり、送信チャネルを経由させることではない。
  - ⚠ **`Run` 自身から送信チャネルへ送ってはならない**: `Run` はそのチャネルの唯一の読み手なので、満杯時に自分へ送ると**自己デッドロック**する。(当初の本節は「ハートビートも同じチャネルを通す」と書いており、この危険を招く曖昧な記述だった。2026-08-05 に上記へ訂正。)
- **ハートビート間隔はテストから差し替え可能にする** (下記 §4.4)。
- `r.Context().Done()` で goroutine を確実に終了させる (リーク防止)。

### 4.4 ハートビート間隔の注入 (2026-08-05 追加)

間隔をパッケージ定数に固定すると、「`: ping` が実際に出るか」を**実時間15秒待つ以外に検証できない** (F.I.R.S.T の Fast/Repeatable に反する)。ハートビートが壊れた場合の症状は「長時間の無通信後に接続が切れる」で、しかも自動再接続が走るため**本番でも気づきにくい**。テストで守れない機構を放置しない。

```go
type HTTPConnOption func(*HTTPConn)

// WithHeartbeatInterval はハートビート間隔を上書きする（既定 15 秒）。テスト用。
func WithHeartbeatInterval(d time.Duration) HTTPConnOption

func NewHTTPConn(w http.ResponseWriter, opts ...HTTPConnOption) (*HTTPConn, error)
```

- **可変長引数にする理由**: 既存の `NewHTTPConn(w)` 呼び出しが**無改変で動く**ため、公開API契約を壊さない (Go の functional options)。
- 既定値は **15秒のまま**。オプションを渡さない本番経路の挙動は変わらない。
- **なぜ非公開の差し替え口にしないのか**: テストは `tests/unit` に `package unit` (外部テストパッケージ) として置く規約のため、`internal/sse` の非公開シンボルには到達できない。内部テストファイルを `internal/sse/` に置くのはテスト配置の規約に反する。
- テストは短い間隔 (例: 20ms) を渡し、一定時間内に `: ping` が2回以上現れることを検証する。**間隔の `case` を削ると赤になることをミューテーションで実測**すること。

> **`-race` 付きでテストすること**: `go test -race ./tests/...`。この構造の正しさは競合検出器でしか実証できない。

---

## 5. フロント側の相関ロジック (W-14/W-15)

```
送信時:   const requestId = ulid();  currentRequestId = requestId;  → POST
受信時:   イベントの request_id !== currentRequestId → 破棄 (ただし error はトーストのみ出す)
         done かつ request_id === currentRequestId → completeSubmission()
```

- これにより INV-6d (「`sending` 以外の `done` を無視」) より**厳密**なガードになる。旧リクエストの遅延 `done` は状態を見ずに ID だけで弾ける。
- `completeSubmission()` の既存シグネチャ (引数なし) は**変更しない**。相関判定は呼び出し側 (`sseClient` 層) で行い、通過したものだけ `completeSubmission()` を呼ぶ。**既存95件のフロントテストを無改変で維持するため** (D-2)。

---

## 6. 機能設計書への同期 (2026-08-05 完了・ユーザー承認済み)

| # | 差分 | 反映先 | 状態 |
|---|---|---|---|
| 1 | 全SSEイベントに `request_id` を追加。`done` のペイロードを「—」から `{"request_id":"..."}` へ | `01_screen_design.md` §7 の表 + **§7.1 新設** (相関の必要性・クライアント採番の理由・接続レベルエラーの例外) | ✅ 反映済 |
| 2 | 接続方式 (常設チャネル・再接続・ハートビート・取りこぼし・複数接続) | `01_screen_design.md` **§7.2 新設** | ✅ 反映済 |
| 3 | エンドポイント一覧 (`/events`・`/messages`・`/stt` の追加) | `01_screen_design.md` **§7.3 新設** + §6 に「画面内で使用するAPI」表を追加 | ✅ 反映済 |
| 4 | **(追加で発見)** §9 の「リトライメッセージ送信時にバックエンドが `{"label":"困惑"}` を emit する」が確定済み設計と矛盾。STT失敗はSSEを経由せずフロント完結 (`02_parent_container_design.md` §8・`handleSttFailure()`) | `01_screen_design.md` §9 に訂正注記 | ✅ 反映済 |

> lessons 2026-07-28 の教訓に従い、**実装着手前に機能設計書側へ反映した**。実装だけ進めて設計書に旧仕様が残ると、次の実装者が `done` に相関キーが無い前提で書いてしまう。
>
> #4 は §7 の更新作業中に「SSE に言及する箇所を全 grep する」手順で発見したもの。**差分だけを見ていたら見逃していた**類の矛盾であり、grep を機械的手順として持っている価値がここに出た。
