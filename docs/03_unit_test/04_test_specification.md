# 単体テスト仕様書 — フェーズ1（純粋ロジック・バリデーション）

| 項目 | 内容 |
|------|------|
| バージョン | 1.0 |
| 作成日 | 2026-07-08 |
| 作成者 | WhiteCUL（テスト仕様書・テストコード作成担当） |
| 入力 | `01_test_plan.md`（テスト計画）、`02_test_perspectives_phase1.md`（テスト観点、めたん作成）、`03_test_cases_phase1.md`（テストケース、ひまり作成） |
| 対象 | フェーズ1（純粋ロジック・バリデーション、5機能・3関数分割を含む計7関数） |
| 次工程 | そらによるQAレビュー |

---

## 目次

1. [目的・対象](#1-目的対象)
2. [設計方針（実装への要求事項）](#2-設計方針実装への要求事項)
3. [パッケージ構成・テスト対象関数シグネチャ一覧](#3-パッケージ構成テスト対象関数シグネチャ一覧)
4. [テストケース一覧](#4-テストケース一覧)
5. [テストケース総数の確認結果](#5-テストケース総数の確認結果)
6. [テストコード配置・実行方法](#6-テストコード配置実行方法)
7. [未決事項の反映状況（F・G）](#7-未決事項の反映状況fg)

---

## 1. 目的・対象

本書は `03_test_cases_phase1.md` で設計されたフェーズ1のテストケースを、実装着手前の正式な単体テスト仕様書として清書したものである。
対象は `01_test_plan.md` フェーズ1に定義された5機能（うち1機能は3関数に責務分離）であり、すべて外部依存（DB・HTTP・ファイルI/O）を持たない純粋関数として実装されることを前提とする。

対応する機能ID・設計根拠は `02_test_perspectives_phase1.md` の通り。

| 観点番号 | 対象関数 | 対応機能ID / 設計根拠 |
|---------|---------|----------------------|
| 1-1 | `IsValidULID` | F-HIST-01, F-HIST-04 / 画面設計書8章 |
| 1-2 | `TruncateFirstText` | F-HIST-05 / DB設計書5.4 |
| 1-3 | `ValidateRole` / `ValidateEmotion` / `ValidateRoleEmotionConsistency` | F-EMO-01 / DB設計書2.2, 5.5 |
| 1-4 | `IsValidInput` | NF-SEC-01 / 画面設計書8章 |
| 1-5 | `IsExpired` | F-HIST-07 / DB設計書5.2 |

---

## 2. 設計方針（実装への要求事項）

`02_test_perspectives_phase1.md` 総括および7章の仕様決定を踏まえ、GREEN phase（実装）着手時に満たすべき設計方針を以下に定める。テストコードはこの方針を前提に記述している。

1. **純粋関数化**: すべて `(入力) → (真偽値 or エラー)` の純粋関数として実装する。DBアクセス・HTTPリクエスト処理・ファイルI/Oを一切含まない。
2. **時刻注入による決定性**: `IsExpired` は関数内部で `time.Now()` を直接呼び出さず、`now time.Time` を引数として外部から注入する。これにより境界値（ちょうどNOWの前後）のテストが決定的に書ける（テスト計画書4章の方針と整合）。
3. **定数化**: マジックナンバー・マジックストリングは、実装コードとテストコードの双方から参照可能な定数として定義する。
   - `FirstTextMaxRunes = 20`（`TruncateFirstText` が参照）
   - 感情ラベル7種は `ValidEmotions []string` としてパッケージ変数化する（`ValidateEmotion` が参照）
4. **責務分離**: 「role単体の妥当性」「emotion単体の妥当性」「role×emotionの組み合わせ整合性」は独立した3関数に分離する（`ValidateRole` / `ValidateEmotion` / `ValidateRoleEmotionConsistency`）。ULID形式チェック（`IsValidULID`）とDB存在確認（フェーズ2の対象）も同様に分離する。
5. **正規表現の事前コンパイル**: `IsValidULID` 等で正規表現を用いる場合、パッケージレベルで一度だけ `regexp.MustCompile` し、関数呼び出しのたびに再コンパイルしない。

---

## 3. パッケージ構成・テスト対象関数シグネチャ一覧

Goの標準的なプロジェクト構成（`.claude/rules/golang-coding-guideline.md` 3章）に倣い、フェーズ1対象の純粋関数群を以下の2パッケージに配置する。

| パッケージ | 役割 |
|-----------|------|
| `internal/validation` | ULID形式・first_textカット・role/emotion・入力バリデーションの純粋関数群 |
| `internal/gc` | 会話セッションのGC（自動削除）判定に関する純粋関数群 |

> **前提条件**: 本書作成時点でリポジトリに `go.mod` が存在しなかったため、テストコード作成にあたり `module zuncha` として新規に初期化した。パッケージパス（`zuncha/internal/validation` 等）は実装チームの合意により変更してよい。変更する場合はテストコードのimport文もあわせて修正すること。

### 関数シグネチャ一覧

| # | 関数シグネチャ | 所属パッケージ | 対応観点 |
|---|---------------|---------------|---------|
| 1 | `func IsValidULID(s string) bool` | `internal/validation` | 1-1 |
| 2 | `func TruncateFirstText(text string) string` | `internal/validation` | 1-2 |
| 3 | `func ValidateRole(role string) error` | `internal/validation` | 1-3 |
| 4 | `func ValidateEmotion(emotion *string) error` | `internal/validation` | 1-3 |
| 5 | `func ValidateRoleEmotionConsistency(role string, emotion *string) error` | `internal/validation` | 1-3 |
| 6 | `func IsValidInput(text string) bool` | `internal/validation` | 1-4 |
| 7 | `func IsExpired(expiresAt, now time.Time) bool` | `internal/gc` | 1-5 |

### 参照定数・パッケージ変数

| 識別子 | 型 | 値 | 所属パッケージ |
|--------|----|----|--------------|
| `FirstTextMaxRunes` | `int` | `20` | `internal/validation` |
| `ValidEmotions` | `[]string` | `{"喜び", "怒り", "悲しみ", "楽しい", "照れ", "困惑", "ドヤ顔"}` | `internal/validation` |

---

## 4. テストケース一覧

`03_test_cases_phase1.md` のGiven/When/Thenをそのまま踏襲し、対応するGoテストコード（サブテスト名）を突き合わせる。テストケースIDと1テストケース＝1サブテストが1対1で対応する。

### 4.1 1-1. `IsValidULID`（21件）

**テストファイル**: `tests/unit/ulid_test.go`

| ID | Given | Then | 対応サブテスト（`TestIsValidULID` 内） |
|----|-------|------|------------------------------------|
| TC-1-1-01 | 有効な26文字ULID | `true` | `TC-1-1-01_有効な26文字ULIDでtrueを返す` |
| TC-1-1-02 | 実際の採番関数で生成したULID100件 | 全件`true` | `TestIsValidULID_生成ULIDの往復整合性`（別関数） |
| TC-1-1-03 | 25文字（1文字不足） | `false` | `TC-1-1-03_25文字1文字不足でfalseを返す` |
| TC-1-1-04 | 27文字（1文字超過） | `false` | `TC-1-1-04_27文字1文字超過でfalseを返す` |
| TC-1-1-05 | 除外文字`I`を含む | `false` | `TC-1-1-05_除外文字Iを含む場合falseを返す` |
| TC-1-1-06 | 除外文字`L`を含む | `false` | `TC-1-1-06_除外文字Lを含む場合falseを返す` |
| TC-1-1-07 | 除外文字`O`を含む | `false` | `TC-1-1-07_除外文字Oを含む場合falseを返す` |
| TC-1-1-08 | 除外文字`U`を含む | `false` | `TC-1-1-08_除外文字Uを含む場合falseを返す` |
| TC-1-1-09 | 空文字列 | `false` | `TC-1-1-09_空文字列でfalseを返す` |
| TC-1-1-10 | UUID形式（36文字） | `false` | `TC-1-1-10_UUID形式でfalseを返す` |
| TC-1-1-11 | 隣接許容文字`H` | `true` | `TC-1-1-11_隣接許容文字Hでtrueを返す` |
| TC-1-1-12 | 隣接許容文字`J` | `true` | `TC-1-1-12_隣接許容文字Jでtrueを返す` |
| TC-1-1-13 | 隣接許容文字`K` | `true` | `TC-1-1-13_隣接許容文字Kでtrueを返す` |
| TC-1-1-14 | 隣接許容文字`M` | `true` | `TC-1-1-14_隣接許容文字Mでtrueを返す` |
| TC-1-1-15 | 隣接許容文字`T` | `true` | `TC-1-1-15_隣接許容文字Tでtrueを返す` |
| TC-1-1-16 | 隣接許容文字`V` | `true` | `TC-1-1-16_隣接許容文字Vでtrueを返す` |
| TC-1-1-17 | 先頭文字`Z`（値域上限超過だが文字種は正当） | `true`（確認事項B） | `TC-1-1-17_先頭文字が値域外でも文字種桁数を満たせばtrueを返す` |
| TC-1-1-18 | 前後空白付き有効ULID | `false`（確認事項C） | `TC-1-1-18_前後空白付きULIDはfalseを返す` |
| TC-1-1-19 | 全小文字化した有効ULID | `false`（確認事項C） | `TC-1-1-19_小文字ULIDはfalseを返す` |
| TC-1-1-20 | 全角文字混入 | `false` | `TC-1-1-20_全角文字混入でfalseを返す` |
| TC-1-1-21 | 絵文字混入 | `false` | `TC-1-1-21_絵文字混入でfalseを返す` |

### 4.2 1-2. `TruncateFirstText`（11件）

**テストファイル**: `tests/unit/first_text_test.go`

| ID | Given | Then | 対応サブテスト（`TestTruncateFirstText` 内） |
|----|-------|------|------------------------------------------|
| TC-1-2-01 | 5文字のテキスト | 無変換で返す | `TC-1-2-01_20文字以下はそのまま返す` |
| TC-1-2-02 | ちょうど20文字 | カットなしで全文返す | `TC-1-2-02_ちょうど20文字はカットなしで返す` |
| TC-1-2-03 | 21文字 | 先頭20文字（`…`なし） | `TC-1-2-03_21文字は先頭20文字にカットされる` |
| TC-1-2-04 | 日本語/絵文字/英数字混在21文字以上 | マルチバイト境界を壊さず20ルーン | `TC-1-2-04_マルチバイト混在でも文字境界を壊さない` |
| TC-1-2-05 | 空文字列 | 空文字列（panicしない） | `TC-1-2-05_空文字列は空文字列を返す` |
| TC-1-2-06 | ZWJ結合絵文字を含む21文字以上 | コードポイント単位でカット（破損しうる仕様を固定） | `TC-1-2-06_結合絵文字はコードポイント単位でカットされる` |
| TC-1-2-07 | 19文字 | カットなし | `TC-1-2-07_19文字はカットなしで返す` |
| TC-1-2-08 | 1文字 | 無変換 | `TC-1-2-08_1文字は無変換で返す` |
| TC-1-2-09 | 1000文字 | 先頭20文字にカット | `TC-1-2-09_1000文字は20文字にカットされる` |
| TC-1-2-10 | 改行・タブを含む21文字以上 | サニタイズせずそのままカット | `TC-1-2-10_制御文字を含んでもサニタイズせずカットする` |
| TC-1-2-11 | 前後空白を含む21文字以上 | trimせずそのままカット | `TC-1-2-11_前後空白をtrimせずカットする` |

### 4.3 1-3. role・emotionバリデーション（3関数・24件）

**テストファイル**: `tests/unit/role_emotion_test.go`

#### `ValidateRole`（6件）

| ID | Given | Then | 対応サブテスト（`TestValidateRole` 内） |
|----|-------|------|--------------------------------------|
| TC-1-3-R01 | `role = "user"` | `nil` | `TC-1-3-R01_userはvalidでnilを返す` |
| TC-1-3-R02 | `role = "assistant"` | `nil` | `TC-1-3-R02_assistantはvalidでnilを返す` |
| TC-1-3-R03 | `role = "admin"` | エラー | `TC-1-3-R03_CHECK制約外のroleはエラーを返す` |
| TC-1-3-R04 | `role = ""` | エラー | `TC-1-3-R04_空文字列roleはエラーを返す` |
| TC-1-3-R05 | `role = "User"` | エラー | `TC-1-3-R05_大文字小文字違いUserはエラーを返す` |
| TC-1-3-R06 | `role = "ASSISTANT"` | エラー | `TC-1-3-R06_大文字小文字違いASSISTANTはエラーを返す` |

#### `ValidateEmotion`（14件）

| ID | Given | Then | 対応サブテスト（`TestValidateEmotion` 内） |
|----|-------|------|------------------------------------------|
| TC-1-3-E01 | `emotion = nil` | `nil` | `TC-1-3-E01_emotionがnilはvalidでnilを返す` |
| TC-1-3-E02 | `emotion = "喜び"` | `nil` | `TC-1-3-E02_喜びはvalidでnilを返す` |
| TC-1-3-E03 | `emotion = "怒り"` | `nil` | `TC-1-3-E03_怒りはvalidでnilを返す` |
| TC-1-3-E04 | `emotion = "悲しみ"` | `nil` | `TC-1-3-E04_悲しみはvalidでnilを返す` |
| TC-1-3-E05 | `emotion = "楽しい"` | `nil` | `TC-1-3-E05_楽しいはvalidでnilを返す` |
| TC-1-3-E06 | `emotion = "照れ"` | `nil` | `TC-1-3-E06_照れはvalidでnilを返す` |
| TC-1-3-E07 | `emotion = "困惑"` | `nil` | `TC-1-3-E07_困惑はvalidでnilを返す` |
| TC-1-3-E08 | `emotion = "ドヤ顔"` | `nil` | `TC-1-3-E08_ドヤ顔はvalidでnilを返す` |
| TC-1-3-E09 | `emotion = "普通"` | エラー | `TC-1-3-E09_7種にない値はエラーを返す` |
| TC-1-3-E10 | `emotion = "happy"` | エラー | `TC-1-3-E10_ローマ字表記はエラーを返す` |
| TC-1-3-E11 | `emotion = " 喜び "` | エラー | `TC-1-3-E11_前後空白付きはエラーを返す` |
| TC-1-3-E12 | `emotion = ""` | エラー（未決事項F、暫定） | `TC-1-3-E12_空文字列はエラーを返す` |
| TC-1-3-E13 | `emotion = "喜"` | エラー | `TC-1-3-E13_部分一致1文字はエラーを返す` |
| TC-1-3-E14 | `emotion = "喜びだ"` | エラー | `TC-1-3-E14_前方一致はエラーを返す` |

#### `ValidateRoleEmotionConsistency`（4件）

| ID | Given | Then | 対応サブテスト（`TestValidateRoleEmotionConsistency` 内） |
|----|-------|------|--------------------------------------------------------|
| TC-1-3-C01 | `role="user"`, `emotion=nil` | `nil` | `TC-1-3-C01_userとnilの組み合わせはvalid` |
| TC-1-3-C02 | `role="assistant"`, `emotion="喜び"` | `nil` | `TC-1-3-C02_assistantと喜びの組み合わせはvalid` |
| TC-1-3-C03 | `role="assistant"`, `emotion=nil` | `nil` | `TC-1-3-C03_assistantとnilの組み合わせもvalid` |
| TC-1-3-C04 | `role="user"`, `emotion="喜び"` | エラー | `TC-1-3-C04_userと喜びの組み合わせは矛盾でエラーを返す` |

### 4.4 1-4. `IsValidInput`（14件）

**テストファイル**: `tests/unit/input_validation_test.go`

| ID | Given | Then | 対応サブテスト（`TestIsValidInput` 内） |
|----|-------|------|--------------------------------------|
| TC-1-4-01 | 通常のテキスト | `true` | `TC-1-4-01_通常テキストはtrueを返す` |
| TC-1-4-02 | 前後空白ありで中身がある文字列 | `true` | `TC-1-4-02_前後空白があっても中身があればtrueを返す` |
| TC-1-4-03 | 空文字列 | `false` | `TC-1-4-03_空文字列はfalseを返す` |
| TC-1-4-04 | 半角スペースのみ | `false` | `TC-1-4-04_半角スペースのみはfalseを返す` |
| TC-1-4-05 | タブ・改行のみ | `false` | `TC-1-4-05_タブ改行のみはfalseを返す` |
| TC-1-4-06 | 全角スペースのみ | `false` | `TC-1-4-06_全角スペースのみはfalseを返す` |
| TC-1-4-07 | ゼロ幅スペースのみ | `true`（確認事項D） | `TC-1-4-07_ゼロ幅スペースのみはtrueを返す` |
| TC-1-4-08 | ゼロ幅スペース＋半角スペース混在 | `true` | `TC-1-4-08_ゼロ幅と半角スペース混在はtrueを返す` |
| TC-1-4-09 | 1文字のみの非空白文字 | `true` | `TC-1-4-09_1文字の非空白文字はtrueを返す` |
| TC-1-4-10 | 1文字のみの空白文字 | `false` | `TC-1-4-10_1文字の空白文字はfalseを返す` |
| TC-1-4-11 | 空白+非空白+空白 | `true` | `TC-1-4-11_空白に挟まれた非空白はtrueを返す` |
| TC-1-4-12 | 1000文字の半角スペースのみ | `false` | `TC-1-4-12_極端に長い空白のみはfalseを返す` |
| TC-1-4-13 | 絵文字のみ | `true` | `TC-1-4-13_絵文字のみはtrueを返す` |
| TC-1-4-14 | ゼロ幅スペースと実文字混在 | `true` | `TC-1-4-14_ゼロ幅スペースと実文字混在はtrueを返す` |

### 4.5 1-5. `IsExpired`（7件）

**テストファイル**: `tests/unit/gc_expiration_test.go`

| ID | Given | Then | 対応サブテスト（`TestIsExpired` 内） |
|----|-------|------|------------------------------------|
| TC-1-5-01 | `expiresAt`が`now`の1日前 | `true` | `TC-1-5-01_1日前はGC対象trueを返す` |
| TC-1-5-02 | `expiresAt`が`now`の1日後 | `false` | `TC-1-5-02_1日後はGC対象外falseを返す` |
| TC-1-5-03 | `expiresAt`がゼロ値 | `true` | `TC-1-5-03_ゼロ値はGC対象trueを返す` |
| TC-1-5-04 | `expiresAt == now`（完全一致） | `false` | `TC-1-5-04_完全一致は等号非対称性によりfalseを返す` |
| TC-1-5-05 | `expiresAt`が`now`の1ナノ秒前 | `true` | `TC-1-5-05_1ナノ秒前はtrueを返す` |
| TC-1-5-06 | `expiresAt`が`now`の1ナノ秒後 | `false` | `TC-1-5-06_1ナノ秒後はfalseを返す` |
| TC-1-5-07 | タイムゾーン相違（同一絶対時刻） | `false` | `TC-1-5-07_タイムゾーンが異なっても絶対時刻で判定する`（別関数`TestIsExpired_タイムゾーン相違`） |

---

## 5. テストケース総数の確認結果

`03_test_cases_phase1.md` の文書サマリーは「計67件」と記載しているが、本書清書にあたり全テストケースIDを機械的に突合したところ、実際の件数は**77件**（重複なし）であることを確認した。内訳は以下の通り。

| 観点 | 件数 |
|------|------|
| 1-1 `IsValidULID` | 21件 |
| 1-2 `TruncateFirstText` | 11件 |
| 1-3 `ValidateRole` | 6件 |
| 1-3 `ValidateEmotion` | 14件 |
| 1-3 `ValidateRoleEmotionConsistency` | 4件 |
| 1-4 `IsValidInput` | 14件 |
| 1-5 `IsExpired` | 7件 |
| **合計** | **77件** |

本書および `tests/unit/` のテストコード、`05_test_results.md` はすべて実際の**77件**を正としてカウントしている。「67件」表記は `03_test_cases_phase1.md` サマリー文言の集計誤りと推測されるが、テストケース本文（表の中身）自体に不足や重複は見つかっていない。原典の数値訂正要否についてはつむぎの判断を仰ぎたい。

---

## 6. テストコード配置・実行方法

### 配置

```
zuncha/
├── go.mod                              # module zuncha（本書作成時に新規初期化）
├── internal/
│   ├── validation/                     # 未実装（GREEN phaseで作成）
│   └── gc/                             # 未実装（GREEN phaseで作成）
└── tests/
    └── unit/
        ├── ulid_test.go                # 1-1（21件）
        ├── first_text_test.go          # 1-2（11件）
        ├── role_emotion_test.go        # 1-3（24件）
        ├── input_validation_test.go    # 1-4（14件）
        └── gc_expiration_test.go       # 1-5（7件）
```

### 実行方法（GREEN phase以降）

```bash
go mod tidy   # testify・oklog/ulid/v2 の依存解決（初回のみ）
go test ./tests/... -v
```

### 現状（RED phase）

`internal/validation` / `internal/gc` パッケージが未実装のため、上記コマンドは import エラーによりコンパイルが通らない。これはTDDのRED状態として意図した挙動である。GREEN phaseで各関数を実装した時点で、テストが1件ずつ通過することを確認しながら進める。

---

## 7. 未決事項の反映状況（F・G）

`03_test_cases_phase1.md` 7章の未決事項について、つむぎの判断（本チャットでの指示）を反映済み。

| # | 対象 | つむぎの判断 | 本書・テストコードへの反映 |
|---|------|-------------|--------------------------|
| F | `ValidateEmotion`の`emotion = ""`（空文字列）の扱い | ひまりの暫定設計のまま変更不要（空文字列はinvalid） | TC-1-3-E12は「エラーを返す」を確定値として採用。変更なし |
| G | role・emotion両方不正時のエラー集約方針 | 決定不要 | テストケース・テストコードとも「エラーが返ること」のみを検証し、エラー内容・件数には言及しない設計を維持（いずれの実装方針でも成立する） |

---

## 変更履歴

### 2026-07-08

初版作成。`03_test_cases_phase1.md` を正式な単体テスト仕様書として清書。テストケース件数の実数確認（67→77件）を実施し記録。

*記録: WhiteCUL*
