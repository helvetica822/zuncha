# 実装指示書: 親コンテナ `ConversationView` (IT-3 本体)

発行: 四国めたん (テックリード) / 2026-07-28
宛先: ずんだもん (実装担当)
根拠設計書: `docs/04_implementation/02_parent_container_design.md`(§8確定事項込み)
根拠画面設計: `docs/02_functional_design/01_screen_design.md` 9-1 / 9-2 / 9-3

---

## 0. 進め方(厳守)

1. **RED**: 先に `tests/unit/frontend/conversation_view.test.ts` を全ケース書き、`npx vitest run tests/unit/frontend/conversation_view.test.ts` が **失敗すること**(import解決不能 or FAIL)を確認・報告。
2. **GREEN**: `src/components/ConversationView.svelte` を実装し、全ケースをパス。
3. **検証**: `npx vitest run tests/unit/frontend/`(既存60件 + 新規、全緑)と `npx tsc --noEmit`(EXIT 0)。
4. 完了報告はするが、**タスクボードを自分で `completed` にしないこと**(完了判定はつむぎ/そら承認後)。

---

## 1. 追加・変更するファイル

| ファイル | 種別 | 内容 |
|---|---|---|
| `src/components/ConversationView.svelte` | 新規 | 親コンテナ本体 |
| `tests/unit/frontend/conversation_view.test.ts` | 新規 | TC-P-01〜TC-P-19 |
| `src/components/MessageBubble.svelte` | 微修正 | ルート div に `data-emotion={emotion}` を追加(下記§2-e) |

**それ以外の既存ファイル(`src/lib/*.ts`・`ModeToggle`・`SendButton`・`Toast`)は一切変更しないこと。** Props契約は凍結ですわ。

---

## 2. `ConversationView.svelte` 仕様(これがコンテナの契約)

### a. 定数(すべてマジックナンバー禁止・ファイル冒頭に定義)

```ts
const MAX_MESSAGES = 20;                       // 全件表示・最大20件(設計書§8)
const TEXT_INPUT_LABEL = 'メッセージ';          // textarea の aria-label(テストの参照点)
const STT_FAILURE_MESSAGE = 'うまく聞き取れなかったのだ。もう一度話しかけてほしいのだ';
const STT_TIMEOUT_MESSAGE = 'あれ、聞こえなくなってしまったのだ。もう一度話しかけてほしいのだ';
const STT_FAILURE_EMOTION: Emotion = '困惑';
```

文言は画面設計 9-1 / 9-2 の「テキスト例」を**そのまま**使うこと(句点なしの末尾も原文どおり)。

### b. 状態(単一の真実の源泉)

```ts
interface AssistantMessage { id: number; text: string; emotion: Emotion }

let mode: InputMode = getStoredInputMode();
let text = '';
let inputState: InputState = 'editable';
let toastMessage: string | null = null;
let messages: AssistantMessage[] = [];
let nextMessageId = 0;   // each の key 用の単調増加ID(乱数・Date禁止)
```

**決定事項(めたん判断・設計書§3の補完)**: 初期 `inputState` は `'editable'`。理由 — 実STT/録音の配線は本フェーズ対象外(§7)であり、`'idle'` を初期値にすると「テキストモードで復帰したユーザーが一度モードを切り替えるまで送信できない」死に状態が生まれる。`'idle'` は実録音パイプライン配線時に再導入する。

### c. メッセージ追加ヘルパー(古い順・最大20件)

```ts
function appendMessage(text: string, emotion: Emotion): void {
  messages = [...messages, { id: nextMessageId++, text, emotion }].slice(-MAX_MESSAGES);
}
```
`slice(-MAX_MESSAGES)` で**古いものから溢れさせ、配列は常に古い順**を維持する。

### d. ハンドラ / 外部イベント受口(すべて「一貫した完全な状態」を生成する)

内部ハンドラ:

| 関数 | 遷移 |
|---|---|
| `handleModeChange(next: InputMode)` | `setStoredInputMode(next)` → `mode = next` → `text = ''` → `inputState = 'editable'` |
| `handleSubmit()` | `inputState = 'sending'` |

`export function` で外部公開する受口(テスト・将来の実配線の注入点。Svelte 4 ではコンポーネントインスタンスのメソッドとして呼べる):

