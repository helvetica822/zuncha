# GREEN phase 実装方針・タスク分割表

- 作成者: 四国めたん (テックリード)
- 作成日: 2026-07-17
- 対象: zuncha GREEN phase (プロダクションコード実装)
- 前提: RED phase 完了済み。テストケース総数 232 件 (フェーズ1:77 / フェーズ2:43 / フェーズ3:52 / フェーズ4:60)、すべて「未実施」。
- 原典: `tasks/instructions_metan_01.md`、`docs/01〜03` 各設計書・テスト仕様書、`tests/` 配下の実テストコード。

---

## 0. 本計画の最重要前提 (ディレクトリ配置の確定)

指示書は `src/backend/` / `src/frontend/` 配下への配置を例示しているが、**既存の `go.mod`・`tsconfig.json`・`vitest.config.ts`・実テストコードのインポートパスと矛盾する**。テストコードは RED phase で確定済みの「実装契約」であり、これを変更するとテストの意味が壊れる。したがって本計画では **テストのインポート契約を正** とし、以下の配置を採用する。

### バックエンド (Go)
- `go.mod` はリポジトリルートに存在し `module zuncha` (go 1.22)。
- テストは `zuncha/internal/...` をインポートしている (例: `zuncha/internal/validation`)。
- Go のモジュール解決上、`internal/` は **`go.mod` と同じ階層 = リポジトリルート直下** に置く必要がある。
- `tests/` もルート直下にあり同一モジュールに属す。**`go.mod` を `src/backend/` へ移動すると `tests/` がモジュール外になり全テストが解決不能になる**ため、移動は不可。
- **結論: バックエンドコードはルート直下 `internal/`・`cmd/` に配置する** (`src/backend/` は使わない)。

### フロントエンド (Svelte/TS)
- テストは `../../../src/components/*.svelte`・`../../../src/lib/*` を参照 (`tests/unit/frontend/` から見てリポジトリルートの `src/`)。
- `tsconfig.json` / `vitest.config.ts` の alias は `$lib → src/lib`、`$components → src/components`。
- **結論: フロントエンドコードはルート直下 `src/lib/`・`src/components/` に配置する** (`src/frontend/` は使わない)。

> この逸脱はテスト契約遵守のための必然。つむぎ・めたん間で合意の上、以降のタスクはこの配置を前提とする。

---

## 1. 実装方針

### 1.1 バックエンド (Go) レイヤー構成・パッケージ構成

レイヤー分離は `.claude/rules/golang-coding-guideline.md` の Handler / Service / Repository / Model 4層に準拠する。テストが要求するパッケージは以下 (すべて新規作成)。

```
zuncha/                       (module root, go.mod 既存)
├── cmd/
│   └── api/
│       └── main.go           # エントリポイント (HTTP サーバ起動・DI 配線)。テスト対象外だが GREEN 完了時に用意
├── internal/
│   ├── model/                # ドメインモデル (Conversation / Message / AudioFile 構造体)
│   ├── validation/           # 純粋バリデーション (ULID / first_text / role / emotion / input trim)
│   ├── gc/                   # 有効期限判定 (IsExpired)
│   ├── repository/           # Repository インターフェース群 + 純粋関数 (ReverseMessages) + センチネルエラー
│   ├── postgres/             # Repository の PostgreSQL 実装 (integration テスト対象)
│   ├── localfs/              # FileStore のローカルファイル実装
│   ├── llm/                  # LLM 応答パース・型・センチネルエラー・LLMClient/ResponseParser I/F
│   ├── stt/                  # STT 失敗判定・タイムアウト判定
│   ├── tts/                  # TTSClient インターフェース
│   ├── sse/                  # EventSink / TextChunker インターフェース
│   └── service/              # オーケストレーション (CreateConversation / FetchAudio / ResponseStreamer)
└── tests/                    # 既存 (unit / integration)
```

**依存方向**: `service → repository(I/F)・llm・tts・sse・validation・gc・model`、`postgres/localfs → repository(I/F)・model`。上位が下位のインターフェースにのみ依存し、実装 (postgres/localfs) は DI で注入する。

