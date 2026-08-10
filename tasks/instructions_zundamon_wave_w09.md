# 実装指示書 — Wave W-09: VOICEVOX TTS実装

- 指示者: 四国めたん(テックリード) / つむぎ代行 / 2026-08-10
- 実装担当: ずんだもん
- **実装前提: `docs/04_implementation/04_realtime_wiring_design.md` D-4(2026-08-10訂正版)を必ず読むこと**。着手前のレビューで設計上の制約が発覚し、既存コードの改修を伴う。当初案(`internal/tts`のI/F・DBスキーマとも無変更)からの変更点なので、旧い記憶で実装しないこと。

---

## 1. 背景(必ず理解してから着手すること)

`audio_files`テーブルへ音声ファイルを登録するには`conversation_id`・`message_id`が要るが、既存の`ResponseStreamer.StreamResponse`の実行順序では、TTS合成(`Synthesize`)が呼ばれる時点でassistantメッセージはまだDBに未保存(保存は`SendDone`のタイミング)。当初の設計(D-4)通りに実装しようとすると`messages`への外部キー制約違反になる。

ユーザー承認済みの解決策: **`audio_files.message_id`のFK制約を撤去**(値は設定するが参照整合性は強制しない)。これにより、TTS合成のタイミングでassistantメッセージがまだDBに存在しなくても`audio_files`へINSERTできる。

**この判断・DBスキーマ変更・機能設計書の更新はつむぎが既に対応済み**。以下のファイルは変更済みなので、あなたが変更する必要はない(確認だけしてください):
- `migrations/0001_initial_schema.up.sql`(`audio_files.message_id`のFK制約削除)
- `docs/02_functional_design/02_database_design.md`(§2.3・§3の記述更新)
- `docs/02_functional_design/00_minutes.md`(インデックス説明の更新)
- `docs/04_implementation/04_realtime_wiring_design.md`(D-4訂正)

あなたのタスクは、この方針に沿ってGoコード側(配線変更 + VOICEVOXクライアント実装)を行うことです。

---

## 2. スコープ

### 2.1 新規実装: `internal/voicevox`

```
internal/voicevox/client.go   Client（tts.TTSClient を実装）
```

- VOICEVOX HTTP API: `POST /audio_query?text=...&speaker=...` → クエリJSON取得 → `POST /synthesis?speaker=...`(bodyにクエリJSON) → WAVバイナリ取得
- speaker IDは定数化する(ずんだもんの標準speaker。VOICEVOX公式のspeaker一覧から適切なIDを確認して選ぶこと。**推測で決めず、コメントに根拠(公式のspeaker名との対応)を残すこと**)
- ベースURLは`Option`パターンで注入可能にする(`httptest`でテストするため。Wave C-1の`internal/anthropic`と同じパターンでよい)
- WAVは`localfs.FileStore.Write`で保存する(パスの命名規則は`internal/localfs`の既存実装を確認し、ULIDベースで一意なパスにすること)
- `internal/postgres.AudioRepository.InsertRecord`は既存(W-04で実装済み)なのでそのまま使う。新規実装は不要。

### 2.2 `internal/tts` のI/F変更

```go
// 変更前
type TTSClient interface {
	Synthesize(ctx context.Context, text string) (string, error)
}
// 変更後
type TTSClient interface {
	Synthesize(ctx context.Context, text, conversationID, messageID string) (string, error)
}
```

### 2.3 `internal/service` の配線変更

**`ChatService.HandleUserMessage`**(`internal/service/chat.go`): `RecordingSink`構築の直前で`assistantMessageID := s.newID()`を生成し、`NewRecordingSink`と`streamer.StreamResponse`の両方に渡す。

**`RecordingSink`**(`internal/service/recording_sink.go`): コンストラクタに`messageID string`引数を追加し、`SendDone`内の`s.newID()`呼び出しをやめて渡された値を使う。

**`ResponseStreamer.StreamResponse`**(`internal/service/response_streamer.go`): シグネチャに`conversationID, messageID string`を追加し、`Synthesize`呼び出し時に渡す。

```go
func (s *ResponseStreamer) StreamResponse(ctx context.Context, sink sse.EventSink, prompt, conversationID, messageID string) error {
	...
	if url, ttsErr := s.ttsClient.Synthesize(ctx, resp.Text, conversationID, messageID); ttsErr == nil {
	...
```

### 2.4 既存テストの改修

