# 実装指示書 — Wave A: 永続化の穴埋め (W-01 / W-01b / W-02)

- 指示者: 四国めたん (テックリード) / 2026-08-05
- 実装担当: ずんだもん
- 原典: `docs/04_implementation/04_realtime_wiring_design.md` (§1.2 B-6/B-7/B-10、§3 Wave A)
- 規約: `.claude/rules/golang-coding-guideline.md`、`.claude/rules/tdd-comprehensive.md`

---

## 0. なぜこれを最初にやるのか

現行コードは Repository の**インターフェースは揃っているのに INSERT 系が丸ごと無い**。具体的には:

- `MessageRepository` は `GetRecentMessages` のみ → **会話が1件もDBに保存されない**。F-AI-03 (直近20件を文脈に使う) が原理的に成立しない。
- `AudioRepository` は Get/Update/Delete のみ → TTS が生成した音声を**登録できない**。`FetchAudio` は「取得して消す」側だけが実装済み。
- `conversations.first_text` に書き込む経路が**どこにも無い** → F-HIST-05 (会話一覧に最初のユーザー発話の冒頭20文字) が常に空。`validation.TruncateFirstText` は実装済みなのに呼ばれていない。

Wave B以降 (SSE配線・TTS・プロンプト組み立て) は全部ここに依存する。**依存ゼロで並行実装できる3タスク**なので最初に片付ける。

---

## 1. 進め方 (TDD厳守)

各タスクごとに **RED → GREEN → REFACTOR** を回す。まとめて実装してから一気にテストを書くのは禁止。

1. **RED**: テストを書き、`go test` で**失敗することを目視確認**する。ビルドエラーで落ちるのも RED として可 (実装が無いため)。
2. **GREEN**: テストを通す最小実装。
3. **REFACTOR**: 定数化・命名・重複排除。既存の `internal/postgres/*.go` の書き方 (エラーラップ `fmt.Errorf("...: %w", err)`、末尾の `var _ repository.X = (*X)(nil)`) に**そのまま揃える**。

integration テストは `ZUNCHA_TEST_DATABASE_URL` 未設定だと `t.Skip` で「緑に見える」。**必ず環境変数を設定して実行し、スキップされていないことをログで確認**すること (実装計画 R-5)。

```bash
go test ./tests/integration/... -v -run 'InsertMessage|SetFirstText|InsertRecord|FileStore'
```

---

## 2. W-01: `MessageRepository.InsertMessage`

### 2.1 インターフェース追加

`internal/repository/repository.go`:

```go
// MessageRepository は会話メッセージの永続化を抽象化する。
type MessageRepository interface {
	GetRecentMessages(ctx context.Context, conversationID string) ([]model.Message, error)
	InsertMessage(ctx context.Context, msg *model.Message) error
}
```

### 2.2 postgres 実装 (`internal/postgres/message.go` に追記)

```sql
INSERT INTO messages (id, conversation_id, role, content, emotion, created_at)
VALUES ($1, $2, $3, $4, $5, COALESCE($6::timestamptz, NOW()))
```

**確定した設計判断 (勝手に変えないこと)**:

| 項目 | 決定 | 理由 |
|---|---|---|
| `created_at` の扱い | `msg.CreatedAt` が**ゼロ値なら `NOW()` に委ねる**。`sql.NullTime` を使い、`Valid=false` で NULL を渡す | Go 側の分岐 (if文で2本のSQLを持つ) を避けて1クエリで済む。ゼロ値をそのまま入れると `0001-01-01` が保存され `GetRecentMessages` の並びが壊れる |
| `::timestamptz` の明示キャスト | **必須** | `COALESCE($6, NOW())` だと Postgres がプレースホルダの型を推論できずエラーになる |
| `emotion` | `*string` をそのまま渡す (nil → NULL) | 既存 `model.Message.Emotion` が `*string`。`sql.NullString` への変換は不要 |
| バリデーション | Repository では**やらない** | role/emotion の検証は `internal/validation` の責務で既にテスト済み。DB側の CHECK 制約が最終防衛線 |
| ID採番 | Repository では**やらない**。呼び出し側 (Wave B の W-07) が `ulid.Make().String()` で採番して渡す | 既存 `CreateConversationService` と同じ流儀 (サービス層が採番) |

### 2.3 テスト (`tests/integration/message_repository_test.go` に追記)

既存の `helpers_test.go` (`setupTestDB` / `insertConversation` / `countRows` / `ulidLike`) を**必ず流用**する。新しいヘルパーを作らない。