**時刻の扱い (Clock 注入方針)**: 過去の設計決定に従い、Clock インターフェース型は導入しない。時刻依存を排除すべき判定関数・DB 操作は **`now time.Time` を引数注入** する (`gc.IsExpired(expiresAt, now)`、`ConversationRepository.GC(ctx, now)`)。サービス層 (`CreateConversationService`) が内部で `time.Now()` を取得して渡す。SQL は `WHERE expires_at < $1` とし、DB の `NOW()` に依存しない (境界値テストの決定性確保のため)。

**エラー設計**: ドメイン固有エラーはセンチネル (`repository.ErrAudioNotFound`、`llm.ErrSyntax/ErrSchema/ErrValue`) を定義し、`fmt.Errorf("...: %w", err)` でラップ、上位は `errors.Is` で判定する。

### 1.2 フロントエンド (Svelte/TS) 構成

`.claude/rules/ts-svelte-coding-guideline.md` に準拠。ロジックは純粋関数として `src/lib/` に切り出し、UI は薄い `src/components/` に保つ (テスト容易性のため純粋関数とコンポーネントを分離)。

```
src/
├── lib/
│   ├── inputMode.ts       # InputMode 型・localStorage 永続化 (getStored/setStored/isInputMode)
│   ├── sendButton.ts      # InputState 型・isSendButtonDisabled 判定
│   └── sseEventRouter.ts  # SSEEvent/Emotion 型・routeSSEEvent・DEFAULT_ERROR_MESSAGE
└── components/
    ├── ModeToggle.svelte   # 入力モード切替 (Segmented Control)
    ├── SendButton.svelte   # 送信ボタン
    ├── Toast.svelte        # エラートースト (自動消滅)
    └── MessageBubble.svelte # ずんだもん応答バブル
```

### 1.3 テストとのインポート整合

| 項目 | 設定 | 状態 |
|------|------|------|
| Go module 名 | `zuncha` (`go.mod`) | 既存・変更不要 |
| Go テストインポート | `zuncha/internal/<pkg>` | `internal/` をルート直下に作れば解決 |
| TS alias | `$lib`→`src/lib`, `$components`→`src/components` (`tsconfig.json`・`vitest.config.ts`) | 既存・変更不要 |
| TS テストインポート | 相対パス `../../../src/...` | `src/` をルート直下に作れば解決 |
| jest-dom マッチャー | `vitest-setup.ts` + `vitest.config.ts` の `setupFiles` | 既存 (フェーズ4 QA 指摘①対応済み)・変更不要 |

### 1.4 使用ライブラリとバージョン方針

**バックエンド (`go.mod` 既存依存で充足、追加は最小限)**:
- `github.com/oklog/ulid/v2 v2.1.0` — ULID 生成 (往復整合性テスト用)。
- `github.com/lib/pq v1.10.9` — PostgreSQL ドライバ (integration の blank import)。
- `github.com/stretchr/testify v1.9.0` — assert / require / mock (テスト専用、実装は依存しない)。
- JSON パースは標準 `encoding/json`。testify/suite は**使用しない** (実テストは標準 `testing` + 表駆動 + `t.Run` 構成)。
- LLM/STT/TTS の外部 SDK は GREEN phase (ユニット) では不要 (すべてインターフェース + モック検証)。実接続実装は後続フェーズ。

**フロントエンド (`package.json` 既存依存で充足)**:
- `svelte ^4.2.0` / `typescript ^5.4.0` / `vite ^5.2.0`。
- テスト: `vitest ^1.6.0` / `@testing-library/svelte ^5.1.0` / `@testing-library/jest-dom ^6.4.0` / `jsdom ^24.0.0`。
- **新規ライブラリ追加は原則なし** (フェーズ4 は純粋ロジック + 薄い UI のみ)。

### 1.5 コーディング規約