| 関数 | 遷移 |
|---|---|
| `startRecording()` | `inputState = 'recording'` |
| `startTranscribing()` | `inputState = 'transcribing'` |
| `handleSttSuccess(transcript: string)` | `text = transcript` → `inputState = 'editable'` |
| `handleSttFailure(reason: SttFailureReason = 'unrecognized')` | `text = ''` → `inputState = 'editable'` → `appendMessage(reason==='timeout' ? STT_TIMEOUT_MESSAGE : STT_FAILURE_MESSAGE, STT_FAILURE_EMOTION)`。**トーストには一切触れない** |
| `receiveSSEEvent(event: SSEEvent)` | `routeSSEEvent(event, { onError, onMessage })` に委譲。`onError(msg)` → `toastMessage = msg`(+ `inputState === 'sending'` なら `'editable'` に解決)。`onMessage(t, e)` → `appendMessage(t, e)`。**バブルとトーストを両方出さない** |
| `completeSubmission()` | SSE `done` 相当。`inputState = 'editable'` → `text = ''` |

`type SttFailureReason = 'unrecognized' | 'timeout';`

**`sseEventRouter.ts` に `done` を足さないこと。** 既存ユニットテスト(TC-4-3-xx)の契約を壊すため、`done` はコンテナ側の `completeSubmission()` で表現する。

### e. マークアップ

```svelte
<section class="conversation-view">
  <div class="conversation-view__log">
    {#each messages as message (message.id)}
      <MessageBubble text={message.text} emotion={message.emotion} />
    {/each}
  </div>

  <ModeToggle
    {mode}
    isRecording={inputState === 'recording'}
    isTranscribing={inputState === 'transcribing'}
    onModeChange={handleModeChange}
  />

  <textarea aria-label={TEXT_INPUT_LABEL} bind:value={text}></textarea>

  <SendButton disabled={isSendButtonDisabled({ mode, text, inputState })} onSubmit={handleSubmit} />

  {#if toastMessage !== null}
    {#key toastSeq}
      <Toast message={toastMessage} />
    {/key}
  {/if}
</section>
```

- `Toast` は `{#if}` の中で**単一インスタンス**。ただし`toastMessage`の差し替えだけでは同一文言のとき同値代入となり再表示されないため、単調増加の`toastSeq`を`{#key}`に用いてerror受信ごとに再マウントする(INV-4)。**トーストを配列で持たないこと**。
- `MessageBubble.svelte` のルート div に `data-emotion={emotion}` を追加する。現状 `emotion` prop は受け取るだけで DOM に出ておらず感情を検証できないため。**Props契約は不変**・既存テストは影響を受けないはずだが、必ず再実行して確認すること。
- import は `$lib` / `$components` エイリアス(`vitest.config.ts` に定義済み)を使う。ただし**テストコード側は既存FEテストに合わせて相対パス** `../../../src/...` で統一する。

---

## 3. テストケース一覧 (`tests/unit/frontend/conversation_view.test.ts`)

`describe` は機能単位で切り、`afterEach(cleanup)` と `localStorage.clear()` を必ず入れること(F.I.R.S.T の Isolated)。状態更新後は `await tick()`(`svelte` から import)で DOM 反映を待つ。

### 初期状態

| ID | 内容 |
|---|---|
| TC-P-01 | localStorage未設定 → 「マイク」ボタンが `aria-pressed="true"`、テキスト欄は `''`、バブル0件、`role="alert"` なし、送信ボタンは**有効**(voice+editable) |
| TC-P-02 | localStorage に `'text'` 保存済み → 「テキスト」ボタンが `aria-pressed="true"`、送信ボタンは**無効**(text+空) |

### 4-1 モード切替 (INV-1/2/5)

| ID | 内容 |
|---|---|
| TC-P-03 | テキスト入力後にモード切替 → テキスト欄が `''` になる。`localStorage.getItem('zuncha:inputMode')` が切替後の値 |
| TC-P-04 | voice→text 切替直後、送信ボタンは無効(text+空)。そこに文字を入力すると有効になる |

### 4-2 STT (INV-1/3/4b・経路分離)

