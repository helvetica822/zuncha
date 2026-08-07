# 実SSE / 実STT / 実録音 配線 — 設計方針・タスク分割

- 作成: 四国めたん (テックリード) / 2026-08-05
- 対象: タスク#1「実SSE/実STT/実録音配線」
- 原典: `docs/01_requirements_definition/requirements.md`、`docs/02_functional_design/01_screen_design.md` (§7 SSEイベント仕様)、`docs/04_implementation/01_implementation_plan.md` (R-3/R-4)、`docs/04_implementation/02_parent_container_design.md` §7-1 (申し送り)

---

## 1. 調査結果

### 1.1 確定していること (要件定義・設計書に明記済み)

| 項目 | 確定内容 | 出典 |
|---|---|---|
| STTエンジン | **Whisper.cpp をサーバー側でローカル動作**。クラウドSTT不使用 | requirements 技術スタック / C-01 |
| TTSエンジン | **VOICEVOX**。APIラッパー層経由で直接依存を排除、差し替え可能に | F-TTS-01/02/03、NF-MAINT-01、C-02 |
| マイク取得 | `MediaTrackConstraints` の `noiseSuppression: true` を使用 | F-STT-04 |
| 音声配信 | SSEで**一時URL** (`/audio/{ulid}`) を配信し、フロントがGET・再生後サーバー側で削除 | F-RT-01 |
| SSEイベント名 | `emotion` / `text` / `audio_url` / `done` / `error` (ペイロード形も確定) | screen_design §7 |
| 再接続 | 接続断時に**自動再接続**する | F-RT-02 |
| 会話履歴コンテキスト | 同一会話の**直近10往復(最大20メッセージ)**をLLMへ渡す | F-AI-03 |
| 感情+テキスト | **同一LLM呼び出しで同時出力** (`{text, emotion}`) | F-AI-02 |
| インフラ | Docker Compose で開発・本番統一 | NF-MAINT-02、C-04 |
| 規模 | 同時接続**最大10人**・**単一インスタンス**前提 (水平スケール非対象) | NF-SCALE-01/02 |

> R-3/R-4 で「未確定」としていたもののうち、**STT/TTSのエンジン自体は要件定義で確定済み**。残っていた3点(LLMプロバイダ・録音ガード・SvelteKit時期)は **2026-08-05 につむぎ経由でユーザー確認済み**。§1.3 参照。**本設計書に未確定事項は残っていない。**

### 1.2 実装ギャップ (現行コードの棚卸し)

**バックエンド** — インターフェースは揃っているが実装が無い、という状態:

| # | 不足 | 現状 |
|---|---|---|
| B-1 | SSEエンドポイント・`EventSink` のHTTP実装 | `internal/sse` は I/F のみ。`cmd/api/main.go:105` に TODO コメントで明示 |
| B-2 | `TextChunker` 実装 | I/F のみ |
| B-3 | `LLMClient` 実装 | I/F のみ (`ParseLLMResponse` 関数は実装済み、`ResponseParser` へのアダプタが必要) |
| B-4 | `TTSClient` (VOICEVOX) 実装 | I/F のみ |
| B-5 | Whisper.cpp 呼び出し | `internal/stt` は判定ロジック(`IsRecognitionFailed`/`IsTimedOut`)のみ。実行部・音声受付エンドポイントなし |
| B-6 | メッセージ永続化 | ~~`MessageRepository` は `GetRecentMessages` **のみ**。`InsertMessage` が無く、会話が保存されない~~ → **✅ W-01 で解消 (2026-08-05)** |
| B-7 | 音声ファイル生成の永続化 | ~~`AudioRepository` に Insert が無い / `localfs.FileStore` に `Write` が無い (Read/Deleteのみ)~~ → **✅ W-02 で解消 (2026-08-05)** |
| B-8 | プロンプト組み立て (F-AI-03) | `StreamResponse(ctx, sink, prompt string)` は文字列を受けるだけ。履歴→プロンプト変換が無い |
| B-9 | Docker Compose | **リポジトリに存在しない** (NF-MAINT-02未達) |

> **本表および下のフロントエンド表の「現状」列は 2026-08-05 時点のもの。解消済みの項目は取り消し線と ✅ で示し、部分解消はその範囲を明記する** (未解消と誤読させないため)。

**フロントエンド**:

| # | 不足 | 現状 |
|---|---|---|
| F-1 | SSE受信 | `EventSource` を使う箇所が無い。`receiveSSEEvent` はテスト/外部からの手動受口 |
| F-2 | SSEイベント契約の乖離 | `sseEventRouter.SSEEvent` は `error`/`message` の**2種のみ**。設計書§7の5種 (`emotion`/`text`チャンク/`audio_url`/`done`/`error`) と粒度が違う |
| F-3 | 録音 | `MediaRecorder`・`getUserMedia`・`noiseSuppression` の実装が無い |
| F-4 | 音声再生 | `/audio/{ulid}` の取得・再生が無い |
| F-5 | 感情画像切替 | F-EMO-02 の画像切替が未実装 (`MessageBubble` は emotion で DOM 構造を変えない設計) |
| F-6 | ルーティング | `src/routes` が無く **SvelteKit 未導入** (`package.json` は svelte + vite + vitest のみ)。`/`・`/c/{id}` が作れない |

