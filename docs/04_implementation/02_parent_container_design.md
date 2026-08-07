# 親コンテナ `ConversationView` 設計書 (IT-3 前提)

作成: 四国めたん (テックリード) / 2026-07-23
目的: 結合テスト申し送りCC(STT失敗による入力欄クリア[4-2] × モード切替[4-1] × エラートースト[4-3]の3機能間状態競合)を検証可能にするため、既存の薄いコンポーネント群を統合する親コンテナの仕様を定義する。

## 1. 位置づけ

- GREEN phaseで実装した各コンポーネント(`ModeToggle`/`SendButton`/`Toast`/`MessageBubble`)と純粋関数(`inputMode`/`sendButton`/`sseEventRouter`)は**単体では競合が起きない薄い部品**である。競合(CC)は「これらを束ね、入力欄と状態を保持する親」が存在して初めて発生する。
- 本コンテナは**その親**であり、状態の単一の真実の源泉(single source of truth)となる。CCの状態競合はここで解決する。

## 2. 統合する既存部品(Props契約・確認済み)

| 部品 | 主なProps/API |
|---|---|
| `ModeToggle` | `mode`, `isRecording`, `isTranscribing`, `onModeChange(mode)` (内部で楽観的更新) |
| `SendButton` | `disabled`, `onSubmit()` |
| `Toast` | `message`, `durationMs`(既定`DEFAULT_TOAST_DURATION_MS`=3000)。`message`**変更時**に再表示・タイマー再張りし多重化しない。同値代入では再表示されないため、同一文言の再来は親側の`{#key toastSeq}`で再マウントして表現する |
| `MessageBubble` | `text`, `emotion` |
| `isSendButtonDisabled` | `{mode, text, inputState}` → boolean。`editable`以外は常に無効、voiceは空でも有効、textはtrimで空なら無効 |
| `routeSSEEvent` | `(event, {onError, onMessage})`。error→onError(空はDEFAULT_ERROR_MESSAGE)、message→onMessage |
| `inputMode` | `getStoredInputMode`/`setStoredInputMode`, `DEFAULT_INPUT_MODE='voice'` |

`InputState = 'idle' | 'recording' | 'transcribing' | 'editable' | 'sending'`

## 3. コンテナが保持する状態(単一の真実の源泉)

```ts
mode: InputMode          // 現在の入力モード(初期値 = getStoredInputMode())
text: string             // テキスト入力欄の内容
inputState: InputState   // idle/recording/transcribing/editable/sending
toastMessage: string | null  // 非nullの間だけ Toast を表示
messages: { text: string; emotion: Emotion }[]  // アシスタント発話
```

`ModeToggle` へは `isRecording = (inputState==='recording')`、`isTranscribing = (inputState==='transcribing')` を渡す。
`SendButton` の `disabled = isSendButtonDisabled({mode, text, inputState})`。

## 4. 状態遷移(各機能)

> ⚠ **本節の遷移は無条件ではない。** 各ハンドラは実行前に §5 のガード条件を満たすかを検査する。**本節だけを読んで実装すると、遅延・重複・逆順で到着したイベントが他の経路の状態を破壊する(実際に6症状を作り込んだ)。必ず §5 と併せて読むこと。**

- **モード切替(4-1)** `handleModeChange(next)`: **【ガード: `next === mode` なら何もしない(INV-1b)】** → `setStoredInputMode(next)` → `mode=next` → **`text=''`(7章U: 旧モードの入力=テキスト欄の文字列のみをクリア。録音バッファは対象外)** → **`inputState='editable'`(ただし `'sending'` 中は `inputState` を書き換えない = INV-6b)**。
- **STT成功** `handleSttSuccess(transcript)`: **【ガード: `'sending'` 中は stale として破棄し何もしない(INV-6c)】** → `text=transcript` → `inputState='editable'`。
- **STT失敗/タイムアウト(4-2, F-STT-02/03)** `handleSttFailure()`: **【ガード: `'sending'` 中は stale として破棄し、バブルも追加しない(INV-6c)】** → **`text=''`** → `inputState='editable'`(再入力可へ復帰) → **`messages` にずんだもんのリトライ発話バブルを追加**(`01_screen_design.md` 9-3: STT失敗は**応答バブル**で表現し、**エラートーストは使わない**)。録音中の音声バッファは破棄する。
- **SSEエラー(4-3, `error`イベント/STT以外のバックエンドエラー)** `routeSSEEvent`の`onError(msg)`: `toastMessage=msg`(**トーストはこの経路のみ**) → `toastSeq += 1`(同一文言の再来でも再表示させる) → **`inputState` が `'sending'` または `'transcribing'` のときに限り `'editable'` へ復帰させる(INV-3b)**。※旧記述「必要に応じ `inputState='editable'`」は条件が曖昧で `'transcribing'` 固着を招いたため、上記に確定した。
- **SSEメッセージ**: `onMessage(text,emotion)` → `messages`に追加。
- **送信** `handleSubmit()`: **【ガード: `inputState === 'editable'` のときのみ遷移(INV-6a)】** → `inputState='sending'`(→SendButton無効)。SSE `done` 相当の `completeSubmission()` で `inputState='editable'`、`text=''`。**ただし `completeSubmission()` は `'sending'` 以外では何もしない(INV-6d)**。

