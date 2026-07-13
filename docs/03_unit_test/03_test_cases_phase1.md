# フェーズ1 単体テストケース一覧

| 項目 | 内容 |
|------|------|
| バージョン | 1.0 |
| 作成日 | 2026-07-08 |
| 作成者 | ひまり（テストケース設計担当） |
| 入力 | `02_test_perspectives_phase1.md`（めたん作成） |
| 前提 | 6章確認事項A〜E：つむぎ仮決定（本書内で反映） |
| 次工程 | WhiteCULが単体テスト仕様書・テストコードへ清書 |

---

## 0. 前提：確認事項A〜Eの決定内容（本書での反映方針）

| # | 対象 | 決定内容 | 本書での反映 |
|---|------|---------|-------------|
| A | 1-2 | `…`はフロント表示専用の装飾。DB格納値には含めない | `TruncateFirstText`は`…`を付与せず先頭20ルーンをそのまま返す純粋関数として設計 |
| B | 1-1 | ULID先頭文字の値域制約（実質0〜7）はチェック対象外。文字種・桁数のみ判定 | 先頭文字が8以上でも文字種・桁数を満たせば`true`とするケースを明示 |
| C | 1-1 | 小文字ULID・前後空白付きULIDは非許容 | いずれも`false`とするケースを設計 |
| D | 1-4 | ゼロ幅スペース（U+200B）等の不可視文字への対応はスコープ外。標準の`TrimSpace`相当の範囲のみ対応する | ゼロ幅スペースのみの入力は「非空」＝valid扱いになるという挙動を、明示的に仕様として固定するケースを追加（除去対応・カスタム実装は行わない） |
| E | 1-5 | GC判定はGo側の純粋関数`IsExpired(expiresAt, now time.Time) bool`を対象とする | Repository層のDELETE実行はフェーズ2の対象として明確に除外 |

---

## 目次