- `tests/unit/response_streamer_test.go`: `StreamResponse`呼び出し16箇所すべてに`conversationID, messageID`の引数を追加。`mockTTSClient.Synthesize`のシグネチャとモック設定(`.On("Synthesize", ...)`)も4引数に合わせる。**既存の振る舞い検証(何を送るか・エラー処理)は変えないこと**。機械的な引数追加のみ。
- `tests/unit/recording_sink_test.go`: `NewRecordingSink`呼び出しに`messageID`引数を追加。`s.newID()`が呼ばれなくなる分、既存の「newIDがどう使われるか」を検証しているテストがあれば、それに合わせて修正する。
- `tests/unit/chat_service_test.go`: `HandleUserMessage`のテストで、事前生成される`assistantMessageID`が`RecordingSink`/`StreamResponse`に正しく伝播していることを検証するケースを追加(または既存のモック期待値を更新)。

---

## 3. `internal/voicevox` のテスト(`tests/unit/voicevox_client_test.go`)

**`httptest.NewServer`でフェイクVOICEVOXを立てる。実VOICEVOXサーバは絶対に呼ばないこと。**

**正常系**:
- [ ] フェイクが`audio_query`→クエリJSON、`synthesis`→WAVバイナリを返す → `Synthesize`が`/audio/{ulid}`形式のURLを返す
- [ ] `audio_query`のリクエストに`text`パラメータと正しいspeaker IDが乗っている
- [ ] `synthesis`のリクエストボディに`audio_query`のレスポンスがそのまま渡っている
- [ ] 戻り値のULIDに対応する`audio_files`レコードがDB(fakeまたはinterface経由のモック)にINSERTされ、`conversation_id`・`message_id`が渡した値と一致する
- [ ] WAVファイルが`localfs`(モック経由)に書き込まれる

**異常系**:
- [ ] `audio_query`が非2xxを返す → エラー
- [ ] `synthesis`が非2xxを返す → エラー
- [ ] `ctx`を即キャンセル → `context.Canceled`系のエラー
- [ ] DBへのINSERT失敗 → エラー(WAVファイルは書き込み済みでも構わない。孤児ファイルの掃除は別Waveの検討事項として申し送りに残すこと)

**ミューテーション実測**: speaker IDの値を変える、`audio_query`の結果を`synthesis`に渡す配線を外す、の2点で対応するテストが赤になることを実測する(`mutation-test-overlay`スキル使用)。

---

## 4. `cmd/api` への配線

- `ttsClient`(現状nilで放置されている)を`voicevox.NewClient(...)`に差し替える
- VOICEVOXのベースURLを環境変数(`VOICEVOX_BASE_URL`等、命名は`ANTHROPIC_API_KEY`と同様の慣習に合わせること)で受け取り、`loadConfig`に追加。未設定時の扱いは`ANTHROPIC_API_KEY`と同様(起動時`log.Fatal`)でよいか、それとも開発時の利便性のためデフォルト値(`http://localhost:50021`など、VOICEVOX標準ポート)を持たせるか、**判断して報告に理由を書くこと**(自己判断でよい範囲。ただし理由は明記)。

これにより、そらが申し送っていた「TTSがnilで最初のユーザー発話でプロセスがクラッシュする」問題が解消される見込み。**配線後、実際にこの問題が解消されたことを示すテスト(またはコメントでの言及)があるとよい**。

---

## 5. 完了条件

- [ ] `go test ./tests/... -count=1` 全緑、`./scripts/test_race.sh`もクリーン(`export ZUNCHA_TEST_DB_OWNER=zundamon`)
- [ ] `gofmt -l .`空 / `go vet ./...` EXIT 0 / `go build ./...`成功
- [ ] `internal/llm`・`internal/anthropic`は無変更であること
- [ ] テストが実VOICEVOXを呼んでいないこと(`httptest`のみ)
- [ ] RED確認(既存テストのシグネチャ変更で一度ビルドが壊れることを含めてよい)を報告に含める
- [ ] ミューテーション実測(§3の2点 + 既存の`response_streamer_test.go`の退行がないこと)
- [ ] `audio_files.message_id`にFK制約が無いことを前提にした実装になっていること(INSERT時にmessagesテーブルへの存在チェックをしていないこと)

## 6. 報告について

- 緑でも`completed`にせず`in_progress`のまま報告
- speaker IDの選定根拠、`VOICEVOX_BASE_URL`未設定時の扱いの判断理由を必ず報告に書くこと
- 実装中に「D-4の訂正方針そのもの」を変えたくなった場合は、実装で埋めずに相談すること
