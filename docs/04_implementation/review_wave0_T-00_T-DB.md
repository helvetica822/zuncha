# QAレビュー結果: Wave 0 (T-00 / T-DB / FEクラスタ1・2)

| 項目 | 内容 |
|------|------|
| レビュアー | 九州そら (品質管理) |
| レビュー日 | 2026-07-21 |
| 対象 | T-00 骨組み / T-DB migration / FEクラスタ1 (T-16+ModeToggle) / FEクラスタ2 (T-17+SendButton) |
| 基準資料 | `docs/04_implementation/01_implementation_plan.md`、`docs/02_functional_design/02_database_design.md`、`docs/03_unit_test/14_test_specification.md`、`tests/` |
| 総合判定 | T-00: **✅承認** / T-DB: **✅承認** / FEクラスタ1: **⚠要修正** (キー名1点) / FEクラスタ2: **✅承認** |

---

## 対象① T-00: ディレクトリ骨組み — ✅承認

### 確認事項

| 観点 | 結果 |
|------|------|
| `internal/` 11パッケージ + `doc.go` | ✅ gc/llm/localfs/model/postgres/repository/service/sse/stt/tts/validation の全11パッケージに `doc.go` 存在。計画書1.1のパッケージ構成と完全一致 |
| パッケージコメント形式 (規約14.1) | ✅ 全ファイルが `// Package xxx は…` 形式で対象名から開始。規約準拠 |
| `cmd/api/main.go` | ✅ 空 `func main(){}` のスタブ。計画書上「テスト対象外・GREEN完了時に用意」の位置づけに合致 |
| 配置 (ルート直下) | ✅ 計画書0章のテスト契約準拠配置 (`internal/`・`src/` をルート直下) と一致 |
| `go build ./...` | ✅ exit 0 |
| `go vet ./internal/... ./cmd/...` | ✅ exit 0 |
| `gofmt -l` 差分 | ✅ 差分なし |
| unit テスト RED 状態 | ✅ `undefined: model.Conversation` 等のクリーンRED。「no Go files」ではなくパッケージは解決済み・シンボル未定義の正しいRED |

### 申し送り (指摘ではない・情報)

- `src/lib/`・`src/components/` は依頼記載の `.gitkeep` ではなく、既に T-16 成果物 (`inputMode.ts`・`ModeToggle.svelte`) に置換済み (T-16 は完了タスク)。骨組みディレクトリとしての役割は果たされており、テストのインポートパス解決に問題なし。**T-00 の合否には影響しない。**

---

## 対象② T-DB: migration + 生成列修正 — ✅承認

### 設計書 (02) との整合

| 観点 | 結果 |
|------|------|
| 3テーブル定義 (conversations/messages/audio_files) | ✅ 列・型・制約が設計書2章と一致 |
| CASCADE 外部キー | ✅ messages→conversations、audio_files→conversations/messages に `ON DELETE CASCADE` |
| role CHECK | ✅ `('user','assistant')` |
| emotion CHECK (7種 + NULL可) | ✅ `喜び/怒り/悲しみ/楽しい/照れ/困惑/ドヤ顔` |
| インデックス5種 | ✅ expires_at / (conversation_id, created_at DESC) / fetched_at 部分index / audio FK補助×2 |
| down.sql | ✅ 子→親順 (audio_files→messages→conversations) の `DROP TABLE IF EXISTS` でFK依存を尊重 |

### 生成列修正の正当性 (最重要レビュー観点)

- **変更**: `started_at + INTERVAL '30 days'` → `(started_at AT TIME ZONE 'UTC' + INTERVAL '30 days') AT TIME ZONE 'UTC'`
- **妥当性**: `timestamptz + interval` (日/月単位) はセッションTZ・DST依存のため STABLE 扱いで、生成列が要求する IMMUTABLE を満たさずエラーになる。UTCへ一旦 `timestamp` 変換 → 加算 → `timestamptz` 復元 の構成は全段 IMMUTABLE。**修正は根本原因に正しく対処しており妥当。**
- **意味論**: UTC固定のため厳密に 30×24h 後を生成する。naive版とはDST跨ぎで最大1時間差が出るが、GCは絶対時刻比較 (`WHERE expires_at < $1`) のため一貫性があり、TTL用途としてむしろ決定的で望ましい。