| ID | 内容 |
|---|---|
| TC-P-05 | `handleSttSuccess('こんにちは')` → テキスト欄が `'こんにちは'`、送信ボタン有効 |
| TC-P-06 | `startTranscribing()` → `handleSttFailure()` → テキスト欄 `''`・バブル1件で文言が `STT_FAILURE_MESSAGE` と完全一致・`data-emotion="困惑"`・**`role="alert"` は0件**・ModeToggleの両ボタンが `disabled` 解除(transcribing固着なし) |
| TC-P-07 | `handleSttFailure('timeout')` → 文言が `STT_TIMEOUT_MESSAGE` と完全一致 |
| TC-P-08 | **CC本丸①**: 文字入力 → `startRecording()` → `startTranscribing()` → `handleSttFailure()` → 直後に `handleModeChange('text')`。最終状態: テキスト欄 `''`・「テキスト」が `aria-pressed="true"`・バブル1件が**残存**・送信ボタン無効 |
| TC-P-09 | **CC本丸②(逆順)**: `handleModeChange('text')` → 直後に `handleSttFailure()`。最終状態: テキスト欄 `''`・mode は `text` のまま(STT失敗はmodeを変えない/INV-2)・バブル1件・トーストなし |
| TC-P-10 | STT失敗2回連続 → バブル2件、1件目が `STT_FAILURE_MESSAGE`・2件目が `STT_TIMEOUT_MESSAGE` の**古い順** |

### 4-3 SSE (INV-4・経路分離)

| ID | 内容 |
|---|---|
| TC-P-11 | `receiveSSEEvent({type:'error',payload:{message:'サーバーエラー'}})` → `role="alert"` 1件・文言一致・**バブルは0件のまま** |
| TC-P-12 | 異なる文言のerrorを連続2回 → `role="alert"` は**1件のみ**、内容は最新の文言(INV-4) |
| TC-P-13 | 同一文言のerrorを連続2回 → `role="alert"` は1件のみ(多重化しない) |
| TC-P-14 | `payload.message` 欠落 → `DEFAULT_ERROR_MESSAGE` が表示される |
| TC-P-15 | `message` イベント → バブル追加・`data-emotion` が指定の感情・**トーストなし** |
| TC-P-16 | `vi.useFakeTimers()` で error 表示後 `DEFAULT_TOAST_DURATION_MS` 経過 → `role="alert"` が消える(`afterEach` で `vi.useRealTimers()`) |
| TC-P-20a | トースト自動消滅後に**同一文言**のerrorが再来 → `role="alert"`が再表示される1件・文言一致 |
| TC-P-20b | トースト自動消滅後に**異なる文言**のerrorが再来 → 再表示される1件・最新の文言(退行防止) |
| TC-P-20c | 表示中に同一文言のerrorが再来 → `role="alert"`は1件のみ(多重化しない/INV-4) |
| TC-P-20d | 表示中に同一文言のerrorが再来 → タイマーが張り直され、再来時点から新たに`DEFAULT_TOAST_DURATION_MS`経過するまで表示が続く |

### 送信・境界・エッジ

| ID | 内容 |
|---|---|
| TC-P-17 | 送信ボタンクリック → `sending` になり送信ボタンが `disabled`。`completeSubmission()` → 有効に戻り、テキスト欄 `''` |
| TC-P-18 | textモードで空白のみ `'   '` を入力 → 送信ボタン無効(trim) |
| TC-P-19 | 1000文字(`'あ'.repeat(1000)`)入力後にモード切替 → テキスト欄が `''`。さらに `messages` を21件追加 → 表示バブルは20件・先頭が2件目に投入したもの(最古が溢れる・古い順維持) |

**TC-P-19 は「長大入力」と「20件上限」で観点が2つあるため、実装時は `it` を2つに分けて構わない**(むしろ推奨。IDは TC-P-19a / TC-P-19b とする)。

---

## 4. 完了条件

- 上記全ケースが緑。
- `npx vitest run tests/unit/frontend/` → 既存60件含め全緑。
- `npx tsc --noEmit` → EXIT 0。
- Goテストは無変更のため実行不要(ただし何か触ったなら報告すること)。
- コーディング規約(`.claude/rules/ts-svelte-coding-guideline.md`)遵守: シングルクォート・末尾カンマ・2スペース・`any` 禁止・関数の引数と戻り値に型注釈・`{#each}` は必ずキー付き。

## 5. 判断に迷ったら

仕様が本書で決まっていない点(状態遷移の細部・文言・DOM構造)が出たら、**自己判断で決めずめたんに確認すること**。特に「トーストとバブルのどちらで出すか」は経路分離が仕様の核心ですので、勝手に増やさないでくださいまし。
