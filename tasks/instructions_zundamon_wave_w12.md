# 実装指示書 — Wave W-12: sseStream.ts (生SSEイベントパーサ)

- 指示者: 四国めたん(テックリード) / つむぎ代行 / 2026-08-14
- 実装担当: ずんだもん
- **実装前提: `/home/masashiohashi/.claude/plans/shiny-napping-treehouse.md`(承認済み実装計画)を必ず読むこと**
- 依存: なし。既存の`src/lib/sseEventRouter.ts`は変更しない(D-2の方針、契約は不変)

---

## 1. スコープ

`src/lib/sseStream.ts`を新規作成する。バックエンドが実際に送出するSSEイベント5種を、型付きの`ServerEvent`へ変換する**純粋関数**のみを持つファイル。ブラウザAPI(EventSource等)への依存は一切持たない。

## 2. 確定済み契約(推測しないこと)

バックエンドの実装コードから確定済みのSSEイベント形式:

| event名 | data JSON(1行) | 備考 |
|---|---|---|
| `emotion` | `{"request_id":"...","label":"喜び"}` | labelは`喜び/怒り/悲しみ/楽しい/照れ/困惑/ドヤ顔`の7種 |
| `text` | `{"request_id":"...","chunk":"..."}` | イベント名は`text`。`text_chunk`ではない |
| `audio_url` | `{"request_id":"...","url":"/audio/{ulid}"}` | |
| `done` | `{"request_id":"..."}` | 追加フィールドなし |
| `error` | `{"request_id":"...","message":"..."}` | 接続レベル障害では`request_id`が空文字列になりうる(正当な値として許容すること) |

## 3. 型定義・関数シグネチャ

```ts
import type { Emotion } from './sseEventRouter';

export type ServerEvent =
  | { type: 'emotion'; requestId: string; emotion: Emotion }
  | { type: 'text'; requestId: string; chunk: string }
  | { type: 'audioUrl'; requestId: string; url: string }
  | { type: 'done'; requestId: string }
  | { type: 'error'; requestId: string; message: string };

// eventName(SSEの`event:`行)とrawData(`data:`行の1行JSON文字列)を受け取り、
// 型付きイベントへ変換する。不正JSON・未知イベント名・必須フィールドの型不一致は
// null を返して黙って無視する(サーバーの実装ミスでフロントをクラッシュさせない防御)。
export const parseServerEvent = (eventName: string, rawData: string): ServerEvent | null => { ... };
```

- `Emotion`型は既存`src/lib/sseEventRouter.ts`からimportして再利用すること(型を重複定義しない)
- `error`の`message`が空/欠落の場合は空文字列にフォールバックする(`DEFAULT_ERROR_MESSAGE`への差し替えは呼び出し側=W-14の責務。このファイルはパースに徹し、文言のフォールバック規則を重複させない)
- `emotion`の`label`が7種外・欠落の場合は「困惑」にフォールバックする(`internal/llm.ParseLLMResponse`と同じ防御方針。根拠としてコメントに明記すること)

## 4. テスト(`tests/unit/frontend/sse_stream.test.ts`)

ブラウザAPIのモック不要(純粋関数のため)。

**正常系**:
- [ ] `emotion`イベント: 正しい`request_id`/`label`(7種それぞれ)が`ServerEvent`に変換される
- [ ] `text`イベント: `request_id`/`chunk`が変換される
- [ ] `audioUrl`イベント: `request_id`/`url`が変換される(実際のevent名は`audio_url`だが型の`type`は`audioUrl`にキャメルケース変換すること)
- [ ] `done`イベント: `request_id`のみで変換される
- [ ] `error`イベント: `request_id`/`message`が変換される
- [ ] `error`の`request_id`が空文字列でも正しく変換される(接続レベル障害のケース)

**異常系(エッジケース)**:
- [ ] 不正なJSON文字列 → `null`
- [ ] 未知のイベント名(`event: unknown`等) → `null`
- [ ] `emotion`で`label`欠落 → `null`または困惑フォールバック(**どちらの挙動にするか設計判断を明記して選ぶこと**。推奨: labelキー自体が無い/型が不正なら`null`(パース失敗)、labelはあるが7種外の値なら困惑フォールバック、という2段階の区別。理由をコメントに残すこと)
- [ ] `emotion`で未知の感情ラベル(7種外の文字列) → 困惑にフォールバックすることを確認
- [ ] `text`で`chunk`欠落 → `null`
- [ ] `audioUrl`で`url`欠落 → `null`
- [ ] `error`で`message`欠落・空文字列 → 空文字列にフォールバック(nullにはしない。`error`イベント自体は届いているため)
- [ ] `request_id`が数値型など文字列以外 → `null`(型不一致)

**ミューテーション実測**: 困惑フォールバックのロジックを削除すると、未知ラベルのテストが赤になることを実測(`mutation-test-overlay`スキル使用。TSファイルなのでGoの`go test -overlay`ではなく、一時ファイルコピー+`vi.mock`または単純にファイルを一時的に書き換えて`npx vitest run`→復元、という手順で代替すること。手順は自分で組み立ててよいが、作業ツリーを最終的に無改変に戻すこと)。

## 5. 完了条件

- [ ] `npx vitest run`全緑(既存95件が無改変で維持されていること。`sseEventRouter.ts`・`conversation_view.test.ts`・`error_routing.test.ts`は一切変更しない)
- [ ] `npx tsc --noEmit` EXIT 0
- [ ] RED確認(テストを先に書いて失敗することを確認)を報告に含める
- [ ] ミューテーション実測を報告に含める
- [ ] `src/components/ConversationView.svelte`は今回変更しない(W-13以降の対象)

## 6. 報告について

- 緑でも`completed`にせず`in_progress`のまま報告
- `emotion`の`label`欠落時の扱い(null vs 困惑フォールバック)の判断根拠を必ず書くこと
- 完了後、`tasks/todo.md`の末尾に対応内容を追記すること