**正常系** — 具体的な値で検証すること (「保存された」で終わらせない):

- [ ] user メッセージを挿入 → 全カラムを SELECT し、`role='user'`・`content` が一致・`emotion IS NULL` を検証。
- [ ] assistant メッセージを挿入 → `emotion='喜び'` が保存されていることを検証。
- [ ] `CreatedAt` を明示指定 → **保存された値がその時刻と一致**する (`NOW()` に上書きされていない)。
- [ ] `CreatedAt` **ゼロ値** → `created_at` が NULL でなく、テスト実行時刻の前後数秒の範囲に入る (`NOW()` が効いている)。

**エッジケース**:

- [ ] `content` が 1000 文字 → 切り捨てられず全長保存される (`messages.content` は TEXT で上限なし)。
- [ ] `content` が空文字列 `''` → NOT NULL 制約は空文字を許すため**保存される**ことを記録 (バリデーションは上位層の責務であることの明示)。
- [ ] 同一会話に複数挿入 → その後 `GetRecentMessages` が**古い→新しい順**で返す。**InsertMessage と既存の読み取りが噛み合うことをここで初めて証明する** (最重要)。
- [ ] 21件挿入 → `GetRecentMessages` が**直近20件のみ**を返し、最古の1件が落ちる (境界値)。

**異常系**:

- [ ] 存在しない `conversation_id` → 外部キー違反でエラー (エラーが `nil` でないこと)。
- [ ] `role='bot'` (許容外) → CHECK 制約違反でエラー。
- [ ] `emotion='ハッピー'` (7種外) → CHECK 制約違反でエラー。
- [ ] 同一 `id` で2回挿入 → 主キー重複でエラー。
- [ ] 会話を削除 → CASCADE で messages も消える (`countRows` で0件を確認)。

---

## 3. W-01b: `ConversationRepository.SetFirstText`

### 3.1 インターフェース追加

```go
type ConversationRepository interface {
	GC(ctx context.Context, now time.Time) (int64, error)
	InsertConversation(ctx context.Context, conv *model.Conversation) error
	// SetFirstText は first_text が未設定の場合のみ text を記録する（最初のユーザー発話のみを残す）。
	SetFirstText(ctx context.Context, conversationID, text string) error
}
```

### 3.2 実装

```sql
UPDATE conversations SET first_text = $2 WHERE id = $1 AND first_text IS NULL
```

**確定した設計判断**:

- **`AND first_text IS NULL` が本体**。「最初のユーザー発話だけを記録する」という仕様を、Go 側で「まず SELECT して空か確認 → UPDATE」とやると**競合する** (10人同時利用・NF-SCALE-01)。SQL 1文で条件付き更新すれば原子的に解決する。**SELECT してから分岐する実装は却下**。
- **0件更新はエラーにしない** (冪等)。2回目以降の呼び出しは「何もしないのが正しい」ため。既存 `UpdateFetchedAt`/`DeleteRecord` と同じ流儀。
- **20文字への切り詰めは Repository でやらない**。呼び出し側 (W-07) が `validation.TruncateFirstText` を通す。カラムは `VARCHAR(20)` なので、21文字以上を渡すとDBエラーになる — これは**バグを早期に発火させる正しい挙動**なので、Repository 側で黙って切らないこと。

### 3.3 テスト (`tests/integration/conversation_repository_test.go` に追記)

- [ ] `first_text` が NULL の会話に設定 → 値が保存される。
- [ ] **2回目の呼び出しでは上書きされない** (1回目の値が残る)。これが本タスクの核心。
- [ ] 存在しない会話ID → **エラーにならない** (0件更新は冪等)。
- [ ] 空文字列 `''` を設定 → 保存され、以降 `IS NULL` ではなくなるため次の呼び出しで上書きされない (境界)。
- [ ] ちょうど20文字 → 保存される / **21文字 → エラー** (`VARCHAR(20)` の境界)。
- [ ] 絵文字・全角を含む20文字 → コードポイント単位で保存される (`VARCHAR(20)` は文字数カウント)。

### 3.4 既存テストへの影響 (必読)

`tests/unit/create_conversation_service_test.go:35` に次のコンパイル時アサーションがある:

```go
var _ repository.ConversationRepository = (*mockConversationRepository)(nil)
```

I/F にメソッドを足すと**このファイルがコンパイルエラーになる**。対応:

- **`mockConversationRepository` に `SetFirstText` メソッドを追加するだけ**にする (呼ばれないので中身は `return nil` でよい)。
- **既存のテストケース・アサーション・期待値は1行も変更しない**。変更が必要になったら手を止めて私に相談すること。