- Go: `.claude/rules/golang-coding-guideline.md` に準拠 (`gofmt`・命名・エラーラップ・コンテキスト伝播・DI コンストラクタパターン)。
- TS/Svelte: `.claude/rules/ts-svelte-coding-guideline.md` に準拠 (型明示・`any` 禁止・`undefined` 優先・script/markup/style 順序・props 型定義)。
- TDD: `.claude/rules/tdd-comprehensive.md` に準拠。GREEN phase は「テストを通す最小実装」を厳守し、リファクタは REFACTOR フェーズで行う。

---

## 2. タスク分割表

規模感: **S** = 半日以内 / **M** = 1日程度 / **L** = 1〜2日。
実装順序は「純粋関数 → モデル → インターフェース → サービス → 実装層 → フロント」の順で依存を解消する。

### 前提タスク

| ID | タスク名・作業内容 | 対象 | 対応テスト | 依存 | 規模 |
|----|------|------|-----------|------|------|
| T-00 | ディレクトリ骨組み作成。`internal/` 各パッケージ・`src/lib`・`src/components` の空ディレクトリと go パッケージ宣言を用意。`go test ./tests/... -run xxx` がビルドエラーで落ちる状態を確認 (RED の再確認) | `internal/*`, `src/*` | (全体の土台) | なし | S |
| T-DB | integration テスト用 DB スキーマ準備。`conversations`/`messages`/`audio_files` テーブル・CASCADE 外部キー・`expires_at` 生成列・各種インデックスを作る migration を用意し、`ZUNCHA_TEST_DATABASE_URL` を指す DB に適用 (`02_database_design.md` 準拠)。※未設定時 integration は `t.Skip` される | `migrations/` | フェーズ2 integration 全件の前提 | なし | M |

### フェーズ1 (純粋ロジック・モック不要 / 77 件)

| ID | タスク名・作業内容 | 対象 | 対応テスト | 依存 | 規模 |
|----|------|------|-----------|------|------|
| T-01 | ULID 形式バリデーション。`IsValidULID(s string) bool` (26文字・Crockford Base32、I/L/O/U 除外・大文字のみ・trim/全角/絵文字不可)。往復整合性テスト対応で ULID 生成も確認 (`oklog/ulid`) | `internal/validation/ulid.go` | 1-1 (`ulid_test.go` 約20件) | T-00 | S |
| T-02 | first_text ルーンカット。`TruncateFirstText(s string) string` (コードポイント単位で先頭20文字、trim/サニタイズしない) | `internal/validation/first_text.go` | 1-2 (`first_text_test.go` 11件) | T-00 | S |
| T-03 | role/emotion バリデーション。`ValidateRole`/`ValidateEmotion(*string)`/`ValidateRoleEmotionConsistency`。emotion は7種完全一致・nil 可、user+emotion 非nil は矛盾エラー | `internal/validation/role_emotion.go` | 1-3 (`role_emotion_test.go` 約24件) | T-00 | S |
| T-04 | 入力 trim 判定。`IsValidInput(s string) bool` (標準 trim 対象=半角/タブ/改行/全角スペース U+3000。ゼロ幅スペース U+200B・絵文字は非空扱い) | `internal/validation/input.go` | 1-4 (`input_validation_test.go` 14件) | T-00 | S |
| T-05 | 有効期限判定。`IsExpired(expiresAt, now time.Time) bool` (`expiresAt.Before(now)` 相当・等号非対称・ゼロ値 true・絶対時刻比較で TZ 非依存) | `internal/gc/expiration.go` | 1-5 (`gc_expiration_test.go` 約7件) | T-00 | S |

### フェーズ2 (DB連携・オーケストレーション / 43 件)