> **エラー表現の経路分離(重要)**: STT失敗系(フロント/4-2)は**応答バブル**、SSE `error`・STT以外のバックエンドエラー(4-3)は**トースト**。両者は別経路であり、STT失敗はトーストを一切使わない。

## 5. 競合解決ルール(CCの核心)

「STT失敗」「モード切替」「エラートースト」が近接して発生した場合でも、以下の**不変条件(invariant)**が最終状態で必ず成立すること。Svelteは単一スレッドでハンドラを逐次処理するため、各ハンドラは「部分更新」ではなく「一貫した完全な状態」を生成する設計とする。

- **INV-1 (text)**: STT失敗・モード切替のいずれの経路も `text=''` にする。両者が続けて起きても最終的に空で一致(矛盾しない)。
  - **INV-1b (同一モード再クリックは「切替」ではない)**: `handleModeChange(next)` は `next === mode` のとき**何もしない**。§4のクリア規定は「**旧モードの**入力をクリアする」であり、モードが変わらない以上 `text` を消す根拠がない。`ModeToggle` は同一モードのクリックでも `onModeChange` を無条件に発火するため、テキスト入力中の誤タップが入力全消しに直結する。
- **INV-2 (mode)**: `mode` は最後のユーザー操作(モード切替)の値を反映する。STT失敗は `mode` を変えない。
- **INV-3 (inputState)**: STT失敗後に `'transcribing'` のまま固着しない。失敗・切替後は必ず `'editable'`(または妥当な非中間状態)へ解決する。
  - **INV-3b (中間状態の行き止まり救済)**: SSE `error` は `'sending'` に加え `'transcribing'` からも `'editable'` へ復帰させる。`'transcribing'` は自力の出口(STT成功/失敗コールバック)が到着しない場合に固着し、かつ `ModeToggle` も disabled のため操作不能に陥るため。
- **INV-4 (toast単一性・SSEエラー系のみ)**: トーストは**SSE `error`/STT以外のバックエンドエラー経路のみ**に発生する。同時に複数のSSEエラーが起きても表示トーストは1つ(親が`{#key toastSeq}`でToastを再マウントし、error受信ごとに再表示・タイマー再張りする。同値代入では再表示されないため`message`変更に依存させない=同一文言の再来でも確実に再表示。多重化しない・最新勝ち)。**STT失敗はトーストではなく応答バブルなので本INVの対象外**。
- **INV-4b (STT失敗バブル)**: STT失敗は `messages` にリトライ発話バブルを1件追加する。モード切替と連続してもバブルは消えず会話ログに残る(文脈の連続性)。
- **INV-5 (SendButton整合)**: `disabled` は常に最終の`{mode,text,inputState}`から`isSendButtonDisabled`で再計算され、途中状態と齟齬しない。
- **INV-6 (sending不可侵・飛行中リクエストの保護)**: `'sending'` は「送信リクエストが飛行中」であることを表す**占有状態**である。これを解けるのは **`completeSubmission()`(`done`受信)と SSE `error`** の2経路のみとし、他のいかなるハンドラも `'sending'` を上書きしない。
  - **⚠ 現時点での適用範囲(既知の乖離)**: 実装上、`startRecording()` / `startTranscribing()` の2つは**ガードを持たず `'sending'` を上書きできる**。両者は実録音パイプラインからの受口であり、その配線が本フェーズの対象外(§7)で**到達不能**なため実害はない。ただし本INVの全称記述とは乖離しているので、ここに明記する。**ガードを機械的に足していないのは意図的である** — 「送信中に次の発話の録音を開始できるか」は未決のプロダクト判断であり、ガードを入れれば「不可」と暗黙に決めてしまうため(§7-1-4)。
  - **a) 入口ガード(冪等性)**: `handleSubmit()` は `inputState === 'editable'` のときのみ `'sending'` へ遷移する。`SendButton` の `disabled` は Svelte の DOM 更新が非同期であるため、同一フレーム内の二連打を単独では防げない。二重送信の防止はコンテナ側の状態ガードが最終防衛線となる。
    - **⚠ 回帰検知の限界(既知・要フォロー)**: 現時点の `handleSubmit()` の副作用は `inputState='sending'` の代入のみで、これは**冪等**である。したがって二連打してもガードの有無で最終状態に差が出ず、**INV-6a は自動テストで回帰検知できない**(ミューテーションテストで実証済み: ガードを外しても `conversation_view.test.ts` 全件が緑のまま)。`TC-P-21` は「二連打後も状態が健全である」ことの記録にとどまり、ガードの保護にはなっていない。ガードが実際に効くのは `handleSubmit()` が実送信を行うようになってからであり、**実SSE配線フェーズで「送信APIの呼び出し回数 = 1」を検証するテストを必ず追加すること**。なお `TC-P-21b` は `SendButton` 単体では二連打を防げない(= 親のガードが必要である)ことの根拠として有効。
  - **b) モード切替**: `handleModeChange()` は `'sending'` 中でも `mode` の更新・永続化と `text=''` は行う(モード切替はユーザーの明示的な意思表示であり、`ModeToggle` は内部で楽観的更新を行うため親が無視すると表示が乖離する)。ただし `inputState` は書き換えない。
  - **c) 陳腐化した STT 結果の破棄**: `'sending'` 中に到着した `handleSttSuccess()` / `handleSttFailure()` は、既に送信済みの旧インタラクションに属する**陳腐化(stale)結果**として破棄する。`text` / `inputState` / `messages` のいずれも変更しない(INV-4b はこの場合を対象外とする)。
  - **d) 遅延 `done` の無視**: `completeSubmission()` は `'sending'` 以外では**何もしない**。旧リクエストの遅延 `done` が、ユーザーが既に打ち始めた新規入力(`text`)を消してはならない。

