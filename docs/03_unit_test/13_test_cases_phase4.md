# フェーズ4 単体テストケース一覧

| 項目 | 内容 |
|------|------|
| バージョン | 1.0 |
| 作成日 | 2026-07-09 |
| 作成者 | ひまり（テストケース設計担当） |
| 入力 | `12_test_perspectives_phase4.md`（めたん作成、7章確認事項T〜Zはつむぎ決定済み） |
| 前提 | 7章確認事項T〜Z：つむぎ決定済み（本書内で反映）。本書設計中に新たに生じた未決事項はAA〜として本書末尾に追記し、つむぎの判断を仰ぐ |
| 次工程 | WhiteCULが単体テスト仕様書・テストコードへ清書 |

---

## 0. 前提：確認事項T〜Zの決定内容（本書での反映方針）

| # | 対象 | 決定内容 | 本書での反映 |
|---|------|---------|-------------|
| T | 4-1 | `localStorage`の無効値・例外はいずれもデフォルト`"voice"`にフォールバック。大文字小文字の正規化はしない | `isInputMode`の異常系・境界値（TC-4-1-03〜10）、`getStoredInputMode`の異常系・境界値（TC-4-1-16〜20）で反映 |
| U | 4-1 | 録音中・STT変換中はモード切替トグル自体をdisabled化。切替時に旧モードの入力はクリア | コンポーネントレベルの例外系（TC-4-1-24〜26）で反映 |
| V | 4-2 | 「録音中」も「録音完了前」に含めdisabled。有効化は録音完了後のみ | `isSendButtonDisabled`の異常系（TC-4-2-04, 08）で反映。`InputState`に`recording`を含める設計とした |
| W | 4-2 | 送信中（クリック後〜SSE `done`受信まで）もdisabled化 | `isSendButtonDisabled`の異常系（TC-4-2-09, 10）で反映。`InputState`に`sending`を含める設計とした |
| X | 4-3 | 単純ルーティングで正しい。フロントに判定ロジックは実装しない。F-RT-02はスコープ外 | 4-3-1の対象関数を`routeSSEEvent`（薄いルーティング）に限定し、判定ロジックのテストケースは設計しない。F-RT-02関連のケースも設計対象に含めない |
| Y | 4-3 | ①ルーティング関数と②Toast/応答バブルの表示コンポーネントの両方をテスト対象とする（CSS具体値は対象外） | 4-3-1（ルーティング）・4-3-2（Toast）・4-3-3（応答バブル）の3層構成で反映 |
| Z | 4-1 | `localStorage`は`vi.stubGlobal`で独自の`Storage`モックに完全に置き換える | `getStoredInputMode`/`setStoredInputMode`のGiven記述をすべて`vi.stubGlobal`前提の記述に統一（TC-4-1-11〜20） |

---

## 目次