---

## 4. W-02: `AudioRepository.InsertRecord` + `FileStore.Write`

### 4.1 インターフェース追加

```go
type AudioRepository interface {
	GetByULID(ctx context.Context, ulid string) (*model.AudioFile, error)
	UpdateFetchedAt(ctx context.Context, ulid string, fetchedAt time.Time) error
	DeleteRecord(ctx context.Context, ulid string) error
	InsertRecord(ctx context.Context, audio *model.AudioFile) error
}
```

```sql
INSERT INTO audio_files (id, conversation_id, message_id, file_path, created_at)
VALUES ($1, $2, $3, $4, COALESCE($5::timestamptz, NOW()))
```

- `fetched_at` は**列に含めない** (未取得 = NULL がINSERT時の正しい初期状態)。`audio.FetchedAt` が非nilで渡ってきても**無視する**。
- `created_at` の扱いは W-01 と同一 (ゼロ値なら `NOW()`)。

### 4.2 `localfs.FileStore.Write`

```go
// Write はパスへファイルを書き込む。親ディレクトリが無ければ作成する。
func (f *FileStore) Write(path string, data []byte) error
```

- `os.MkdirAll(filepath.Dir(path), 0o755)` → `os.WriteFile(path, data, 0o644)`。エラーは `fmt.Errorf("...: %w", err)` でラップ。
- **`internal/service/filestore.go` の `FileStore` インターフェースには Write を足さない**。`FetchAudioService` は Read/Delete しか必要としておらず、足すと既存 unit テスト8件のモックが壊れる。書き込みを使うのは TTS 側 (W-09) なので、必要な I/F はその時に消費側で定義する。**構造体にメソッドを追加するだけ**なら既存 I/F への影響はゼロ。

### 4.3 テスト

**integration (`tests/integration/audio_fetch_test.go` に追記)**:

- [ ] 挿入 → `GetByULID` が全フィールド一致で取得でき、`FetchedAt` が nil。
- [ ] `FetchedAt` に値を入れて渡しても、保存後は NULL (無視される仕様の明示)。
- [ ] `InsertRecord` → `FetchAudio` (既存サービス) が成功する**一連の往復**。書き込み側と読み取り側が噛み合う証明。
- [ ] 存在しない `message_id` / `conversation_id` → 外部キー違反でエラー。
- [ ] 同一IDの二重挿入 → 主キー重複エラー。
- [ ] メッセージ削除 → CASCADE で audio_files も消える。

**FileStore.Write (unit / `tests/unit` 側でよい。`t.TempDir()` を使う)**:

- [ ] 書き込んだ内容が `Read` でバイト列一致で読み戻せる。
- [ ] **存在しない親ディレクトリ** → 自動作成されて書き込める。
- [ ] 既存ファイルへの書き込み → 上書きされる (追記でない)。
- [ ] 空バイト列 `[]byte{}` → 0バイトのファイルが作られ、`Read` が空を返す。
- [ ] 書き込み → `Delete` → `Read` がエラー (ライフサイクル一巡)。

### 4.4 既存テストへの影響

`tests/unit/fetch_audio_service_test.go:40` の `var _ repository.AudioRepository = (*mockAudioRepository)(nil)` が同様にコンパイルエラーになる。**`InsertRecord` メソッドを追加するだけ**。既存ケースは触らない。

---

## 5. 完了条件

- [ ] `go test ./tests/... -v` が全緑。**integration が Skip されていないことをログで確認**し、報告に実行ログの該当部分を添える。
- [ ] `gofmt -l .` が空出力 / `go vet ./...` が EXIT 0 / `go build ./...` が成功。
- [ ] 既存テストの変更が「モックへのメソッド追加」のみであること (`git diff tests/unit/` で確認できる状態)。
- [ ] 各タスクで **RED を目視確認したこと**を報告に含める (どのテストがどう失敗したか)。

## 6. 報告について

- 実装が緑になっても**タスクを `completed` にしないこと**。`in_progress` のまま私に報告する。完了判定はつむぎ(PM)の最終ゲート。
- 判断に迷ったら**実装で埋めずに私に聞く**。特に「§2.2 / §3.2 / §4.1 の確定した設計判断」を変えたくなった場合は必ず相談すること。理由まで書いてあるので、変えるなら理由を上回る根拠が必要ですわ。
- 緑になったら私がレビュー観点をまとめ、そら(レビュー担当)へ依頼します。