**申し送り事項 (`02_parent_container_design.md` §7-1)**:

| # | 内容 | 本フェーズでの扱い |
|---|---|---|
| 1 | INV-6a の回帰テスト (二連打でも**送信API呼び出し回数=1**) | **必須**。`handleSubmit` が実送信を持つ本フェーズで初めて書ける |
| 2 | リクエストIDによる `done` 相関 | **本フェーズで解消**。下記 D-1 の設計で自然に満たす |
| 3 | `ModeToggle` の無変化コールバック | クリーンアップ候補。本フェーズでも見送り (Props契約への波及が大きい) |
| 4 | `startRecording`/`startTranscribing` の INV-6 適用 | **判断確定 (2026-08-05): 送信中の録音開始は「不可」**。W-16 で `if (inputState === 'sending') return;` のガードを追加する (§1.3 D-2) |

### 1.3 確定した判断 (2026-08-05・つむぎ経由でユーザー確認済み)

| ID | 決定事項 | 内容 | 影響タスク |
|---|---|---|---|
| **決-1** | LLMプロバイダ | **クラウドAPI (Claude API) を採用**。`internal/anthropic` ラッパーで局所化する (R-3 解決) | W-08 (ブロック解除) |
| **決-2** | 送信中の録音開始 | **不可**。`startRecording`/`startTranscribing` に `'sending'` ガードを追加し、INV-6 の全称記述と実装の乖離を解消する (申し送り4 解決) | W-16 |
| **決-3** | SvelteKit 導入 | **本タスク#1から分離**。実配線の完了後に別タスクで実施する。**Wave E は本タスクのスコープ外** (F-6) | W-19/W-20 |

> 決-1 により、`internal/anthropic` は `llm.LLMClient` を実装する形で TTS の VOICEVOX ラッパー (D-4) と同じ「実装は差し替え可能な外縁に置く」構造を取る。`internal/llm` の I/F は変更しない。

---

## 2. 設計方針 (技術判断)

### D-1. SSEトランスポート: 常設イベントチャネル + POST送信 + `request_id` 相関

> **実装レベルの確定仕様は `docs/04_implementation/05_sse_protocol_spec.md`**(ヘッダ・イベント形式・ハートビート・切断処理・Hub契約・二重送信の第二防衛線)。Wave B/D の実装は同書を唯一の契約とする。本節は方針とその根拠のみを述べる。

```
会話画面表示時:  GET  /conversations/{id}/events      (EventSource・常設・切断時はブラウザが自動再接続 = F-RT-02)
発話送信:        POST /conversations/{id}/messages    (body: {request_id, text}) → 202 Accepted
サーバー→クライアント: 上記チャネルへ emotion / text / audio_url / done / error を配信 (各イベントに request_id を含める)
```

- **なぜ `POST /chat` の直レスポンスでストリームしないか**: `EventSource` は GET 専用。POST で SSE を受けるには fetch + 自前パースになり、**F-RT-02 の自動再接続を手書きする**ことになる。常設チャネル方式ならブラウザ標準の再接続がそのまま効く。
- **なぜ `GET /chat?text=...` にしないか**: 発話テキストがURL・アクセスログに露出し、長文でURL長制限に当たる。
- **申し送り2 (リクエストID相関) がこれで解ける**: 全イベントに `request_id` を載せ、フロントは**自分が送信した最新の `request_id` と一致する `done` のみ**で `completeSubmission()` を呼ぶ。旧リクエストの遅延 `done` は ID 不一致で捨てられ、INV-6d の「`sending` 以外は無視」より厳密なガードになる。
- **配信ハブ**: 会話ID → `EventSink` のインメモリ registry (`internal/sse/hub.go`)。NF-SCALE-01/02 (10人・単一インスタンス) の前提下でこれが最小・十分。Redis 等の外部依存は入れない。
- **DBに残るのは会話内容**、飛行中のストリームはインメモリ。プロセス再起動で飛行中の応答は失われるが、10人社内利用では許容する (再送信で回復)。

### D-2. フロントのSSEイベント契約: 既存の純粋関数契約を壊さず、上に集約層を足す

設計書§7のイベントは「`emotion` 1発 → `text` チャンク複数 → `audio_url` → `done`」という**ストリーミング粒度**。一方 `sseEventRouter.routeSSEEvent` は「`message` = text+emotion 完結型」で、既存テスト(95件)の契約になっている。**既存契約は変更しない**。

```
EventSource
  ↓ src/lib/sseStream.ts    … 生イベント(5種)を型付き ServerEvent へパース (純粋関数 + 薄いwrapper)
  ↓ ConversationView        … emotion で新バブル開始 → text チャンクで逐次追記 → audio_url で再生 → done で完了
     └ routeSSEEvent はそのまま error 経路に使う (トースト経路の単一性 = INV-4 を維持)
```