1. [4-1. 入力モード切替（マイク/テキスト）とlocalStorage保存](#4-1-入力モード切替マイクテキストとlocalstorage保存)
2. [4-2. 送信ボタンの活性制御](#4-2-送信ボタンの活性制御)
3. [4-3. エラー表現の出し分け（ずんだもん応答バブル vs 3秒トースト）](#4-3-エラー表現の出し分けずんだもん応答バブル-vs-3秒トースト)
4. [セルフチェック（重複排除・不足確認）](#セルフチェック重複排除不足確認)
5. [未決事項（WhiteCUL・つむぎへの申し送り）](#未決事項whitecul・つむぎへの申し送り)

---

## 4-1. 入力モード切替（マイク/テキスト）とlocalStorage保存

責務分離の指摘に従い、「①型ガード（純粋関数）」「②`localStorage`読み書きラッパー（純粋関数＋副作用境界）」「③トグルコンポーネント（クリック挙動の少数の正常系＋4-1固有の例外系）」の3層に分けて設計する。異常系・境界値は①②に厚く配置し、③は薄い層としての結線確認に絞る。

### 4-1-1. `isInputMode` 型ガード（純粋関数、モック不要）

**対象関数**: `isInputMode(value: unknown): value is InputMode`。`type InputMode = 'voice' | 'text'`に対する型ガードとして、`unknown`を受け取り真偽値のみを返す。この関数の`false`が、呼び出し元での`"voice"`フォールバックの唯一の入口になる（7章T確定）。

#### 正常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-01 | `value = "voice"` | `isInputMode(value)`を呼ぶ | `true`を返す |
| TC-4-1-02 | `value = "text"` | `isInputMode(value)`を呼ぶ | `true`を返す |

#### 異常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-03 | `value = "foo"`（Union型にない文字列） | `isInputMode(value)`を呼ぶ | `false`を返す |
| TC-4-1-04 | `value = "1"` | `isInputMode(value)`を呼ぶ | `false`を返す |
| TC-4-1-05 | `value = ""`（空文字列） | `isInputMode(value)`を呼ぶ | `false`を返す |
| TC-4-1-06 | `value = null` | `isInputMode(value)`を呼ぶ | `false`を返す |
| TC-4-1-07 | `value = undefined` | `isInputMode(value)`を呼ぶ | `false`を返す |
| TC-4-1-08 | `value = 123`（数値型） | `isInputMode(value)`を呼ぶ | `false`を返す |

#### 境界値

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-09 | `value = "Voice"`（大文字小文字のみ異なる） | `isInputMode(value)`を呼ぶ | `false`を返す（7章T確定：正規化せず無効値として扱う） |
| TC-4-1-10 | `value = "TEXT"`（大文字小文字のみ異なる） | `isInputMode(value)`を呼ぶ | `false`を返す |

### 4-1-2. `getStoredInputMode` / `setStoredInputMode`（localStorageラッパー）

**対象関数**: `getStoredInputMode(): InputMode` / `setStoredInputMode(mode: InputMode): void`。キー名は`zuncha_input_mode`固定。`localStorage`は`vi.stubGlobal('localStorage', mockStorage)`で独自の`Storage`モックに完全置換し、`getItem`/`setItem`の呼び出し引数をスパイ検証する（7章Z確定）。無効値・例外時のフォールバックは`isInputMode`の`false`判定に委譲し、コンポーネント側で個別分岐しない（7章T確定）。

#### 正常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-11 | `mockStorage.getItem('zuncha_input_mode')`が`null`を返す設定にする | `getStoredInputMode()`を呼ぶ | `"voice"`を返す（キー未存在時のデフォルト） |
| TC-4-1-12 | `mockStorage.getItem('zuncha_input_mode')`が`"text"`を返す設定にする | `getStoredInputMode()`を呼ぶ | `"text"`を返す |
| TC-4-1-13 | `mockStorage.getItem('zuncha_input_mode')`が`"voice"`を返す設定にする | `getStoredInputMode()`を呼ぶ | `"voice"`を返す |
| TC-4-1-14 | `vi.stubGlobal('localStorage', mockStorage)`でモックを注入する | `setStoredInputMode("text")`を呼ぶ | `mockStorage.setItem`が`("zuncha_input_mode", "text")`で呼ばれる |
| TC-4-1-15 | 同上のモックを用意する | `setStoredInputMode("voice")`を呼ぶ | `mockStorage.setItem`が`("zuncha_input_mode", "voice")`で呼ばれる |

#### 異常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-16 | `mockStorage.getItem('zuncha_input_mode')`が`"foo"`（無効値）を返す設定にする | `getStoredInputMode()`を呼ぶ | `"voice"`にフォールバックする（7章T確定） |
| TC-4-1-17 | `mockStorage.getItem('zuncha_input_mode')`が空文字列を返す設定にする | `getStoredInputMode()`を呼ぶ | `"voice"`にフォールバックする |
| TC-4-1-18 | `mockStorage.getItem`が例外（`SecurityError`相当）をthrowする設定にする | `getStoredInputMode()`を呼ぶ | 例外を握り、`"voice"`を返す（クラッシュしない、7章T確定） |
| TC-4-1-19 | `mockStorage.setItem`が例外（`QuotaExceededError`相当）をthrowする設定にする | `setStoredInputMode("text")`を呼ぶ | 例外が呼び出し元に伝播しない（握って正常終了する） |

#### 境界値

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-20 | `mockStorage.getItem('zuncha_input_mode')`が`"Voice"`（大文字小文字のみ異なる）を返す設定にする | `getStoredInputMode()`を呼ぶ | `"voice"`にフォールバックする（正規化はしない、7章T確定） |

### 4-1-3. モード切替トグルコンポーネント（`@testing-library/svelte`）

正常系はクリック挙動の結線確認のみに絞り、異常系・境界値は4-1-1・4-1-2で厚くカバー済みのため重複させない。4-1固有の例外系（トグル自体のdisabled化・入力クリア）はコンポーネントでしか検証できないため、ここに配置する。

#### 正常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-21 | `mode="voice"`の状態でトグルをマウントする | 「テキスト」ボタンをクリックする | 変更コールバックが`"text"`で呼ばれ、「テキスト」側が選択状態の表示（例：`aria-pressed="true"`）になる |
| TC-4-1-22 | `mode="text"`の状態でトグルをマウントする | 「マイク」ボタンをクリックする | 変更コールバックが`"voice"`で呼ばれ、「マイク」側が選択状態の表示になる |
| TC-4-1-23 | `localStorage`に`"text"`が保存された状態でコンポーネントをマウントする（再マウント相当） | 初期表示を確認する | 「テキスト」側が選択状態で表示される（`localStorage`の値がデフォルト`"voice"`より優先される） |

#### 例外系（7章U確定）

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-24 | `isRecording=true`の状態でトグルをマウントする | トグルの各ボタンの`disabled`属性を確認する | 「マイク」「テキスト」両ボタンとも`disabled`になっている |
| TC-4-1-25 | `isTranscribing=true`の状態でトグルをマウントする | トグルの各ボタンの`disabled`属性を確認する | 「マイク」「テキスト」両ボタンとも`disabled`になっている |
| ~~TC-4-1-26~~ | ~~`mode="text"`かつテキスト入力欄に`"こんにちは"`が入力された状態でマウントする~~ | ~~「マイク」ボタンをクリックする~~ | **欠番**（そらのQAレビュー指摘反映、2026-07-09）：`ModeToggle`はテキスト入力欄propを持たない薄いコンポーネントであり、本ケースが前提とする「`ModeToggle`自身が入力欄をクリアする」という想定は`14_test_specification.md` §3.4のProps定義と矛盾する。仕様自体（旧モードの入力クリア、7章U確定）は取り下げず、入力欄を保持する親コンポーネント（未設計）側の責務として、その設計時に別途テストケース化する |

#### 境界値

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-1-27 | `mode="voice"`の状態からトグルをマウントする | 「テキスト」→「マイク」→「テキスト」→「マイク」の順に連続してクリックする | 変更コールバックがクリック順と一致する`"text", "voice", "text", "voice"`の順で呼ばれ、最終的に`localStorage`へ保存される値が最後の操作`"voice"`と一致する（中間状態が誤って残らない） |

---

## 4-2. 送信ボタンの活性制御

責務分離の指摘（booleanの組み合わせではなく単一の状態機械で表現する）に従い、`type InputState = 'idle' | 'recording' | 'transcribing' | 'editable' | 'sending'`を前提とする。`'idle'`は音声モードの録音前待機、`'editable'`は音声モードの録音完了後（音声データ確定）またはテキストモードの入力可能状態を指す。この設計により、「録音中かつ変換中」のような本来あり得ない組み合わせを型レベルで排除し、テストケースの組み合わせ爆発を防ぐ（7章V・W確定：`InputState`に`recording`・`sending`を含める）。

### 4-2-1. `isSendButtonDisabled`（純粋関数、モック不要）

**対象関数**: `isSendButtonDisabled(params: { mode: InputMode; text: string; inputState: InputState }): boolean`。

#### 正常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-2-01 | `mode="voice"`, `inputState="editable"`（録音完了・音声データ確定） | `isSendButtonDisabled({ mode, text: "", inputState })`を呼ぶ | `false`を返す（有効） |
| TC-4-2-02 | `mode="text"`, `inputState="editable"`, `text="こんにちは"` | `isSendButtonDisabled(...)`を呼ぶ | `false`を返す（有効） |
| TC-4-2-03 | `mode="text"`, `inputState="editable"`, `text="STTにより反映された文字列"`（STT成功による自動入力後、空でない） | `isSendButtonDisabled(...)`を呼ぶ | `false`を返す（有効） |

#### 異常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-2-04 | `mode="voice"`, `inputState="idle"`（録音前・待機中） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効） |
| TC-4-2-05 | `mode="text"`, `inputState="editable"`, `text=""`（空文字列） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効） |
| TC-4-2-06 | `mode="text"`, `inputState="editable"`, `text="   "`（半角スペースのみ） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効） |
| TC-4-2-07 | `mode="text"`, `inputState="transcribing"`, `text="変換中の暫定文字列"`（値があっても） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効、入力欄の内容に関わらず） |
| TC-4-2-08 | `mode="voice"`, `inputState="recording"`（録音中） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効、7章V確定） |
| TC-4-2-09 | `mode="text"`, `inputState="sending"`, `text="こんにちは"`（有効な文字列があっても） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効、7章W確定：二重送信防止） |
| TC-4-2-10 | `mode="voice"`, `inputState="sending"` | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効、モードを問わず送信中は無効化） |

#### 境界値

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-2-11 | `mode="text"`, `inputState="editable"`, `text="あ"`（1文字） | `isSendButtonDisabled(...)`を呼ぶ | `false`を返す（有効、「空でない」の最小境界） |
| TC-4-2-12 | `mode="text"`, `inputState="editable"`, `text="\t"`（タブのみ） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効） |
| TC-4-2-13 | `mode="text"`, `inputState="editable"`, `text="\n"`（改行のみ） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効） |
| TC-4-2-14 | `mode="text"`, `inputState="editable"`, `text="　"`（全角スペースのみ） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効、`String.prototype.trim()`が全角スペースを空白と判定することの明示的確認） |
| TC-4-2-15 | `mode="text"`, `inputState="editable"`, `text=" \t\n　 "`（半角・タブ・改行・全角スペースの混在） | `isSendButtonDisabled(...)`を呼ぶ | `true`を返す（無効） |
| TC-4-2-16 | `mode="text"`, `inputState="editable"`, `text="あ".repeat(1000)`（1,000文字） | `isSendButtonDisabled(...)`を呼ぶ | `false`を返す（有効、文字数上限による無効化は発生しない） |

### 4-2-2. 送信ボタンコンポーネント（`@testing-library/svelte`）

`isSendButtonDisabled`の組み合わせ網羅は4-2-1で完了しているため、コンポーネント側は戻り値がDOMへ正しく結線されているかの確認に絞る。

#### 正常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-2-17 | `isSendButtonDisabled`が`false`を返す状態（例：`mode="text"`, `text="こんにちは"`, `inputState="editable"`）でコンポーネントをマウントする | `screen.getByRole('button', { name: '送信' })`を取得する | ボタンが`disabled`属性を持たない（有効） |

#### 異常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-2-18 | `isSendButtonDisabled`が`true`を返す状態（例：`text=""`）でコンポーネントをマウントする | 送信ボタンを取得する | ボタンが`disabled`属性を持つ（無効化されている） |

#### 例外系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-2-19 | 送信ボタンが`disabled`状態のコンポーネントをマウントする | 送信ボタンをクリックする | 送信コールバック（`onSubmit`相当）が呼ばれない（`disabled`によるガードが実効していることの確認） |

---

## 4-3. エラー表現の出し分け（ずんだもん応答バブル vs 3秒トースト）

7章X確定により、フロントに独自の判定ロジックは存在せず、SSEイベント種別への単純ルーティングであることが前提。7章Y確定により、①ルーティング関数と②表示コンポーネント（Toast・応答バブル）の両方をテスト対象とする。

### 4-3-1. `routeSSEEvent`（イベントルーティング。呼び出し先はモック化した関数）

**対象関数**: `routeSSEEvent(event: SSEEvent, handlers: { onError: (message: string) => void; onMessage: (text: string, emotion: Emotion) => void }): void`相当。イベント種別に応じて`onError`または`onMessage`のいずれか一方のみを呼ぶ薄い実装であることを前提とする（7章X確定）。

#### 正常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-3-01 | `{ type: "error", payload: { message: "サーバーエラーが発生しました" } }`のイベントを用意する | `routeSSEEvent(event, { onError, onMessage })`を呼ぶ | `onError`が`"サーバーエラーが発生しました"`で1回呼ばれる。`onMessage`は呼ばれない |
| TC-4-3-02 | `text="こんにちは"`, `emotion="喜び"`の通常応答イベントを用意する | `routeSSEEvent(event, { onError, onMessage })`を呼ぶ | `onMessage`が`("こんにちは", "喜び")`で呼ばれる。`onError`は呼ばれない |
| TC-4-3-03 | `text="もう一度お話しください"`, `emotion="困惑"`（STT失敗由来の困惑・リトライ文言）のイベントを用意する | `routeSSEEvent(event, { onError, onMessage })`を呼ぶ | TC-4-3-02と全く同一の`onMessage`呼び出しパスで`("もう一度お話しください", "困惑")`が渡される（STT失敗由来かどうかを判定する特別分岐が存在しないことの確認、7章X確定） |

#### 異常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-3-04 | `{ type: "error", payload: {} }`（`message`フィールドが欠落したイベント）を用意する | `routeSSEEvent(event, { onError, onMessage })`を呼ぶ | 【暫定・要確認事項AA】`onError`がデフォルトメッセージ（文言未確定）で呼ばれる想定とし、例外は投げない |
| TC-4-3-05 | `{ type: "error", payload: { message: "" } }`（`message`が空文字列）を用意する | `routeSSEEvent(event, { onError, onMessage })`を呼ぶ | 【暫定・要確認事項AA】TC-4-3-04と同様にデフォルトメッセージへフォールバックする想定とする |

#### 例外系（排他性の契約）

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-3-06 | `{ type: "error", ... }`のイベントを用意する | `routeSSEEvent(event, { onError, onMessage })`を呼ぶ | `onMessage`が一度も呼ばれない（`error`と通常応答が同時に処理されないことの契約確認） |
| TC-4-3-07 | 通常応答イベント（`text`/`emotion`）を用意する | `routeSSEEvent(event, { onError, onMessage })`を呼ぶ | `onError`が一度も呼ばれない（逆方向の排他性確認） |

### 4-3-2. Toastコンポーネント（3秒消滅。`vi.useFakeTimers()`で決定的に検証）

#### 正常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-3-08 | `message="サーバーエラーが発生しました"`をpropsに渡してToastコンポーネントをマウントする | 表示内容を確認する | `role="alert"`要素に`"サーバーエラーが発生しました"`が表示される |
| TC-4-3-09 | Toastコンポーネントをマウントする | `vi.useFakeTimers()`環境で`vi.advanceTimersByTime(3000)`を実行する | トースト要素がDOMから消滅する（`queryByRole('alert')`が`null`になる） |

#### 異常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-3-10 | トースト表示中（3秒タイマー作動中）にコンポーネントを`unmount()`する | `unmount()`後に`vi.advanceTimersByTime(3000)`を実行する | 状態更新エラー・警告（destroyed componentへの書き込み等）が発生しない（タイマーが正しくクリアされている） |

#### 境界値

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-3-11 | Toastコンポーネントをマウントする | `vi.advanceTimersByTime(2999)`を実行した時点で確認する | トースト要素はまだDOMに存在する |
| TC-4-3-12 | Toastコンポーネントをマウントする | `vi.advanceTimersByTime(3000)`を実行した時点（ちょうど3秒）で確認する | トースト要素がDOMから消滅している（TC-4-3-11との対比で境界を確定） |

#### 例外系（暫定・未定義挙動）

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-3-13 | `error`イベントに対応するトースト表示処理を、短時間（例：100ms間隔）で2回連続して実行する | 2回目のトースト表示処理を実行した時点のDOM状態を確認する | 【暫定・要確認事項BB】1件目のトーストが2件目に上書きされる想定とし、多重表示にならないことのみを暫定的に検証する（上書き／キューイングいずれを正式仕様とするかは未定義） |

### 4-3-3. 応答バブル（ずんだもん応答表示、既存パスとの共通性検証）

#### 正常系

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-4-3-14 | 通常のtext/emotionイベント（`emotion="喜び"`, `text="こんにちは"`）を会話ログへの追記処理に渡す | 追記処理を実行する | 会話ログにずんだもんの発話として`"こんにちは"`が表示される |
| TC-4-3-15 | 困惑・リトライ文言を含むtext/emotionイベント（`emotion="困惑"`, `text="もう一度お願いします"`）を会話ログへの追記処理に渡す | 追記処理を実行する | TC-4-3-14と完全に同一の追記関数・コンポーネントパスで表示される（DOM構造・使用コンポーネントがTC-4-3-14と同一であることを比較検証し、STT失敗由来の特別な表示分岐が存在しないことを確認するネガティブテスト） |

---

## セルフチェック（重複排除・不足確認）

### 重複排除の実施内容

- 4-1: 「無効値のフォールバック」（7章T）は`isInputMode`（型ガード単体、TC-4-1-03〜10）と`getStoredInputMode`（フォールバック込みの結果、TC-4-1-16〜20）の2レイヤーでテストしているが、前者は「値が有効か」のみ、後者は「無効時に`"voice"`が返るか」のみを検証しており、検証対象が異なるため重複ではない。
- 4-2: 「録音中」（V）と「送信中」（W）を個別のbooleanフラグではなく`InputState`の値として1パラメータに統合したことで、めたんの初期観点にあった「録音中かつ変換中」のような矛盾状態の組み合わせテストは型レベルで到達不能となり、設計不要と判断した（総括4章の指摘通り）。
- 4-3: `routeSSEEvent`の正常系（TC-4-3-01〜03）と排他性契約（TC-4-3-06〜07）は、前者が「呼ばれるべき関数が正しい引数で呼ばれるか」、後者が「呼ばれるべきでない関数が呼ばれないか」を検証しており、観点が逆方向のため重複ではない。

### めたんの観点との突合結果

| 観点（めたん） | 反映状況 |
|---|---|
| 4-1 純粋関数・薄いコンポーネントへの分離（前提1・4-1責務分離） | `isInputMode`（4-1-1）、`getStoredInputMode`/`setStoredInputMode`（4-1-2）、コンポーネント（4-1-3）の3層で反映 |
| 4-1 不正値・例外時のフォールバック（7章T） | TC-4-1-03〜10、TC-4-1-16〜20で反映 |
| 4-1 録音中・変換中のトグルdisabled、入力クリア（7章U） | TC-4-1-24〜26で反映 |
| 4-1 高速連続操作時の最終値一致 | TC-4-1-27で反映 |
| 4-1 `localStorage`完全モック方針（7章Z） | 4-1-2の対象関数説明、TC-4-1-11〜20のGiven記述で反映 |
| 4-2 disabled判定の単一純粋関数への集約 | `isSendButtonDisabled`（4-2-1）で反映 |
| 4-2 `InputState`による状態機械化（録音中・送信中を含む、7章V・W） | 4-2冒頭の型定義説明、TC-4-2-04・08（録音中）、TC-4-2-09・10（送信中）で反映 |
| 4-2 空白判定のtrim対象文字種の網羅 | TC-4-2-06、TC-4-2-12〜15で反映 |
| 4-2 文字数上限なしの確認 | TC-4-2-16で反映 |
| 4-2 STT変換中は入力内容に関わらず無効 | TC-4-2-07で反映 |
| 4-3 単純ルーティングへのスコープ縮小（7章X） | 4-3-1の対象関数を薄いルーティングに限定し、判定ロジックのテストケースを設計していないことで反映 |
| 4-3 F-RT-02のスコープ外化（7章X） | 4-3にF-RT-02関連のテストケースを含めていないことで反映（セルフチェックとして明記） |
| 4-3 ルーティング層と表示層の2階層テスト（7章Y） | 4-3-1（ルーティング）と4-3-2・4-3-3（表示コンポーネント）で反映 |
| 4-3 3秒タイマーの`vi.useFakeTimers()`による決定的検証 | TC-4-3-09、TC-4-3-11〜12で反映 |
| 4-3 困惑・リトライ文言が通常応答と同一パスであることのネガティブテスト | TC-4-3-03（ルーティング層）、TC-4-3-15（応答バブル層）の2層で反映 |

### 抽出した不足ケース（初期観点になかったが追加したもの）

- TC-4-1-19: `setStoredInputMode`側の例外（`QuotaExceededError`相当）は7章Tの決定に`getItem`/`setItem`両方が含まれていたが、めたんの異常系列挙は`getItem`側の例外が主だったため、`setItem`側の例外注入ケースを明示的に追加した。
- TC-4-2-19: 送信ボタンの`disabled`属性がDOMに反映されるだけでなく、クリックしてもコールバックが実行されないことまでを確認するガード確認ケースを追加した（属性の見た目だけでなく実効性を担保する）。
- TC-4-3-06・07: ルーティング関数の排他性（一方が呼ばれたら他方は呼ばれない）は、めたんの観点整理では前提として述べられていたのみだったため、契約テストとして明示的にケース化した。

---

## 未決事項（WhiteCUL・つむぎへの申し送り）

7章確認事項T〜Zとは別に、本書のテストケース設計中に新たに識別した論点をAA以降として申し送る。**AA〜CCはつむぎ判断により決定済み（2026-07-09）。**

| # | 対象 | 論点 | 決定内容 |
|---|------|------|------------------|
| AA | 4-3 | SSE `error`イベントのペイロードに`message`フィールドが欠落・空文字列の場合、デフォルトメッセージへのフォールバックが必要か（フォールバック文言も含めて画面設計書に明記なし） | ✅決定：暫定対応（デフォルトメッセージへフォールバック）のまま確定。TC-4-3-04・05の期待値は変更なし |
| BB | 4-3 | `error`イベントが短時間で連続して複数回届いた場合のトースト表示の挙動（上書き・キューイング・多重表示のいずれか）が画面設計書に明記なし | ✅決定：暫定対応（上書き方式、キューイングなし）のまま確定。シンプルさ優先。TC-4-3-13の期待値は変更なし |
| CC | 4-1×4-2×4-3 | STT失敗によりテキスト入力欄が空に戻る遷移（4-2）と、モード切替（4-1）・エラートースト表示（4-3）が同時に発生した場合の3機能間の状態競合（めたんが4-2例外系で言及、今回の仕様決定の対象外として明示的にスコープ外） | ✅決定：ひまりの判断（単体テスト対象外）を承認。結合テスト／E2Eテストフェーズ以降で対応。本フェーズでの対応は不要 |

以上、フェーズ4（3機能・純粋関数レベル/コンポーネントレベル分割含む）で計 **61件** のテストケースを設計（4-1: 27件、4-2: 19件、4-3: 15件）。未決事項AA〜CCはつむぎ判断により決定済みのため、WhiteCULへの単体テスト仕様書清書・テストコード化に着手可能。

**2026-07-09追記（そらのQAレビュー指摘反映）**: TC-4-1-26は`ModeToggleProps`の責務範囲との不整合により欠番とした（詳細は上表参照）。本フェーズの実施対象は**60件**（4-1: 26件、4-2: 19件、4-3: 15件）に確定する。
