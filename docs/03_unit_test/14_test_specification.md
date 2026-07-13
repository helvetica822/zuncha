# 単体テスト仕様書 — フェーズ4（フロントエンドロジック、TypeScript/Svelte側）

| 項目 | 内容 |
|------|------|
| バージョン | 1.0 |
| 作成日 | 2026-07-09 |
| 作成者 | WhiteCUL（テスト仕様書・テストコード作成担当） |
| 入力 | `01_test_plan.md`（テスト計画）、`12_test_perspectives_phase4.md`（テスト観点、めたん作成）、`13_test_cases_phase4.md`（テストケース、ひまり作成） |
| 対象 | フェーズ4（3機能、純粋関数層／コンポーネント層分割を含む、計60件。TC-4-1-26は責務不整合のため欠番） |
| 次工程 | そらによるQAレビュー |

---

## 目次

1. [目的・対象](#1-目的対象)
2. [設計方針（フェーズ1〜3との違い、純粋関数層とコンポーネント層の使い分け）](#2-設計方針フェーズ13との違い純粋関数層とコンポーネント層の使い分け)
3. [ディレクトリ構成・型／関数シグネチャ一覧](#3-ディレクトリ構成型関数シグネチャ一覧)
4. [テストケース一覧](#4-テストケース一覧)
5. [テストケース総数の確認結果](#5-テストケース総数の確認結果)
6. [テストコード配置・実行方法](#6-テストコード配置実行方法)
7. [未決事項の反映状況（T〜Z・AA〜CC）](#7-未決事項の反映状況tzaacc)

---

## 1. 目的・対象

本書は `13_test_cases_phase4.md` で設計されたフェーズ4のテストケースを、実装着手前の正式な単体テスト仕様書として清書したものである。
対象は `01_test_plan.md` フェーズ4に定義された3機能であり、フェーズ1〜3（Go＋testify）とは異なりTypeScript/Svelte＋Vitest＋`@testing-library/svelte`が対象となるため、`12_test_perspectives_phase4.md` の指摘に基づき「純粋関数層（モック不要、または`vi.stubGlobal`によるブラウザAPIモックのみ）」と「コンポーネント層（`@testing-library/svelte`でマウントして検証）」に分離して構成する。

| 観点番号 | 機能 | 対応機能ID / 設計根拠 |
|---------|------|----------------------|
| 4-1 | 入力モード切替（マイク/テキスト）とlocalStorage保存 | 画面設計書「モード状態の永続化」 |
| 4-2 | 送信ボタンの活性制御 | 画面設計書8章 |
| 4-3 | エラー表現の出し分け（ずんだもん応答バブル vs 3秒トースト） | F-STT-02, F-STT-03 / 画面設計書9章 |

---

## 2. 設計方針（フェーズ1〜3との違い、純粋関数層とコンポーネント層の使い分け）

`12_test_perspectives_phase4.md` 前提・総括の指摘に基づき、以下の2階層構成を採用する。フェーズ1〜3で確立した「純粋ロジックはモック不要で軽量にテストする」という思想を、Svelteコンポーネントの世界に持ち込むための構成である。

### 純粋関数層（モック不要、またはブラウザAPI/タイマーのみモック）

- 「判定・状態管理ロジック」のみを担当する層。Svelteコンポーネントの`$:`宣言や`export let`直下には分岐を書かず、この層に切り出す。
- `localStorage`のような副作用を伴うブラウザAPIは、薄いラッパー関数（`getStoredInputMode` / `setStoredInputMode`）越しに呼び、テストでは`vi.stubGlobal('localStorage', mockStorage)`で完全に置き換える（7章Z決定）。
- 対象: `isInputMode`（4-1-1）、`getStoredInputMode` / `setStoredInputMode`（4-1-2）、`isSendButtonDisabled`（4-2-1）、`routeSSEEvent`（4-3-1）

### コンポーネント層（`@testing-library/svelte`でマウント）

- 「純粋関数層の戻り値がDOM・ユーザー操作に正しく結線されているか」を担当する層。内部state名やクラス名ではなく、ロール・表示テキストで検証する（`ts-svelte-coding-guideline.md`のテスト方針、`tdd-comprehensive.md`の「実装の詳細をテストしない」原則に整合）。
- 純粋関数層で異常系・境界値を厚く網羅済みのため、コンポーネント層は正常系の結線確認と、コンポーネントでしか検証できない例外系（トグル自体のdisabled化、3秒タイマーのアンマウント時クリア等）に絞る。
- 時刻・タイマーに依存する処理（4-3のトースト3秒消滅）は`vi.useFakeTimers()` + `vi.advanceTimersByTime()`で決定的に検証する（`tdd-comprehensive.md`の「実際の時間に依存しない」原則）。
- 対象: `ModeToggle.svelte`（4-1-3）、`SendButton.svelte`（4-2-2）、`Toast.svelte`（4-3-2）、`MessageBubble.svelte`（4-3-3）

| 層 | モック方針 | 対応するテストファイル |
|----|-----------|----------------------|
| 純粋関数層（4-1-1・4-1-2） | `localStorage`のみ`vi.stubGlobal`でモック化 | `tests/unit/frontend/input_mode.test.ts` |
| コンポーネント層（4-1-3） | `@testing-library/svelte`でマウント | `tests/unit/frontend/input_mode.test.ts`（同ファイル内で層を分けて配置） |
| 純粋関数層（4-2-1） | モック不要 | `tests/unit/frontend/send_button.test.ts` |
| コンポーネント層（4-2-2） | `@testing-library/svelte`でマウント | `tests/unit/frontend/send_button.test.ts`（同ファイル内） |
| 純粋関数層（4-3-1） | モック不要（呼び出し先ハンドラのみ`vi.fn()`でモック） | `tests/unit/frontend/error_routing.test.ts` |
| コンポーネント層（4-3-2・4-3-3） | `@testing-library/svelte` + `vi.useFakeTimers()` | `tests/unit/frontend/error_routing.test.ts`（同ファイル内） |

> フェーズ2の`tests/integration/`（実DB接続）のような、フェーズ4特有の別ディレクトリは不要と判断した。フェーズ4はいずれも「モック化されたブラウザAPI／モジュール」で完結し、実ブラウザ・実サーバーへの接続を要する層が存在しないため（結合テストは別フェーズ）。純粋関数層とコンポーネント層は同一ファイル内で`describe`ブロックを分けて配置することで、Go側の「ファイル分割」に相当する構造上の区別を保った。

---

## 3. ディレクトリ構成・型／関数シグネチャ一覧

Goの`internal/`パッケージ構成に相当する配置として、`ts-svelte-coding-guideline.md`の推奨構成（`src/lib/`＝ロジック、`src/components/`＝コンポーネント）を採用する。いずれもフェーズ4のGREEN phaseで新規作成する未実装のファイルである。

| パス | 役割 |
|------|------|
| `src/lib/inputMode.ts` | `InputMode`型、`isInputMode`型ガード、`getStoredInputMode`／`setStoredInputMode`ラッパー |
| `src/lib/sendButton.ts` | `InputState`型、`isSendButtonDisabled` |
| `src/lib/sseEventRouter.ts` | `SSEEvent`／`Emotion`型、`routeSSEEvent` |
| `src/components/ModeToggle.svelte` | 入力モード切替トグル |
| `src/components/SendButton.svelte` | 送信ボタン |
| `src/components/Toast.svelte` | エラートースト（3秒自動消滅） |
| `src/components/MessageBubble.svelte` | ずんだもん応答バブル |

### 3.1 `src/lib/inputMode.ts`

```typescript
export type InputMode = 'voice' | 'text';

export const INPUT_MODE_STORAGE_KEY = 'zuncha_input_mode';

// value is InputMode の型ガード。"voice"/"text"以外（大文字小文字違いを含む）はすべてfalse（7章T確定）。
export function isInputMode(value: unknown): value is InputMode;

// localStorageから読み出し、無効値・例外時は"voice"にフォールバックする（7章T・Z確定）。
export function getStoredInputMode(): InputMode;

// localStorageへ書き込む。例外は握り、呼び出し元に伝播させない（7章T確定）。
export function setStoredInputMode(mode: InputMode): void;
```

> `getStoredInputMode`／`setStoredInputMode`は`isInputMode`の判定結果に委譲し、コンポーネント側で個別にフォールバック分岐を書かない（`12_test_perspectives_phase4.md` 4-1責務分離の指摘）。`localStorage`アクセスはテストでは`vi.stubGlobal('localStorage', mockStorage)`で完全に置き換える独自の`Storage`モックを注入する（7章Z決定）。

### 3.2 `src/lib/sendButton.ts`

```typescript
import type { InputMode } from './inputMode';

// 録音中(recording)・送信中(sending)を含む状態機械。booleanフラグの組み合わせによる
// 矛盾状態（例：録音中かつ変換中）を型レベルで排除する（7章V・W確定）。
export type InputState = 'idle' | 'recording' | 'transcribing' | 'editable' | 'sending';

export interface IsSendButtonDisabledParams {
  mode: InputMode;
  text: string;
  inputState: InputState;
}

// disabled判定を集約する単一の純粋関数。
export function isSendButtonDisabled(params: IsSendButtonDisabledParams): boolean;
```

### 3.3 `src/lib/sseEventRouter.ts`

```typescript
// フェーズ3 internal/llm.LLMResponse.Emotion のフロント側表現。7種の値はフェーズ1-3の資産と一致させる。
export type Emotion = '喜び' | '怒り' | '悲しみ' | '楽しい' | '照れ' | '困惑' | 'ドヤ顔';

export type SSEEvent =
  | { type: 'error'; payload: { message?: string } }
  | { type: 'message'; payload: { text: string; emotion: Emotion } };

export interface SSEEventHandlers {
  onError: (message: string) => void;
  onMessage: (text: string, emotion: Emotion) => void;
}

// messageフィールド欠落・空文字列時のフォールバック文言（7章AA決定の暫定対応）。
export const DEFAULT_ERROR_MESSAGE = 'エラーが発生しました。もう一度お試しください。';

// SSEイベント種別に応じてonError／onMessageのいずれか一方のみを呼ぶ薄いルーティング関数。
// フロント独自の判定ロジックは持たない（7章X確定）。
export function routeSSEEvent(event: SSEEvent, handlers: SSEEventHandlers): void;
```

> `routeSSEEvent`は「振り分け」のみを責務とし、`error`イベントと`message`イベントを同時に処理しない排他性を持つ（TC-4-3-06・07で契約テスト化）。`message`イベントは、STT失敗由来の「困惑＋リトライ文言」であっても、通常応答と全く同一の`onMessage`呼び出しパスを通る（7章X確定、TC-4-3-03）。

### 3.4 `src/components/ModeToggle.svelte`（想定Props）

```typescript
interface ModeToggleProps {
  mode: InputMode;
  isRecording: boolean;
  isTranscribing: boolean;
  onModeChange: (mode: InputMode) => void;
}
```

> **責務の所在（そらのQAレビュー指摘反映）**: `ModeToggle`自身はテキスト入力欄（`textbox`）を持たない薄いコンポーネントであり、上記4プロパティ以外のpropを追加しない。7章U確定の「モード切替時、旧モードのテキスト入力欄をクリアする」責務は、`ModeToggle`と入力欄を組み合わせる呼び出し元の親コンポーネント（現時点では未設計）側が`onModeChange`コールバックを受けて実行する責務であり、`ModeToggle`内部の責務ではない。

### 3.5 `src/components/SendButton.svelte`（想定Props）

```typescript
interface SendButtonProps {
  disabled: boolean; // isSendButtonDisabled(...)の戻り値をそのまま渡す薄い結線
  onSubmit: () => void;
}
```

### 3.6 `src/components/Toast.svelte`（想定Props）

```typescript
interface ToastProps {
  message: string;
  durationMs?: number; // デフォルト3000
}
```

> 3秒後に自身をDOMから除去する（`setTimeout`を内部で保持し、`onDestroy`でクリアする。TC-4-3-10でアンマウント時のクリアを検証）。

### 3.7 `src/components/MessageBubble.svelte`（想定Props）

```typescript
interface MessageBubbleProps {
  text: string;
  emotion: Emotion;
}
```

---

## 4. テストケース一覧

`13_test_cases_phase4.md` のGiven/When/Thenをそのまま踏襲し、対応するTypeScript/Vitestテストコード（ファイル・テスト名）を突き合わせる。

### 4.1 4-1. 入力モード切替（26件、TC-4-1-26欠番）— `tests/unit/frontend/input_mode.test.ts`

#### 4.1.1 `isInputMode`（10件）

| ID | Given | Then | 対応テスト（`describe('isInputMode')` 内） |
|----|-------|------|------------------------------------------|
| TC-4-1-01 | `value = "voice"` | `true` | `it('TC-4-1-01: "voice"はtrueを返す')` |
| TC-4-1-02 | `value = "text"` | `true` | `it('TC-4-1-02: "text"はtrueを返す')` |
| TC-4-1-03 | `value = "foo"` | `false` | `it('TC-4-1-03: Union型にない文字列はfalseを返す')` |
| TC-4-1-04 | `value = "1"` | `false` | `it('TC-4-1-04: 数字文字列はfalseを返す')` |
| TC-4-1-05 | `value = ""` | `false` | `it('TC-4-1-05: 空文字列はfalseを返す')` |
| TC-4-1-06 | `value = null` | `false` | `it('TC-4-1-06: nullはfalseを返す')` |
| TC-4-1-07 | `value = undefined` | `false` | `it('TC-4-1-07: undefinedはfalseを返す')` |
| TC-4-1-08 | `value = 123` | `false` | `it('TC-4-1-08: 数値型はfalseを返す')` |
| TC-4-1-09 | `value = "Voice"` | `false` | `it('TC-4-1-09: 大文字小文字違いはfalseを返す（正規化しない）')` |
| TC-4-1-10 | `value = "TEXT"` | `false` | `it('TC-4-1-10: 大文字小文字違い（TEXT）はfalseを返す')` |

#### 4.1.2 `getStoredInputMode` / `setStoredInputMode`（10件）

| ID | Given | Then | 対応テスト |
|----|-------|------|-----------|
| TC-4-1-11 | `getItem`が`null`を返す | `"voice"` | `describe('getStoredInputMode')` 内 `it('TC-4-1-11: キー未存在時はvoiceを返す')` |
| TC-4-1-12 | `getItem`が`"text"`を返す | `"text"` | `it('TC-4-1-12: 保存値がtextならtextを返す')` |
| TC-4-1-13 | `getItem`が`"voice"`を返す | `"voice"` | `it('TC-4-1-13: 保存値がvoiceならvoiceを返す')` |
| TC-4-1-14 | モック注入 | `setItem("zuncha_input_mode", "text")`で呼ばれる | `describe('setStoredInputMode')` 内 `it('TC-4-1-14: textを渡すとsetItemがキーと値で呼ばれる')` |
| TC-4-1-15 | モック注入 | `setItem("zuncha_input_mode", "voice")`で呼ばれる | `it('TC-4-1-15: voiceを渡すとsetItemがキーと値で呼ばれる')` |
| TC-4-1-16 | `getItem`が`"foo"`を返す | `"voice"`にフォールバック | `it('TC-4-1-16: 無効値はvoiceにフォールバックする')` |
| TC-4-1-17 | `getItem`が空文字列を返す | `"voice"`にフォールバック | `it('TC-4-1-17: 空文字列はvoiceにフォールバックする')` |
| TC-4-1-18 | `getItem`が例外をthrow | `"voice"`を返す（クラッシュしない） | `it('TC-4-1-18: getItemが例外を投げてもvoiceを返す')` |
| TC-4-1-19 | `setItem`が例外をthrow | 例外が伝播しない | `it('TC-4-1-19: setItemが例外を投げても伝播しない')` |
| TC-4-1-20 | `getItem`が`"Voice"`を返す | `"voice"`にフォールバック | `it('TC-4-1-20: 大文字小文字違いもvoiceにフォールバックする（正規化しない）')` |

#### 4.1.3 `ModeToggle.svelte`（6件、TC-4-1-26は欠番）

| ID | Given | Then | 対応テスト |
|----|-------|------|-----------|
| TC-4-1-21 | `mode="voice"`でマウント | 「テキスト」クリックで`onModeChange("text")`、選択状態表示 | `describe('ModeToggle')` 内 `it('TC-4-1-21: テキストをクリックするとtextで変更コールバックが呼ばれる')` |
| TC-4-1-22 | `mode="text"`でマウント | 「マイク」クリックで`onModeChange("voice")`、選択状態表示 | `it('TC-4-1-22: マイクをクリックするとvoiceで変更コールバックが呼ばれる')` |
| TC-4-1-23 | `localStorage`に`"text"`保存済みでマウント | 「テキスト」が初期選択表示 | `it('TC-4-1-23: 保存済みの値が初期値としてvoiceより優先される')` |
| TC-4-1-24 | `isRecording=true`でマウント | 両ボタンが`disabled` | `it('TC-4-1-24: 録音中は両ボタンがdisabledになる')` |
| TC-4-1-25 | `isTranscribing=true`でマウント | 両ボタンが`disabled` | `it('TC-4-1-25: 変換中は両ボタンがdisabledになる')` |
| ~~TC-4-1-26~~ | ~~`mode="text"`・入力欄`"こんにちは"`でマウント~~ | ~~「マイク」クリックで入力欄が空にクリア~~ | **欠番**（そらのQAレビュー指摘反映。`ModeToggle`は入力欄を持たない薄いコンポーネントのため、`ModeToggle`単体のテストからは削除。「モード切替時に旧モードの入力をクリアする」仕様（7章U確定）自体は取り下げず、入力欄を保持する親コンポーネント（現時点では未設計）の設計時に別途テストケース化する） |
| TC-4-1-27 | `mode="voice"`でマウント | テキスト→マイク→テキスト→マイクの順にコールバックが呼ばれ、最終値が一致 | `it('TC-4-1-27: 高速連続操作でも最終的な変更コールバックの順序が一致する')` |

### 4.2 4-2. 送信ボタンの活性制御（19件）— `tests/unit/frontend/send_button.test.ts`

#### 4.2.1 `isSendButtonDisabled`（16件）

| ID | Given | Then | 対応テスト（`describe('isSendButtonDisabled')` 内） |
|----|-------|------|----------------------------------------------------|
| TC-4-2-01 | `mode="voice"`, `inputState="editable"` | `false` | `it('TC-4-2-01: 音声モードで録音完了後は有効')` |
| TC-4-2-02 | `mode="text"`, `editable`, `text="こんにちは"` | `false` | `it('TC-4-2-02: テキストモードで非空文字列は有効')` |
| TC-4-2-03 | `mode="text"`, `editable`, `text="STTにより反映された文字列"` | `false` | `it('TC-4-2-03: STT反映後の非空文字列は有効')` |
| TC-4-2-04 | `mode="voice"`, `inputState="idle"` | `true` | `it('TC-4-2-04: 音声モードで録音前待機中は無効')` |
| TC-4-2-05 | `mode="text"`, `editable`, `text=""` | `true` | `it('TC-4-2-05: 空文字列は無効')` |
| TC-4-2-06 | `mode="text"`, `editable`, `text="   "` | `true` | `it('TC-4-2-06: 半角スペースのみは無効')` |
| TC-4-2-07 | `mode="text"`, `inputState="transcribing"`, `text="変換中の暫定文字列"` | `true` | `it('TC-4-2-07: 変換中は入力内容に関わらず無効')` |
| TC-4-2-08 | `mode="voice"`, `inputState="recording"` | `true` | `it('TC-4-2-08: 録音中は無効（7章V確定）')` |
| TC-4-2-09 | `mode="text"`, `inputState="sending"`, `text="こんにちは"` | `true` | `it('TC-4-2-09: 送信中は有効な文字列があっても無効（7章W確定）')` |
| TC-4-2-10 | `mode="voice"`, `inputState="sending"` | `true` | `it('TC-4-2-10: 送信中はモードを問わず無効')` |
| TC-4-2-11 | `mode="text"`, `editable`, `text="あ"` | `false` | `it('TC-4-2-11: 1文字のみでも有効（空でない最小境界）')` |
| TC-4-2-12 | `mode="text"`, `editable`, `text="\t"` | `true` | `it('TC-4-2-12: タブのみは無効')` |
| TC-4-2-13 | `mode="text"`, `editable`, `text="\n"` | `true` | `it('TC-4-2-13: 改行のみは無効')` |
| TC-4-2-14 | `mode="text"`, `editable`, `text="　"` | `true` | `it('TC-4-2-14: 全角スペースのみは無効')` |
| TC-4-2-15 | `mode="text"`, `editable`, `text=" \t\n　 "` | `true` | `it('TC-4-2-15: 空白文字混在は無効')` |
| TC-4-2-16 | `mode="text"`, `editable`, `text="あ".repeat(1000)` | `false` | `it('TC-4-2-16: 1000文字でも文字数上限による無効化はない')` |

#### 4.2.2 `SendButton.svelte`（3件）

| ID | Given | Then | 対応テスト（`describe('SendButton')` 内） |
|----|-------|------|-------------------------------------------|
| TC-4-2-17 | `disabled=false`でマウント | ボタンに`disabled`属性がない | `it('TC-4-2-17: disabled=falseのときボタンは有効')` |
| TC-4-2-18 | `disabled=true`でマウント | ボタンに`disabled`属性がある | `it('TC-4-2-18: disabled=trueのときボタンは無効')` |
| TC-4-2-19 | `disabled=true`でマウント | クリックしても`onSubmit`が呼ばれない | `it('TC-4-2-19: disabled状態でクリックしてもonSubmitは呼ばれない')` |

### 4.3 4-3. エラー表現の出し分け（15件）— `tests/unit/frontend/error_routing.test.ts`

#### 4.3.1 `routeSSEEvent`（7件）

| ID | Given | Then | 対応テスト（`describe('routeSSEEvent')` 内） |
|----|-------|------|------------------------------------------------|
| TC-4-3-01 | `error`イベント、`message`あり | `onError`が1回、`onMessage`は呼ばれない | `it('TC-4-3-01: errorイベントはonErrorのみを呼ぶ')` |
| TC-4-3-02 | 通常応答（`text`/`emotion="喜び"`） | `onMessage`が引数付きで呼ばれる、`onError`は呼ばれない | `it('TC-4-3-02: 通常応答はonMessageのみを呼ぶ')` |
| TC-4-3-03 | 困惑・リトライ文言の応答（`emotion="困惑"`） | TC-4-3-02と同一の`onMessage`呼び出しパス | `it('TC-4-3-03: 困惑・リトライ文言も通常応答と同一パスで処理される（ネガティブテスト）')` |
| TC-4-3-04 | `error`イベント、`message`フィールド欠落 | `onError`がデフォルトメッセージで呼ばれる（7章AA決定） | `it('TC-4-3-04: messageフィールド欠落時はデフォルトメッセージにフォールバックする')` |
| TC-4-3-05 | `error`イベント、`message=""` | `onError`がデフォルトメッセージで呼ばれる（7章AA決定） | `it('TC-4-3-05: messageが空文字列でもデフォルトメッセージにフォールバックする')` |
| TC-4-3-06 | `error`イベント | `onMessage`が一度も呼ばれない | `it('TC-4-3-06: errorイベントでonMessageは呼ばれない（排他性）')` |
| TC-4-3-07 | 通常応答イベント | `onError`が一度も呼ばれない | `it('TC-4-3-07: 通常応答でonErrorは呼ばれない（逆方向排他性）')` |

#### 4.3.2 `Toast.svelte`（6件）

| ID | Given | Then | 対応テスト（`describe('Toast')` 内） |
|----|-------|------|----------------------------------------|
| TC-4-3-08 | `message="サーバーエラーが発生しました"`でマウント | `role="alert"`に表示 | `it('TC-4-3-08: propsのmessageがalertロールに表示される')` |
| TC-4-3-09 | マウント後 | `advanceTimersByTime(3000)`でDOMから消滅 | `it('TC-4-3-09: 3秒後にDOMから消滅する')` |
| TC-4-3-10 | 表示中に`unmount()` | タイマーがクリアされ警告が出ない | `it('TC-4-3-10: アンマウント時にタイマーがクリアされ警告が出ない')` |
| TC-4-3-11 | マウント後 | `advanceTimersByTime(2999)`ではまだ存在 | `it('TC-4-3-11: 2999msの時点ではまだDOMに存在する')` |
| TC-4-3-12 | マウント後 | `advanceTimersByTime(3000)`で消滅（TC-4-3-11との対比） | `it('TC-4-3-12: ちょうど3000msの時点で消滅している')` |
| TC-4-3-13 | 短時間で2回連続表示 | 1件目が上書きされ多重表示にならない（7章BB決定） | `it('TC-4-3-13: 連続表示時は上書きされ多重表示にならない（暫定仕様）')` |

#### 4.3.3 `MessageBubble.svelte`（2件）

| ID | Given | Then | 対応テスト（`describe('MessageBubble')` 内） |
|----|-------|------|------------------------------------------------|
| TC-4-3-14 | `emotion="喜び"`, `text="こんにちは"` | 会話ログにずんだもん発話として表示 | `it('TC-4-3-14: 通常応答がずんだもん発話として表示される')` |
| TC-4-3-15 | `emotion="困惑"`, `text="もう一度お願いします"` | TC-4-3-14と同一のコンポーネントパスで表示（ネガティブテスト） | `it('TC-4-3-15: 困惑・リトライ文言も通常応答と同一コンポーネントパスで表示される')` |

---

## 5. テストケース総数の確認結果

`13_test_cases_phase4.md` のサマリー「計61件（4-1: 27件、4-2: 19件、4-3: 15件）」について、全テストケースIDを機械的に突合した結果、記載通り**61件**（重複なし）であることを確認した。フェーズ1（67→77件）で発生した集計誤りの教訓を踏まえ、着手前の確認を徹底した。

**2026-07-09追記（そらのQAレビュー指摘反映）**: TC-4-1-26が`ModeToggleProps`の責務範囲（§3.4）と矛盾していたため、`ModeToggle`単体のテストケースから削除（欠番）した。これにより本フェーズの実施対象は**60件**に確定する（詳細は[7章](#7-未決事項の反映状況tzaacc)参照）。

| 観点 | 件数 |
|------|------|
| 4-1 入力モード切替（純粋関数層19件＋コンポーネント層6件、TC-4-1-26欠番） | 26件 |
| 4-2 送信ボタン活性制御（純粋関数層16件＋コンポーネント層3件、計19件） | 19件 |
| 4-3 エラー表現出し分け（ルーティング7件＋Toast6件＋応答バブル2件、計15件） | 15件 |
| **合計（実施対象）** | **60件** |

---

## 6. テストコード配置・実行方法

### 配置

```
zuncha/
├── package.json          # フロントエンド用スカフォールド（新規）
├── tsconfig.json         # 新規
├── vitest.config.ts      # 新規
├── go.mod
├── src/
│   ├── lib/              # 未実装（GREEN phaseで作成）
│   │   ├── inputMode.ts
│   │   ├── sendButton.ts
│   │   └── sseEventRouter.ts
│   └── components/       # 未実装（GREEN phaseで作成）
│       ├── ModeToggle.svelte
│       ├── SendButton.svelte
│       ├── Toast.svelte
│       └── MessageBubble.svelte
├── internal/              # フェーズ1〜3（Go側、未実装）
└── tests/
    └── unit/
        ├── (フェーズ1〜3の既存Goテストファイル)
        └── frontend/
            ├── input_mode.test.ts     # 4-1（26件、TC-4-1-26欠番）
            ├── send_button.test.ts    # 4-2（19件）
            └── error_routing.test.ts  # 4-3（15件）
```

Go側テスト（`tests/unit/*.go`）とTypeScript側テスト（`tests/unit/frontend/*.test.ts`）は拡張子とサブディレクトリの両方で明確に区別されるため、`go test ./tests/unit/...`が`frontend/`配下を誤って対象にすることはない。

### 実行方法（GREEN phase以降）

```bash
npm install
npm test
```

### 現状（RED phase）

`src/lib/` / `src/components/` のいずれも未実装のため、上記コマンドは各テストファイル内のimport文でモジュール解決エラーとなり実行できない。これはTDDのRED状態として意図した挙動である。

---

## 7. 未決事項の反映状況（T〜Z・AA〜CC）

### 7.1 確認事項T〜Z（`12_test_perspectives_phase4.md` 7章、つむぎ決定済み）

本書・テストコードは、めたんが `12_test_perspectives_phase4.md` に反映済みのT〜Z決定を前提として作成した。以下、各決定内容と本書での反映箇所を再掲する。

| # | 決定内容 | 本書での反映 |
|---|---------|-------------|
| T | `localStorage`無効値・例外はデフォルト`"voice"`にフォールバック。正規化はしない | `isInputMode`異常系・境界値（TC-4-1-03〜10）、`getStoredInputMode`異常系・境界値（TC-4-1-16〜20） |
| U | 録音中・変換中はモード切替トグル自体をdisabled化。切替時は旧モード入力をクリア | `ModeToggle`例外系（TC-4-1-24〜26） |
| V | 「録音中」も無効化対象（有効化は録音完了後のみ） | `isSendButtonDisabled`（TC-4-2-04, 08）、`InputState`に`recording`を含める設計 |
| W | 送信中（クリック後〜`done`受信まで）も無効化 | `isSendButtonDisabled`（TC-4-2-09, 10）、`InputState`に`sending`を含める設計 |
| X | 単純ルーティングで正しい。フロント独自の判定ロジックは実装しない。F-RT-02はスコープ外 | `routeSSEEvent`を薄いルーティング関数として設計（3.3節）、F-RT-02関連ケースなし |
| Y | ①ルーティング関数、②表示コンポーネント（Toast・応答バブル）の両方をテスト対象とする | 4.3節を4-3-1／4-3-2／4-3-3の3層で構成 |
| Z | `localStorage`は`vi.stubGlobal`で独自の`Storage`モックに完全置換 | `getStoredInputMode`/`setStoredInputMode`のテスト方針として3.1節に明記 |

### 7.2 確認事項AA・BB・CC（`13_test_cases_phase4.md` 末尾、つむぎ決定済み）

| # | 対象 | つむぎの判断 | 本書・テストコードへの反映 |
|---|------|-------------|--------------------------|
| AA | 4-3 | 暫定対応（`message`欠落・空文字列時はデフォルトメッセージへフォールバック）のまま確定。変更なし | `routeSSEEvent`に`DEFAULT_ERROR_MESSAGE`定数を設け、TC-4-3-04・05のフォールバック期待値として反映（3.3節） |
| BB | 4-3 | 暫定対応（トースト連続表示時は上書き方式、キューイングなし）のまま確定。シンプルさ優先で変更なし | TC-4-3-13を「1件目が2件目に上書きされ多重表示にならない」という暫定仕様のテストとして反映 |
| CC | 4-1×4-2×4-3の状態競合 | スコープ外（結合テスト以降で対応）として承認。本フェーズでは対応不要、ケース化しない | 本書・テストコードにケース化していない。将来の結合テスト／E2Eテストフェーズへの申し送り事項として本節に記録するのみ |

### 7.3 そらのQAレビュー指摘②（TC-4-1-26と`ModeToggleProps`の責務不整合）

| 対象 | つむぎの判断 | 本書・テストコードへの反映 |
|------|-------------|--------------------------|
| TC-4-1-26 | 責務分離を優先。`ModeToggleProps`（`mode`/`isRecording`/`isTranscribing`/`onModeChange`のみ）を正としてpropを追加しない。TC-4-1-26は`ModeToggle`単体のテストから削除し、欠番とする | §3.4のProps定義に「`ModeToggle`は入力欄を持たない」旨の注記を追加。§4.1.3の該当行を欠番として明示。本フェーズの実施対象を61件→**60件**に更新（本節冒頭・5章参照） |

> 「モード切替時、旧モードのテキスト入力欄をクリアする」という7章U決定自体の仕様は取り下げない。この責務は`ModeToggle`と入力欄を組み合わせる親コンポーネント（テキスト入力欄の状態を保持する側）にあると整理し、当該コンポーネントの設計時に別途テストケース化する。

---

## 変更履歴

### 2026-07-09

初版作成。`13_test_cases_phase4.md` を正式な単体テスト仕様書として清書。テストケース件数の実数確認（61件、差異なし）を実施し記録。

### 2026-07-09（そらのQAレビュー指摘反映）

指摘②を反映。TC-4-1-26が§3.4の`ModeToggleProps`定義（入力欄propを持たない）と矛盾していたため、`ModeToggle`単体のテストケースから削除し欠番とした。本フェーズの実施対象テストケース数を61件→60件に更新。§3.4に責務の所在（親コンポーネント側の責務である旨）を注記として追加。

*記録: WhiteCUL*