- 設計書262行が「SSE `text` イベントによる**ストリーミング追記**」と明記しているため、チャンクをバッファして最後に1件 `appendMessage` する簡略化は**採らない**。`ConversationView` に逐次追記の受口 (`beginAssistantMessage(emotion)` / `appendAssistantChunk(text)` / `endAssistantMessage()`) を**テスト先行で**追加する。
- `receiveSSEEvent`(既存) は `error`/`message` 経路として維持 → 既存95件は無改変で緑を保つ。

### D-3. STT: 同期 `POST /stt` + whisper-server (HTTP) + サーバー側 WAV 変換

```
ブラウザ  MediaRecorder(webm/opus, noiseSuppression:true)
   ↓ multipart/form-data
POST /conversations/{id}/stt  →  ffmpeg で 16kHz mono WAV へ変換  →  whisper-server (HTTP)
   ↓ 200 {text, confidence}
IsRecognitionFailed() で判定 → 失敗なら 200 {failed:true} を返し、フロントは handleSttFailure()
```

- **SSEではなく同期レスポンス**にする理由: STT結果はテキストモードで**入力欄に反映して編集させる**必要があり、リクエスト/レスポンスの対応が本質的に1:1。SSEに載せると相関の複雑さだけが増える。
- **voice / text で経路を分けない**: 両モード共通で `POST .../stt` を叩き、**voiceモードは結果を入力欄に出さずそのまま送信フローへ、textモードは入力欄へ反映**。バックエンドの経路が1本で済む (シンプルさ優先)。
- **Whisper.cpp の連携方式は `whisper-server` (HTTP) を別コンテナ**とする。cgo バインディングはビルドの複雑さ・クロスコンパイル問題を持ち込み、C-04 (Docker Compose前提) と整合しない。プロセス都度起動 (CLI exec) はモデルロードが毎回走り NF-PERF-01 に反する。
- 音声形式変換は **API コンテナに ffmpeg を同梱**して行う。フロントで 16kHz PCM WAV を手書きエンコードする案より、標準API (`MediaRecorder`) のままにできてフロントが薄い。

### D-4. TTS: VOICEVOX ラッパー (F-TTS-02/03)

- `internal/voicevox` に HTTP クライアントを置き、`tts.TTSClient` を実装する。**`internal/tts` の I/F は変更しない** (差し替え可能性の担保 = NF-MAINT-01)。
- `Synthesize` の内部: VOICEVOX `audio_query` → `synthesis` → WAV を `localfs.Write` で保存 → `audio_files` に INSERT → `/audio/{ulid}` を返す。
- したがって **B-6/B-7 (InsertMessage / InsertAudio / FileStore.Write) は D-4 の前提タスク**。

### D-5. プロンプト組み立てと応答の永続化 (F-AI-03) — **2026-08-05 改訂**

当初案は「`ResponseStreamer.StreamResponse` の引数を `prompt string` から会話コンテキストへ拡張する」だったが、**既存の 3-2 テスト15件との契約衝突を招くだけで、得るものが無い**ため撤回する。代わりに `ResponseStreamer` を**一切変更せず**、周辺に2つの部品を足す。

| 部品 | 配置 | 役割 |
|---|---|---|
| `BuildPrompt(history []model.Message) string` | `internal/llm/prompt.go` | 純粋関数。`GetRecentMessages` の結果 (直近20件・古い順) をプロンプト文字列へ整形する。**システムプロンプト・JSON形式の強制は含めない** (それは W-08 のプロバイダ実装の責務) |
| `RecordingSink` | `internal/service/recording_sink.go` | `EventSink` のデコレータ。配信を内側の sink へ委譲しつつ、`SendEmotion` の label と `SendTextChunk` のチャンクを蓄積し、`SendDone` の**直前**に assistant メッセージを `InsertMessage` する |
| `ChatService` | `internal/service/chat.go` | オーケストレーション。保存 → 履歴取得 → プロンプト組み立て → sink 構築 → `StreamResponse` |

> **`Fanout` は常に `nil` を返すため、`ResponseStreamer` の「sink書き込み失敗→SendError に切替え中断」分岐は本番経路では到達しない** (申し送り B1-3)。既存3-2テスト15件はモック sink 上でその契約を固定しており、`Fanout` の契約を変える際は15件が守っている意味が変わることに注意する。

**なぜデコレータか**: assistant の応答テキストは `ResponseStreamer` の内部 (パース結果) にしか存在しない。永続化のために `NewResponseStreamer` へ Repository を注入すると、コンストラクタが変わり既存テスト15件が壊れる。`EventSink` を通過するイベントを外側で拾えば、`ResponseStreamer` は「配信」の単一責務のまま、永続化は別の型が担える。