| ID | タスク名・作業内容 | 対象 | 対応テスト | 依存 | 規模 |
|----|------|------|-----------|------|------|
| T-06 | ドメインモデル定義。`Conversation`/`Message`/`AudioFile` 構造体 (nullable は `*T`) | `internal/model/*.go` | フェーズ2 全般の前提 | T-00 | S |
| T-07 | Repository インターフェース群 + 純粋関数 + センチネルエラー。`ConversationRepository`(GC/Insert)・`MessageRepository`(GetRecentMessages)・`AudioRepository`(GetByULID/UpdateFetchedAt/DeleteRecord)・`ReverseMessages([]Message) []Message`(空でも非nil)・`ErrAudioNotFound`。※**2026-08-05: Wave A (W-01/W-01b/W-02) で `MessageRepository.InsertMessage`・`ConversationRepository.SetFirstText`・`AudioRepository.InsertRecord` を追加済み**。**FileStore インターフェースの配置先を決定** (下記リスク R-2 参照) | `internal/repository/*.go` | 2-2 unit (`reverse_messages_test.go` 4件)・各 I/F | T-06 | M |
| T-08 | CreateConversation サービス。`NewCreateConversationService(repo)`・`CreateConversation(ctx)` (内部順序: `time.Now()`→`GC(now)`→`InsertConversation`。GC エラーは握り潰す・毎回 ULID 新規採番) | `internal/service/create_conversation.go` | 2-1 unit (`create_conversation_service_test.go` 7件) | T-07 | M |
| T-09 | FetchAudio サービス。`NewFetchAudioService(repo, files)`・`FetchAudio(ctx, ulid)` (順序: GetByULID→Read→UpdateFetchedAt→Delete→DeleteRecord。途中失敗で中断しエラー返却) | `internal/service/fetch_audio.go` | 2-3 unit (`fetch_audio_service_test.go` 8件) | T-07 | M |
| T-10 | PostgreSQL Repository 実装。`NewConversationRepository/NewMessageRepository/NewAudioRepository(db *sql.DB)`。GC は `WHERE expires_at < $1` + CASCADE、GetRecentMessages は `ORDER BY created_at DESC, id DESC LIMIT 20` を古い順に整列、Update/Delete は0件でもエラーにしない (冪等) | `internal/postgres/*.go` | 2-1/2-2/2-3 integration (24件) | T-07, T-DB | L |
| T-11 | FileStore ローカル実装。`NewFileStore()` (引数なし)・`Read(path)`/`Delete(path)`。存在しないパスはエラー。※**2026-08-05: Wave A (W-02) で `Write(path, data)` を追加済み**(親ディレクトリを自動作成・既存は上書き)。`service.FileStore` I/F には**含めない** | `internal/localfs/filestore.go` | 2-3 integration の実ファイル削除検証 | T-07 | S |

### フェーズ3 (LLM/STT/SSE ロジック・Goバックエンド / 52 件)

| ID | タスク名・作業内容 | 対象 | 対応テスト | 依存 | 規模 |
|----|------|------|-----------|------|------|
| T-12 | LLM 応答パース。`LLMResponse{Text,Emotion}`・`ParseLLMResponse(body []byte)`・センチネル `ErrSyntax/ErrSchema/ErrValue`(階層化・`errors.Is` 判定)・`LLMClient`/`ResponseParser` I/F。emotion 7種外/空は「困惑」フォールバック、text=null/emotion数値型は ErrValue、Markdown コードブロックは ErrSyntax | `internal/llm/*.go` | 3-1 (`parse_llm_response_test.go` 25件) | T-00 | M |
| T-13 | STT 判定。`STTResult{Text,Confidence}`・`const SttConfidenceThreshold=0.5`・`IsRecognitionFailed`(空/trim後空白/conf<0.5 で true、trim は `validation.IsValidInput` 再利用)・`IsTimedOut(silenceStart,now,threshold)`(経過>=threshold で true、8秒ちょうど true) | `internal/stt/*.go` | 3-3 (`stt_judgment_test.go` 12件) | T-04 | S |
| T-14 | SSE / TTS インターフェース。`EventSink`(SendEmotion/SendTextChunk/SendAudioURL/SendDone/SendError)・`TextChunker`(Chunk)・`TTSClient`(Synthesize(ctx,text)→url) | `internal/sse/*.go`, `internal/tts/*.go` | 3-2 の前提 | T-00 | S |
| T-15 | ResponseStreamer オーケストレーション。`NewResponseStreamer(llmClient, parser, ttsClient, chunker)`・`StreamResponse(ctx, sink, prompt)`。送出順: LLM生成→パース→防御的 `ValidateEmotion`→SendEmotion→チャンク毎 SendTextChunk→TTS(成功時のみ SendAudioURL、失敗はスキップし error にしない)→SendDone。各ステップ失敗 (EventSink 書込失敗含む) は SendError に切替え中断 | `internal/service/response_streamer.go` | 3-2 (`response_streamer_test.go` 15件) | T-03, T-12, T-14 | M |