> **INV-6 の根拠**: `'sending'` を破壊する経路が残ると、(1) 飛行中に `'editable'` へ戻り二重送信が可能になる、(2) 遅延 `done` がユーザーの新規入力を消す、という**ユーザーのデータを失う**クラスの障害に直結する。各ハンドラを個別に場当たり修正するのではなく、「中間状態はそれを開始した経路だけが終了させる」という**状態オーナーシップ**の一般原則として定義する。

## 6. テスト方針とハーネス

- **ハーネス**: `vitest` + `@testing-library/svelte`(既存FEテスト基盤を流用。新規ツール不要)。競合の非同期タイミングは `vi.useFakeTimers()` と `await` 制御で決定化する。Playwright E2Eは実バックエンド・ルーティングが無い現段階では過剰につき**見送り**(後続のE2Eフェーズで検討)。
- **配置**: `tests/unit/frontend/conversation_view.test.ts`(既存FEテストと同ディレクトリ)。
- **テストケース案(競合中心・INVに対応)**:
  1. モード切替でテキスト入力がクリアされる(INV-1)。
  2. STT失敗でテキストが空・`messages`にリトライ応答バブルが追加・`editable`復帰。**トーストは表示されない**(INV-1/3/4b)。
  3. **STT失敗とモード切替が連続**しても text 空・mode は切替後・応答バブルは残存(消えない)・SendButton整合(INV-1/2/3/4b/5の複合=CC本丸)。
  4. SSE `error` 表示中に別のSSEエラー→メッセージ上書き・多重化しない(INV-4)。
  5. STT失敗直後の SendButton disabled が最終状態と一致(INV-5)。
  6. 送信中(`sending`)は SendButton 無効、`done` で `editable`・text空(境界)。
  7. **経路分離**: STT失敗は応答バブルのみ(トーストなし)/ SSE `error` はトーストのみ(バブルなし)を各々検証(INV-4/4b)。
  8. **トースト自動消滅後に同一文言のSSEエラーが再来**→再表示される。表示中の再来ではタイマーが張り直され表示が延長される(INV-4)。
- **状態競合(INV-6/INV-3b/INV-1b)のテストケース**: `docs/03_unit_test/16_test_cases_conversation_view_race.md` に `TC-P-21`〜`TC-P-26` として定義。実装は同じく `tests/unit/frontend/conversation_view.test.ts`。
  9. 同一フレームの二連打(INV-6a)。**ただし回帰検知にはならない — §5 INV-6a の「⚠回帰検知の限界」を参照。**
  10. 送信中のモード切替: `mode`/`text` は更新されるが `sending` は維持され送信ボタンは無効のまま。切替を挟んでも `done` が正常に効く(INV-6b)。
  11. 送信中に遅着したSTT成功/失敗の破棄。バブルも増えない(INV-6c)。
  12. 送信中でないときの遅延 `done` が、ユーザーの打ち直した入力を消さない(INV-6d)。
  13. `transcribing` 中のSSEエラーでトースト表示＋`editable` 復帰。トグルのdisabledが解ける(INV-3b)。
  14. 同一モードの再クリックで入力が消えず、`localStorage` への再書き込みも起きない(INV-1b)。