**`BuildPrompt` が `userText` を引数に取らない理由**: `ChatService` はユーザー発話を**先に保存**してから `GetRecentMessages` を呼ぶため、履歴の末尾に当該発話が既に含まれる。別途渡すと二重になる。保存を先に行うのは「LLM が失敗してもユーザーの発話を失わない」ためでもある (仕様書 §3.2)。

### D-6. 状態ガード (申し送り1・4)

- 申し送り1: `handleSubmit()` が送信APIを呼ぶようになった時点で「**同一フレーム二連打でも fetch 呼び出し回数 = 1**」を検証。ガードを外して赤になることをミューテーションで実測する (lessons 2026-07-29)。
- 申し送り4: **決-2 で「送信中の録音開始は不可」と確定**したため、W-16 で `startRecording()` / `startTranscribing()` の先頭に `if (inputState === 'sending') return;` を追加する。これで INV-6 (sending不可侵) の全称記述と実装が一致し、`02_parent_container_design.md` §5 の「⚠現時点での適用範囲(既知の乖離)」注記は**解消される** (同§と§7-1-4 の記述更新も W-16 の完了条件に含める)。
  - ガードは冪等な代入のみを守るものではなく、**録音開始という観測可能な副作用**(`getUserMedia` の呼び出し)を止める。したがって INV-6a と違い「送信中に `startRecording()` を呼んでも `getUserMedia` が呼ばれない」形で回帰検知できる。ミューテーション実測まで行うこと。

### D-7. 規約・時刻

- Go は `.claude/rules/golang-coding-guideline.md` 準拠 (Handler/Service/Repository 4層・DIコンストラクタ・`%w` ラップ・ctx伝播)。
- TS/Svelte は `.claude/rules/ts-svelte-coding-guideline.md` 準拠 (型明示・`any` 禁止・`undefined` 優先)。
- 時刻は既存方針どおり **Clock 型を導入せず `now time.Time` を引数注入**。
- TDD: 各タスクは RED → GREEN → REFACTOR。外部サービス (LLM/STT/TTS) は**テストではフェイクHTTPサーバ (`httptest`)** で駆動し、実サービスへの依存をテストに持ち込まない。

---

## 3. タスク分割案

規模: S = 半日以内 / M = 1日程度 / L = 1〜2日。

### Wave A — 永続化の穴埋め (他すべての前提)

| ID | 内容 | 対象 | 依存 | 規模 |
|---|---|---|---|---|
| W-01 | `MessageRepository.InsertMessage` を I/F + postgres 実装 + integration テストで追加 (B-6) | `internal/repository`, `internal/postgres` | — | S |
| W-01b | `ConversationRepository.SetFirstText` を追加 (下記 B-10)。`UPDATE ... WHERE id=$1 AND first_text IS NULL` で**最初のユーザー発話のみ**を記録し、2回目以降は上書きしない (競合時も SQL 単体で冪等) | `internal/repository`, `internal/postgres` | — | S |
| W-02 | `AudioRepository.InsertRecord` + `FileStore.Write` を追加 (B-7)。`FetchAudio` の既存契約は不変 | `internal/repository`, `internal/postgres`, `internal/localfs` | — | S |

### Wave A レビュー(そら・2026-08-05)由来の申し送り

| # | 内容 | 判断 |
|---|---|---|
| A-1 | `TruncateFirstText` は20コードポイントで切るため、ZWJ絵文字列や結合文字の**途中で切れて末尾に孤立ZWJ/結合記号が残り得る**。DBエラーにはならず F-HIST-05 の一覧表示だけが崩れる | **許容する**。F-HIST-05 は「最大20文字、超過分は省略」であり、境界の見た目は仕様の範囲内。文字境界を意識した切り詰めは実装コストに見合わない |
| A-2 | 同関数は20ルーン以下だと入力を**そのまま返す**ため、不正UTF-8がサニタイズされずDBに届き encoding エラーになる(21ルーン以上は `[]rune` 変換で U+FFFD に置換されるため**非対称**) | **⚠ 前提が誤りだった (2026-08-05 訂正)**。下記「A-2 の訂正」参照。JSON経路では `encoding/json` が先に無害化するため、この経路は**到達不能**。ガードは多層防御として残すが、そのことを明記する |
| A-3 | `SetFirstText` を**単独で呼ぶ経路**を将来作ると、`InsertMessage` の外部キー制約による「会話の存在証明」が失われ、0件更新の冪等性が上位層のバグを隠す安全網の穴になる | 現状 `InsertMessage → SetFirstText` の順序が仕様書§3.2で確定しており到達不能。**単独呼び出し経路を追加する際はこの前提が崩れることを再確認する** |
| A-4 | `W-01-04`/`W-02-02` の「`NOW()` で補完される」検証が±3秒の範囲アサーションで、ホストとDBコンテナのクロック差に依存する(WSL2のサスペンド復帰で3秒超のドリフトが起こり得る) | **✅ 対応済 (2026-08-05)**。`helpers_test.go` の `dbNow(t, db)` で `SELECT NOW()` を before/after 両方DB側から取り、±3秒の許容幅を撤去した(ホスト時計由来の値は該当2テストから消えている) |
| A-5 | `first_text` の21文字目以降が**半角スペースのみ**の場合、Postgres は仕様上エラーにせず黙って20文字に切る(全角スペース・改行はエラー) | **✅ 対応済 (2026-08-05)**。W-01b-06 のコメントに例外を併記(元の主張は残す形)。挙動自体は許容 |