### フェーズ4 (フロントエンド / 60 件、TC-4-1-26 欠番)

| ID | タスク名・作業内容 | 対象 | 対応テスト | 依存 | 規模 |
|----|------|------|-----------|------|------|
| T-16 | 入力モード永続化。`InputMode`型・`INPUT_MODE_STORAGE_KEY`・`isInputMode`(正規化しない)・`getStoredInputMode`(無効値/例外時 'voice')・`setStoredInputMode`(例外握り潰し) | `src/lib/inputMode.ts` | フェーズ4 入力モード純粋関数 (約19件) | T-00 | S |
| T-17 | 送信ボタン判定。`InputState`型・`isSendButtonDisabled({mode,text,inputState})` (editable 以外は常に true、voice+editable は空でも false、text+editable は trim 後空で true。全角/タブ/改行対応、文字数上限なし) | `src/lib/sendButton.ts` | 送信ボタン純粋関数 (約16件) | T-16 | S |
| T-18 | SSE イベントルーティング。`Emotion`(7種)・`SSEEvent`(error/message)・`SSEEventHandlers`・`DEFAULT_ERROR_MESSAGE`・`routeSSEEvent`。error は message 欠落/空で既定文言フォールバック、message は onMessage のみ、両者排他 | `src/lib/sseEventRouter.ts` | error_routing の routeSSEEvent (7件) | T-00 | S |
| T-19 | Svelte コンポーネント群。`ModeToggle`(mode/isRecording/isTranscribing/onModeChange。入力欄を持たない・aria-pressed・録音/認識中は disabled)・`SendButton`(disabled/onSubmit)・`Toast`(message/durationMs=3000・role=alert・onDestroy で clearTimeout・多重表示なし)・`MessageBubble`(text/emotion・emotion で DOM 構造を変えない) | `src/components/*.svelte` | 各コンポーネントテスト (input_mode 6・send_button 3・error_routing 8) | T-16,T-17,T-18 | M |

### 推奨実装順序と理由

1. **T-00 / T-DB (土台)** — 全タスクの前提。DB スキーマは integration がスキップされないための準備。
2. **フェーズ1 (T-01〜T-05)** — 依存ゼロの純粋関数。最速で緑化でき、後続 (STT/emotion 検証) が再利用する。並行実装可。
3. **T-06 (model) → T-07 (I/F・純粋関数)** — フェーズ2 サービス/実装層すべての土台。
4. **サービス層 (T-08/T-09) と 実装層 (T-10/T-11)** — T-07 完了後は並行可能。unit(モック) が先に緑化でき、integration は DB 準備後。
5. **フェーズ3 (T-12〜T-15)** — T-13 は T-04、T-15 は T-03/T-12/T-14 に依存。llm/stt/sse の下地を先に。
6. **フェーズ4 (T-16〜T-19)** — バックエンドと独立。純粋関数 (T-16〜T-18) を先に緑化し、最後にコンポーネント (T-19)。フロントは別担当と並行可能。

> フェーズ1・フェーズ4 前半 (純粋関数) は相互依存が薄く、着手初期に並行投入すると早期に多数の緑を確保できる。フェーズ2 integration (T-10) が唯一の重量級 (L) かつ DB 依存のため、T-DB を早めに片付けること。

---

## 3. リスク・懸念事項