### テスト契約との整合

- `tests/integration/helpers_test.go` のINSERT列と照合:
  - conversations `(id, started_at)` … expires_atは生成列につき非指定。計画書 R-6 (「InsertConversationはexpires_atを明示指定しない」) と一致 ✅
  - messages `(id, conversation_id, role, content, emotion, created_at)` … 全列一致 ✅
  - audio_files `(id, conversation_id, message_id, file_path, fetched_at)` … 一致 (created_atはDEFAULT委譲) ✅

### 実DB検証 (PostgreSQL 16 で実施)

| 検証 | 結果 |
|------|------|
| up.sql 適用 | ✅ 全 CREATE 成功 (生成列のIMMUTABLEエラーなし) |
| 生成列 diff | ✅ `2026-01-01+00` → `2026-01-31+00` = 正確に30日 |
| role CHECK違反 (`bot`) | ✅ 拒否 (`messages_role_check`) |
| emotion CHECK違反 (`happy`) | ✅ 拒否 (`messages_emotion_check`) |
| CASCADE削除 | ✅ conversations削除で messages/audio_files が0件に連鎖削除 |
| インデックス | ✅ 5種 + pkey×3 生成確認 |
| down.sql | ✅ 全テーブルDROP、public に0テーブル復元 |

---

## 対象③ FEクラスタ1: T-16 (inputMode.ts) + ModeToggle.svelte — ⚠要修正 (1点)

### 仕様書 (§4.1) / テスト契約との整合

| 観点 | 結果 |
|------|------|
| `isInputMode` (10件) | ✅ 正規化しない厳密判定 (`=== 'voice' \|\| === 'text'`)。TC-4-1-01〜10 の Given/Then と一致 |
| `getStoredInputMode` (フォールバック) | ✅ `isInputMode` で判定し無効値・例外時 `'voice'`。TC-4-1-11〜13/16〜18/20 一致 |
| `setStoredInputMode` (例外握り潰し) | ✅ try/catch で伝播させない。TC-4-1-14/15/19 一致 |
| `ModeToggle` props/挙動 | ✅ mode/isRecording/isTranscribing/onModeChange。コールバックprop方式、aria-pressed、録音/変換中disabled、入力欄prop無し (R-9/QA指摘②準拠、TC-4-1-26欠番の整理と一致) |
| テスト実行 | ✅ `vitest run input_mode.test.ts` → 26 passed 全緑 |
| TS/Svelte 規約 | ✅ 型明示・`any`不使用・`unknown`+型ガード・script/markup/style順序・props型定義。規約準拠 |

### 設定補完の妥当性

| 変更 | 評価 |
|------|------|
| `vitest.config.ts` に `vitePreprocess()` 追加 | ✅ `.svelte` の `<script lang="ts">` 解釈に必須。妥当 |
| `tsconfig.json` の include に `vitest-setup.ts` 追加 | ✅ jest-dom 型解決のため。妥当 |
| `setupFiles` 不変更 | ✅ R-8 (jest-dom登録スカフォールド維持) 遵守 |

### ⚠要修正 (1点): localStorageキー名の不整合

- **不整合**: 実装 `INPUT_MODE_STORAGE_KEY = 'zuncha:input-mode'` に対し、仕様書 §4.1.2 TC-4-1-14/15 は `setItem("zuncha_input_mode", ...)` を明示。両者が食い違っている。テストコードは定数参照のため緑になるが、**仕様書のリテラル値を検証しておらず不整合が顕在化しない (偽陽性的緑)**。
- **是正方針 (ユーザー確定)**: キー名を **`zuncha:inputMode`** に統一する (コロン名前空間 + camelCase)。localStorageキーはオリジン単位共有のためアプリ固有プレフィックスでの名前空間化が必須級のベストプラクティスであり、コロン区切りは最も普及した慣例 (Redis由来) に合致する。
  - 実装: `INPUT_MODE_STORAGE_KEY` の値を `'zuncha:input-mode'` → `'zuncha:inputMode'` に修正 (実装担当対応)。
  - 仕様書: §4.1.2 TC-4-1-14/15 のリテラルを `"zuncha:inputMode"` に修正。
  - テスト: `INPUT_MODE_STORAGE_KEY` 定数参照のため変更不要 (修正後も緑を維持できる)。