1. [1-1. `IsValidULID`](#1-1-isvalidulid)
2. [1-2. `TruncateFirstText`](#1-2-truncatefirsttext)
3. [1-3. role・emotionバリデーション（3関数）](#1-3-roleemotionバリデーション3関数)
4. [1-4. `IsValidInput`](#1-4-isvalidinput)
5. [1-5. `IsExpired`](#1-5-isexpired)
6. [セルフチェック（重複排除・不足確認）](#6-セルフチェック重複排除不足確認)
7. [未決事項（WhiteCUL・つむぎへの申し送り）](#7-未決事項whitecul・つむぎへの申し送り)

---

## 1-1. `IsValidULID`

**対象関数**: `IsValidULID(s string) bool`（文字列→真偽値の純粋関数。DB存在確認は含まない）

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-1-1-01 | 有効な26文字ULID（例: `01ARZ3NDEKTSV4RRFFQ69G5FAV`）を用意する | `IsValidULID(s)`を呼ぶ | `true`を返す（ちょうど26文字の境界値も兼ねる） |
| TC-1-1-02 | 実際の採番関数（`oklog/ulid/v2`）でULIDを100件生成する | 生成した各ULIDを`IsValidULID`に通す | 全件`true`を返す（生成⇔検証の往復整合性） |
| TC-1-1-03 | 有効なULIDの末尾1文字を削除し25文字にする | `IsValidULID(s)`を呼ぶ | `false`を返す（1文字不足の境界値） |
| TC-1-1-04 | 有効なULIDに1文字追加し27文字にする | `IsValidULID(s)`を呼ぶ | `false`を返す（1文字超過の境界値） |
| TC-1-1-05 | 26文字中1文字を`I`に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す |
| TC-1-1-06 | 26文字中1文字を`L`に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す |
| TC-1-1-07 | 26文字中1文字を`O`に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す |
| TC-1-1-08 | 26文字中1文字を`U`に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す |
| TC-1-1-09 | 空文字列`""`を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す |
| TC-1-1-10 | UUID形式（例: `550e8400-e29b-41d4-a716-446655440000`、ハイフンあり36文字）を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す |
| TC-1-1-11 | 26文字中1文字を`H`（Iの直前の許容文字）に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `true`を返す（除外文字の隣接境界） |
| TC-1-1-12 | 26文字中1文字を`J`（Iの直後の許容文字）に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `true`を返す |
| TC-1-1-13 | 26文字中1文字を`K`（Lの直前の許容文字）に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `true`を返す |
| TC-1-1-14 | 26文字中1文字を`M`（Lの直後の許容文字）に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `true`を返す |
| TC-1-1-15 | 26文字中1文字を`T`（Uの直前の許容文字）に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `true`を返す |
| TC-1-1-16 | 26文字中1文字を`V`（Uの直後の許容文字）に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `true`を返す |
| TC-1-1-17 | 先頭文字を`Z`にした26文字・文字種は正当なULID風文字列を用意する（値域上は128bit超） | `IsValidULID(s)`を呼ぶ | `true`を返す（確認事項B：値域チェックは対象外） |
| TC-1-1-18 | 有効な26文字ULIDの前後に半角スペースを付与した文字列（`" 01ARZ...FAV "`）を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す（確認事項C：trimしない） |
| TC-1-1-19 | 有効な26文字ULIDを全て小文字化した文字列を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す（確認事項C：小文字は非許容） |
| TC-1-1-20 | 26文字中1文字を全角文字（例: `Ａ`）に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す |
| TC-1-1-21 | 26文字中1文字を絵文字（例: `😀`）に差し替えた文字列を用意する | `IsValidULID(s)`を呼ぶ | `false`を返す |

---

## 1-2. `TruncateFirstText`

**対象関数**: `TruncateFirstText(text string) string`（内部で`const FirstTextMaxRunes = 20`参照。確認事項Aにより`…`は付与しない純粋関数）

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-1-2-01 | 5文字のテキスト`"こんにちは"`を用意する | `TruncateFirstText(text)`を呼ぶ | `"こんにちは"`をそのまま返す |
| TC-1-2-02 | ちょうど20文字のテキストを用意する | `TruncateFirstText(text)`を呼ぶ | カットなしで全20文字を返す（末尾文字も欠落しない） |
| TC-1-2-03 | 21文字のテキストを用意する | `TruncateFirstText(text)`を呼ぶ | 先頭20文字（ルーン単位）を`…`なしで返す（確認事項A） |
| TC-1-2-04 | 日本語・絵文字・英数字が混在する21文字以上のテキストを用意する | `TruncateFirstText(text)`を呼ぶ | マルチバイト文字境界を壊さず先頭20ルーンを返す |
| TC-1-2-05 | 空文字列`""`を用意する | `TruncateFirstText(text)`を呼ぶ | 空文字列を返す（panicしない） |
| TC-1-2-06 | ZWJ結合家族絵文字など結合文字列を含む21文字以上のテキストを用意する | `TruncateFirstText(text)`を呼ぶ | コードポイント単位でカットされ、結合絵文字が破損しうる（`[]rune`変換の仕様として固定） |
| TC-1-2-07 | 19文字のテキストを用意する | `TruncateFirstText(text)`を呼ぶ | カットなしで全文を返す |
| TC-1-2-08 | 1文字のテキストを用意する | `TruncateFirstText(text)`を呼ぶ | 無変換でそのまま返す |
| TC-1-2-09 | 1000文字のテキストを用意する | `TruncateFirstText(text)`を呼ぶ | 先頭20文字にカットされる |
| TC-1-2-10 | 改行・タブを含む21文字以上のテキスト（例: `"あ\nい\tう..."`）を用意する | `TruncateFirstText(text)`を呼ぶ | サニタイズせず、制御文字を含んだまま先頭20ルーンを返す |
| TC-1-2-11 | 前後に半角スペースを含む21文字以上のテキスト（例: `" 　+20文字超の本文 "`）を用意する | `TruncateFirstText(text)`を呼ぶ | trimせず、空白を含んだまま先頭20ルーンを返す（1-4との責務分離） |

---

## 1-3. role・emotionバリデーション（3関数）

めたんの指摘に基づき、以下3関数に責務分離してケースを設計する。

### 1-3-1. `ValidateRole(role string) error`

| ID | Given | When | Then |
|----|-------|------|------|
| TC-1-3-R01 | `role = "user"` | `ValidateRole(role)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-R02 | `role = "assistant"` | `ValidateRole(role)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-R03 | `role = "admin"`（CHECK制約外） | `ValidateRole(role)`を呼ぶ | エラーを返す |
| TC-1-3-R04 | `role = ""`（空文字列） | `ValidateRole(role)`を呼ぶ | エラーを返す |
| TC-1-3-R05 | `role = "User"`（大文字小文字違い） | `ValidateRole(role)`を呼ぶ | エラーを返す（完全一致のみ許容） |
| TC-1-3-R06 | `role = "ASSISTANT"`（大文字小文字違い） | `ValidateRole(role)`を呼ぶ | エラーを返す |

### 1-3-2. `ValidateEmotion(emotion *string) error`

| ID | Given | When | Then |
|----|-------|------|------|
| TC-1-3-E01 | `emotion = nil` | `ValidateEmotion(emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-E02 | `emotion = "喜び"` | `ValidateEmotion(emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-E03 | `emotion = "怒り"` | `ValidateEmotion(emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-E04 | `emotion = "悲しみ"` | `ValidateEmotion(emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-E05 | `emotion = "楽しい"` | `ValidateEmotion(emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-E06 | `emotion = "照れ"` | `ValidateEmotion(emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-E07 | `emotion = "困惑"` | `ValidateEmotion(emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-E08 | `emotion = "ドヤ顔"` | `ValidateEmotion(emotion)`を呼ぶ | `nil`（valid）を返す（7種全件網羅） |
| TC-1-3-E09 | `emotion = "普通"`（7種にない値） | `ValidateEmotion(emotion)`を呼ぶ | エラーを返す |
| TC-1-3-E10 | `emotion = "happy"`（ローマ字表記） | `ValidateEmotion(emotion)`を呼ぶ | エラーを返す |
| TC-1-3-E11 | `emotion = " 喜び "`（前後空白付き） | `ValidateEmotion(emotion)`を呼ぶ | エラーを返す（trimせず非許容） |
| TC-1-3-E12 | `emotion = ""`（空文字列。NULLとは区別） | `ValidateEmotion(emotion)`を呼ぶ | エラーを返す（※要WhiteCUL確認、7章参照） |
| TC-1-3-E13 | `emotion = "喜"`（7種のうち1文字のみ） | `ValidateEmotion(emotion)`を呼ぶ | エラーを返す（部分一致でvalid化しない） |
| TC-1-3-E14 | `emotion = "喜びだ"`（前方一致） | `ValidateEmotion(emotion)`を呼ぶ | エラーを返す |

### 1-3-3. `ValidateRoleEmotionConsistency(role string, emotion *string) error`

**前提**: `role`・`emotion`は各々`ValidateRole`/`ValidateEmotion`で単体valid判定済みの値を渡す想定（本関数は組み合わせルールのみを検証する）。

| ID | Given | When | Then |
|----|-------|------|------|
| TC-1-3-C01 | `role = "user"`, `emotion = nil` | `ValidateRoleEmotionConsistency(role, emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-C02 | `role = "assistant"`, `emotion = "喜び"` | `ValidateRoleEmotionConsistency(role, emotion)`を呼ぶ | `nil`（valid）を返す |
| TC-1-3-C03 | `role = "assistant"`, `emotion = nil` | `ValidateRoleEmotionConsistency(role, emotion)`を呼ぶ | `nil`（valid）を返す（assistantでもemotion未設定を許容する仕様を明示固定） |
| TC-1-3-C04 | `role = "user"`, `emotion = "喜び"`（単体は各々valid、組み合わせが矛盾） | `ValidateRoleEmotionConsistency(role, emotion)`を呼ぶ | エラーを返す（★本関数の存在意義そのものである核心ケース） |

---

## 1-4. `IsValidInput`

**対象関数**: `IsValidInput(text string) bool`（trim後非空なら`true`。確認事項Dにより、不可視文字への個別対応は行わず標準`TrimSpace`相当の範囲のみを対象とする）

| ID | Given | When | Then |
|----|-------|------|------|
| TC-1-4-01 | 通常のテキスト`"こんにちは"` | `IsValidInput(text)`を呼ぶ | `true`を返す |
| TC-1-4-02 | 前後空白ありで中身がある文字列`" こんにちは "` | `IsValidInput(text)`を呼ぶ | `true`を返す |
| TC-1-4-03 | 空文字列`""` | `IsValidInput(text)`を呼ぶ | `false`を返す |
| TC-1-4-04 | 半角スペースのみ`"   "` | `IsValidInput(text)`を呼ぶ | `false`を返す |
| TC-1-4-05 | タブ・改行のみ`"\t\n"` | `IsValidInput(text)`を呼ぶ | `false`を返す |
| TC-1-4-06 | 全角スペースのみ`"　　　"` | `IsValidInput(text)`を呼ぶ | `false`を返す |
| TC-1-4-07 | ゼロ幅スペース（U+200B）のみ | `IsValidInput(text)`を呼ぶ | `true`を返す（確認事項D：スコープ外。標準`TrimSpace`では除去されないため「非空」＝valid扱いになる仕様をそのまま固定する） |
| TC-1-4-08 | ゼロ幅スペースと半角スペースが混在するのみ（可視文字なし） | `IsValidInput(text)`を呼ぶ | `true`を返す（半角スペース部分のみtrimされ、ゼロ幅スペースが残るためvalid扱いになる仕様を明示） |
| TC-1-4-09 | 1文字のみの非空白文字`"あ"` | `IsValidInput(text)`を呼ぶ | `true`を返す |
| TC-1-4-10 | 1文字のみの空白文字`" "` | `IsValidInput(text)`を呼ぶ | `false`を返す |
| TC-1-4-11 | 空白+非空白+空白`" a "` | `IsValidInput(text)`を呼ぶ | `true`を返す |
| TC-1-4-12 | 1000文字の半角スペースのみ | `IsValidInput(text)`を呼ぶ | `false`を返す |
| TC-1-4-13 | 絵文字のみ`"😀"` | `IsValidInput(text)`を呼ぶ | `true`を返す（絵文字はtrim対象外） |
| TC-1-4-14 | ゼロ幅スペースと実文字が混在`"こんにちは​"` | `IsValidInput(text)`を呼ぶ | `true`を返す（実文字が残るため、いずれの解釈でもvalid） |

---

## 1-5. `IsExpired`

**対象関数**: `IsExpired(expiresAt, now time.Time) bool`（確認事項E：GC対象の削除実行自体はフェーズ2のRepository層で扱うためスコープ外）

| ID | Given | When | Then |
|----|-------|------|------|
| TC-1-5-01 | `expiresAt`が`now`の1日前 | `IsExpired(expiresAt, now)`を呼ぶ | `true`を返す |
| TC-1-5-02 | `expiresAt`が`now`の1日後（30日以内） | `IsExpired(expiresAt, now)`を呼ぶ | `false`を返す |
| TC-1-5-03 | `expiresAt`がゼロ値（`time.Time{}`） | `IsExpired(expiresAt, now)`を呼ぶ | `true`を返す（防御的観点として明示） |
| TC-1-5-04 | `expiresAt == now`（完全一致） | `IsExpired(expiresAt, now)`を呼ぶ | `false`を返す（★`<`の等号非対称性、最重要境界値） |
| TC-1-5-05 | `expiresAt`が`now`の1ナノ秒前 | `IsExpired(expiresAt, now)`を呼ぶ | `true`を返す |
| TC-1-5-06 | `expiresAt`が`now`の1ナノ秒後 | `IsExpired(expiresAt, now)`を呼ぶ | `false`を返す |
| TC-1-5-07 | `expiresAt`をUTCで指定し、`now`を同一絶対時刻のJSTで指定する（タイムゾーン相違） | `IsExpired(expiresAt, now)`を呼ぶ | タイムゾーンに関わらず絶対時刻として正しく`false`を返す |

---

## 6. セルフチェック（重複排除・不足確認）

### 重複排除の実施内容

- 1-1: 「正常系：ちょうど26文字」と「境界値：ちょうど26文字」はTC-1-1-01に統合。「異常系：25/27文字」と「境界値：25/27文字」はTC-1-1-03/04に統合。
- 1-2: 「正常系：ちょうど20文字」「21文字カット」と、それぞれの境界値ケースはTC-1-2-02/03に統合。
- 1-3: 役割ごとに関数を分離したことで、「role×emotion両方妥当だが組み合わせ矛盾」（旧・単一関数案での曖昧ケース）が、単体バリデーション（R/E系列）と組み合わせバリデーション（C系列）に明確に分解され、テスト名の曖昧化を防止。

### めたんの観点との突合結果

| 観点（めたん） | 反映状況 |
|---|---|
| 1-1 往復整合性（生成⇔検証） | TC-1-1-02で反映 |
| 1-1 隣接除外文字の境界（H/J, K/M, T/V） | TC-1-1-11〜16で反映 |
| 1-1 責務分離（DB存在確認を含まない） | 対象関数を`IsValidULID(s string) bool`の純粋関数と明記（テストケースというより実装契約として本書冒頭に明示） |
| 1-2 マジックナンバー20の定数化 | 対象関数の説明に`FirstTextMaxRunes`定数参照を明記 |
| 1-3 7種全件個別ケース化 | TC-1-3-E02〜E08で反映 |
| 1-3 3関数への責務分離 | R/E/C系列として反映 |
| 1-4 Go/Svelte両フェーズでの観点共有 | 本書のケース群をフェーズ4（4-2）設計時にも再利用可能な形で記載（実装非依存の入出力ベース） |
| 1-5 Clock注入による決定性 | `IsExpired`が`now`を引数で受け取る設計そのものにより自動的に満たされる（TC-1-5-04〜06はこの設計が前提） |
| 1-5 複数件リストからのフィルタ | 対象関数が`IsExpired`（単一レコード判定）であるため、フェーズ1のスコープ外と判断し本書では対象外（7章参照） |

### 抽出した不足ケース（初期観点になかったが追加したもの）

- TC-1-1-17: 確認事項Bの決定（値域チェック対象外）を反映した「先頭文字8以上でもtrue」の明示ケース。
- TC-1-3-C03: 「assistant×emotion=NULL」を許容する仕様の明示固定（めたんは「要確認」としていたが、組み合わせ関数のvalidケースとして確定）。
- TC-1-4-08: ゼロ幅スペース＋半角スペースが混在するケース。標準`TrimSpace`では半角スペースのみ除去されゼロ幅スペースが残るため`true`になる、という組み合わせ時の挙動を明示的に固定（確認事項Dがスコープ外決定であることの裏付けケース）。

---

## 7. 決定事項F・G（つむぎ決定・2026-07-08）

| # | 対象 | 論点 | 決定内容 | 本書への反映 |
|---|------|------|---------|-------------|
| F | 1-3 `ValidateEmotion` | `emotion = ""`（空文字列）と`emotion = nil`（NULL）を区別して扱うか | **空文字列はinvalidとして扱う**。NULLは「意図的な未設定」という明示的な状態だが、空文字列は呼び出し側の実装ミス（nilにし忘れ等）の可能性が高く、区別して弾くことでバグの早期発見につながるため | TC-1-3-E12（空文字列→invalid）は変更不要、確定 |
| G | 1-3 全体 | `role`・`emotion`両方が不正な場合、エラーを1件で止めるか集約するか | **今回は決定不要**。本書のテストケースは「エラーが返ること」のみを検証しており、いずれの実装方針でも成立するため、実装確定後に必要であれば集約検証ケースを追加する | 本書のケースはそのまま確定。追加ケースは実装確定後に別途起票 |

以上、フェーズ1（5機能・3関数分割含む）で計 **77件**（訂正: 当初「67件」と記載していたが、テストケースIDの機械的突合により集計ミスと判明したため訂正。表の中身自体に不足・重複はなし） のテストケースを設計。未決事項F・Gも決定済みとなったため、本書は確定版としてWhiteCULへの単体テスト仕様書清書・テストコード化に引き継ぐ。
