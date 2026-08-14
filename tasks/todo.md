# zuncha GREEN phase (プロダクションコード実装) — 実装計画・スケジュール

作成: 春日部つむぎ (PM) / 2026-07-17
※ 旧内容(RED phase 引き継ぎメモ)は全フェーズ完了済みのため本内容に更新。旧版はgit履歴(コミット b78885c 時点)を参照。

## チーム体制 (tmuxセッション `cc_c`)

| ペイン | 担当 | 役割 |
|---|---|---|
| 0 | 春日部つむぎ | PM / Team Lead (進捗管理・指示出し) |
| 1 | USAGE MONITOR | (チームメイト割り当て禁止) |
| 2 | ずんだもん | 実装担当 (めたんの指示後にのみ着手) |
| 3 | 雨晴はう | テスト実施担当 |
| 4 | 九州そら | レビュアー / 品質管理 |
| 5 | 四国めたん | テックリード (技術判断・実装指示) |

## 進行フロー (タスクごとに繰り返す)

1. つむぎがタスクを指定 → 2. めたんが実装方針を詳細化しずんだもんに指示 → 3. ずんだもんが実装・完了報告 → 4. そらがレビュー (✅で次へ / ⚠・❌はずんだもん修正→再レビュー) → 5. はうがテスト実行・報告 (合格で次タスクへ / 不合格はずんだもん修正→ステップ4へ)

## 参照資料

- 実装計画書 (めたん作成・承認済み): `docs/04_implementation/01_implementation_plan.md`
- テスト仕様: `docs/03_unit_test/` (テストケース232件、全件「未実施」)
- テストコード: `tests/unit/`・`tests/integration/`・`tests/unit/frontend/`

## ✅ ユーザー確認事項 (解決済み)

- [x] **R-1: ソースコード配置のチームルールとの矛盾** → ユーザー承認済み(2026-07-17)。**めたん案(テスト契約準拠)を採用**。バックエンドはルート直下 `internal/`・`cmd/`、フロントエンドはルート直下 `src/lib/`・`src/components/` に配置する。チームルールの `src/backend/`・`src/frontend/` は不採用。

## スケジュール表 (Wave = 依存関係で区切った実装順序)

規模: S=半日以内 / M=1日程度 / L=1〜2日。タスク管理ツールに #3〜#23 として登録済み(依存関係設定済み)。

### Wave 0: 土台 (Day 1 午前)
- [ ] T-00 ディレクトリ骨組み作成 (S) — タスク#3
- [ ] T-DB integrationテスト用DBスキーマ準備 (M) — タスク#4 ※T-10の前提。早めに着手 (並行可)

### Wave 1: 純粋関数の早期緑化 (Day 1〜2) ※全て並行可能
- [ ] T-01 ULIDバリデーション (S) — #5
- [ ] T-02 first_textルーンカット (S) — #6
- [ ] T-03 role/emotionバリデーション (S) — #7
- [ ] T-04 入力trim判定 (S) — #8
- [ ] T-05 有効期限判定 (S) — #9
- [ ] T-06 ドメインモデル定義 (S) — #10
- [ ] T-16 入力モード永続化 FE (S) — #20
- [ ] T-18 SSEイベントルーティング FE (S) — #22

### Wave 2: インターフェース層 (Day 2)
- [ ] T-07 Repository I/F + ReverseMessages (M) — #11 ※リスクR-2(FileStore I/F配置)をここで確定
- [ ] T-12 LLM応答パース (M) — #16
- [ ] T-14 SSE/TTSインターフェース (S) — #18
- [ ] T-13 STT判定 (S) — #17 (T-04後)
- [ ] T-17 送信ボタン判定 FE (S) — #21 (T-16後)

### Wave 3: サービス層・実装層 (Day 3〜4)
- [ ] T-08 CreateConversationサービス (M) — #12
- [ ] T-09 FetchAudioサービス (M) — #13
- [ ] T-11 FileStoreローカル実装 (S) — #15
- [ ] T-10 PostgreSQL Repository実装 (L) — #14 ★唯一の重量級。T-DB完了必須。遅延リスク最大 (要ウォッチ)
- [ ] T-15 ResponseStreamer (M) — #19 (T-03/T-12/T-14後)

### Wave 4: フロントエンドコンポーネント・総仕上げ (Day 4〜5)
- [ ] T-19 Svelteコンポーネント群 (M) — #23 (R-7 Toastタイマー / R-9 ModeToggle責務に注意)
- [ ] 全体緑化確認: `go test ./tests/unit/...` / `go test ./tests/integration/...` (スキップされてないことをログ確認・R-5) / `npm test` / `gofmt`・`tsc`

## 開発環境セットアップ (2026-07-21 つむぎが構築済み)

- Go 1.22.10: `~/.local/bin/go`(symlink) 経由で利用可能。
- psql 16.14: `~/.local/bin/psql`(ラッパー) 経由で利用可能。
- テスト用PostgreSQL: Dockerコンテナ`zuncha-test-pg`(ポート55432)で稼働中。
- integrationテスト実行時は毎回 `source scripts/test_env.sh` してから `go test ./tests/integration/...` を実行すること(環境変数はシェル呼び出しをまたいで引き継がれないため)。
- 詳細な構築手順は `tasks/lessons.md`(2026-07-21のエントリ)参照。

## 遅延リスクと監視ポイント

- **T-10 (L・DB依存)**: 春日部の夕方の渋滞ポイント。T-DBを先行させ、integration がskipで偽緑にならないよう実行ログを必ず確認 (R-5)。
- **R-2 (FileStore I/F配置)**: T-07着手時に実テストのインポートを確認して確定。めたん判断。
- **R-3/R-4 (LLM/STT/TTS実接続)**: GREEN phaseはI/F+モックで完結。実接続は本計画のスコープ外 (後続フェーズ)。

## 完了の定義 (GREEN phase)

- テストケース232件が全緑 (integrationはskipなしで)。各タスクは「対応テスト緑化 + 最小実装」で完了。過剰実装禁止。

## 完了サマリー

### GREEN phase 最終実装レビュー結果 (2026-07-22 記録: めたん)

※タスクボードの揮発対策として、承認記録をテキストで永続化。

**sora による最終実装レビュー: 全13タスク ✅承認**(差し戻し⚠/❌なし)。
めたんが技術判断のうえ承認、該当タスクを completed 化済み。