> **B-10 (追加で発見したギャップ / 部分解消)**: `conversations.first_text` カラムと `validation.TruncateFirstText` は実装済みだが、**書き込む経路がどこにも無い**。このままでは F-HIST-05 (会話一覧に最初のユーザー発話の冒頭20文字を表示) が常に空になる。
>
> - Repository (`ConversationRepository.SetFirstText`) → **✅ W-01b で解消 (2026-08-05)**
> - 呼び出し (ユーザー発話の保存時に `TruncateFirstText` を通して渡す) → **W-07 で未着手**
>
> したがって本項は**部分解消**であり、F-HIST-05 が機能するのは W-07 完了後。

#### A-2 の訂正: JSON経路では `encoding/json` が先に無害化する (2026-08-05・ずんだもんの実測)

W-06 で「不正UTF-8を `utf8.ValidString` で400にする」と指示したが、**テストが400を期待して落ちた**。原因を実測したところ、**私の前提が誤っていた**。

`encoding/json` の `Decode` は不正UTF-8バイトを **U+FFFD (置換文字) へ置換する**。したがって JSON body 経路では `req.Text` に不正UTF-8が残ることはない:

| 入力 | デコード後のバイト列 | `utf8.ValidString` |
|---|---|---|
| `{"text":"\xff\xfe"}` | `ef bf bd ef bf bd` | **true** |
| `{"text":"\x80"}` | `ef bf bd` | **true** |
| `{"text":"\ud800"}` (孤立サロゲート) | `ef bf bd` | **true** |
| JSONを通さない生バイト列 `\xff\xfe` | `ff fe` | false |

帰結:

- **JSON経路では `utf8.ValidString(req.Text)` は常に true** で、ガードは**到達不能**。
- U+FFFD は正当なUTF-8なので **DB の encoding エラーも起きない**。そらの申し送り A-2 が想定した障害は、JSON経路に限れば**発生しない**。
- 実際に効くのは **STT結果の経路**(Whisper出力を生バイトで扱う場合)のみ。W-10 で JSON 以外の経路を作る場合に初めて意味を持つ。

**確定した扱い**: ガードは**多層防御として残す**(将来 body 形式が変わった場合とSTT経路との一貫性のため)。ただし**到達不能であることをコード・テストの両方に明記**する。テストは「400になる」ではなく「**202で受理され、U+FFFD 置換された状態で保存され、`first_text` を含めて INSERT が壊れない**」を検証する実態に合わせた内容とする。

> **「嘘の期待値を書いたテストを残さない」**という判断が正しい。到達不能なガードを「守っているつもりのテスト」で覆うのは、テストの不在より有害である(lessons 2026-07-29 と同じ構図)。

#### B2-1: 中断時に `sending` が固着する問題 (ずんだもんの申し送り・2026-08-05 対応確定)

`ChatService.HandleUserMessage` が `InsertMessage`(user) 失敗や `GetRecentMessages` 失敗で中断すると、ハンドラは `_ = HandleUserMessage(...)` でログのみのため、**フロントには `done` も `error` も届かず `inputState` が `'sending'` のまま固着する**。

**確定した対応**: `HandleUserMessage` の冒頭で `Fanout` を作り、**中断する各経路で `SendError` を送ってからエラーを返す**。

- **フロント側にタイムアウトを新設しない理由**: フロントは既に **INV-3b「SSE `error` を受けたら `'sending'`/`'transcribing'` から `'editable'` へ復帰する」**(`02_parent_container_design.md` §5)を持っている。バックエンドが `SendError` を1発送れば**既存機構でそのまま解決**する。新しい機構を足すのは筋が悪い。
- これは INV-3b が「中間状態の行き止まり救済」として設計された意図そのままの用途である。

### Wave B-1 レビュー(そら・2026-08-05)由来の申し送り