- **命名規約 (申し送り・全体方針)**: 今後のアプリ内の永続化キーはすべて **`zuncha:<camelCaseName>`** 形式 (コロン名前空間 + camelCase) で統一する。将来のキー追加時 (例: 送信状態・SSE設定等) の基準とする。

### 申し送り (軽微・合否に影響しない)

- `ModeToggle` の `selected` は mount 時 `let selected = mode` の初期化のみで、`mode` prop の後続変更に追従しない (楽観的更新の設計意図はコメント済)。現テスト契約 (初期 mount 検証のみ) では問題なし。親が `mode` を再制御する設計になった場合は `$: selected = mode` 等の追従要否を再検討すること。

---

## 対象④ FEクラスタ2: T-17 (sendButton.ts) + SendButton.svelte — ✅承認

### 仕様書 (§4.2) / テスト契約との整合

| 観点 | 結果 |
|------|------|
| `isSendButtonDisabled` (16件) | ✅ editable以外は常に無効 / voice+editableは空でも有効 / text+editableは `text.trim() === ''` で無効。TC-4-2-01〜16 の Given/Then と完全一致 |
| 空白判定 (JS標準 `trim()`) | ✅ 半角スペース・タブ`\t`・改行`\n`・全角スペースU+3000・混在をすべて無効判定 (TC-4-2-06/12/13/14/15)。上限なし (TC-4-2-16: 1000文字で有効) |
| 状態優先 | ✅ recording/transcribing/sending は text 有無・mode を問わず無効 (TC-4-2-07〜10) |
| `SendButton` props/挙動 | ✅ disabled/onSubmit、アクセシブル名「送信」、disabled属性反映、防御的クリックガードで disabled時 onSubmit不発火 (TC-4-2-17〜19) |
| テスト実行 | ✅ `vitest run send_button.test.ts` → 19 passed 全緑 |
| TS/Svelte 規約 | ✅ 型明示・`interface SendButtonParams`・アロー関数・判定関数は `is` 接頭辞・script/markup/style順序。規約準拠 |

### 設定変更 (vitest.config.ts 冒頭コメント) の妥当性

- RED phase用スカフォールド記述 → 「実装を順次追加中」の実態に沿ったコメントへ更新。**コメントのみの変更で挙動に影響なし。** 回帰 (input_mode + send_button) 45 passed で無退行を確認。妥当。

### 回帰実行についての注記 (指摘ではない)

- `tests/unit/frontend/` 一括実行では `error_routing.test.ts` が Test File 単位で失敗するが、原因は `../../../src/lib/sseEventRouter` 未解決 = **T-18 (SSEイベントルーティング) 未実装によるクリーンRED**。FEクラスタ2 の合否とは無関係。実装済み2ファイルの 45 tests は全緑。

---

## 結論

- **T-00: ✅承認** — 骨組み・規約・ビルド・RED状態すべて適合。
- **T-DB: ✅承認** — 設計書準拠、生成列修正は妥当、テスト契約一致、実DBでエンドツーエンド検証済み。
- **FEクラスタ1 (T-16+ModeToggle): ⚠要修正** — 実装・テスト・設定・規約はいずれも適合水準だが、localStorageキー名の仕様書不整合1点の是正が必要。キー名を `zuncha:inputMode` に統一 (実装値 + 仕様書§4.1.2)、テストは定数参照のため変更不要。
- **FEクラスタ2 (T-17+SendButton): ✅承認** — 仕様§4.2と完全一致、19件全緑、規約準拠、設定変更はコメントのみ。指摘なし。

T-00/T-DB/FEクラスタ2 は後続タスク着手を妨げる問題なし。FEクラスタ1 はキー名是正 (実装担当対応) を条件に承認。