| ID | 区分 | 内容 | 対応方針 |
|----|------|------|---------|
| R-1 | 配置の逸脱 | 指示書の `src/backend/`・`src/frontend/` は既存 go.mod・tsconfig・テストのインポート契約と矛盾。誤って従うと全テストが解決不能になる | 本計画 0章の通り `internal/`・`src/` をルート直下に配置。つむぎと合意済みとする |
| R-2 | I/F 配置未確定 | `FetchAudioService` が受ける FileStore インターフェースの所属パッケージが不明瞭。テストのインポート実態には `internal/filestore` が**現れない** (unit のモックは Read/Delete のみ、明示アサーションなし)。`localfs.NewFileStore()` は実装を返す | 実装着手時に `fetch_audio_service_test.go`・`localfs` の実インポートを確認し、I/F を `internal/repository` (他 Repo I/F と同居) もしくは consumer 側 `internal/service` に置くか確定。`internal/filestore` 新設はテストが要求する場合のみ |
| R-3 | LLM プロバイダ未定 | 設計書に LLM プロバイダ/モデルの記載なし。ただし GREEN(ユニット) は `LLMClient` インターフェース + モックで完結し、実装フェーズの緑化には影響しない | GREEN phase では I/F 抽象のみ実装。実接続は後続フェーズで別途決定 (SDK 追加を伴う) |
| R-4 | STT/TTS 連携方式未定 | Whisper.cpp の Go 連携方式、VOICEVOX ラッパー具体 I/F、STT のストリーミング可否が未確定 | R-3 同様、GREEN(ユニット) はインターフェース化で吸収。実接続は後続 |
| R-5 | integration の DB 前提 | integration テストはスキーマを自前で作らず、事前マイグレーション済み DB (`ZUNCHA_TEST_DATABASE_URL`) を前提。未設定だと `t.Skip` で「緑」に見えるが実検証されない | T-DB を必須先行タスク化。CI/ローカルで環境変数を設定し、スキップされていないことを実行ログで確認する |
| R-6 | 生成列の実装差異 | `expires_at` は `GENERATED ALWAYS AS (...) STORED`。DB バージョン・INSERT 時の列指定方法により挙動差が出得る | T-DB のスキーマで生成列を正しく定義。`InsertConversation` は `expires_at` を明示指定しない (DB 生成に委ねる) |
| R-7 | フェーズ4 偽陽性 | TC-4-3-10 (Toast アンマウント時タイマークリア) は Svelte が破棄後警告を出さないため、未クリア実装でも偽陽性でパスし得る (QA 申し送り) | GREEN phase では `onDestroy` で確実に `clearTimeout` を実装。将来テスト強化 (`vi.spyOn(global,'clearTimeout')`) を `00_minutes.md` 議題11 へ申し送り継続 |
| R-8 | jest-dom 前提 | コンポーネント層テストは `vitest-setup.ts` の jest-dom 登録に依存 (QA 指摘①対応済み) | 既存スカフォールド (`vitest-setup.ts`・`vitest.config.ts` の setupFiles) を変更しない。T-19 着手前に実在を再確認済み |
| R-9 | ModeToggle 責務 | QA 指摘②により ModeToggle は入力欄を持たない。実装者が誤って textbox を持たせると TC-4-1-26 欠番の整理と矛盾 | T-19 で ModeToggle は mode/isRecording/isTranscribing/onModeChange のみ。入力欄クリアは親責務 (今回スコープ外) |

---

## 4. 検証方針 (GREEN 完了の定義)

- バックエンド unit: `go test ./tests/unit/... -v` が全緑。
- バックエンド integration: `ZUNCHA_TEST_DATABASE_URL` 設定下で `go test ./tests/integration/... -v` が全緑 (**スキップされていないこと**をログで確認)。
- フロントエンド: `npm test` (`vitest run`) が全緑 (TC-4-1-26 欠番を除く)。
- 各タスクは「対応テストが緑化し、かつ最小実装であること」をもって完了とする。過剰実装 (テストのない機能) は行わない。
- 完了時に `gofmt`・型チェック (`tsc`) を通す。

<!-- PLAN_COMPLETE -->