| # | 内容 | 判断 |
|---|---|---|
| B1-1 | **表示用チャンクと読み上げ用テキストを同一視しない**。`SentenceChunker` の出力は `SendTextChunk`(画面へのストリーミング追記)専用で、`。。。` が3チャンクに割れる・空白のみのチャンクが出る等、TTSに渡すと不適切なものが含まれる | **現状は問題なし**。`ResponseStreamer` は `Synthesize(ctx, resp.Text)` と**全文**をTTSへ渡しており、チャンクは使っていない(`response_streamer.go:52/59`)。ただし将来チャンク単位TTSにする誘惑があるため**本表に明記して固定する**。W-09でチャンクをTTSに渡す設計変更をする場合は「空白のみ・記号のみをスキップ」が必須 |
| B1-2 | `Broadcast` が配信先件数を返さないため、**全接続が居なくなったことを検知できない**。W-07/W-09 で全員切断後もLLMとVOICEVOXを回し続け、誰も聞かない音声ファイルを生成する | **今は変更しない**(NF-SCALE-01の10人規模では実害が小さい)。ただし W-09 で TTS が実コストを持つため、**`Broadcast` が配信先件数を返す形への変更余地を残す**。W-09 着手時に「配信先0なら以降のTTSをスキップ」を検討する |
| B1-3 | `ResponseStreamer` の「sink書き込み失敗→SendErrorに切替え中断」分岐は、`Fanout` が常に `nil` を返すため**本番経路では到達しない** | **設計として意図的**。ただし将来 `Fanout` の契約を変えたときに「15件のテストが守っている契約の意味が変わった」ことに気づけないため、D-5 と本表に明記する。`EventSink` I/F に対して書かれており別 sink が来れば分岐は生きるので死コードではない |
| B1-4 | 60ルーン到達時の強制分割は文中で切れるため、読み上げが不自然になり得る | **今回は対応しない**(B1-1のとおりTTSは全文を受けるため影響なし)。チャンク単位TTSへ変更する場合は「上限到達時は直近の `、` で切る」フォールバックを検討 |

### Wave B — SSE 実配線 (バックエンド)

> Wave B の実装契約は `docs/04_implementation/05_sse_protocol_spec.md`。ヘッダ・イベント形式・`request_id` の採番元・`context.WithoutCancel` の必要性・**Conn / Hub / Fanout の三層構造と単一ライタ方式**などはすべて同書で確定済み。

| ID | 内容 | 対象 | 依存 | 規模 |
|---|---|---|---|---|
| W-03 | `EventSink` のHTTP実装 (`http.Flusher`・`event:`/`data:` 整形・`request_id` 埋め込み・書き込み失敗の伝播) (B-1) | `internal/sse/http_sink.go` | — | M |
| W-04 | 会話ID→sink のインメモリ Hub (登録/解除/配信・並行安全) | `internal/sse/hub.go` | W-03 | M |
| W-05 | `TextChunker` 実装 (句読点基準のチャンク分割。境界値・空文字・長文をテスト) (B-2) | `internal/sse/chunker.go` | — | S |
| W-06 | `GET /conversations/{id}/events` / `POST /conversations/{id}/messages` ハンドラ配線 + ULIDバリデーション + 結合テスト | `cmd/api`, `internal/handler` | W-03,W-04 | M |
| W-07 | プロンプト組み立て (`BuildPrompt`) + `RecordingSink` デコレータ + `ChatService` (B-8, D-5)。ユーザー発話/応答の `InsertMessage` 保存を含む。**`ResponseStreamer` と既存3-2テスト15件は無改変** | `internal/llm/prompt.go`, `internal/service` | W-01 | M |

### Wave C — 外部サービス実接続

| ID | 内容 | 対象 | 依存 | 規模 |
|---|---|---|---|---|
| W-08 | **Claude API** クライアント実装 (決-1)。`llm.LLMClient` を実装し、F-AI-02 の `{text, emotion}` JSON 同時出力をシステムプロンプトで強制。`ResponseParser` アダプタ・タイムアウト・APIキーは環境変数 (`ANTHROPIC_API_KEY`)・ログにキーと発話内容を出さない (NF-SEC) (B-3) | `internal/anthropic` | — | M |
| W-09 | VOICEVOX ラッパー `TTSClient` 実装 (D-4) (B-4) | `internal/voicevox` | W-02 | M |
| W-10 | Whisper.cpp クライアント + `POST .../stt` ハンドラ + ffmpeg 変換 (D-3) (B-5) | `internal/whispercpp`, `cmd/api` | — | L |
| W-11 | Docker Compose 一式 (api / postgres / voicevox / whisper-server、api イメージに ffmpeg 同梱) (B-9, NF-MAINT-02) | `docker-compose.yml`, `Dockerfile` | W-09,W-10 | M |

### Wave D — フロント実配線

| ID | 内容 | 対象 | 依存 | 規模 |
|---|---|---|---|---|
| W-12 | `sseStream.ts`: 生SSEイベント→型付き `ServerEvent` パーサ (純粋関数・不正JSON/未知イベント/欠落フィールドをテスト) (D-2) | `src/lib/sseStream.ts` | — | S |
| W-13 | `ConversationView` の逐次追記受口 (`beginAssistantMessage`/`appendAssistantChunk`/`endAssistantMessage`) をテスト先行で追加。既存95件は無改変で緑維持 (D-2) | `src/components/ConversationView.svelte` | W-12 | M |
| W-14 | `EventSource` 接続層 (onmount接続・onDestroy切断・`request_id` 照合で `done` を相関) (F-1, 申し送り2) | `src/lib/sseClient.ts`, `ConversationView` | W-12,W-13 | M |
| W-15 | 実送信 (`POST .../messages`) 配線 + **INV-6a 回帰テスト (呼び出し回数=1) + ミューテーション実測** (申し送り1) | `ConversationView` | W-14 | M |
| W-16 | 録音配線: `getUserMedia({audio:{noiseSuppression:true}})` + `MediaRecorder` + 無音タイムアウト + `POST .../stt` (F-3, F-STT-04)。**決-2 のガード追加 + `02_parent_container_design.md` §5/§7-1-4 の乖離注記の解消**を含む | `src/lib/recorder.ts`, `ConversationView` | W-10 | L |
| W-17 | 音声再生 (`audio_url` → GET → 再生。再生完了/失敗の扱い) (F-4) | `src/lib/audioPlayer.ts` | W-14 | S |
| W-18 | 感情画像切替 (F-EMO-02) | `src/components/*` | W-13 | M |