| タスク | 内容 | 判定 |
|---|---|---|
| T-00b (#5) | バックエンド契約スキャフォールド | ✅ |
| T-06 (#11) | ドメインモデル定義 | ✅ |
| T-07 (#12) | Repository I/F + ReverseMessages (R-2: FileStore I/Fをservice消費側配置=妥当) | ✅ |
| T-08 (#13) | CreateConversationサービス | ✅ |
| T-09 (#14) | FetchAudioサービス | ✅ |
| T-10 (#15) | PostgreSQL Repository実装 (★最重点・SQL全プレースホルダ/CASCADE委譲/冪等/ctx伝播/%wラップ確認) | ✅ |
| T-11 (#16) | FileStoreローカル実装 | ✅ |
| T-12 (#17) | LLM応答パース | ✅ |
| T-13 (#18) | STT判定 | ✅ |
| T-14 (#19) | SSE/TTSインターフェース定義 | ✅ |
| T-15 (#20) | ResponseStreamerオーケストレーション | ✅ |
| T-18 (#23) | SSEイベントルーティング (FE, R-7 onDestroy+clearTimeout確認) | ✅ |
| T-19 (#24) | Svelteコンポーネント群 (FE, R-9 ModeToggle責務分離確認) | ✅ |

**実機再検証 (sora実行)**:
- `go test ./tests/unit/...` = 全緑
- `source scripts/test_env.sh` 後 integration = 全緑。skipされず実DB接続で TC-2-1〜2-3 全24件PASS (R-5偽緑なし確認)
- `npx vitest run tests/unit/frontend/` = 60件PASS / `gofmt -l` = 差分なし / `go vet` = 警告なし

**申し送り (差し戻し非該当・任意対応、据え置き)**:
1. T-10 (conversation.go:27): `RowsAffected()` のエラーのみ `%w` 未ラップ。lib/pqで実害なし、一貫性観点のみ。
2. T-19: MessageBubble `emotion` prop 未使用(将来クラス切替用)/ ModeToggle `selected` がmount後のmode prop変更に非追従(現テスト対象外)。

**残タスク (フェーズ完全完了の唯一の残件)** → 解消済み:
- `npx tsc --noEmit` が未使用import (`tests/unit/frontend/error_routing.test.ts:3` の `fireEvent`) で1件エラー。ずんだもんが除去、つむぎが実機で `tsc --noEmit` EXIT 0 を確認済み。

## GREEN phase 完全完了 (2026-07-23 つむぎ最終確認)

以下をつむぎが実機で最終検証し、全緑を確認:
- `npx tsc --noEmit` → EXIT 0 (エラーなし)
- `npx vitest run tests/unit/frontend/` → 60/60 passed
- `go test ./tests/unit/...` → ok
- `source scripts/test_env.sh && go test ./tests/integration/... -v` → ok (skipなし、実DB接続で全件PASS確認)

テストケース232件+FE60件、全件通過。設計書・テスト仕様に基づく実装は全タスク(T-00〜T-20)完了。GREEN phaseはこれにて正式クローズとする。

---

# REFACTOR phase 計画 (2026-07-23 めたん策定 / タスク#2)

## 大原則

- **挙動を変えない**。全テスト(unit 232 / FE 60 / integration 実DB)を緑に保ったまま進める。各タスク完了ごとに該当スイートを実行し緑を確認。
- **影響最小化・シンプルさ優先**(CLAUDE.md コア原則)。「テストが契約として固定している挙動」は変えない。挙動変更を伴うもの・自動テストの安全網が無いものは REFACTOR ではなく後続フェーズへ。
- 進行フロー(実装=ずんだもん / レビュー=そら / テスト=はう)は GREEN phase と同じ。completed 化はPM(つむぎ)の✅後。

## 調査サマリ (Explore 2件 + テスト契約確認済み)

- 対象コードは小規模・層分離良好。命名は概ね規約準拠。
- テスト契約の確認で判明した重要事実:
  - `stt.SttConfidenceThreshold` の参照は `judgment.go` 内のみ → リネーム安全。
  - ModeToggle の楽観的更新(クリックで `aria-pressed` 反転)は TC-4-1-21/22 が検証 → **`selected` の mode 追従化は緑を壊す**。
  - inputMode の4シンボルはテストが単一パス(`src/lib/inputMode`)からimport → 永続化層分離は再export必須。

## タスク分割

### Tier 1 — 低リスク純粋リファクタ (実施推奨)

- [x] **R-B1 (Go/申し送り1)**: `internal/postgres/conversation.go:27` の `res.RowsAffected()` を `fmt.Errorf("gc rows affected: %w", err)` でラップ。postgres pkg内で唯一の非一貫箇所を解消。
- [x] **R-A1 (Go定数化)**: `postgres/message.go` の `LIMIT 20` → `const recentMessagesLimit = 20`。`cmd/api/main.go` の `10s`×2 → `readHeaderTimeout`/`shutdownTimeout`、`"8080"` → `defaultPort`、env名(`ZUNCHA_DATABASE_URL`/`PORT`)・Content-Type(`audio/wav`/`application/json`) を const 化。
- [x] **R-A2 (Go現代化)**: `cmd/api/main.go` の `interface{}` → `any`。
- [x] **R-C1 (Go命名)**: `SttConfidenceThreshold` → `STTConfidenceThreshold`(略語統一・guideline 2.2)。参照は同ファイル内のみ。
- [x] **R-E1 (FE定数化)**: `Toast.svelte` の `durationMs = 3000` デフォルトを `DEFAULT_TOAST_DURATION_MS`($lib)へ。
- [x] **R-E2 (FE定数化)**: `inputMode.ts` に `DEFAULT_INPUT_MODE: InputMode = 'voice'` を導入し内部の `'voice'` 重複を解消。**公開シグネチャ・挙動・import面は不変**(既存4シンボルはそのままexport維持)。
- [x] **R-E3 (FE定数化)**: 表示文字列を定数化 — `MessageBubble` の `data-role="assistant"`、`SendButton` の「送信」、`ModeToggle` の「マイク/テキスト」。※ボタン名はテストの `getByRole({name})` が参照するため**文字列値は変更しない**(定数の値=現行文字列)。
- [x] **R-E4 (FE命名)**: `ModeToggle.svelte` の内部関数 `select` → `selectMode`。DOM経由テストのため非影響。

### Tier 2 — 中規模・挙動保存 (実施推奨、要レビュー丁寧め)

- [x] **R-D1 (Go重複排除)**: `llm/parser.go:36-50` の text/emotion 解析ブロック(isJSONNull→Unmarshal→ErrValue)を `parseRequiredString` ヘルパへ抽出。
- [x] **R-D2 (Go重複排除)**: 感情ラベル集合とフォールバック値(`"困惑"`)の真実の源泉を `validation/role_emotion.go` に一元化し、`llm/parser.go` はそれを参照。二重メンテ解消。

## REFACTOR phase 完了サマリー (2026-07-23)

Tier1/Tier2 全10件(R-B1〜R-D2)完了。**そら ✅承認・差し戻しなし**。

三重で緑確認:
- めたん実地検証: `go test ./tests/unit/...`=ok / `npx vitest run`=60/60 / `gofmt -l`差分なし / `go vet`無警告 / `go build`OK / `npx tsc --noEmit`OK
- integration: つむぎが実DB接続で全緑確認(skipなし)
- そらレビュー: 挙動不変・値不変(8080/各10s/CT/0.5/3000/表示文字列)・%wはerrors.Is互換・STTリネーム旧識別子残存なし・R-D2はllm→validation片方向で循環なし・R-E2はexport面追加のみ

据え置き(Tier3・対象外の判断どおり現状維持を確認): 申し送り2a(MessageBubble emotion prop未使用)・2b(ModeToggle selected非追従)。いずれも挙動変更/機能追加につき後続フェーズ。

### Tier 3 — 今回スコープ外を推奨 (team-lead判断待ち)

- **申し送り2b (ModeToggle mode追従)**: 現テストが楽観的更新を仕様固定。追従化は挙動変更+新規テスト要 → **REFACTOR対象外**。親画面(mode制御側)の設計時に扱う。
- **申し送り2a (MessageBubble emotion 実消費)**: 未使用propのクラス切替消費は機能追加(挙動変更) → 後続フェーズ。propは現状維持。
- **Go handler層切り出し**: `cmd/api/main.go` の handler/JSON応答ヘルパを `internal/handler` へ分離+`main`分割。アーキ改善だが handler の自動テストが無く安全網なし → 影響最小化原則で今回据え置き推奨。実施するなら独立タスク+手動起動確認をセットで。
- **inputMode 永続化層分離**: 再export barrier 必須で現規模では過剰(シンプルさ優先) → 据え置き。

## 完了の定義 (REFACTOR phase)

- 採用 Tier の全タスク実装後、`go test ./tests/unit/...`・`source scripts/test_env.sh && go test ./tests/integration/...`(skipなし)・`npx vitest run tests/unit/frontend/`・`npx tsc --noEmit`・`gofmt -l`・`go vet` が全て緑/差分なし。
- そらのレビュー✅を経て、つむぎが completed 化。

## REFACTOR phase 完全完了 (2026-07-23 つむぎ最終確認)

**R-B1・R-A1・R-A2・R-C1(Go Tier1)、R-E1〜R-E4(FE Tier1)、R-D1・R-D2(Go Tier2)、全10件実装完了・そら✅承認(差し戻しなし)。**

三重検証(ずんだもん実装時・めたん実地再検証・つむぎ最終確認)で以下すべて緑:
- `gofmt -l .` → 差分なし
- `go vet ./...` → EXIT 0
- `go build ./...` → EXIT 0
- `go test ./tests/unit/... -count=1` → ok
- `npx tsc --noEmit` → EXIT 0
- `npx vitest run tests/unit/frontend/` → 60/60 passed
- `source scripts/test_env.sh && go test ./tests/integration/... -count=1 -v` → 全件PASS(skipなし)

Tier3(申し送り2件・handler層切り出し・inputMode永続化層分離)は挙動変更を伴う/安全網なしのため今回スコープ外、据え置きを維持。

REFACTOR phaseはこれにて正式クローズとする。

---

# 結合テスト phase 計画 (2026-07-23 めたん策定 / タスク#17)

## 対象と根拠

`docs/03_unit_test/01_test_plan.md` 8章スコープ外のうち「結合テストで検証」とされた2件 + phase4申し送りCC の計3件。
NF-SCALE-01(同時接続10)・NF-PERF-01(過負荷)は負荷テスト、外部サービス(LLM/STT/TTS)実接続はスコープ外。全テスト緑維持が大前提。

## 現状調査サマリ

- **CORS**: `cmd/api` に CORS ミドルウェア実装は**皆無**(grep確認)。ルータは `net/http.ServeMux`(Go1.22パターン)。要件 NF-SEC-02=「社内ドメインのみ許可」、C-08=IP制限は運用側担保。**許可オリジンの具体値は設計書に未定義**(=構成判断)。
- **グレースフル停止**: `main()` に `signal.Notify`+`httpServer.Shutdown(ctx, 10s)` 実装済み。GREEN phaseで手動スモーク確認済み。ただし `main()` は一枚岩で、サーバ構築/起動/停止が関数分離されておらず**自動テストのフックが無い**(REFACTOR Tier3で据え置いた「main分割/setupServer抽出」が前提になる)。
- **フロント3機能競合(CC)**: `src/` に入力欄を保持する**親/コンテナコンポーネントは未設計・未実装**。ModeToggle/SendButton/Toast/MessageBubble/sseEventRouter は個別に存在するのみ。CCは docs(13章)で「結合/E2Eフェーズ対応」と決定済み。競合の主体(旧モード入力クリア×モード切替×エラートースト)を統合する親が無ければテスト対象が存在しない。

## タスク分割(案)

### IT-1: CORS設定 (NF-SEC-02) — 実装+結合テスト 【実施推奨・独立着手可】
- 実装: `cmd/api` に `corsMiddleware` を追加し `mux` をラップ。許可オリジンは**環境変数によるallowlist**(例 `ZUNCHA_ALLOWED_ORIGINS`、カンマ区切り)で供給。リクエストの `Origin` がallowlistに一致した時のみ `Access-Control-Allow-Origin` に当該オリジンを反映。プリフライト(OPTIONS)に対応。
- テスト: `tests/integration/` に `cors_test.go` を新設。`httptest` で mux+middleware を構築(**DB不要=skipなし**)。①許可オリジン→ヘッダ反映、②非許可オリジン→ヘッダ無し/拒否、③OPTIONSプリフライト→期待ステータス+Allow-Methods/Headers、④Originヘッダ無し→素通し。
- 要確認 Q1: 許可する「社内ドメイン」の具体値と供給方法(環境変数allowlistで可か)、credentials(`Allow-Credentials`)の要否。

### IT-2: 単一インスタンス動作 (NF-SCALE-02) — 自動化の是非を判断 【要判断】
- 論点: 「単一インスタンス動作」の検証価値は主にグレースフル停止時の**in-flightリクエストの完遂**と**新規接続の遮断**。これを決定的に自動テスト化するには `main()` からサーバ構築/起動/停止を関数抽出する小リファクタが前提。
- 選択肢A(価値ある部分だけ薄く自動化): `setupServer()`/`run(ctx)` を抽出し、実ポート起動→処理中リクエストの最中に `Shutdown`→当該リクエストが正常完了しつつ新規接続が拒否されることを結合テスト化。main分割はREFACTOR Tier3据え置き分と重なる。
- 選択肢B(自動化しない): 単一インスタンス動作は本質的にプロセス運用の話。手動スモーク手順を `docs/` に文書化するに留め、自動テストは設けない(決定的テストの費用対効果が低い)。
- **めたん推奨**: B基本 + Aの「Shutdownのin-flight drain」だけ薄く自動化。要 Q2 判断。

### IT-3: フロント3機能競合 (CC) — 親コンテナ設計が前提 【要判断・規模大】
- CCは「結合テスト単体」では完結しない。(a)入力欄を持つ**親コンテナの仕様策定**→(b)実装(TDD RED/GREEN)→(c)競合の結合/E2Eテスト、の一連が必要。
- テストハーネス: vitest(@testing-library/svelte)による親コンポーネント結合テスト、または Playwright によるE2E。競合は非同期タイミング依存のため fake timers / `act` 制御で決定化。
- **めたん推奨**: これは新規フィーチャ設計に近く「結合テスト計画」の枠を超える。**親コンテナ設計タスクを先行**させ、その完了後に競合テストを設計・実装。今回の結合テスト計画では対象・方針・前提・ハーネス選定の定義に留める。要 Q3 判断。

## 依存関係

- IT-1: 独立、即着手可。
- IT-2(選択肢A): main分割リファクタに依存。
- IT-3: 親コンテナ設計→実装に依存。

## 要確認事項(PM/ユーザー) → ✅全て承認済み(2026-07-23 ユーザー確認)

- **Q1(CORS)** → ✅ 環境変数allowlist方式(`ZUNCHA_ALLOWED_ORIGINS`)で承認。実ドメイン未確定につきデプロイ時に環境変数設定できる実装に。テストはダミードメインで許可/非許可分岐を検証。**credentials不要**。
- **Q2(単一インスタンス)** → ✅ B基本(手動スモーク文書化)+ A最小自動化で承認。**setupServer抽出等の大掛かりなリファクタは不要**、必要最小限の自動化のみ。
- **Q3(フロント競合)** → ✅ 親コンテナの設計・実装を**先行タスク化**で承認。まず親コンテナ設計(めたん)→実装→競合テストの順。

## 完了の定義(結合テスト phase)

- 採用したITの実装+テストが緑。新規Goテストは `source scripts/test_env.sh` 後 `go test ./tests/integration/...` でskipなし実行。CORSテストはDB非依存。既存の全テスト(unit232/FE60/integration)も緑維持。

## IT-1(CORS) 完了 (2026-07-23)

三重確認(ずんだもん実装→めたん実地検証→そら✅承認、差し戻しなし)で完了。
- 実装: `internal/middleware/cors.go`(環境変数allowlist方式、credentials非対応)+ `cmd/api/main.go` 配線。
- テスト: `tests/integration/cors_test.go` 5ケース(許可/非許可GET・許可/非許可OPTIONS・Origin無し)、DB非依存・skipなし全PASS。
- そら任意提案(`parseAllowedOrigins`境界ユニット未カバー)はIT-2作業と合わせて`main_test.go`で拾う予定。
- 検証: `gofmt -l`差分なし・`go vet`無警告・`go build`OK・既存unit/integration緑維持。

## IT-2(単一インスタンス) 完了 (2026-07-23)

三重確認(ずんだもん実装→めたん実地検証→そら✅承認、差し戻しなし)で完了。
- 実装: `internal/httpserver/httpserver.go`(`Run(ctx, srv, ln, shutdownTimeout)`抽出)+ `cmd/api/main.go`最小改修(`signal.NotifyContext`でSIGINT/SIGTERM→ctx連携)。大掛かりなmain分割は不要のまま。
- テスト: `tests/integration/graceful_shutdown_test.go`(channel同期で決定的・in-flight完遂+新規接続拒否を検証、`-count=10`でフレークなし確認)。`cmd/api/main_test.go`に`TestParseAllowedOrigins`7ケース(そら任意提案分・境界値網羅)を追加。
- 手動スモーク文書: `docs/04_implementation/03_single_instance_smoke.md`(自動化範囲外の停止ログ・多重起動不可・実DB疎通を手動確認手順として明記)。
- **要認識(そら指摘・据え置き)**: Shutdownエラー時の挙動が`log.Printf`(非致命)→`log.Fatalf`(致命)に変化。異常系を握りつぶさず顕在化する方針は本プロジェクト一貫の設計(errors.Isラップ徹底等)と整合するため、つむぎ判断で**このまま採用**。
- 検証: `gofmt -l`差分なし・`go vet`無警告・`go build`OK・`go test ./cmd/...`=ok・unit=ok・integration=ok(skipなし、CORS5件+graceful shutdown含め全PASS)。

## 結合テスト phase 完全完了 (2026-07-23 つむぎ最終確認)

つむぎが実機で最終グリーンスイープを実施、全緑を確認:
- `gofmt -l` → 差分なし / `go vet ./...` → 無警告 / `go build ./...` → OK
- `go test ./tests/unit/...` → ok / `go test ./cmd/...` → ok
- `source scripts/test_env.sh && go test ./tests/integration/... -v` → 全件PASS(skipなし、CORS5件+graceful shutdown1件含む)
- `npx tsc --noEmit` → EXIT 0 / `npx vitest run tests/unit/frontend/` → 60/60 passed

IT-1(CORS)・IT-2(単一インスタンス)は実装+そら✅承認で完了。IT-3(親コンテナ)は設計完了、実装(ConversationView本体+CC競合テスト)は後続フェーズへ。結合テストphaseはこれにて正式クローズとする。

## 親コンテナ(ConversationView)実装 phase 着手 (2026-07-28)

設計書: `docs/04_implementation/02_parent_container_design.md`(§8確定済み)。
実装指示書: `tasks/instructions_zundamon_conversation_view.md`(めたん発行・仕様の正)。

- [ ] RED: `tests/unit/frontend/conversation_view.test.ts` に TC-P-01〜TC-P-19b を記述し、失敗を確認(ずんだもん)
- [ ] GREEN: `src/components/ConversationView.svelte` 実装 + `MessageBubble.svelte` に `data-emotion` 追加(ずんだもん)
- [ ] 検証: `npx vitest run tests/unit/frontend/`(既存60件込み全緑)・`npx tsc --noEmit` EXIT 0(めたん実地検証)
- [ ] そらレビュー → つむぎが `completed` 化

### めたん判断(設計書§3の補完・記録)
- 初期 `inputState = 'editable'`。`'idle'` 初期化は「textモード復帰時に一度モード切替するまで送信不能」という死に状態を生むため。実録音パイプライン配線時に `'idle'` を再導入する。
- SSE `done` は `sseEventRouter.ts` に追加せず、コンテナの `completeSubmission()` で表現。既存TC-4-3-xxの契約を壊さないため。
- `MessageBubble` の `emotion` prop は現状DOMに出ておらず検証不能なので `data-emotion` を追加(Props契約は不変・追加のみ)。

### GREEN完了 (2026-07-28) — めたん実地検証済み

- [x] RED: TC-P-01〜19bの20ケース記述・失敗確認(ずんだもん) → めたんが実地でRED再現を確認、判断3点を承認、INV-5検証1点の追加を指示
- [x] GREEN: `ConversationView.svelte` 実装 + `MessageBubble.svelte` に `data-emotion` 追加(ずんだもん)
- [x] 検証(めたん実地): FE **80/80 passed**(既存60 + 新規20) / `npx tsc --noEmit` EXIT 0 / `gofmt -l` 差分なし・`go vet` 無警告・`go build` OK・`go test ./tests/unit/... ./cmd/...` ok
- [x] そらレビュー → つむぎが `completed` 化

#### 途中で発生した仕様衝突と決着(A案採用)
`data-emotion` 追加により既存 TC-4-3-15(`container.innerHTML` のバイト一致で「同一コンポーネントパス」を検証)がFAIL。仕様書299行のThen欄は「同一のコンポーネントパスで表示」までしか要求しておらず、バイト一致は**過剰実装**と判定。`data-emotion` のみ正規化除外して意図を維持し、除外した属性値自体を別assertで検証する形に修正。テストIDは増やさず(仕様書§5の60件集計を動かさないため)、`14_test_specification.md` 299行のThen欄に注記を追記。教訓は `tasks/lessons.md` に記録済み。

## 親コンテナ(ConversationView)実装 完全完了 (2026-07-28)

そら1回目レビューで⚠要修正(トースト自動消滅後、同一文言のSSEエラー再来で再表示されない)。根本原因は「可視状態を子(Toast)が持ち値を親が持つ構成で、同値のprops代入は何も伝播しない」こと。`toastSeq`単調増加カウンタ+`{#key toastSeq}`再マウント方式で解決(Toast.svelteは無変更、既存契約・テスト保持)。

そら2回目レビューで⚠要修正2件: ①設計書・指示書に旧記述(「message変更で再表示」)が残存し次の実装者が同じバグを再現する懸念、②INV-4が明記する「タイマー再張り」が未テスト。めたんがgrepで矛盾箇所を全て特定・6箇所同期、TC-P-20d(タイマー延長の境界テスト、RED確認込み)を追加して解消。

そら再レビューで**✅承認・指摘なし**。つむぎが最終独立検証(vitest84/84・tsc EXIT0・矛盾記述残存0件・gofmt/vet/build OK)を実施し、`completed`化。

教訓は`tasks/lessons.md`(2026-07-28エントリ、「可視状態を子が持ち値を親が持つ構成では同値のprops代入は何も伝播しない」)に記録済み。

## 状態競合6症状の診断・修正 完全完了 (2026-07-29)

はうの重点診断(連続操作/二重送信・キャンセル/最新結果優先・矛盾状態組み合わせ・トーストライフサイクル)で6症状を実証発見:
1. sending中モード切替可能(重篤・二重送信経路) 2. 遅延doneでユーザー入力消失(重篤) 3. 送信中の遅延STT結果がsending破壊(中) 4. transcribing中のSSEエラーで中間状態固着(中・INV-3の穴) 5. 二連打の冪等ガード欠如(軽) 6. 同一モード再クリックでテキスト消失(軽〜中)

めたんが「sending不可侵」の単一原則に体系化、設計書§5にINV-6a〜d・INV-3b・INV-1bを追記。ずんだもんが7箇所実装(全て`ConversationView.svelte`)。はうがTC-P-21〜26(11件、1テスト1振る舞いに分割)を追加、修正前再現コンポーネントで9/11の回帰検知力を実証。

**めたんが独自にミューテーションテストを実施**(95/95緑は「テストが通る」証明であって「ガードを守っている」証明ではないと看破)、7ガード中INV-6aのみ回帰検知不能と判明(handleSubmitの副作用が冪等なため)。はうの独自申し送りと一致し、クロス検証成立。ガードは維持しつつ設計書に限界を明記、実SSE配線フェーズでの「送信API呼び出し回数=1」テスト追加を申し送り。

そら1回目レビューで⚠要修正(文書のみ): 設計書§4が旧記述のまま・§6/§7の参照/数値乖離。めたんが§4に警告追加・全遷移にガード条件をインライン明記・曖昧語「必要に応じ」を撲滅(症状4の直接原因と特定)・§6/§7を整合。再レビューで**✅承認**(軽微な申し送り1件: startRecording/startTranscribingの未ガード、DOM未配線で実害なし、を後続申し送り)。

つむぎが最終独立検証(vitest95/95・tsc EXIT0・gofmt/vet/build OK・設計書反映確認)を実施し、`completed`化。

教訓は`tasks/lessons.md`に記録済み(「文書に追記すると、追記した本人には文書全体が更新されたように見える」という認知バイアスと、その対策となる5項目のチェックリスト)。

## 実SSE/実STT/実録音配線 — 設計方針・タスク分割 (2026-08-05, めたん)

方針書: `docs/04_implementation/04_realtime_wiring_design.md`

- [x] 要件定義書・設計書・実装計画書R-3/R-4・申し送り§7-1の再確認
- [x] 現行コードの実装ギャップ棚卸し(バックエンド9件・フロント6件)
- [x] 技術判断の確定(D-1〜D-7) — SSEトランスポート/イベント契約/STT経路/TTSラッパー/プロンプト/ガード
- [x] タスク分割案(W-01〜W-20、Wave A〜E)
- [ ] **ユーザー確認3件**: Q-1 LLMプロバイダ選定 / Q-2 送信中の録音可否(申し送り4) / Q-3 SvelteKit導入時期
- [ ] Wave A 着手(W-01 InsertMessage / W-02 InsertRecord+FileStore.Write) ※Q-1非依存で着手可

**確定した主要判断**
- SSEは「常設 `GET /conversations/{id}/events` (EventSource・自動再接続=F-RT-02) + `POST /conversations/{id}/messages`」。全イベントに `request_id` を載せることで申し送り2(遅延`done`の誤完了)を構造的に解消。
- 既存 `sseEventRouter` の契約(error/message)は**変更しない**。上に `sseStream.ts` を足し、`ConversationView` に逐次追記の受口を新設(既存95件を無改変で維持)。
- STTは同期 `POST .../stt`(multipart)。Whisper.cppは `whisper-server`(HTTP)を別コンテナ、cgo/CLI都度起動は不採用。音声変換はAPIコンテナ同梱のffmpeg。
- Docker Compose が**未整備**(NF-MAINT-02未達)であることを発見 → W-11 として明示。
- 申し送り1(送信API呼び出し回数=1)はW-15で、ミューテーション実測込みで解消する。

### 確定事項 (2026-08-05・つむぎ経由でユーザー確認済み)

- [x] **決-1 LLM**: クラウドAPI(Claude API)を採用。`internal/anthropic` ラッパーで局所化、`internal/llm` の I/F は不変。W-08 のブロック解除。
- [x] **決-2 送信中の録音開始**: **不可**。W-16 で `startRecording`/`startTranscribing` に `'sending'` ガードを追加し、`02_parent_container_design.md` §5/§7-1-4 の「既知の乖離」注記を解消する(申し送り4クローズ)。
- [x] **決-3 SvelteKit**: 本タスクから分離。**Wave E(W-19/W-20)はスコープ外**、実配線完了後に別タスク化。
- [x] 設計書の Q-1〜Q-3 参照9箇所を確定版へ同期(grepで旧記述0件を確認、2026-07-28の教訓の手順を適用)
- [x] **B-10 追加発見**: `conversations.first_text` に書き込む経路が皆無でF-HIST-05が常に空になる穴 → W-01b としてWave Aに追加
- [x] Wave A 実装指示書作成: `tasks/instructions_zundamon_wave_a.md`(W-01/W-01b/W-02)、ずんだもんへ送付済
- [ ] Wave A 実装(ずんだもん) → めたんレビュー観点整理 → そらレビュー依頼

**Wave Aで判明した既存テストへの影響**: `tests/unit/create_conversation_service_test.go:35`・`fetch_audio_service_test.go:40` に `var _ repository.Xxx = (*mock)(nil)` のコンパイル時アサーションがあり、I/F拡張はモックへのメソッド追加が不可避。既存のテストケース・期待値は不変とする方針を指示書に明記。
- [x] SSEワイヤ仕様の確定: `docs/04_implementation/05_sse_protocol_spec.md`(Wave B/Dの唯一の実装契約)。設計書D-1・Wave B節から参照を張り、`grep -rn "05_sse_protocol_spec" docs/` が非0件であることを確認済
- [ ] **【要ユーザー確認】`01_screen_design.md` §6/§7 の同期**: `done` に `request_id` を追加(現状「—」)、新エンドポイント3本(`/events`・`/messages`・`/stt`)の追記。つむぎへ確認依頼済
- [x] **`01_screen_design.md` §6/§7 の同期完了**(2026-08-05・ユーザー承認済): §7表に`request_id`追加(`done`は「—」→`{"request_id"}`)、§7.1(相関の必要性・クライアント採番の理由)・§7.2(接続方式)・§7.3(エンドポイント一覧)を新設、§6に「画面内で使用するAPI」表を追加
- [x] **§9の矛盾記述を訂正**: 「リトライメッセージ送信時にバックエンドが`{"label":"困惑"}`をemitする」は確定済み設計(STT失敗はフロント完結・`02_parent_container_design.md`§8)と矛盾。grep手順で発見し訂正注記を追加
- [x] Wave B-1 実装指示書作成: `tasks/instructions_zundamon_wave_b1.md`(W-03 HTTPConn+Fanout / W-04 Hub / W-05 SentenceChunker)。依存ゼロのためWave A完了を待たず着手可
- [x] **Hub契約の自己修正**: 当初案`Broadcast(convID, func(EventSink) error)`は`request_id`の注入点が定まらない欠陥があったため、Conn/Hub/Fanoutの三層構造+単一ライタ方式(バッファ付きチャネル+書き込みgoroutine1本)へ改訂。設計書側の旧記述もgrepで同期

### Wave A 実装完了・めたん独立検証済 (2026-08-05)

ずんだもんがW-01/W-01b/W-02を実装、新規テスト33件。**めたんが独立検証**:
- `go test ./tests/... -count=1 -v` → **239 PASS / 0 FAIL / 0 SKIP**(サブテスト含む)。integration実行時間3.1s(Skipなら0.0s)
- **ミューテーションをめたん自身で1件再現**: `AND first_text IS NULL` 除去 → W-01b-02「2回目は上書きされない」・W-01b-04「空文字列も上書きされない」がFAIL → 復元後全緑。ずんだもんの実測と一致(検証用一時ファイルは削除済み)
- 指示チェックリストと全項目突合 → **抜けなし**。外部キー違反をmessage_id/conversation_idの2ケースに分割しており「1テスト1振る舞い」も遵守
- 既存テスト差分はモックスタブ追加のみ(`callOrder`行と`error_routing.test.ts`はセッション開始時点で既に`M`=前フェーズの変更)
- `gofmt -l .` 空 / `go vet ./...` OK / `go build` OK / `npx vitest run` 95 passed(フロント回帰なし)

**ずんだもんの相談への回答**: ファイルローカルな読み取りヘルパー3つ(`queryMessageRow`/`queryAudioRow`/`seedConversationAndMessage`)の追加は**問題なし**。§2.3「新しいヘルパーを作らない」の意図は「`helpers_test.go`のsetup系を再発明せず流用せよ」であり、追加禁止ではない。指示の書き方が曖昧だった点はめたんの反省事項(意図まで書く)。

- [x] Wave A 実装・独立検証
- [ ] そらレビュー(観点6件で依頼済: ①`NOW()`依存と実装計画59行の方針衝突の有無 ②`TruncateFirstText`とVARCHAR(20)の文字数定義の一致 ③モックスタブの将来の偽陽性 ④0件更新の冪等性が上位層バグを隠さないか ⑤`fetched_at`無視仕様の驚き ⑥文書の旧記述残存)
- [ ] Wave B-1(W-03/W-04/W-05)実装中(ずんだもん)
- [x] **D-5 方針を改訂(2026-08-05)**: 当初案「`StreamResponse`の引数を会話コンテキストへ拡張」は既存3-2テスト15件を壊すだけで得るものがないため**撤回**。`ResponseStreamer`無改変のまま、周辺に`BuildPrompt`(純粋関数)・`RecordingSink`(EventSinkデコレータで応答を蓄積しdone直前に永続化)・`ChatService`(オーケストレーション)の3部品を足す形へ。設計書のW-07行もgrepで同期
- [x] Wave B-2 実装指示書作成: `tasks/instructions_zundamon_wave_b2.md`(W-06 ハンドラ配線 / W-07 BuildPrompt+RecordingSink+ChatService)。B-1完了後に着手
  - ハンドラは`internal/handler`を新設し既存2つも移設。**移設可否は実測で判断**(`main_test.go`は`parseAllowedOrigins`のみ検証、cors/gracefulテストは独自mux=無影響)
  - `context.WithoutCancel`の罠に対し「202返却後も応答生成が完走する」テストとミューテーション実測を完了条件に明記
- [x] **W-08(Claude API実接続)の実装前提を公式リファレンスで確定**(設計書 決-1a)。記憶で書くと間違える箇所を確定事項化:
  - モデルIDは`claude-opus-5`を素の文字列で(型付き定数の存在を前提にしない)
  - **thinkingを無効化しない**(無効化すると`<thinking>`タグが可視応答へ漏れる既知の失敗モード。本アプリは応答をそのままVOICEVOXが読み上げるため致命的)。低effortで待ち時間を抑える
  - `temperature`/`top_p`/`top_k`はOpus 5では400。「ずんだもんらしさ」はシステムプロンプトで作る
  - `MaxTokens`はthinking+応答の合計上限。絞ると途中で切れる
  - **リトライ既定2を1以下に明示**(既定のままだと最悪タイムアウト×3でW-06の60秒予算をLLMだけで食い潰す)
  - `StopReason`を`Content`読み出し前に検査(拒否時は空/部分的で`Content[0]`が落ちる)
  - JSON強制は構造化出力を優先するがGoバインディング名が未文書 → 推測せずコンパイルエラーで確定。使えなければプロンプト指示+既存`ParseLLMResponse`にフォールバック

### Wave B-1 実装完了・めたん独立検証済 (2026-08-05)

ずんだもんがW-03(HTTPConn+Fanout)/W-04(Hub)/W-05(SentenceChunker)を実装、新規55件。**めたんが独立検証**:
- `go test ./tests/... -count=1 -v` → **301 PASS**(報告と一致)
- **`-race` をめたん自身がdocker経由で再現** → integration 3.8s(Skipなし)・DATA RACE 0件
- `internal/sse/sse.go` の内容を確認し**無改変**を担保、`conn.go` を通読して単一ライタ設計の妥当性を確認
- ミューテーション5件(Hub의RWMutex全削除で`-race`がDATA RACE検出、失敗接続の解除漏れ、ブロッキング送信、バイト単位分割、nil返し)すべて検知能力あり

**ずんだもんの相談3点への回答**
1. **`-race`実行環境 → docker継続**。ホストはCGO_ENABLED=0+gcc未導入で`-race`が原理的に実行不能。ユーザーへのgcc導入依頼はせず(解決済みの問題に新しい依存を持ち込まない)、めたんが `scripts/test_race.sh` を作成・動作確認済み
2. **ハートビート未検証 → 直す(functional options)**。壊れた症状が「長時間の無通信後に切断」+自動再接続で本番でも気づきにくいため穴を残さない。`(c)`内部差し替え口は不可(テストが`package unit`=外部テストパッケージで非公開シンボルに到達できない)。`NewHTTPConn(w, opts ...HTTPConnOption)`の可変長引数なら既存呼び出しは無改変=公開API契約を壊さない。仕様書§4.4に追記
3. **§1.2「同じチャネルを通す」の解釈 → ずんだもんの実装が正しく、めたんの指示文が曖昧だった**。`Run`は送信チャネルの唯一の読み手なので自分へ送れば満杯時に自己デッドロックする。守るべき不変条件は「`ResponseWriter`へ書くgoroutineが1本」。仕様書§4.3・指示書§1.2の文面を訂正(2026-07-28の教訓「曖昧語の撲滅」の再適用)

- [x] Wave B-1 実装・独立検証
- [ ] ハートビート間隔の注入 + テスト + ミューテーション実測(ずんだもん、対応中)
- [ ] Wave A・B-1 まとめてそらレビュー依頼(ハートビート対応後)

### Wave A そらレビュー: ⚠要修正 → めたん対応 (2026-08-05)

そらの判定は**コードは承認**、要修正は文書4箇所+テストコメント1行のみ。①〜⑤の設計判断はすべて支持・承認。
- **①`NOW()`依存の方針衝突懸念 → 矛盾しないと確認**。そらが原典(`08_test_specification.md:122`)まで遡り「対象はGCの**判定述語**」であることを特定。DDLが既に`DEFAULT NOW()`を持ち前例もある。**めたんの当事者バイアスの心配は不要**との結論
- **②`TruncateFirstText`とVARCHAR(20) → 一致を実測確認**。ZWJ家族=7、結合文字「が」=2でGoのrune数と完全一致、W-07で落ちない
- **③モックスタブの偽陽性 → 危険は`AssertNotCalled`と順序検証の2系統に限定**(testify v1.9.0 `mock.go:532`を読んで特定)。W-07の「2回目は呼ばない」テストがまさに当たり所 → **W-B2指示書の着手前提条件として`m.Called()`形式への差し替えを明記**
- **④0件更新の冪等性 → 妥当**(`InsertMessage`のFK制約が会話の存在を先に証明するため危険経路は到達不能)
- **⑤`fetched_at`無視 → 許容**
- **⑥文書4箇所の旧記述 → めたんが同期済**: `04_realtime_wiring_design.md`のB-6/B-7に解消済み注記、`08_test_specification.md`のI/F定義に3メソッド追加、`01_implementation_plan.md`のT-07/T-11、`scaffold_spec_integration.md`のFileStore

**申し送りをA-1〜A-5として設計書に記録**。うち判断を要した2件:
- **A-2(不正UTF-8)**: `TruncateFirstText`は20ルーン以下だと入力をそのまま返すため不正UTF-8がDBに届きencodingエラー(21ルーン以上は`[]rune`変換でU+FFFDに置換される**非対称**)。既存11件で固定された契約は変えず、**W-06のハンドラ入口で`utf8.ValidString`により弾く**(1箇所で`content`と`first_text`の両方を守れる)。STT結果にも適用
- **A-1(ZWJ途中切れ)**: F-HIST-05は「最大20文字、超過分は省略」なので**許容**。文字境界を意識した切り詰めはコストに見合わない

### ハートビート検証の穴を塞ぐ対応 完了・めたん独立検証済 (2026-08-05)

ずんだもんが `NewHTTPConn(w, opts ...HTTPConnOption)` + `WithHeartbeatInterval` で実装、新規5件(W-03-15〜19)。
- **めたんが独自にミューテーション実測**: `Run` の `case <-ticker.C:` を丸ごと削除 → W-03-15/16/17 が FAIL、復元後に全緑(`ticker.C` の出現回数0→1も確認)。検証用の一時ファイルは削除済み
- `go test ./tests/... -count=1 -v` → **307 PASS**(報告と一致)、`gofmt`/`go vet` OK
- **後方互換の実測証跡**: テスト内に単一引数 `NewHTTPConn(w)` の呼び出しが10箇所そのまま残り全て緑 = 公開API契約は壊れていない
- ずんだもんの追加判断も妥当: W-03-15は「2回以上」で"繰り返し送出"を証明、W-03-18/19は sleep ではなく「通知が来ないこと」の待機で自己文書化、非正間隔のフォールバック(H)まで検証

**そら指摘2件も反映済みを確認**:
- W-01b-06 のコメントに半角スペース例外を追記(元の主張を残したまま例外を併記する正しい形)
- `dbNow(t, db)` ヘルパーで `SELECT NOW()` による skew 排除(2ファイルから使うため `helpers_test.go` に配置=適切な判断)

- [x] ハートビート間隔の注入 + テスト + ミューテーション実測
- [x] そら指摘のテスト側2件
- [ ] Wave B-1 そらレビュー(依頼済)
- [ ] Wave B-2(W-06/W-07)着手 — ずんだもんへ許可済。§0-0のモックスタブ差し替えが前提条件

### 共有テストDBの並行実行衝突を解消 (2026-08-05, めたん)

ずんだもんが**インフラ面の問題を発見**: `setupTestDB` が毎回 `TRUNCATE conversations, messages, audio_files CASCADE` するため、同一DB(`zuncha_test`固定)に2プロセスが同時に走ると互いの行を消し合い、「広範囲かつ実行ごとに違うテストが落ちる」切り分け困難な失敗になる。めたん/ずんだもん/そらが各自テストを回す体制なので繰り返し発生する(めたん自身も原因の一部だった)。

**採用: (b) 実行者ごとにDB分離**。(c)スキーマ分離は既存全integrationテストに影響し過剰、(a)運用回避は人間の注意力に依存するため却下。
- ただし `test_env.sh` を各自別ファイルにするとドリフトするので、**1ファイルで環境変数から導出**する形に
- `scripts/test_env.sh` 改修: `ZUNCHA_TEST_DB_OWNER` があれば `zuncha_test_<owner>`、無ければ従来の `zuncha_test`(**後方互換**・未設定時は警告)
- `scripts/create_test_db.sh` 新規: DB作成 + migration適用。**冪等**(存在チェックで CREATE DATABASE を回避、テーブル存在チェックで migration 二重適用を回避)
- **テストコードは1行も変更していない**(TRUNCATE方式のままDBを分けるだけ)

**対照実験で実証**:
- 同一DBに2プロセス同時実行 → **18 FAIL / 15 FAIL**(衝突を再現)
- DB分離で2プロセス同時実行 → **両者 0 FAIL**、それぞれ ok 3.7s

- [x] 共有DB衝突の解消(scripts整備 + 対照実験で実証)

### そらレビュー2巡目の指摘に対応 (2026-08-05)

そらは⑥の文書4箇所と申し送りA-1〜A-5を**承認**。新規指摘2件は**私の凡例追加が生んだ不整合**で、正確な指摘だった:
- [x] **凡例の配置ミス**: 凡例を170行(Wave Aタスク表の直前)に置いたが、取り消し線があるのは§1.2の表(31-45行)で130行離れていた。§1.2バックエンド表の直後(44行)へ移動し、フロントエンド表も対象に含めた
- [x] **B-10が未マーク**: 凡例を新設した結果B-10が全面未解消と読めるようになった。実際はRepositoryはW-01bで解消・呼び出しはW-07未着手の**部分解消**なので、範囲を分けて明示
- [x] A-4/A-5 の申し送り行が「改善する」「訂正する」の未来形のままだったので **✅対応済** に更新(状態変化を記録に反映しないと同じ罠の再発になる)
- [x] 参考指摘: `08_test_specification.md` §3.3 の `internal/filestore` → `internal/service (filestore.go)` を3箇所修正

**そらが「未反映」とした2件(A-4/A-5)は反映済みを再確認**(半角スペース2ヒット、±3秒0ヒット、`dbNow`4箇所)。そらのgrepはタイミングで空振りしたもの。

**共有DB衝突はそらの提案①(担当者ごとにDB分離)で既に解決済み**(前ターンで対応・対照実験で実証)。そら/つむぎの提案とは入れ違いだったため周知した。
- [x] Wave C-1 実装指示書作成: `tasks/instructions_zundamon_wave_c1.md`(W-08 Claude API実接続)。W-06/W-07と独立して着手可
  - システムプロンプト文言を**仕様から根拠を引いて確定**: 語尾「〜のだ」「〜なのだ」と一人称「ボク」は既存実装(`ConversationView.svelte`のSTT_FAILURE_MESSAGE)と`01_screen_design.md` 536/594行のテキスト例に、感情7種とその意味は`02_database_design.md` 78-88行(DDLのCHECK制約と同一)に整合
  - 「textは音声で読み上げられる」ことをプロンプトに明記(記号の羅列・絵文字連打をVOICEVOXが不自然に読むため)
  - 「迷ったら困惑」を既存`ParseLLMResponse`の困惑フォールバックと方向を揃え、**プロンプトとパーサの二重の防壁**に
  - `ResponseParser`アダプタ(`DefaultParser`)は`internal/llm`に新規ファイルで追加(既存`parser.go`は無変更、3-1テスト25件に影響なし)
  - `ThinkingBlock`を連結対象から除外する要件を明記(thinkingは有効なので届き得る。JSONに混ざると`ErrSyntax`)
  - テストは`httptest`+`option.WithBaseURL`のみ。**`ANTHROPIC_API_KEY`未設定でも緑**であることを実行して示させる
  - リクエストボディに`temperature`/`top_p`/`top_k`の**キーが存在しないこと**を検証させる(混ざると本番で400)
  - ミューテーション: `WithMaxRetries(1)`を外すと429のリクエスト回数テストが赤になることを実測

### そらWave B-1レビュー: ⚠要修正 → 対応 (2026-08-05)

**②スナップショット方式は実測で承認**(RLock保持中に書く実装へ改変すると`TestFanout/W-03-F1`が20秒でデッドロックすることをそらが実証)、**⑤フレーク耐性も承認**(遅いマシンでは長く待つ方向にしか働かない)。

**①に本物の穴 → `Done()` 方式で確定**(仕様書§2.4.1に追記):
`Run` が書き込み失敗で抜けてもHubに通知されず、①解除は`ErrSinkOverflow`経路のみ=§2.4未達 ②ハートビートの存在理由(死んだ接続の検出)が解除に繋がらない ③`Run`終了後も`WriteEvent`が64件まで成功を返す ④**W-06で`go Run(ctx)`+`<-ctx.Done()`と書くと書き込み失敗でctxがキャンセルされずハンドラが永久ブロック**(net/httpはハンドラreturn後に接続を閉じるため)。half-open接続で顕在化。
- 防壁1: ハンドラが`select { case <-r.Context().Done(): case <-conn.Done(): }`で抜ける → **既存の`defer unregister()`がそのまま解除を担いHubに新機構を足さずに済む**
- 防壁2: `WriteEvent`が終了後に`ErrConnClosed`を返す → `Broadcast`の既存の失敗解除パスで即解除、64件遅延が消える
- **onCloseコールバックは却下**: `Run`のgoroutineからHubの解除(書き込みロック)を呼ぶとデッドロックの考慮が新たに生まれる
- W-06の前提条件として「**ハンドラは`Run`が戻るまでreturnしてはならない**」を明記(`ResponseWriter`はハンドラreturn後は使用禁止)

**④はそらの前提を訂正**: 「チャンクをそのままVOICEVOXの読み上げ単位にしてはいけない」というご懸念は正しいが、**実装ではチャンクをTTSに渡していない**(`response_streamer.go:59`は`Synthesize(ctx, resp.Text)`と全文を渡し、チャンクは`SendTextChunk`専用)。1文字チャンク・空白のみチャンクがVOICEVOXに届く経路は現時点で存在しない。ただし将来の誘惑に備え申し送りB1-1として固定。

**申し送りB1-1〜B1-4を設計書に記録**:
- B1-1 表示用チャンクと読み上げ用テキストの分離(将来チャンク単位TTSにする場合は空白のみ・記号のみのスキップが必須)
- B1-2 `Broadcast`が配信先件数を返さず全員切断を検知できない → W-09でTTSが実コストを持つため「配信先0ならTTSスキップ」を検討
- B1-3 `Fanout`のnil返しにより`ResponseStreamer`の失敗分岐は本番未到達(D-5にも明記)
- B1-4 60ルーン強制分割の不自然さ

**⑤ `assert.NotPanics(t, func() { go conn.Run(ctx) })` は別goroutineのpanicを捕捉できない** → 修正依頼済

**運用課題(めたんの運用ミス)**: B-1のレビュー依頼中にW-06着手を許可したため、レビュー対象が動く状態を作った。対応方針:
1. レビュー依頼時に**その時点の全緑を実測保証**し対象ファイル一覧を明示
2. レビュー中は緑を壊す作業(次タスクのRED)を止める
3. 根本策のWIPコミットによる対象固定はユーザー許可が必要 → つむぎ経由で提案

### そらWave Aレビュー: ✅承認 + 補足対応 (2026-08-05)

そらが全項目の反映を実測で確認し**✅承認**。加えて:
- そら自身も `nulltime.go` を `Valid: true` に改変して再検証し、**許容幅を撤去した後もW-01-04/W-02-02の検知能力が落ちていない**ことを実測(復元済み・diffで一致確認)
- 凡例について「フロントエンド表も対象に含め、部分解消の規則まで追加されており指摘より一段良い形」と評価
- **DB分離の効果をそら側でも実測**: Wave A分のintegrationが71 PASS/0 SKIP/0 FAIL、**6回連続で安定**(共有DB時代は8回中5回が赤)

**そらの補足(非対称性)に対応**: `test_env.sh` は未設定時に共有DBへフォールバックする一方 `create_test_db.sh` は未設定をエラーにするため、設定漏れが再発口として残る。
- [x] `setupTestDB` に共有DB検知の警告を追加。**`t.Log` はテスト失敗時と `-v` 時に出力されるため、まさに衝突で落ちたときに目に入る**(排他機構ではなく1行の警告なので「テストコードに機構を持ち込まない」原則には反しない)
- 検証: 分離DB → 警告0件、共有DB → 警告1件を実測

**そらの運用要望を受諾**: 「全緑」報告時に**どのパッケージがコンパイル可能な状態か**を添える。そらは今回 `go test -overlay` でW-06のRED3ファイルを除外して検証(リポジトリは無変更)。

### そらの実装前レビュー: めたんの仕様記述に欠陥2件 (2026-08-05)

そらが `Done()` の**実装前**に仕様の穴を指摘。どちらもめたんの記述ミスで、実装されてから気づくと痛い箇所だった:
- [x] **防壁2の「select に `case <-c.done:` を追加」は誤り**。Goの`select`は複数caseが同時readyなとき**一様ランダムに選ぶ**ため、`done`が閉じていて`frames`に空きがある状態では**約半分の確率で`nil`(成功)を返す**。防壁2が「たまに効かない」機構になり64件遅延が確率的に復活する → **先に`done`だけを単独で非ブロッキング判定する二段構え**へ訂正(仕様書§2.4.1)
- [x] **`Run`が2回呼ばれると`close(c.done)`の二重クローズでpanic**。多重起動ガードが無い → `sync.Once`推奨、テストで多重起動ケースを1件持つことを要件化
- ミューテーション要件も追加: 二段構えを1つのselectに戻すと閉鎖後の`WriteEvent`テストが**確率的に**赤になること(複数回実行で確認)

**そらの運用補足も受諾**: 実測報告時は**どのDBで測ったか**も添える(DB分離後は各自別DBなので同条件の再現に必要)。そらは`zuncha_test_sora`を使用。

**`strings` import漏れの件**: そらが見たのはめたんが警告を追加した瞬間の一時状態で、同ターン内に解消済み(現在 `go vet ./tests/...` EXIT 0)。ずんだもんの作業とは無関係。

### Wave B-2 (W-06/W-07) 完了・めたん独立検証済 + 相談3件に回答 (2026-08-05)

ずんだもんが W-06(handler新設+既存2ハンドラ移設) / W-07(BuildPrompt+RecordingSink+ChatService) を実装、ミューテーション9件すべて狙ったテストが赤。
**めたんの独立検証**: `go vet ./...` EXIT 0 / `go build` OK / `go test ./tests/...` FAIL 0件 / 全PASS **399件**・SKIP 0件 / `response_streamer.go`(75行)・`sse.go`(15行)・`llm/parser.go`(66行) 行数不変 / `main.go` に残るのは `loadConfig`・`parseAllowedOrigins`・`main` のみ。実測DB: `zuncha_test_metan`。

**相談1【最重要】: めたんの§1.3の前提が誤っていた。ずんだもんの実測が正確。**
めたんも独立に検証: `encoding/json` の `Decode` は不正UTF-8を **U+FFFD へ置換する**(`{"text":"\xff\xfe"}` → `ef bf bd ef bf bd`、ValidString=**true**)。生バイト列のみ false。
- JSON経路では `utf8.ValidString(req.Text)` は**常に true = ガードは到達不能**
- U+FFFDは正当なUTF-8なので**DBのencodingエラーも起きない**。そらの申し送りA-2が想定した障害はJSON経路では**発生しない**
- 実際に効くのはSTT結果の経路(生バイトを扱う場合)のみ
- **(a)承認**: ガードは多層防御として残すが到達不能を明記、テストは実態(202受理+U+FFFD置換で保存され INSERT が壊れない)に合わせる
- **ずんだもんの「嘘の期待値を書いたテストは残したくない」判断が最も価値ある部分**。到達不能なガードを「守っているつもりのテスト」で覆うのはテストの不在より有害(2026-07-29の教訓と同じ構図)
- 設計書に「A-2の訂正」、指示書§1.3に訂正注記を記録

**相談2: `Exists` / `ConnCount` の追加を両方承認**。「404を返す」「切断後に登録数0」を要求しながら判定・観測手段を与えていなかった**めたんの指示の不備**。

**相談3: 署名は変えず `context.WithTimeout(context.Background(), 5*time.Second)`**。署名にctxを足すと§2.2の契約が変わる。`Background`単体だとDBハング時に`SendDone`が無制限に待つ。

**申し送り(sending固着)にも判断 → B2-1として確定**: `HandleUserMessage` の冒頭で `Fanout` を作り、中断する各経路で `SendError` を送ってからエラーを返す。**フロント側にタイムアウトを新設しない**理由は、フロントが既に INV-3b「SSE errorで sending/transcribing から editable へ復帰」を持っており、`SendError` 1発で既存機構が解決するため。INV-3bが「中間状態の行き止まり救済」として設計された意図そのままの用途。

- [x] Wave B-2 実装・独立検証・相談回答
- [ ] 残作業3件(ずんだもん): ①W-03の`Done()`(二段構え+`sync.Once`) ②RecordingSinkの5秒タイムアウト ③中断時の`SendError`
- [ ] 上記完了後、めたんが全緑+コンパイル可能パッケージを実測保証 → そらへまとめてレビュー依頼(そらは待機中)

### 防壁1の矛盾: そらとずんだもんが独立に同じ指摘 (2026-08-05)

**めたんの記述ミス**。§2.4.1の防壁1 `select { case <-r.Context().Done(): case <-conn.Done(): }` は、同じ節の「ハンドラは`Run`が戻るまでreturnしてはならない」と**両立しない**。`r.Context().Done()`が先に発火した場合(=クライアント切断という最も普通のケース)、両者に順序保証が無く`ResponseWriter`無効化後に`Run`が`c.w.Write`を呼び得る。そらはさらに**keep-aliveでコネクション再利用時に別リクエストのレスポンスへフレームが混入する**という帰結まで特定。

- [x] **ずんだもんの同goroutineブロック呼び出し(`conn.Run(r.Context())`)を正として仕様書§2.4.1を書き換え**。採用理由: ①`Run`はctxキャンセルでも書き込み失敗でも戻るので防壁1の目的を満たす ②**「Run実行中にハンドラがreturnする」状態が構造的に作れない**(規律ではなく構造で保証) ③goroutineが1本減る
- [x] 将来`go conn.Run(ctx)`にする場合の必須知識も併記: 待ち受けは`<-conn.Done()`**だけ**。`case <-r.Context().Done():`を足すと`Run`の終了を追い越して保証が壊れる
- [x] **そらの改善提案①を採用**: ミューテーション検証を「複数回実行」→「**1テスト内で20回呼ぶ**」へ。確率的な穴は複数回実行しても確実には赤にならない(1回50%なら10回実行で約0.1%の見逃し)。20回なら1−0.5²⁰≈99.9999%で**単一実行でも**赤になる
- [x] **そらの改善提案②を採用**: 二段構えでも残る狭いレース(`done`判定通過直後に`Run`終了 → そのフレームは積まれたまま`nil`が返る)を明記。1フレームで収束するので実害なしだが、将来テストが不安定に見えたときの誤診を防ぐ
- [x] **そらの軽微指摘**: §2.4の3番目の箇条書き(バッファ段数)が§2.4.1配下に取り込まれていたのを本文へ戻した。「追記が元記述の構造を崩す例」

**ずんだもんのT対応が今回最も光った点**: 「`closeOnce`を外しても赤にならず、多重呼び出し安全性を守るテストが1件も無かった → W-03-27を追加して塞いだ」。**検知できないガードを黙って残さない**方針を指示されずに自分で実践。

**めたん独立検証**: `TestHTTPConn` 27件PASS、`-race`クリーン、W-03-20〜27の8件確認、二段構え`WriteEvent`・`closeOnce`・Runのdoc comment修正すべて設計どおり(実測DB: `zuncha_test_metan`)。
現在`W-07-C15`(中断時SendError)と`W-07-R12`(保存用ctx)が赤だが、これは**めたんが出した追加要件をずんだもんがTDDで実装中の状態**。

### WIPコミット(d41188b)でレビュー対象固定 → そらWave B-2レビュー: ⚠要修正(新規3件) (2026-08-07)

ユーザー承認によりWIPコミット運用を開始(以降レビュー依頼時は対象をWIPコミットで固定する)。つむぎが実機再検証(unit 60トップレベルPASS/0 FAIL、integration 18トップレベルPASS/0 FAIL、DB: `zuncha_test_tsumugi`)した上でそらへ再レビュー依頼、残作業3件(①`Done()`二段構え+`sync.Once` ②5秒タイムアウト ③中断時`SendError`)はいずれも実装・テスト担保済みと確認された。

そらが401 PASS/0 FAIL(実測DB: `zuncha_test_sora`)・`-race`クリーンに加え、新規3件を指摘:

- **①【最重要】SSE接続が1本でもあるとグレースフル停止が必ず失敗する**: `Events`は`conn.Run(r.Context())`でブロックするが、`http.Server.Shutdown`はin-flightハンドラの`r.Context()`をキャンセルしない。会話画面はSSEを常設する設計(仕様書§2.3)なのでこれは通常運用の既定経路。`tasks/todo.md:271`の「Shutdownエラーを`log.Fatalf`で顕在化する」という過去のつむぎの判断は「長時間ブロックするハンドラが存在しない」前提だったが、W-06がその前提を崩した。
- **②SSEのerrorイベントに内部エラー文字列がそのまま出る**: `response_streamer.go`の`fail`が`err.Error()`をそのまま`SendError`に渡し、`"llm generate: ..."`等がブラウザのトーストに出る。同Wave内の`chat.go`は`errMsgSaveUserMessage`等の日本語定数を使っており不整合。
- **③防壁2のミューテーション要件「20回」が3回にとどまる**: `sse_conn_test.go`のW-03-24が`for i := 0; i < 3; i++`。理論上の見逃し確率3.1%、実測30回中1回すり抜けを観測。

**対応方針(めたん判断・つむぎ代行)を`tasks/instructions_zundamon_wave_b2_fix.md`に指示書化**:
- ①`internal/httpserver/httpserver.go`の`Run`に`BaseContext`+能動キャンセルを追加。`events.go`・`main.go`は無改変で済む。そらの要望により「SSE接続を張ったまま停止させ`httpserver.Run`が`nil`を返すこと」の回帰テストを`graceful_shutdown_test.go`に追加必須。
- ②`fail`に利用者向け文言(`errMsgGenerateResponse = "応答の生成に失敗しました"`、仕様書§2.2の例文言)を渡す形に変更。全ステップ同一文言に統一(過剰な細分化を避ける)。テストで内部エラー文字列が漏れないことを固定。
- ③`for i := 0; i < 20; i++`への1行修正。

- [x] そら指摘3件への対応方針決定・指示書作成
- [x] ずんだもんによる実装(RED確認→GREEN→ミューテーション実測)
- [ ] WIPコミットで再固定→そらへ再レビュー依頼(DB: `zuncha_test_sora`、コミットハッシュ明示の要望あり)

### Wave B-2 差し戻し対応(そら指摘①②③) 完了 (2026-08-10, ずんだもん)

指示書 `tasks/instructions_zundamon_wave_b2_fix.md` の①②③をTDDで実装したのだ。**指示書の対処方針から外れた点は無し**。実測DB: `zuncha_test_zundamon`。

**①SSE接続中のグレースフル停止(`internal/httpserver/httpserver.go`)**
- `Run` に `BaseContext` を追加し、`ctx.Done()` 時に `baseCancel()` で in-flight の `r.Context()` を能動的にキャンセル。`events.go`・`main.go`・`conn.go` は**無改変**(`main.go` は `BaseContext` を自前で設定していないため衝突なし)
- 回帰テスト `TestGracefulShutdown_SSE接続中でも停止がエラーなく完了する` を `tests/integration/graceful_shutdown_test.go` に追加。実ハンドラ・実DB・実リスナで `/conversations/{id}/events` に接続し、初回フレーム `retry: 3000` の受信＋`hub.ConnCount==1` で接続確立を確定させてから停止をトリガする(sleep非依存)
- テスト用にハンドラ組み立てだけを `newTestHandler` として `handler_helpers_test.go` から切り出し(`newHandlerFixture` はこれを呼ぶだけの純リファクタ。httptest ではなく実リスナ上で `httpserver.Run` を回す必要があるため)
- **RED実測**: 対処前は `context deadline exceeded` / 経過 **3.001秒**(=shutdownTimeout満了) / `ConnCount` が 1 のまま、の3点で赤
- **GREEN実測**: **0.05秒**で `nil` 返却、`ConnCount` 0。既存の `InFlightCompletes...` も引き続き緑
- **ミューテーション2件**: (A)`baseCancel()` の呼び出しを削除 → FAIL(3.08秒) (B)`srv.BaseContext = ...` の代入を削除 → FAIL(3.08秒)。いずれも復元済み

**②SSE errorイベントの内部エラー文字列(`internal/service/response_streamer.go`)**
- `const errMsgGenerateResponse = "応答の生成に失敗しました"` を追加し、`fail` は `err.Error()` ではなくこの定数を `SendError` に渡す。`err` は戻り値としてそのまま返す(ログ・呼び出し側用)。`fail` の呼び出し側7箇所は**無変更**
- 既存テストの `sink.On("SendError", mock.Anything)` 5箇所を固定文言期待へ変更 + 新規テスト `TestResponseStreamer_errorイベントに内部エラー文字列を出さない` を追加。中断が起こり得る**全7ステップ**(llm/parse/emotion検証/emotion送出/textチャンク/audio_url/done)を網羅し、`SendError` の実引数が固定文言と一致し、かつ内部エラー断片9種(`"llm generate:"`, `"client disconnected"` 等)を含まないことを検証
- **RED実測**: 既存TC-3-2-06が `(string=llm generate: llm unavailable) != (string=応答の生成に失敗しました)` で赤。新規テストも7サブケース全てが赤(`send emotion: client disconnected` 等が実際に漏れていた)
- **GREEN実測**: `TestResponseStreamer` 系 29 PASS。既存15件は本文無改変のまま緑
- **ミューテーション2件**: (A)`fail` を `err.Error()` に戻す → FAIL (B)文言を `"エラーが発生しました"` に変える → FAIL。いずれも復元済み

**③防壁2のミューテーション要件20回(`tests/unit/sse_conn_test.go` W-03-24)**
- `for i := 0; i < 3; i++` → `for i := 0; i < 20; i++` の1行修正(ループ回数以外は無変更)。理由をコメントで固定
- **ミューテーション実測**(二段構えselectを1つのselectに戻す改変を適用して比較):
  - 修正後(20回): **10試行中10回FAIL**(見逃し0)
  - 修正前相当(3回): **30試行中23回FAIL = 7回すり抜け(23%)**。そらの実測(30回中1回すり抜け)より悪い結果で、3回では検知が確率的に破綻していることを追認
  - `internal/sse/conn.go` は改変→実測→**復元済み**(`git diff` 無出力で確認)

**完了条件の実測結果**
- `go build ./...` OK / `go vet ./...` OK / `gofmt -l .` 無出力
- `go test ./tests/... -count=1 -v` → **410 PASS / 0 FAIL / 0 SKIP**(integration 2.9s、unit 0.22s)。前回401から+9(①の1件、②の新規1関数7サブケース+親1)
- `./scripts/test_race.sh`(docker経由 `-race -count=1`) → integration 5.2s / unit 1.3s、**DATA RACE 0件**
- コンパイル可能パッケージ: 全パッケージ(`go build ./...` / `go vet ./...` が EXIT 0)

- [x] WIPコミット(8a5e934)で再固定→そらへ再レビュー依頼

### そら再レビュー(8a5e934): ⚠要修正(新規2件) (2026-08-10)

①②③自体はミューテーション実測込みで解消確認(そら実測DB: `zuncha_test_sora`、410 PASS/0 FAIL、raceクリーン)。①の設計変更(BaseContext能動キャンセル)に伴う新たな不整合2件:

- **【A】in-flight完遂の保証喪失と文書の食い違い**: `baseCancel()`が`Shutdown()`前に呼ばれるため、`r.Context()`を参照するハンドラ(`CreateConversation`等)は停止トリガで打ち切られる(そら実測: 200→500)。`httpserver.go`のdocコメント・`03_single_instance_smoke.md:46`・`graceful_shutdown_test.go:75`が「in-flightは完遂する」という古い記述のまま。
- **【B】①の回帰テストがDB未設定で無言スキップ**: `TestGracefulShutdown_SSE接続中でも停止がエラーなく完了する`は`newTestHandler`経由でDBに依存し、ファイル冒頭の「DB非依存=skipなし」というコメントと矛盾。

**対応方針(つむぎ判断・シンプルさ優先で確定)**: 猶予時間を挟む案(`time.AfterFunc`でノブ追加)は不採用。SSE等の長寿命接続はどのみち`shutdownTimeout`で打ち切られ効果が薄く、複雑さが増すだけ。**実態に文書を合わせる**方向とし、`tasks/instructions_zundamon_wave_b2_fix.md`に追補として指示書化(3ファイルのコメント修正+DB非依存の新規回帰テスト1件)。

- [x] 追補①②の対応方針決定・指示書追記
- [x] ずんだもんによる追補実装
- [x] WIPコミット(034af09)で再固定→そらへ再レビュー依頼

### Wave B-2 追補対応(そら再レビュー指摘【A】【B】) 完了 (2026-08-10, ずんだもん)

指示書 `tasks/instructions_zundamon_wave_b2_fix.md` の「追補」セクションのみを対応したのだ(①②③本体は無改変)。**指示書の対処方針から外れた点は無し**。実測DB: `zuncha_test_zundamon`。

**【A】文書・コメントを実態に合わせる(挙動変更なし)**
- `internal/httpserver/httpserver.go` の `Run` docコメント: 「in-flight リクエストの完遂を待って新規接続を遮断する」→「新規接続を遮断し、同時に in-flight の `r.Context()` も能動的にキャンセルする。`r.Context()` を参照しないハンドラの処理は完遂するが、`r.Context()` に依存する処理は打ち切られる」に書き換え。**コードは1行も変更していない**(`git diff` は docコメントのみ)
- `docs/04_implementation/03_single_instance_smoke.md`: §3の期待値を「中断されずレスポンスを受け取れる」→「`r.Context()`を参照しない処理は完遂する」に修正し、能動キャンセルにより`r.Context()`依存の処理(DBクエリ・SSE配信)は打ち切られエラー応答/接続断になり得る旨を子項目で明記。あわせて§3見出し(「in-flight完遂の目視」)と「自動テストとの対応」表の該当行も実態に合わせた
- `tests/integration/graceful_shutdown_test.go:75` 付近: アサーション自体は無変更(このスタブは`r.Context()`を参照しないため実際に完遂する)。「このスタブは`r.Context()`を参照しないため打ち切られない/`r.Context()`依存の挙動は`TestGracefulShutdown_CtxDependentHandlerIsCanceled`を参照」という補足コメントを追加し、メッセージ文言も「r.Context()を参照しないin-flightリクエストは完遂する」に具体化

**【B】DB非依存の回帰テストを追加**
- ファイル冒頭コメントの「DB非依存＝skipなし」を、「原則DB非依存。ただし`TestGracefulShutdown_SSE接続中でも...`は実ハンドラ経由でDBに依存するため例外でskipされ得る。同じ性質はDB非依存の新規テストが担保する」という趣旨に修正
- 新規テスト `TestGracefulShutdown_CtxDependentHandlerIsCanceled` を追加。`<-r.Context().Done()` で待つだけのスタブハンドラ(= `events.go` の `conn.Run(r.Context())` と同じ性質だけを抽出)を使い、**SSEハンドラ・Hub・DBに一切依存せず**に ①`r.Context()`が停止トリガでキャンセルされる(`ErrorIs(context.Canceled)`) ②`Run`が`nil`を返す ③`shutdownTimeout/2`未満で戻る、の3点を検証。sleep非依存(channel同期)
- **ミューテーション実測2件**(いずれも実施後に復元済み・`git diff`でコード差分なしを確認):
  - (A)`baseCancel()` の呼び出しを削除 → **FAIL**(`context deadline exceeded` / 経過 3.0007秒 = shutdownTimeout満了)
  - (B)`srv.BaseContext = ...` の代入を削除 → **FAIL**(「r.Context() 依存ハンドラが打ち切られなかった」/ 8.00秒)

**完了条件の実測結果**
- `go build ./...` OK / `go vet ./...` OK / `gofmt -l .` 無出力
- `go test ./tests/... -count=1 -v`(DB: `zuncha_test_zundamon`) → **411 PASS / 0 FAIL / 0 SKIP**(トップレベル63件。integration 3.7s、unit 0.22s)。前回410から+1(新規テスト1件)
- `unset ZUNCHA_TEST_DATABASE_URL` 状態での `go test ./tests/integration/... -run TestGracefulShutdown -v` → `InFlightCompletesAndNewConnRefused` **PASS** / `CtxDependentHandlerIsCanceled` **PASS(skipされない)** / `SSE接続中でも停止がエラーなく完了する` **SKIP**。指示どおり両方を確認
- `./scripts/test_race.sh`(docker経由 `-race -count=1`) → integration 5.9s / unit 1.3s、**DATA RACE 0件**

### そら再レビュー(034af09): ⚠要修正(残1件・軽微) (2026-08-10)

観点1〜5(そらが事前提示した確認観点)は全てOK、ミューテーション3件もDB無し環境で実測しRED確認(`baseCancel()`削除・`BaseContext`代入削除・`fail`を`err.Error()`に戻す)。指摘A/Bは実質解消。`TestGracefulShutdown`系20回連続実行もflaky無し。

- **【C】`03_single_instance_smoke.md:4`の目的行だけ旧表現「in-flight完遂」が残存**: 46行目・63行目は修正済みだったが、冒頭の目的行が未修正で「最初に読む一文で誤った前提を与える」との指摘。「この1行が直れば即承認できる状態」とのコメント付き。

単純で明白な1行修正のため、つむぎが直接対応(過剰なエンジニアリング回避、CLAUDE.md方針どおり)。目的行を「in-flightの扱い(`r.Context()`非参照なら完遂・依存する処理は打ち切り)・停止後の新規接続遮断」に修正。

- [x] 指摘C対応(つむぎ直接修正)
- [x] WIPコミット(cb7a3fe)で再固定→そらへ最終確認依頼

### そら最終確認(cb7a3fe): ✅承認 — Wave B-2完了 (2026-08-10)

指摘Cの解消を確認(旧表現「in-flight完遂」の残存ゼロをdocs/internal/tests全体でgrep確認済み)。`docs/`・`tasks/`以外の差分ゼロ(コード無変更)をmachine的に確認。411 PASS/0 FAIL/0 SKIP、raceクリーン、`git status`空(DB: `zuncha_test_sora`)。

**Wave B-2 ✅承認。** そらの総評: ①の設計変更で失われた保証(`r.Context()`非参照なら完遂/依存する処理は打ち切り)を隠さず文書化した点、指摘Bへの対応(DB非依存の番人テスト)が要求水準を上回った点、③の20回という数字に根拠をコメントで残した点を評価。

- [x] Wave B-2 完了(つむぎ最終ゲート通過)

**そらからの申し送り(今回は対応対象外・記録のみ)**: `internal/handler/messages.go`の応答生成goroutineは`context.WithoutCancel(r.Context())` + 60秒タイムアウトで動くため`http.Server.Shutdown`(≒`httpserver.Run`のBaseContext能動キャンセル)の追跡対象外。停止時にassistantメッセージの保存中でもプロセスが終了しうる。Wave B-2の変更で新規に生まれた問題ではなく既存の性質。永続化の確実性が要件に上がるWaveでの検討事項として申し送り(詳細は`tasks/lessons.md` 2026-08-10)。

**今回の教訓を`tasks/lessons.md`に記録**: ①挙動を変える修正は影響範囲を「変更したファイル」でなく「その挙動を語っている全箇所」でgrepしないと1往復で終わらない(指摘Cの見落としが好例) ②「skipなし」等の断定コメントは将来の変更で嘘になりうる前提を疑う ③`WithoutCancel`箇所はプロセス終了時の扱いを設計時点で合わせておく。

- [x] 次タスク判断: Wave C-1(Claude API実接続 W-08)着手を決定。ユーザーからANTHROPIC_API_KEY未発行・実API疎通確認は後日ユーザー自身が行うとの申し送りあり。指示書§5.2がhttptestフェイクのみで完結する設計のため実装は続行可と判断

### Wave C-1 (W-08) 実装完了・相談5件に回答 (2026-08-10, ずんだもん→つむぎ)

ずんだもんが `internal/anthropic`(Client/prompt/schema)・`internal/llm/parser_adapter.go` を実装。§2確定事項は無変更、§6のJSON強制方式は**推測せずSDKソースを実測**して`OutputConfigParam`(`effort`)と`JSONOutputFormatParam`(`json_schema`)が両方使えることを確認し採用(`ParseLLMResponse`は二重防壁として維持)。

実測: RED確認済み、`go test ./tests/...`全緑(integration 9.0s/unit 0.9s)、`test_race.sh`全緑、`gofmt`/`vet`/`build`クリーン、`internal/llm`の4ファイル無変更(`git diff --stat`空)、`ANTHROPIC_API_KEY`未設定でも全緑。ミューテーション3件(`WithMaxRetries(1)`除去/StopReason先行検査除去/textブロックフィルタ除去)すべて狙った箇所が赤。`mutation-test-overlay`スキルを早速活用。

**相談5件への回答(つむぎ判断)**:

1. **go.mod の go 1.22→1.24引き上げ(SDK依存で回避不能)**: 承認。実測(`GOTOOLCHAIN=local`でのビルド失敗)で必然性確認済み。`scripts/test_race.sh`の`GO_IMAGE`更新も連動して承認。
2. **testify 1.9.0→1.11.1強制アップグレード**: つむぎが直接検証。両バージョンの`mock.go`で`AssertNotCalled`・`methodWasCalled`・`isArgsEqual`の実装を比較し**バイト単位で完全一致**(行番号のみズレ)と確認。`tasks/todo.md`§0-0の「偽陽性は`AssertNotCalled`と順序検証の2系統に限定」という調査結果はv1.11.1でも**そのまま有効**、再調査不要。
3. **§7配線を実施した判断**: 承認(指示書の「W-07未完成なら配線しない」という但し書きは非該当、W-07完成済みのため妥当)。ただし`ANTHROPIC_API_KEY`未設定で`log.Fatal`する設計により、**ユーザーがキー発行前は`go run ./cmd/api`が起動できない**。ユーザーへ申し送り済み(テストには影響なし、`cmd/api`非依存を確認済み)。
4. **感情7種enumの四重管理(DDL/validation/SystemPrompt/schema.go)**: 一元化は次Waveでの検討事項として申し送り(今回は最小変更で見送り、`internal/validation`に公開スライスを足すかは別途判断)。
5. **実APIでの疎通確認は未了**: ユーザー自身が今後キー発行後に確認予定。`output_config.format`(json_schema) + adaptive thinking + `effort:low`の組み合わせが実APIで400にならないかが未検証。400の場合は`OutputConfig.Format`の1行を落とすだけで退避できる構造になっている旨、申し送り済み。

- [x] Wave C-1 実装完了・相談5件回答(つむぎ最終ゲートは実API疎通確認保留のためin_progressのまま)
- [ ] ユーザーによるANTHROPIC_API_KEY発行・実API疎通確認(申し送り、対応時期未定)
- [x] そらへWave C-1レビュー依頼 → ⚠要修正(指摘3件+重要申し送り1件)

### そらWave C-1レビュー(8d018ec): ⚠要修正(指摘3件) (2026-08-10)

実測: 424 PASS/0 FAIL/0 SKIP(DB: `zuncha_test_sora`)、raceクリーン、`env -u ANTHROPIC_API_KEY`でも全緑、`internal/llm`4ファイル無変更、`cmd/api`非依存を確認。§2確定事項9項目・§3プロンプト文言(バイト一致)・§5.1設計判断8項目はすべて一致。§6構造化出力の採用も実際のリクエストボディダンプで確認(事実と一致)。

ミューテーション14件中**3件が緑(検知不能)**:
- **【指摘1・必須】`schema.go`が1行もテストで守られていない**: `OutputConfig.Format`丸ごと除去/感情enum破壊/`required`空、いずれも検知不能。相談5で「実APIが400なら`OutputConfig.Format`を落として退避」と明言した箇所であり、近く触られる予定の1行。enumが1文字ズレると構造化出力が7種外を返し`ParseLLMResponse`が毎回「困惑」フォールバックでF-AI-02が静かに壊れる。
- **【指摘2・軽微】設計文書の旧記述残存**: `04_realtime_wiring_design.md` L309(決-1a、拒否は「`llm`のセンチネルエラー」→実際は`internal/anthropic.ErrRefused`)、L248(W-08行、「システムプロンプトで強制」→実際は構造化出力+プロンプトの二重防壁)。
- **【指摘3・軽微】`go mod tidy`未実行**: `anthropic-sdk-go`が直接依存なのに`// indirect`誤記。

**申し送り(今回の欠陥ではないが重要)**: `cmd/api`は現状、最初のユーザー発話でプロセスごと落ちる。`ttsClient`がnilのまま渡され`response_streamer.go`の`Synthesize`呼び出しがnilインターフェース呼び出しになり、`recover()`を持たない裸のgoroutine内で発生するためAPIプロセス全体が落ちる。W-08以前も`llmClient`がnilで最初の行で落ちていたため新規の欠陥ではないが、症状が「LLM成功後に突然プロセスが死ぬ」に変化した。W-09(TTS実装)完了までは1往復成立しない前提。ユーザーがキー発行後の疎通確認で最初に踏む可能性が高いため申し送り。

**対応(つむぎ)**:
- [x] 指摘2: 設計文書2箇所をつむぎが直接修正
- [x] 指摘3: `go mod tidy`実行、`go build`/`go vet`で無事故確認
- [x] 指摘1: ずんだもんへ差し戻し(TDDでのテスト追加) → 対応完了
- [ ] W-09までのcmd/apiクラッシュ問題をユーザーへ申し送り(対応は別Wave)

### ずんだもん指摘1対応・そら最終確認: ✅承認 — Wave C-1品質面完了 (2026-08-10)

`tests/unit/anthropic_client_test.go`のみ変更(プロダクションコード無変更)。`TestGenerateResponse_構造化出力で感情7種のJSONスキーマを強制する`を新規追加し、format.type/emotion enum(7種)/required/additionalPropertiesを検証。enumはリテラル一致に加え`validation.ValidateEmotion()`との突合も追加し、四重管理による静かなズレ(テスト側とschema.go側が同時にズレるケース)も検知できる形にした。WIPコミット(638e88f)で固定、つむぎ独立検証(build/vet/gofmt/全緑、DB: `zuncha_test_tsumugi`)後にそらへ再レビュー依頼。

**そら最終確認**: 独自に組み直したミューテーション12件すべて赤を実測(そらの要求3件より広い範囲: additionalProperties/enum件数/型/正本突合まで確認)。特に「二重ズレ」検証(schema.goとテストの①リテラルを同時に8種目に書き換えても②の`validation.ValidateEmotion`突合で赤になる)を独立実測し、②の設計が空振りでないことまで裏取り。425 PASS/0 FAIL/0 SKIP、raceクリーン(DB: `zuncha_test_sora`)。指摘2・3(設計文書修正・go mod tidy)も再確認済み。

**判定: ✅承認。** ずんだもん提案の教訓(`tasks/lessons.md`)も「今回の穴の本質を正確に言い当てている」と評価。

- [x] Wave C-1 品質面完了(つむぎ最終ゲート: そら承認済み)
- [ ] **実API疎通確認待ちのためcompletedにはしない**(ユーザーのANTHROPIC_API_KEY発行後に対応)
- [ ] 次タスク判断: W-09(VOICEVOX/TTS実装)着手 or 他の優先度確認

**実API疎通確認時の切り分けガイド(ずんだもん申し送り、ユーザーがキー発行後に踏む可能性が高い順)**:
1. **最初に踏むのはTTSクラッシュの可能性が高い**: 症状は「LLMは成功しテキストは流れるが、その直後にAPIプロセスごと落ちる」。W-09未実装による既知の状態でW-08の欠陥ではない。
2. **400が出た場合の切り分け順**: `output_config.format`(json_schema) → `output_config.effort:"low"` → `system`のCacheControl、の順で外して試す。退避は`internal/anthropic/client.go`の`Format:`1行を消すだけで済む構造(ただしテストが赤くなるのでテスト側の対応も必要)。
3. **401なら`errors.As`で`*anthropic.Error`の`StatusCode`が取れる**のでログで判別可能。APIキー・プロンプト本文はログに出ない設計。
4. **プロンプトキャッシュの確認**は`usage.cache_read_input_tokens`を見る。ゼロが続くならシステムプロンプトが揺れているか、Opus 5の最小キャッシュ長(512トークン)に`SystemPrompt`が届いていない可能性(定数なので通常揺れないはずだが実測が必要)。

### W-09着手前レビューで設計課題を発見・ユーザー承認で解決方針確定 (2026-08-10)

W-09(VOICEVOX/TTS実装)着手前に設計書D-4を精査したところ、実装前に解決すべき制約を発見。`audio_files.message_id`は`NOT NULL REFERENCES messages(id) ON DELETE CASCADE`だが、現行の`ResponseStreamer.StreamResponse`の順序では、TTS合成(`Synthesize`)が呼ばれる時点でassistantメッセージはまだDB未保存(保存は`SendDone`のタイミング)。D-4通り「Synthesize内部でaudio_filesにINSERT」すると外部キー違反になる。

検討した選択肢: (A)FK制約を外す(最小変更) (B)assistantメッセージ保存を前倒しする(ResponseStreamer/RecordingSink/ChatServiceの責務分割が必要で影響大) (C)別アプローチ。**ユーザーが(A)を選択**。

**対応済み(つむぎ)**:
- [x] `migrations/0001_initial_schema.up.sql`: `audio_files.message_id`のFK制約撤去(値は設定するが参照整合性は強制しない、`conversation_id`のFK/CASCADEは維持)
- [x] `docs/02_functional_design/02_database_design.md`・`00_minutes.md`: 記述を実態に合わせて更新
- [x] `docs/04_implementation/04_realtime_wiring_design.md` D-4を訂正(2026-08-10訂正として経緯を明記、`internal/tts`のI/F変更も必要と判明したことを含む)
- [x] `tasks/instructions_zundamon_wave_w09.md`作成: `internal/voicevox`新規実装、`tts.TTSClient.Synthesize`のシグネチャ拡張(conversationID/messageID追加)、`ChatService`でのID事前生成、`ResponseStreamer.StreamResponse`のシグネチャ拡張、既存テスト(response_streamer_test.go 16箇所等)の改修を指示

**スキーマ変更の影響で既存テスト2件も修正(つむぎ)**: `tests/integration/audio_fetch_test.go`の`W-02-05`(存在しないmessage_idは外部キー違反でエラー→**FK制約なしでエラーにならない**仕様に書き換え)、`W-02-08`(メッセージ削除でCASCADEにより音声レコードも消える→**FKが無いため連鎖しない、レコードは残る**仕様に書き換え)。テストDB再作成(スキーマ変更反映)後、全緑を実測(DB: `zuncha_test_tsumugi`、build/vet/gofmt/test全緑)。

- [ ] ずんだもんによるW-09実装
- [ ] そらへW-09レビュー依頼

### W-09 VOICEVOX TTS実装（ずんだもん・2026-08-10）

**実装（TDD: RED→GREEN→REFACTOR）**:
- `internal/voicevox/client.go` 新規: `tts.TTSClient` 実装。`POST /audio_query?text=&speaker=` → クエリJSON取得 → `POST /synthesis?speaker=`（bodyにクエリJSONを**バイト列のまま**渡す）→ WAV → `FileWriter.Write` → `AudioRepository.InsertRecord` → `/audio/{ulid}` を返す。`AudioRepository`/`FileWriter` は消費側で最小I/F定義（`localfs.FileStore.Write` のコメント方針に沿う）。ベースURL・`newID`・`now` は引数注入（`os.Getenv` を本パッケージで読まない既存方針）。
- `internal/tts/client.go`: `Synthesize(ctx, text, conversationID, messageID)` へ拡張（D-4 訂正1）。
- `internal/service/chat.go`: `assistantMessageID := s.newID()` を `RecordingSink` 構築直前で事前採番し、`NewRecordingSink` と `StreamResponse` の両方へ渡す。
- `internal/service/recording_sink.go`: `newID func() string` を `messageID string` に置換（内部採番をやめる。未使用フィールドを残すと「採番元が2つある」誤読を招くため削除した）。
- `internal/service/response_streamer.go`: `StreamResponse(ctx, sink, prompt, conversationID, messageID)`。TTSへ素通しのみ、既存の中断・TTS失敗スキップの契約は無変更。
- `cmd/api/main.go`: `ttsClient` nil を `voicevox.NewClient` へ差し替え（そら申し送りの「最初のユーザー発話でプロセスが落ちる」問題の解消）。`VOICEVOX_BASE_URL` を `loadConfig` に追加。

**speaker ID の選定根拠**: `speakerID = 3`（ずんだもん／ノーマル）。VOICEVOX ENGINE の `GET /speakers` が返す `styles[].id` で、既定エンジンの標準割り当て（あまあま=1 / セクシー=5 / ツンツン=7 / ささやき=22 と同系列）。本アプリのキャラクターがずんだもんであり、平常の読み上げに使うため「ノーマル」を選択。コードにコメントで根拠と確認コマンド（`curl -s :50021/speakers | jq ...`）を残した。

**`VOICEVOX_BASE_URL` 未設定時の扱い**: **デフォルト値 `http://localhost:50021`（VOICEVOX標準ポート）を採用**し、`log.Fatal` にはしない。`ANTHROPIC_API_KEY` は秘密情報で推測不能・環境ごとに異なるため未設定即死が妥当だが、VOICEVOXのベースURLは秘密でなく標準ポートが事実上一意に決まるため、未設定を起動時エラーにすると開発時の手間だけが増える。`voicevox.NewClient` は空文字を受けたら生成時にエラーを返すので、「空のまま実行時に初めて落ちる」経路は塞いである。W-11のCompose ではサービス名（例 `http://voicevox:50021`）で上書きする。

**実測（DB: `zuncha_test_zundamon`、スキーマ変更反映のため再作成済み）**:
- `go test ./tests/... -count=1` 全緑（455 PASS / 0 FAIL / 0 SKIP）、`./scripts/test_race.sh` クリーン
- `gofmt -l .` 空 / `go vet ./...` EXIT 0 / `go build ./...` 成功
- `internal/llm`・`internal/anthropic` 無変更（`git diff --stat` で0行）
- テストは `httptest` のフェイクVOICEVOXのみ。実VOICEVOX（:50021）へは接続しない
- RED実測2段: ①`internal/voicevox` 未作成で `package ... is not in std`、②I/F変更で `not enough arguments in call to s.ttsClient.Synthesize (have 2, want 4)`

**ミューテーション実測7件（`go test -overlay`、作業ツリー無改変・一時ファイル削除済み・`git status` 確認済み）**:

| # | 改変 | 赤になったテスト |
|---|---|---|
| M1 | `speakerID` 3→1（あまあま） | V2 / V3 / V9 |
| M2 | `audio_query` の結果を `synthesis` へ渡さず自作JSONにする | V4 / V20 / クエリJSON構造非依存 |
| M3 | `StreamResponse` が TTS へ空文字を渡す | 3-2 新規（TTSへ会話ID/メッセージIDを渡す）/ C19 / C20 |
| M4 | `ChatService` が TTS 側だけ別採番（IDを共有しない） | C19 |
| M5 | `Write` と `InsertRecord` の順序入れ替え | V8 / V17 / V18 |
| M6 | `InsertRecord` 失敗を無視してURLを返す | V18 |
| M7 | TTS失敗でも `audio_url` を送る（既存テストの退行確認） | **TC-3-2-07** / C11 / C16 / C18 |

M7 により、既存 `response_streamer_test.go` の検知力が引数追加によって落ちていないことを確認した。

**申し送り**:
1. **孤児ファイルの掃除（別Wave）**: `InsertRecord` 失敗時、WAVは書き込み済みでレコードが無い孤児ファイルが残る（テスト W-09-V18 で挙動を固定）。逆順（登録→書き込み）にすると「レコードはあるがファイルが無い」＝`GET /audio/{id}` が500になる窓ができるため、孤児ファイル側を選んだ。掃除は `internal/gc` の期限切れGCと同じ枠で扱えるはずだが、`audio_files` に載らないファイルなのでファイル走査が必要。W-11以降で検討。
2. **音声保存先ディレクトリ**: 現状 `defaultAudioDir = /tmp/zuncha/audio`（`WithAudioDir` で差し替え可）。環境変数化はしていない。W-11でボリュームを割り当てる際に `ZUNCHA_AUDIO_DIR` 等を足すか判断が必要。
3. **申し送り B1-2（配信先0ならTTSをスキップ）は今回入れなかった**: `Hub.Broadcast` が配信先件数を返さないため、スキップ判定には Hub/Fanout/`EventSink` のいずれかの契約変更が必要で、既存の sse 系テストへの波及が大きい。NF-SCALE-01（10人・社内利用）では無駄なTTS 1回のコストが小さく、費用対効果が合わないと判断。B1-2 は「変更余地を残す」までを維持する。
4. **VOICEVOXの長文リクエスト**: `text` はクエリパラメータで送る（公式の `--get --data-urlencode` と同じ形）。URLエンコード後のリクエストラインが長くなるため、極端な長文（日本語1000文字以上）で実VOICEVOX側の行長上限に当たる可能性がある。テストではフェイク相手に1000文字を通しているが、実エンジンでの上限は未実測。W-11の疎通確認で長めの応答を試すとよい。

- [x] ずんだもんによるW-09実装（緑・race・ミューテーション実測まで完了、**自己完了扱いにはせずin_progress**）
- [x] つむぎによる独立検証: build/vet/gofmt OK、`go test ./tests/...` 83トップレベルPASS/0 FAIL、`test_race.sh`クリーン(DB: `zuncha_test_tsumugi`)。`internal/llm`/`internal/anthropic`無変更・httptestのみ使用を確認。WIPコミットで固定してそらへレビュー依頼
- [x] そらへW-09レビュー依頼 → ⚠要修正(指摘3件)

### そらW-09レビュー(e4d3b5b): ⚠要修正(指摘3件) (2026-08-10)

実測: 455 PASS/0 FAIL/0 SKIP(DB: `zuncha_test_sora`)、raceクリーン、`internal/llm`/`internal/anthropic`無変更、実VOICEVOX呼び出しなしを確認。FK制約撤去(D-4訂正2)は`pg_constraint`を直接照会し`conversation_id`のFKのみであることまで実測。speaker ID/VOICEVOX_BASE_URLの判断は妥当と評価。ミューテーション7件すべて赤、既存テストの退行なし。「孤児ファイル」の設計判断も妥当と評価。

**共有DB`zuncha_test`(旧スキーマのまま残存)の注意喚起**: そらの実測中、共有DBが`audio_files_message_id_fkey`付きの旧スキーマのまま放置されていることが判明(そら自身の環境ミスが発端で発覚)。`test_env.sh`を忘れた人が同じ罠を踏む可能性があるため、W-11前に削除または再作成が必要(申し送り)。

**指摘①【つむぎ担当・対応済み】`02_database_design.md`§4のER図が旧FK記述のまま**: §2.3・§3は更新済みだったが、ER図(L200/L206)だけ「1:1 (ON DELETE CASCADE)」「FK message_id TEXT ──→ messages.id」という撤去前の記述が残存。つむぎが直接修正済み。

**指摘②【ずんだもんへ差し戻し】`VOICEVOX_BASE_URL`のデフォルト補完が未検証**: lessons.md 2026-08-10の判定基準1(値が壊れてもエラーにならずフォールバックが黙って吸収するか)に該当。ポート打ち間違え等があっても起動は成功し、TTS失敗は非致命なので「文字は出るが一生無音」のまま気づけない。`TestLoadConfig`追加(未設定時にデフォルトURLが入ること、設定時はその値が優先されることの2ケース)を依頼。

**指摘③【ずんだもんへ差し戻し】TTS失敗がログに一切残らない**: `response_streamer.go`の`ttsErr`が握りつぶされ記録されない。W-08以前はttsClientがnilで即クラッシュしたため気づけたが、W-09でデフォルトURL+非致命化が揃った結果、VOICEVOXの停止やURL設定ミスが完全に無症状になる。`log.Printf`でのログ追加(発話内容は載せない)を依頼。

- [x] 指摘①対応(つむぎ直接修正)
- [ ] 指摘②③をずんだもんへ差し戻し
- [x] 共有DB`zuncha_test`の旧スキーマ問題への対応: つむぎが`DROP DATABASE zuncha_test`で削除。次回`ZUNCHA_TEST_DB_OWNER`未設定で使われた際は`create_test_db.sh`経由で新スキーマで再作成される

**そらが追加発見**: `zuncha_test_metan`も旧スキーマ(`message_id`のFK付き)のまま残存していた。`create_test_db.sh`は「`conversations`テーブルが存在するか」だけでスキーマ適用済みと判定するため、**DDL変更時に既存DBへ再実行しても何も反映されない**構造的な弱点があると判明。

- [x] `zuncha_test_metan`をDROP DATABASE→再作成(新スキーマ反映)
- [x] `create_test_db.sh`に注意書きを追記(DDL変更時は各自DROP DATABASEしてから再実行することを明記)
- [x] 申し送り: migration管理ツールの導入は今回は見送り(W-09の範囲外、規模に対して過剰)。DDL変更のたびに同じ取りこぼしが起きうる構造なので、次にDDLを変更するWaveでは「全員のテストDBを再作成する」手順を指示書に明記すること

### ずんだもんによる指摘②③対応・つむぎ独立検証完了 (2026-08-14)

**指摘②**: `cmd/api/main_test.go`に`TestLoadConfig`を追加(3ケース: 未設定時デフォルト・空文字列時デフォルト・設定時は優先)。`t.Setenv`+`os.Unsetenv`で「真に未設定」の状態を作りテスト。期待値はデフォルト定数だけでなく`"http://localhost:50021"`という具体値でも固定(定数側が書き換わった際に気づけるように)。

**指摘③**: `internal/service/response_streamer.go`でTTS失敗時に`log.Printf`で`conversation_id`/`message_id`/エラー内容を記録するよう変更。発話内容(`resp.Text`)は含めない。`tests/unit/response_streamer_test.go`に`TestResponseStreamer_TTS失敗はログに記録される`を追加し、①失敗理由と会話IDがログに残る②発話内容はログに出さない③TTS成功時は何もログに出さない、の3点を検証。

**つむぎ独立検証**: build/vet/gofmt OK、`go test ./tests/...`全緑、`go test ./cmd/...`で`TestLoadConfig`3ケースPASS、`test_race.sh`クリーン(DB: `zuncha_test_tsumugi`)。ログに発話内容が含まれないこともコード・テスト両面で確認。

**運用上の注意(記録)**: 今回、ずんだもんの並行作業中に`git add -A`でコミットしてしまい、`cmd/api/main_test.go`と`response_streamer_test.go`の変更が意図せず前のコミット(`a07f02a`)に混入した。実害はなかった(内容は正しく完成していた)が、以降はコミット前に`git status`で意図しないファイルが混ざっていないか確認すること。

- [x] 指摘②③対応・独立検証完了
- [ ] WIPコミットで再固定→そらへ再レビュー依頼

#### ずんだもんの実測記録(RED確認・ミューテーション)

**RED確認**: 指摘③のテストは実装前に実行し`Should NOT be empty`で赤になることを確認済み(`失敗理由と会話IDがログに残る`のみFAIL、他2サブテストは実装前でも緑=正しく失敗経路だけを突いている)。指摘②は既存挙動へのテスト追加のため素の実行では緑スタートになる。そこで`go test -overlay`(スキル`mutation-test-overlay`)で作業ツリー無改変のままガードを外し、検知力を実測した。

| # | ミューテーション | 結果 |
|---|---|---|
| M1 | `loadConfig`の`if c.voicevoxBaseURL == ""`フォールバックを削除 | 🔴 検知 (`actual: ""`) |
| M2 | `defaultVoicevoxBaseURL`を`:50022`に改竄 | 🔴 検知 (ポート差分を検出) |
| M3 | TTS失敗時の`log.Printf`を削除 | 🔴 検知 (ログ空) |
| M4 | ログに`text=%s`で発話内容を含める | 🔴 検知 (NF-SEC違反を検出) |
| M5 | 成否に関わらずログ出力(`if ttsErr != nil`の外へ移動) | 🔴 検知 (成功時ログ検出) |

5件すべて赤。検証後、一時ファイルは削除済み・`git status`に一時ファイルの残骸がないことを確認済み(作業ツリーは1バイトも改変していない)。

**全体実測**: `go test ./... -count=1 -v` → 471 PASS / 0 FAIL / 0 SKIP(DB: `zuncha_test_zundamon`をDROP→`create_test_db.sh`で新スキーマ再作成)、`test_race.sh`クリーン、`gofmt -l .`空、`go vet ./...` EXIT 0、`go build ./...`成功、`npm test` 95 PASS。ログにAPIキー・発話内容が含まれないことをM4で実測。

### そらW-09指摘②③再レビュー: ✅承認 — W-09完了 (2026-08-14)

実測: 459 PASS/0 FAIL/0 SKIP(DB: `zuncha_test_sora`、実DB照会で`message_id`のFK残存なしを確認)、`go test ./cmd/...`で`TestLoadConfig`3ケースPASS、raceクリーン。`internal/llm`・`internal/anthropic`は0ファイル変更を確認。指摘②(デフォルト補完の二重固定)・指摘③(ログ追加が既存の非致命セマンティクスを破壊していないこと)いずれも妥当と評価。

ミューテーション実測8件中7件検知、**M8(ログ行から`message_id`を削除)のみ検知不能**という限界を発見・申し送り。

**判定: ✅承認。** ただし「検知不能な限界を明記する」という自身の教訓(2026-07-29)に従い、M8を差し戻し事由にはせず記録のみとした。

**M8対応(つむぎ直接修正)**: `tests/unit/response_streamer_test.go`の既存テストに`assert.Contains(t, logged, streamerMessageID, ...)`を1行追加し、message_idの検証漏れを解消。全体テスト・build/vet/gofmt再確認済み。

- [x] W-09(VOICEVOX TTS実装) 完了(つむぎ最終ゲート: そら承認済み、M8の1行対応込み)。そらがM8ミューテーションを再実行し対応後は正しく赤化することまで実測確認済み(全体退行なし、`8eabd3f`のまま無改変)
- [x] 次タスク判断: W-10(Whisper.cpp STT実装)着手を決定(W-11はW-10依存のため先にW-10から)

### W-10着手前調査: whisper-server API仕様の確認・confidence算出方針決定 (2026-08-14)

W-10着手前に、whisper-serverの`/inference`エンドポイントのレスポンス形式を公式リポジトリ(ggml-org/whisper.cpp)で調査。`response_format=json`だと`{"text": "..."}`のみで信頼度相当の値が無いことが判明。`verbose_json`にすると`segments[].no_speech_prob`(無音確率)が得られる。

**ユーザー承認**: `confidence = 1 - max(segments[].no_speech_prob)`(複数segmentsの場合は最も無音確率が高い区間を全体のconfidenceとして採用、保守的に判定)。segments空なら`confidence=0`。`docs/04_implementation/04_realtime_wiring_design.md` D-3aとして記録。

`tasks/instructions_zundamon_wave_w10.md`作成: whisper-serverクライアント・ffmpeg変換(`internal/audioconv`、interface抽象化)・オーケストレーション層・`POST /conversations/{id}/stt`ハンドラの4点セット。**この開発環境にffmpegが未インストール**のため、ffmpeg依存部分はinterfaceで抽象化し、実際のffmpeg呼び出しテストは`exec.LookPath`で存在確認しskip可能な設計にする方針を明記。

- [x] whisper-server API仕様調査・confidence算出方針決定・指示書作成
- [ ] ずんだもんによるW-10実装
- [ ] そらへW-10レビュー依頼

### W-10実装(ずんだもん / 2026-08-14) — 実装完了・レビュー待ち

**新規**: `internal/whispercpp/client.go`(whisper-server HTTPクライアント)、`internal/audioconv/converter.go`(ffmpeg変換)、`internal/service/speech_to_text.go`(オーケストレーション)、`internal/handler/stt.go`(`POST /conversations/{id}/stt`)。
**変更**: `internal/handler/handler.go`(DI追加+ルート登録)、`cmd/api/main.go`(`WHISPER_SERVER_BASE_URL`+配線)、`cmd/api/main_test.go`、`tests/integration/handler_helpers_test.go`(フェイク2種追加)、`tests/integration/graceful_shutdown_test.go`(戻り値追随)。
**無変更確認済み**: `internal/llm` / `internal/anthropic` / `internal/tts` / `internal/voicevox` / `internal/stt`。

**RED実測**: スタブ実装(ゼロ値返し)を先に置いた状態でテストを走らせ、**62ケースがコンパイルエラーではなくアサーション失敗で赤**になることを確認してから実装した。既存テストは終始緑。

**確定事項からの逸脱1件(要確認)**: ffmpegの出力を「WAV直出し(`-f wav pipe:1`)」ではなく**生PCM(`-f s16le`)+ Go側でWAVヘッダ付与**にした。根拠は指示書§3.1の「推測で書かない」に従って一次情報を確認した結果:
- FFmpeg `libavformat/riffenc.c` の `ff_start_tag` はサイズに `-1`(0xFFFFFFFF)を書き、`wavenc.c` の `wav_write_trailer` は `AVIO_SEEKABLE_NORMAL` のときしか `ff_end_tag` で埋め戻さない → **パイプ出力ではRIFF/dataサイズが 0xFFFFFFFF のまま残る**。
- whisper.cpp が使う miniaudio(dr_wav) は 0xFFFFFFFF のサイズ欄を番兵として走査し直す実装のため、実害は無いことがそら(レビュー)の実測で判明(下記レビュー節参照)。ただしデコーダ側のこの番兵処理という実装依存の挙動に頼らず、サイズが正しい正準WAVを自前で組み立てる方が堅牢という判断は変わらない。
- 引数自体は whisper.cpp 本体の `convert_to_wav`(`ffmpeg -i <in> -y -ar 16000 -ac 1 -c:a pcm_s16le <out>`) に合わせた。D-3 の「16kHz mono WAV へ変換」という結論は変えていない(ヘッダを誰が書くかだけが違う)。

**ffmpeg未インストール環境のSkip**: 実ffmpeg依存テストは**2件がskip**(`TestConverter_実ffmpegでWAVへ変換できる` / `TestConverter_実ffmpegで壊れた入力はエラー`)。skip時は「緑ではなく未実行である」旨を`t.Log`で明示。ただし**偽ffmpegバイナリ(シェルスクリプト)経由の結合検証9件は常時実行**しており、引数列・stdin転送・stdout→WAV包装・stderr伝播・終了コード・空入力ガードは実測済み。実ffmpegとの引数互換だけがW-11(ffmpeg同梱イメージ)まで未実証。

**ミューテーション実測(9件すべて検知)**: M1 no_speech_prob最大→最小 / M2 最大→平均 / M3 `verbose_json`→`json` / M4 dataサイズ→0xFFFFFFFF / M5 ffmpeg変換スキップ(unit+integration両方) / M6 `IsRecognitionFailed`判定削除 / M7 サンプリングレート8kHz / M8 multipartフィールド名`audio`→`file` / M10 `cmd.Stdin`未接続。`go test -overlay`で実施し、一時ファイルは削除済み・`git status`に残骸なしを確認。

**全体実測**: `go test ./... -count=1` 全緑(524 PASS / 0 FAIL / 2 SKIP、DB: `zuncha_test_zundamon`)、`./scripts/test_race.sh` クリーン、`gofmt -l .` 空、`go vet ./...` EXIT 0、`go build ./...` 成功。

- [x] ずんだもんによるW-10実装(緑。完了判定はPM最終ゲート待ちのため未クローズ)
- [x] 逸脱1件(ffmpeg出力を生PCM+Go側WAVヘッダ付与に変更)をつむぎが承認: パイプ出力(非シーク可能)ではRIFF/dataチャンクサイズが確定できず0xFFFFFFFFのまま残る。一次情報(libavformat/riffenc.c, wavenc.cの挙動)に基づく判断で妥当と判断(※「miniaudioが巨大メモリ確保を試みる」という当初の理由はそらの実測で誤りと判明、後述のレビュー節参照。デコーダ側の番兵処理に頼らず正準WAVを自前で組み立てる方が堅牢、という結論自体は変わらない)
- [x] つむぎ独立検証: build/vet/gofmt OK、全緑(2 SKIP=ffmpeg未インストール環境の既知skip)、raceクリーン、`internal/llm`/`internal/anthropic`/`internal/tts`/`internal/voicevox`/`internal/stt`無変更を確認(DB: `zuncha_test_tsumugi`)。WIPコミット(e41c6f6)で固定
- [ ] そらへW-10レビュー依頼

**ずんだもんからの申し送り(そらへ転送済み)**: ①ミューテーション9件は`go test -overlay`実施のためレビュー対象コミットは無改変 ②未検証領域は「実ffmpegとの引数互換」(skip 2件)のみ。偽ffmpegバイナリ経由の結合検証で引数列・stdin/stdout・stderr・終了コードは実測済みだが、実バイナリが`-f s16le pipe:1`を期待どおり扱うかはW-11(ffmpeg同梱イメージ)で初めて実証される。**W-11の完了条件に実ffmpegでの往復変換テストが緑になることを含めること**(申し送り)。

### そらW-10レビュー(e41c6f6): ⚠要修正(指摘3件) (2026-08-14)

実測: 539 PASS/0 FAIL/2 SKIP(DB: `zuncha_test_sora`)、raceクリーン、保護対象5パッケージ無変更、実whisper-server非接続を確認。D-3a(confidence算出)は確定事項どおり実装済み。

**ffmpeg出力方式の逸脱を、実際にffmpeg 7.1をコンパイル・実行してソースレベルで検証**: パイプ出力WAVのRIFF/dataサイズが0xFFFFFFFFのまま残ることを実測確認(逸脱の前提は事実、承認可)。ただし「miniaudioが巨大メモリ確保を試みる」という当初の理由は誤りと判明: whisper.cppが使うdr_wavは0xFFFFFFFFを番兵として走査し直す実装のため実害はない(同一デコーダ設定でのコンパイル実測で確認)。逸脱自体は「デコーダ側の番兵処理に依存しない正準WAVを自前で作る方が堅牢」という理由で妥当と評価。

`WHISPER_SERVER_BASE_URL`未設定時のlog.Fatal判断、タイムアウト値(30秒/25秒)は妥当と評価。ミューテーション5件中4件検知、**M5(`sttMaxAudioBytes`を10MB→10GBに拡大)のみ未検知**という死角を発見。

**指摘①②【必須・つむぎ直接修正】コメント/文書の事実誤り訂正**: `converter.go`70-71行・`audioconv_test.go`71-72行・`tasks/todo.md`885/895行に複製されていた「miniaudioが巨大メモリ確保を試みる」という誤った理由を、実測結果に基づき訂正。`stt.go`22-23行のタイムアウト内訳コメントも、存在チェック・multipart受信が30秒ctxの外側であるという実態に合わせて訂正。

**指摘③【推奨・ずんだもんへ依頼】`sttMaxAudioBytes`を守るテストの追加**: 10MB+1バイトのmultipartが弾かれることを検証するテストが無い(M5で検知不能を実測)。413化の検討も推奨。

**申し送り(別Wave)**: `01_screen_design.md`§7.3にmultipartフィールド名`audio`の明記、`http.Server`の`ReadTimeout`未設定(既存の性質、W-10の退行ではない)、W-11完了条件に実ffmpeg往復変換テスト+opusデコーダ同梱確認を含めること。

- [x] 指摘①②対応(つむぎ直接修正、build/vet/gofmt/全緑を再確認)
- [x] 指摘③をずんだもんへ依頼

#### 指摘③対応(ずんだもん): `sttMaxAudioBytes`の境界テスト追加 + 413化 (2026-08-14)

**変更(2ファイル)**:
- `internal/handler/stt.go`: `readAudioUpload`で`ParseMultipartForm`のエラーを`errors.As(err, &*http.MaxBytesError)`で判別し、上限超過のみ **413 Payload Too Large** +`{"error":"音声データが大きすぎます"}`へ変更(それ以外は従来どおり400「音声データの形式が不正です」)。レスポンス形式は既存どおり`respondError`を使用し、他のエラーパスとの一貫性を維持。
- `tests/integration/handler_stt_test.go`: 境界テスト2本を追加。
  - `TestHandlerSTT_上限を超える音声は413で拒否し変換も認識も行わない`: `bytes.Repeat`で`sttMaxAudioBytes+1`バイトを1回だけ確保して送信 → 413、かつffmpeg変換(`recordedInputs()`空)・whisper呼び出し(`callCount()==0`)のどちらにも到達しないことを検証。
  - `TestHandlerSTT_上限ぎりぎりの音声は受け付ける`: 上限が不当に縮小された場合を検知するため境界の内側(`sttMaxAudioBytes - 1024`。1024はmultipartヘッダ/境界行の見積り。`MaxBytesReader`が数えるのはボディ全体で実測オーバーヘッドは250バイト前後)も200で固定。

**テスト側に契約値を書き写した理由**: `sttMaxAudioBytes`を実装から共有(export)すると、上限をいくら書き換えてもテストが追従して緑のままになりM5の死角が再発する。あえて二重に持ち、実装側の定数を変えたらテストが赤くなる形にした(テスト側にその旨コメント済み)。

**413化の判断(採用)**: ①`errors.As`で`*http.MaxBytesError`が確実に取れることをGo 1.24の`httptest`で実測確認済み(`ParseMultipartForm`が`*http.MaxBytesError`をそのまま返す) ②上限超過は「形式が不正(=クライアントのバグ)」とは別事象で、413ならフロントは「録音が長すぎた」と判別して回復動線を出せる ③フロントは現時点で`/stt`を未実装(`src`配下に呼び出しなし)、`01_screen_design.md`§7.3も成功時レスポンスしか規定していないため、既存契約を壊さない。**申し送り**: §7.3の追記(multipartフィールド名`audio`)を行う別Waveで、エラー時のステータス(400/404/413/500)も併記すること。

**実測**:
- RED確認: 413テストが`expected: 413 / actual: 400`で赤 → 実装後に緑(上限自体は元から効いていたが、ステータスが未区別だった)。
- `go test ./... -count=1` 全緑(DB: `zuncha_test_zundamon`)、`./scripts/test_race.sh` 緑(integration/unitともok)、`gofmt -l .`空、`go vet ./...` EXIT 0、`go build ./...`成功。
- ミューテーション実測(`go test -overlay`、作業ツリー無改変。一時ファイル削除済み・`git status`が対象2ファイルのみであることを確認): **4/4検知**
  - M1 `sttMaxAudioBytes`を10MB→10GB(そらのM5と同一): 413テストが赤(200) → **死角の解消を実証**
  - M2 10MB→1MB(縮小方向): ぎりぎりテストが赤(413)
  - M3 `http.MaxBytesReader`の行を削除: 413テストが赤(200)
  - M4 413分岐を削除(従来の400のみ): 413テストが赤(400)
