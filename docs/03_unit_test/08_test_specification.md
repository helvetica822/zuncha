# 単体テスト仕様書 — フェーズ2（DB連携ロジック、Repository層）

| 項目 | 内容 |
|------|------|
| バージョン | 1.0 |
| 作成日 | 2026-07-08 |
| 作成者 | WhiteCUL（テスト仕様書・テストコード作成担当） |
| 入力 | `01_test_plan.md`（テスト計画）、`06_test_perspectives_phase2.md`（テスト観点、めたん作成）、`07_test_cases_phase2.md`（テストケース、ひまり作成） |
| 対象 | フェーズ2（3機能、オーケストレーション層／I/O層分割を含む、計43件） |
| 次工程 | そらによるQAレビュー |

---

## 目次

1. [目的・対象](#1-目的対象)
2. [設計方針（オーケストレーション層／I/O層の使い分け）](#2-設計方針オーケストレーション層io層の使い分け)
3. [パッケージ構成・インターフェース／関数シグネチャ一覧](#3-パッケージ構成インターフェース関数シグネチャ一覧)
4. [テストケース一覧](#4-テストケース一覧)
5. [テストケース総数の確認結果](#5-テストケース総数の確認結果)
6. [テストコード配置・実行方法](#6-テストコード配置実行方法)
7. [未決事項の反映状況（K・L・M）](#7-未決事項の反映状況klm)

---

## 1. 目的・対象

本書は `07_test_cases_phase2.md` で設計されたフェーズ2のテストケースを、実装着手前の正式な単体テスト仕様書として清書したものである。
対象は `01_test_plan.md` フェーズ2に定義された3機能であり、いずれもDBアクセス・ファイルI/Oを伴うため、`06_test_perspectives_phase2.md` の指摘に基づき「オーケストレーション層（モックでテスト）」と「I/O層（実DB接続・実ファイルシステムでテスト）」の2階層で構成する。

| 観点番号 | 機能 | 対応機能ID / 設計根拠 |
|---------|------|----------------------|
| 2-1 | POST /conversations（ULID採番＋GC実行） | F-HIST-01, F-HIST-07 / DB設計書5.2 |
| 2-2 | 会話履歴コンテキスト構築（直近10往復／最大20メッセージ） | F-AI-03 / DB設計書3, 5.6 |
| 2-3 | GET /audio/{ulid}（読込→fetched_at更新→ファイル削除→レコード削除） | F-RT-01 / DB設計書5.3 |

---

## 2. 設計方針（オーケストレーション層／I/O層の使い分け）

`06_test_perspectives_phase2.md` 総括の指摘に基づき、以下の2階層構成を採用する。

### オーケストレーション層（Service層）

- 手順の順序制御・失敗時の後続スキップ・呼び出し回数を検証する層。
- Repository・FileStoreは**インターフェース**として定義し、テストでは`testify/mock`によるモック実装を注入する。
- 実DB・実ファイルシステムには一切依存しない。テスト実行速度が速く、F.I.R.S.T原則の「Fast」「Isolated」を満たす。
- 対象: `CreateConversationService`（2-1）、`FetchAudioService`（2-3）

### I/O層（Repository層／FileStore実装）

- 実際のSQL実行結果（CASCADE削除・`ORDER BY`の順序保証・generated column等）を検証する層。
- `01_test_plan.md` 5章の方針通り、テスト用DB（Dockerコンテナ）に実接続して検証する。ファイルシステム操作は実際の一時ディレクトリ（`t.TempDir()`等）を用いて検証する。
- 対象: `ConversationRepository`・`MessageRepository`・`AudioRepository`の各Postgres実装、`FileStore`のローカルファイルシステム実装

### 例外: 2-2の純粋ロジック層

2-2の並び替え処理（`ReverseMessages`）はDB・ファイルI/Oを一切持たない純粋関数であるため、オーケストレーション層・I/O層のどちらにも属さない**純粋ロジック層**として、フェーズ1と同様にモック・DB不要で直接テストする。

| 層 | モック方針 | 対応するテストディレクトリ |
|----|-----------|--------------------------|
| オーケストレーション層 | Repository・FileStoreをtestify/mockでモック化 | `tests/unit/` |
| 純粋ロジック層 | モック・DB不要 | `tests/unit/` |
| I/O層 | 実DB接続・実ファイルシステム | `tests/integration/` |

---

## 3. パッケージ構成・インターフェース／関数シグネチャ一覧

`.claude/rules/golang-coding-guideline.md` の標準レイヤー構成（handler / service / repository / model）に倣い、以下のパッケージに配置する。

| パッケージ | 役割 |
|-----------|------|
| `internal/model` | `Conversation` / `Message` / `AudioFile` のデータ構造 |
| `internal/repository` | Repository層のインターフェース定義、および`ReverseMessages`純粋関数 |
| `internal/filestore` | 音声一時ファイルの読込・削除を抽象化する`FileStore`インターフェース |
| `internal/service` | オーケストレーション層（`CreateConversationService` / `FetchAudioService`） |
| `internal/postgres` | `internal/repository`の各インターフェースに対するPostgreSQL実装（GREEN phaseで作成、I/O層テストの対象） |
| `internal/localfs` | `FileStore`のローカルファイルシステム実装（GREEN phaseで作成、I/O層テストの対象） |

### 3.1 データモデル（`internal/model`）

```go
type Conversation struct {
    ID         string
    StartedAt  time.Time
    ExpiresAt  time.Time
    FirstText  *string
}

type Message struct {
    ID             string
    ConversationID string
    Role           string
    Content        string
    Emotion        *string
    CreatedAt      time.Time
}

type AudioFile struct {
    ID             string
    ConversationID string
    MessageID      string
    FilePath       string
    CreatedAt      time.Time
    FetchedAt      *time.Time
}
```

### 3.2 Repository層インターフェース・関数（`internal/repository`）

| # | シグネチャ | 対応観点 |
|---|-----------|---------|
| 1 | `type ConversationRepository interface { GC(ctx context.Context, now time.Time) (int64, error); InsertConversation(ctx context.Context, conv *model.Conversation) error }` | 2-1 |
| 2 | `type MessageRepository interface { GetRecentMessages(ctx context.Context, conversationID string) ([]model.Message, error) }` | 2-2 |
| 3 | `func ReverseMessages(messages []model.Message) []model.Message` | 2-2 |
| 4 | `type AudioRepository interface { GetByULID(ctx context.Context, ulid string) (*model.AudioFile, error); UpdateFetchedAt(ctx context.Context, ulid string, fetchedAt time.Time) error; DeleteRecord(ctx context.Context, ulid string) error }` | 2-3 |
| 5 | `var ErrAudioNotFound error`（`GetByULID`が対象なしの場合に返すセンチネルエラー） | 2-3 |

> `UpdateFetchedAt` / `DeleteRecord` は対象0件（CASCADE削除等により既に消えている場合）でもエラーを返さない冪等な契約とする（`07_test_cases_phase2.md` TC-2-3-08 / `06_test_perspectives_phase2.md` 2-3例外系の指摘に対応）。
>
> **`GC`のシグネチャ変更（そらのレビュー指摘・2026-07-08）**: 当初`GC(ctx context.Context) (int64, error)`とし、内部SQLで`WHERE expires_at < NOW()`のようにPostgres側の`NOW()`に判定を委ねる設計を想定していたが、そらのレビューで「INSERTからGC呼び出しまでの経過時間分だけ`expires_at`が必ず過去になり、`expires_at`がちょうど現在時刻に一致する境界値のテストが原理的に成立しない」との指摘を受けた。フェーズ1の1-5（`IsExpired(expiresAt, now time.Time) bool`）で確立したClock注入の思想と一貫させ、`GC`にも判定基準時刻`now`を呼び出し元から明示的に注入する設計（`WHERE expires_at < $1`にGoから`now`を渡す）に変更した。呼び出し元（`CreateConversationService`）は内部で`time.Now()`を都度取得して渡す。

### 3.3 FileStoreインターフェース（`internal/filestore`）

```go
type FileStore interface {
    Read(path string) ([]byte, error)
    Delete(path string) error
}
```

### 3.4 オーケストレーション層（`internal/service`）

| # | シグネチャ | 対応観点 |
|---|-----------|---------|
| 1 | `type CreateConversationService struct { /* unexported */ }` | 2-1 |
| 2 | `func NewCreateConversationService(repo repository.ConversationRepository) *CreateConversationService` | 2-1 |
| 3 | `func (s *CreateConversationService) CreateConversation(ctx context.Context) (*model.Conversation, error)` | 2-1 |
| 4 | `type FetchAudioService struct { /* unexported */ }` | 2-3 |
| 5 | `func NewFetchAudioService(repo repository.AudioRepository, files filestore.FileStore) *FetchAudioService` | 2-3 |
| 6 | `func (s *FetchAudioService) FetchAudio(ctx context.Context, ulid string) ([]byte, error)` | 2-3 |

`CreateConversation`内部の処理順序は`06_test_perspectives_phase2.md` 7章F決定に基づき **GC → InsertConversation** の順で、GCのエラーは握りつぶす（ログ記録のみ）。`FetchAudio`内部の処理順序は **GetByULID → Read → UpdateFetchedAt → Delete → DeleteRecord** で、いずれかのステップが失敗した時点で後続ステップは呼び出さずエラーを返す（7章I決定：現行のファイル削除→レコード削除の順序を維持）。

---

## 4. テストケース一覧

`07_test_cases_phase2.md` のGiven/When/Thenをそのまま踏襲し、対応するGoテストコード（ファイル・サブテスト名）を突き合わせる。

### 4.1 2-1. POST /conversations（15件）

#### オーケストレーション層（7件） — `tests/unit/create_conversation_service_test.go`

| ID | Given | Then | 対応サブテスト |
|----|-------|------|----------------|
| TC-2-1-01 | GC成功・InsertConversation成功のモック | 新規conversationが返り、GC・InsertConversationが各1回呼ばれる | `TC-2-1-01_GC成功時に新規会話が作成されGC・Insertが1回ずつ呼ばれる` |
| TC-2-1-02 | GCが削除0件を返すモック | 新規会話作成が成功する | `TC-2-1-02_GC対象なしでも新規会話作成が成功する` |
| TC-2-1-03 | CreateConversationを3回連続呼ぶ | 毎回必ず1回ずつGCが呼ばれる | `TestCreateConversation_呼び出しのたびに毎回GCが1回呼ばれる`（別関数） |
| TC-2-1-04 | GCがエラーを返すモック（確認事項F） | GCエラーは握りつぶされInsertConversationが実行され成功する | `TC-2-1-04_GC失敗時もエラーを握りつぶし新規会話作成は成功する` |
| TC-2-1-05 | InsertConversationがエラーを返すモック | CreateConversationはエラーを返す | `TC-2-1-05_Insert失敗時はエラーを返す` |
| TC-2-1-06 | GC・InsertConversationの呼び出し順序を記録可能にする | GCがInsertConversationより先に呼ばれる | `TestCreateConversation_GCがInsertConversationより先に呼ばれる`（別関数） |
| TC-2-1-07 | InsertConversationへの引数をキャプチャし2回連続呼ぶ | 1回目と2回目で異なるULIDが渡される | `TestCreateConversation_連続呼び出しで異なるULIDが採番される`（別関数） |

#### I/O層（8件） — `tests/integration/conversation_repository_test.go`

| ID | Given | Then | 対応サブテスト |
|----|-------|------|----------------|
| TC-2-1-08 | 期限切れ`conversations`が0件 | InsertConversationで新規レコードが1件作成される | `TC-2-1-08_期限切れなしでも新規レコードが作成される` |
| TC-2-1-09 | 期限切れ`conversations`を複数件、`messages`・`audio_files`とともに用意 | GC実行で全件削除、CASCADE先も連鎖削除される | `TC-2-1-09_GCでCASCADE削除が連鎖する` |
| TC-2-1-10 | 新規`conversations`レコードを作成 | `started_at`≒NOW、`first_text`はNULL、`expires_at`=`started_at`+30日 | `TC-2-1-10_新規レコードのカラム初期値が正しい` |
| TC-2-1-11 | `expires_at`がGCに注入する`now`とちょうど一致するレコードを1件用意（`now`はGoから明示的に注入、そらの指摘によりPostgres側`NOW()`依存から変更） | `GC(ctx, now)`実行で削除されない | `TC-2-1-11_ちょうどNOWのレコードはGC対象外` |
| TC-2-1-12 | 期限切れレコードを1件用意 | GC実行で1件削除される | `TC-2-1-12_期限切れ1件が削除される` |
| TC-2-1-13 | 期限切れレコードを1000件用意 | GC実行で1000件全件削除される | `TC-2-1-13_期限切れ1000件が全件削除される` |
| TC-2-1-14 | 同一ULIDでINSERTを試みる（確認事項K） | エラーを返す（リトライしない） | `TC-2-1-14_ULID衝突時は即エラーを返す` |
| TC-2-1-15 | CreateConversation相当処理を最大10並列で同時実行 | GC重複実行・ULID衝突なく全件正常完了 | `TC-2-1-15_10並列実行でも衝突なく完了する` |

### 4.2 2-2. 会話履歴コンテキスト構築（15件）

#### 純粋ロジック層（4件） — `tests/unit/reverse_messages_test.go`

| ID | Given | Then | 対応サブテスト（`TestReverseMessages` 内） |
|----|-------|------|------------------------------------------|
| TC-2-2-01 | 新しい→古い順の5件`Message`スライス | 古い→新しい順に反転される | `TC-2-2-01_新しい順の5件を古い順に反転する` |
| TC-2-2-02 | 空スライス | 空スライスを返す（`nil`ではない） | `TC-2-2-02_空スライスは空スライスのまま返す` |
| TC-2-2-03 | 1件のみのスライス | 同じ1件をそのまま返す | `TC-2-2-03_1件のみはそのまま返す` |
| TC-2-2-04 | 新しい→古い順の20件`Message`スライス | 古い→新しい順に正しく反転される | `TC-2-2-04_20件でも正しく反転される` |

#### I/O層（11件） — `tests/integration/message_repository_test.go`

| ID | Given | Then | 対応サブテスト |
|----|-------|------|----------------|
| TC-2-2-05 | 対象conversation_idに5件のmessages | 全5件を古い→新しい順で返す | `TC-2-2-05_5件は全件古い順で返る` |
| TC-2-2-06 | 対象conversation_idに30件のmessages | 直近20件のみ古い→新しい順で返す | `TC-2-2-06_30件は直近20件のみ古い順で返る` |
| TC-2-2-07 | user/assistant交互20件（10往復） | 20件全件が古い→新しい順で返る | `TC-2-2-07_交互20件は10往復として一致する` |
| TC-2-2-08 | 対象conversation_idにmessagesが0件 | 空スライスを返す（`nil`ではない） | `TC-2-2-08_0件は空スライスを返す` |
| TC-2-2-09 | 存在しないconversation_idを指定（確認事項L） | エラーにせず空配列を返す | `TC-2-2-09_存在しないconversation_idは空配列を返す` |
| TC-2-2-10 | ちょうど20件のmessages | 全20件が正しい順で取得される | `TC-2-2-10_ちょうど20件は全件取得される` |
| TC-2-2-11 | 19件のmessages | 全19件が取得される | `TC-2-2-11_19件は全件取得される` |
| TC-2-2-12 | 21件のmessages | 最新20件のみ取得、最古1件が除外される | `TC-2-2-12_21件は最古の1件が除外される` |
| TC-2-2-13 | `created_at`が完全同一時刻の複数messages | タイブレーカーにより順序が一意に安定する | `TC-2-2-13_同一ミリ秒でも順序が安定する` |
| TC-2-2-14 | role非交互の22件（確認事項G） | role交互を問わず単純に直近20件が取得される | `TC-2-2-14_role非交互でも単純20件上限で取得される` |
| TC-2-2-15 | 作成順が判別できる30件のmessages | 戻り値が常に古い→新しい順である契約が実DB経由でも成立する | `TC-2-2-15_実DB経由でも古い新しい順の契約が成立する` |

### 4.3 2-3. GET /audio/{ulid}（13件）

#### オーケストレーション層（8件） — `tests/unit/fetch_audio_service_test.go`

| ID | Given | Then | 対応サブテスト |
|----|-------|------|----------------|
| TC-2-3-01 | 全モックが正常応答 | Read→UpdateFetchedAt→Delete→DeleteRecordの順で1回ずつ呼ばれ、ファイル内容が返る | `TC-2-3-01_全ステップが順序通り実行され成功する` |
| TC-2-3-02 | Repositoryが404相当を返す | 404相当エラー、UpdateFetchedAt以降は呼ばれない | `TC-2-3-02_レコード不存在時は404相当でUpdateFetchedAt以降を呼ばない` |
| TC-2-3-03 | FileStore.Readがエラー | エラーを返し、UpdateFetchedAt・Delete・DeleteRecordは呼ばれない | `TC-2-3-03_Read失敗時は後続を呼ばない` |
| TC-2-3-04 | UpdateFetchedAtがエラー（DB接続断） | エラーを返し、Delete・DeleteRecordは呼ばれない | `TC-2-3-04_UpdateFetchedAt失敗時はファイル削除以降を呼ばない` |
| TC-2-3-05 | FileStore.Deleteがエラー（確認事項I） | エラーを返し、DeleteRecordは呼ばれない | `TC-2-3-05_Delete失敗時はDeleteRecordを呼ばない` |
| TC-2-3-06 | FileStore.Deleteがエラー（確認事項H） | 自動再削除リトライは発生しない（Delete呼び出しは1回のみ） | `TC-2-3-06_Delete失敗時に自動リトライしない` |
| TC-2-3-07 | Delete成功・DeleteRecordがエラー | エラーを返す（幽霊レコード状態を許容） | `TC-2-3-07_DeleteRecord失敗時は幽霊レコード状態を許容する` |
| TC-2-3-08 | UpdateFetchedAt・DeleteRecordが対象0件を返す | エラーとせず冪等に成功扱いとする | `TC-2-3-08_対象0件は冪等に成功扱いとする` |

#### I/O層（5件） — `tests/integration/audio_fetch_test.go`

| ID | Given | Then | 対応サブテスト |
|----|-------|------|----------------|
| TC-2-3-09 | 実DB・実ファイルシステムに音声ファイル1件を用意 | 処理完了後、物理ファイル・レコードともに削除されている | `TC-2-3-09_処理完了後にファイルとレコードが削除される` |
| TC-2-3-10 | TC-2-3-09完了後の状態 | 同一ULIDへの再アクセスは404相当 | `TC-2-3-10_削除済みULIDへの再アクセスは404相当` |
| TC-2-3-11 | レコードは存在するが物理ファイル未配置 | エラーを返し、fetched_at更新・レコード削除とも実行されない | `TC-2-3-11_ファイル不存在時は更新も削除も実行しない` |
| TC-2-3-12 | 同一ULIDへ同時に2件のリクエスト（確認事項J） | 1回目完走、2回目は404相当（排他制御なし） | `TC-2-3-12_同時リクエストは2回目が404相当になる` |
| TC-2-3-13 | `fetched_at`が意図的に非NULLの異常データ | 実装済み分岐に従い一貫した挙動を示す | `TC-2-3-13_fetched_at非NULLの異常データでも一貫した挙動を示す` |

---

## 5. テストケース総数の確認結果

`07_test_cases_phase2.md` のサマリー「計43件（2-1: 15件、2-2: 15件、2-3: 13件）」について、全テストケースIDを機械的に突合した結果、記載通り**43件**（重複なし）であることを確認した。フェーズ1で発生した集計誤り（67→77件）のような差異は今回は見つかっていない。

| 観点 | オーケストレーション層／純粋ロジック層 | I/O層 | 合計 |
|------|----------------------------------------|-------|------|
| 2-1 | 7件 | 8件 | 15件 |
| 2-2 | 4件 | 11件 | 15件 |
| 2-3 | 8件 | 5件 | 13件 |
| **合計** | **19件** | **24件** | **43件** |

---

## 6. テストコード配置・実行方法

### 配置

```
zuncha/
├── go.mod
├── internal/
│   ├── model/          # 未実装（GREEN phaseで作成）
│   ├── repository/     # 未実装（GREEN phaseで作成）
│   ├── filestore/       # 未実装（GREEN phaseで作成）
│   ├── service/         # 未実装（GREEN phaseで作成）
│   ├── postgres/        # 未実装（GREEN phaseで作成、I/O層の対象）
│   └── localfs/          # 未実装（GREEN phaseで作成、I/O層の対象）
└── tests/
    ├── unit/
    │   ├── ulid_test.go                     # フェーズ1
    │   ├── first_text_test.go               # フェーズ1
    │   ├── role_emotion_test.go             # フェーズ1
    │   ├── input_validation_test.go         # フェーズ1
    │   ├── gc_expiration_test.go            # フェーズ1
    │   ├── create_conversation_service_test.go  # 2-1オーケストレーション層（7件）
    │   ├── reverse_messages_test.go              # 2-2純粋ロジック層（4件）
    │   └── fetch_audio_service_test.go           # 2-3オーケストレーション層（8件）
    └── integration/
        ├── conversation_repository_test.go  # 2-1 I/O層（8件）
        ├── message_repository_test.go       # 2-2 I/O層（11件）
        └── audio_fetch_test.go              # 2-3 I/O層（5件）
```

### 実行方法（GREEN phase以降）

```bash
go mod tidy

# オーケストレーション層・純粋ロジック層（フェーズ1含む、モック・DB不要）
go test ./tests/unit/... -v

# I/O層（テスト用DB・ファイルシステムが必要）
export ZUNCHA_TEST_DATABASE_URL="postgres://..."   # 未設定時は各テストがt.Skipする
go test ./tests/integration/... -v
```

### 現状（RED phase）

`internal/model` / `internal/repository` / `internal/filestore` / `internal/service` / `internal/postgres` / `internal/localfs` のいずれも未実装のため、上記コマンドは import エラーによりコンパイルが通らない。これはTDDのRED状態として意図した挙動である。

I/O層テストは、GREEN phaseでパッケージが実装された後も、環境変数`ZUNCHA_TEST_DATABASE_URL`が未設定の場合は`t.Skip`によりスキップされる設計とした（テスト用DBを用意していないローカル環境やCI設定でオーケストレーション層のテストだけを高速に回せるようにするため）。

---

## 7. 未決事項の反映状況（K・L・M）

`07_test_cases_phase2.md` 7章の未決事項について、つむぎの判断（本チャットでの指示：「いずれも本書の暫定対応のまま確定、変更なし」）を反映済み。

| # | 対象 | つむぎの判断 | 本書・テストコードへの反映 |
|---|------|-------------|--------------------------|
| K | 2-1 ULID衝突時の挙動 | ひまりの暫定設計のまま変更不要（即エラー、リトライしない） | TC-2-1-14は「即エラーを返す」を確定値として採用 |
| L | 2-2 存在しないconversation_id指定時の挙動 | ひまりの暫定設計のまま変更不要（空配列を返す） | TC-2-2-09は「エラーにせず空配列を返す」を確定値として採用 |
| M | 2-2 conversation_idの形式バリデーションを本関数内でも防御的に行うか | ひまりの暫定設計のまま変更不要（呼び出し元でバリデーション済みの値を受け取る前提） | `GetRecentMessages`内での形式バリデーションのテストケースは追加しない |

---

## 変更履歴

### 2026-07-08

初版作成。`07_test_cases_phase2.md` を正式な単体テスト仕様書として清書。テストケース件数の実数確認（43件、差異なし）を実施し記録。

### 2026-07-08（そらのレビュー指摘対応）

そらのQAレビュー（⚠要修正）を受け、`ConversationRepository.GC`のシグネチャを`GC(ctx context.Context) (int64, error)`から`GC(ctx context.Context, now time.Time) (int64, error)`に変更（3.2節）。TC-2-1-11の記載を、Postgres側`NOW()`依存からGo側`now`注入方式に合わせて更新。

*記録: WhiteCUL*