### Wave E — 画面統合 (**決-3 により本タスク#1のスコープ外・後続タスクへ繰り越し**)

| ID | 内容 | 対象 | 依存 | 規模 |
|---|---|---|---|---|
| W-19 | SvelteKit 導入 + `/`・`/c/{id}`・`?view=history` ルーティング (F-6)。既存 vitest 設定を壊さないことが必須条件 | `src/routes`, 設定一式 | 後続タスク | L |
| W-20 | 会話一覧 (F-HIST-02/03/05) の API + 画面 | 全体 | W-19 | M |

### 推奨投入順序

1. **Wave A** (W-01/W-02) — 依存ゼロ、並行可。ここが空いていると Wave B/C が書けない。
2. **Wave B** (W-03→W-04→W-06、W-05/W-07 は並行可) — バックエンドSSEの骨格。
3. **Wave C と Wave D 前半を並行** — W-12/W-13 はバックエンド完成前でも書ける (純粋関数 + 受口)。
4. **W-11 (Compose)** を Wave C 後半に置き、実サービス結合の動作確認をここで初めて行う。
5. **Wave E は本タスク#1のスコープ外** (決-3)。実配線の完了後に別タスクとして実施する (実配線とルーティング刷新を同時にやると失敗の切り分けが困難)。

---

## 4. 確定事項とその根拠 (2026-08-05 ユーザー確認済み)

> 本節は当初「確認事項 (Q-1〜Q-3)」だったが、**3件すべて確定したため決定事項として確定版に書き換えた**。以降に未確定事項はない。要約表は §1.3。

### 決-1. LLM は クラウドAPI (Claude API) を採用 — R-3 解決

要件定義・設計書に記載が無く R-3 として未解決だったが、**C-01 (STTはクラウド不使用) に相当する制約が LLM には課されていない**ことを確認し、クラウドAPI採用が可となった。

- 実装は `internal/anthropic` に閉じ込め、`llm.LLMClient` を実装する。`internal/llm` の I/F・`ParseLLMResponse` は変更しない (差し替え可能性の担保)。
- F-AI-02 の `{text, emotion}` 同時出力を強制する。逸脱は既存の `ParseLLMResponse` のセンチネル (`ErrSyntax`/`ErrSchema`/`ErrValue`) で吸収し、**emotion が7種外/空なら「困惑」フォールバック**という既存の実装済み挙動をそのまま活かす。
- APIキーは環境変数 `ANTHROPIC_API_KEY`。**キーと発話内容をログに出さない** (NF-SEC-01 の趣旨・ガイドライン10.2)。
- テストは `httptest` のフェイクサーバで駆動し、**実APIをテストから呼ばない**。

#### 決-1a. W-08 の実装前提 (公式リファレンスで確認済・2026-08-05)

Go SDK `github.com/anthropics/anthropic-sdk-go` を `go.mod` に追加する (現在未依存)。以下は記憶で書くと間違える箇所なので確定事項として記録する。