- **検証方法の要件**: 上記のガード系テストは「緑であること」だけでは不十分で、**ガードを1つずつ外して当該テストが赤に転じるか(ミューテーションテスト)まで確認する**。実施記録は §5 INV-6a の限界注記を参照。

## 7. スコープと非対象

- 本コンテナは**フロントの状態統合と競合解決**に限定。LLM/STT/TTSの実接続・実SSE配線は対象外(モック/スタブで駆動)。
- 既存の全テスト(Goユニット/Go結合/フロントユニット)は緑を維持。`ModeToggle`等の既存部品のProps契約は変更しない(親が上位で状態を制御する)。**フロントユニットは状態競合対応(TC-P-21〜26)の追加で 84 → 95 件**(`npx vitest run` で4ファイル全緑を確認、2026-07-29時点)。※件数は増え続けるため、本文書では**確認コマンド**を正とし、数値は時点情報として扱う。
- 申し送り2b(ModeToggleの`selected`がmode追従しない件)は、**親が`text`と`inputState`で制御し`mode`を一方向に流す**本設計では実害が顕在化しにくいが、必要なら親統合テストで追従挙動を確認し、別途対応可否を判断する(関連: §7-1-3)。

### 7-1. 後続フェーズへの申し送り(状態競合対応で確定・2026-07-29)

1. **【必須】INV-6a の回帰テスト**: 実SSE配線で `handleSubmit()` が実送信を行うようになった時点で、「同一フレーム二連打でも**送信APIの呼び出し回数 = 1**」を検証するテストを追加する。現時点では原理的に書けない(§5 INV-6a の限界注記)。
2. **【要検討】リクエストIDによる相関**: 現行の `completeSubmission()` は引数を持たないため、「送信#1が飛行中に、送信#0の遅延 `done` が届いて誤完了する」ケースを防げない。INV-6d は「`sending` 以外での `done` を無視する」までしか担保しない。実SSE配線ではリクエストIDでの相関を検討すること。
3. **【クリーンアップ候補】`ModeToggle` の無変化コールバック**: `ModeToggle.selectMode()` は `next === selected` でも `onModeChange` を無条件に発火する(症状6の発火元)。現在は親の INV-1b で実害を封じているが、部品としては「変化していないのに変更コールバックを発火する」筋の悪さが残る。既存部品のProps契約・テストへの波及があるため本フェーズでは見送った。
4. **【要プロダクト判断】`startRecording`/`startTranscribing` の INV-6 適用**: この2つの受口はガードを持たず `'sending'` を上書きできる(§5 INV-6 の「現時点での適用範囲」)。実録音パイプラインの配線時に、まず**「送信中に次の発話の録音を開始できるか」をプロダクト判断として確定**すること。
   - 「不可」なら他の受口と同じく `if (inputState === 'sending') return;` のガードを追加する。
   - 「可」なら `'sending'` と録音の同時進行をどう表現するか(状態の直交化、飛行中リクエストのキャンセル、録音のキュー等)を設計し直す必要があり、`InputState` の単一列挙では表現しきれない可能性が高い。
   - **ガードを先回りで足さないのは意図的**。足せば「不可」を暗黙に決めてしまい、未決の判断がコードに埋没する。
5. **【見送り】送信中の `ModeToggle` 無効化**: 送信中にトグルを無効化する案は Props 契約の変更(本節の制約)に触れるため見送り、INV-6b で実害を封鎖した。

## 8. 確定事項(2026-07-23 つむぎ経由・既存仕様で裏取り済み)

- **STT失敗の表現** → **ずんだもん応答バブル**で会話ログに表示。**エラートーストは使わない**(`01_screen_design.md` 9-3・475/549行)。トーストは SSE `error`/STT以外のバックエンドエラーのみ。文言は「リトライを促すずんだもんの発話」体裁(具体文言は実装時に画面設計と突合)。
- **モード切替時のクリア対象** → **テキスト入力欄の文字列のみ**。録音バッファは対象外(録音中・変換中はトグルがdisabledで、録音中のモード切替は発生しない。7章U・`12_test_perspectives_phase4.md` 64行)。
- **`messages`表示方針** → **全件表示(最大20件)・古い順**。バックエンド `GetRecentMessages`(直近20件・古い順)と一致(`01_screen_design.md` 376行)。

本3点を§4/§5/§6に反映済み。実装フェーズ着手時は具体文言のみ画面設計と最終突合する。