| 項目 | 確定内容 | 根拠・理由 |
|---|---|---|
| モデルID | **`claude-opus-5`** を素の文字列で渡す (`Model: "claude-opus-5"`)。日付サフィックスを付けない | `anthropic.Model` は `string` エイリアス。Opus 5 の型付き定数は未提供の可能性があるため、定数の存在を前提にしない |
| thinking | **無効化しない**。既定で有効なので `Thinking` を未設定のまま使い、`output_config.effort` を **`low`** にして待ち時間を抑える | Opus 5 で thinking を無効化すると `<thinking>` タグが可視応答へ漏れる既知の失敗モードがある。本アプリは応答テキストを**そのままVOICEVOXが読み上げ画面に出す**ため、漏洩は即座にユーザー体験の破壊になる。公式にも「無効化より低effortを推奨」と明記 |
| 温度パラメータ | **`temperature`/`top_p`/`top_k` は使用不可** (Opus 5 では 400) | 「ずんだもんらしさ」は**システムプロンプトで作る**。素朴に温度を上げようとすると失敗する |
| システムプロンプト | `System: []anthropic.TextBlockParam{{Text: ..., CacheControl: anthropic.NewCacheControlEphemeralParam()}}` | ずんだもんのペルソナ指示は毎回不変 = キャッシュ対象。会話履歴は `Messages` 側 (揮発) に置くので、**安定→揮発の順序が自然に守られる**。Opus 5 の最小キャッシュ長は512トークン |
| `MaxTokens` | 16000 程度。**小さく絞らない** | Opus 5 では `MaxTokens` が thinking と応答テキストの**合計**上限。絞ると応答が途中で切れる。課金は実使用量なので大きめで害はない |
| リトライ | **`option.WithMaxRetries` を明示的に 1 以下へ**。既定は2 | 既定のままだと最悪 `タイムアウト×3` の実時間を消費し、W-06 の60秒予算をLLMだけで食い潰してTTSに回らない |
| タイムアウト | `option.WithRequestTimeout(30*time.Second)` 程度 | ハンドラ側60秒のうちTTS・DB保存の余地を残す |
| エラー処理 | `errors.As` で `*anthropic.Error` を取り出し `StatusCode` で分岐 (429/5xx は再試行可、4xx は不可) | Go SDK は全非2xxを単一のエラー型で返す |
| 拒否応答 | **`StopReason` を `Content` の読み出し前に検査**する。拒否時は `content` が空または部分的 | Opus 5 は安全性分類器で拒否し得る (HTTP 200 + `stop_reason: "refusal"`)。`Content[0]` を無条件に読む実装は落ちる。拒否は `llm` のセンチネルエラーとして返し、既存の `ResponseStreamer.fail` 経由で `SendError` に落とす |

**JSON強制の方式 (要実測)**: 公式には構造化出力 (`output_config.format` の `json_schema`) が Opus 5 で利用可能で、プロンプトでの指示より確実。ただし**Go SDK での正確なバインディング名はスキルの Go 版ドキュメントに記載がない**。推測で書かずに、SDK追加後の**コンパイルエラーを手掛かりに確定**すること (公式が静的型付け言語で推奨している手順)。

- 構造化出力が使えた場合: スキーマで `{text, emotion}` を強制。`ParseLLMResponse` はそのまま2重の防壁として残す。
- ピン留めした SDK バージョンで使えない場合: **フォールバックはシステムプロンプトでのJSON指示**。`ParseLLMResponse` (25ケースでテスト済み) が既に逸脱を吸収する設計なので、これで機能要件は満たせる。**どちらを採ったかを実装報告に明記すること。**

### 決-2. 送信中の録音開始は「不可」 — 申し送り4 解決

`InputState` の単一列挙で表現でき、「送信中はマイク無効」という UI が自然。「可」とすると状態の直交化・飛行中リクエストのキャンセル設計が必要で規模が跳ねる。

- W-16 で `startRecording()` / `startTranscribing()` に `if (inputState === 'sending') return;` を追加する。
- `02_parent_container_design.md` の §5 INV-6「⚠現時点での適用範囲(既知の乖離)」と §7-1-4 は、この確定により**役割を終える**。W-16 の完了条件として両箇所を更新し、乖離注記を残さないこと (lessons 2026-07-28: 追記だけで終わらせず旧記述を grep で同期する)。

### 決-3. SvelteKit 導入は本タスクから分離

要件定義はフロントを SvelteKit としているが、現状は svelte + vite + vitest のみで `src/routes` が無く、`/c/{conversationID}` (F-HIST-04) は導入なしには実現できない。**実配線とルーティング刷新を同時に行うと失敗の切り分けが困難**なため、Wave E (W-19/W-20) を後続タスクへ繰り越す。

- 本タスク#1 の動作証明は、SvelteKit 抜きの検証手段 (Vite dev サーバ上のマウント + 手動の会話ID指定) で行う。§5 の完了条件はこれを前提とする。

---

## 5. 本フェーズの完了条件 (Wave A〜D。Wave E は決-3 により対象外)

- `go test ./tests/...` 全緑 (integration がスキップされていないことをログで確認)。
- `npx vitest run` 全緑 (既存95件を**無改変で**維持 + 新規テスト)。
- `gofmt` / `go vet` / `tsc` が EXIT 0。
- **ミューテーション実測 2件** (lessons 2026-07-29 のチェックリスト3点セット):
  - 申し送り1 (W-15): 二連打で送信APIの呼び出し回数=1。ガードを外すと赤になることを実測。
  - 決-2 (W-16): 送信中の `startRecording()` で `getUserMedia` が呼ばれない。ガードを外すと赤になることを実測。
- **文書同期**: `02_parent_container_design.md` §5 INV-6 の乖離注記・§7-1 の申し送り1/2/4 が、解消済みの記述へ更新されていること (`grep -n "既知の乖離\|要プロダクト判断\|要検討" docs/04_implementation/02_parent_container_design.md` で確認)。
- `docker compose up` で api / postgres / voicevox / whisper-server が起動し、**実際に1往復の音声会話が成立する** (動作の証明)。SvelteKit 未導入のため、検証は Vite dev サーバ上に `ConversationView` をマウントし会話IDを手動指定する形で行う (決-3)。
