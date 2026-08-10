# 実装指示書 — Wave B-2 差し戻し対応（そら指摘①②③）

- 指示者: 四国めたん（テックリード）/ 2026-08-07
- 実装担当: ずんだもん
- 対象: HEAD d41188b（WIPコミット）からの差分
- 背景: そらのWave B-2レビューが⚠要修正（新規3件）。`tasks/todo.md` 末尾のそらの報告全文を先に読むこと。

---

## ①【最重要】SSE接続が1本でもあるとグレースフル停止が必ず失敗する

### 原因
`internal/handler/events.go` の `Events` は `conn.Run(r.Context())` で `r.Context()` がキャンセルされるまで戻らない。一方 `http.Server.Shutdown` は in-flight ハンドラの `r.Context()` を**キャンセルしない**（キャンセルされるのは `Close()` を呼んだ場合、または `BaseContext` で配ったコンテキストを能動的にキャンセルした場合のみ）。会話画面はSSEを常設する設計（仕様書§2.3）なので、SSE接続が1本でも張られていると `Shutdown` は必ず `shutdownTimeout` 満了までブロックしてタイムアウトする。

### 対処方針（確定・`events.go`・`main.go` は無改変）
`internal/httpserver/httpserver.go` の `Run` に `BaseContext` を追加し、停止トリガ時に能動的にキャンセルする。

```go
func Run(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error {
	baseCtx, baseCancel := context.WithCancel(context.Background())
	defer baseCancel()
	srv.BaseContext = func(net.Listener) context.Context { return baseCtx }

	serveErr := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErr <- err
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		baseCancel() // in-flight の r.Context() を全て即座にキャンセルする
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}
```

`srv.BaseContext` は `Serve` 開始前にセットすること（goroutine 起動前に書く）。これにより `r.Context()` は `baseCtx` の子になり、`baseCancel()` 一発で全ての in-flight リクエストの `r.Context()` が閉じる。`Events` ハンドラの `conn.Run(r.Context())` はその時点で戻り、`defer unregister()` → ハンドラ return → `Shutdown` 完了、の順に進む。

### テスト要件（そらの要望・必須）
`tests/integration/graceful_shutdown_test.go` に、**SSE接続を張ったまま停止させ `httpserver.Run` が `nil` を返すこと**を検証するテストケースを追加する。既存のテストがどう `httpServer`/`ln` を組み立てているかを確認し、同じパターンで:
1. サーバを起動する
2. `GET /conversations/{id}/events` にクライアントとして接続し、SSE接続を張ったままにする（レスポンスの読み取りループを別goroutineで回す、または最初の1バイトが読めたら「接続確立」とみなす）
3. 接続を維持した状態で停止トリガ（ctxキャンセル）を発生させる
4. `httpserver.Run` の戻り値が `nil` であること、かつ `shutdownTimeout` より十分速く戻ること（タイムアウト満了を待っていないこと）を検証する

このテストが対処前（`BaseContext`追加前）に赤くなることを一度確認してから直すこと（RED確認）。

---

## ② SSEのerrorイベントに内部エラー文字列を出さない

### 原因
`internal/service/response_streamer.go` の `fail` が `err.Error()`（`"llm generate: ..."` 等の内部エラー文字列）をそのまま `sink.SendError` に渡し、ブラウザのトーストに内部情報が出てしまう。

### 対処方針（確定）
`fail` に利用者向け文言を引数で渡す形に変える。文言は仕様書§2.2の例に統一し、**全ステップ同一の1文にする**（過剰な細分化はしない。シンプルさ優先）。

```go
const errMsgGenerateResponse = "応答の生成に失敗しました"

func (s *ResponseStreamer) fail(sink sse.EventSink, err error) error {
	_ = sink.SendError(errMsgGenerateResponse)
	return err
}
```

`fail` の呼び出し側は変更しない（第2引数の `fmt.Errorf(...)` はログ/返り値用に残す。ユーザーに見せるのは定数のみ）。`chat.go` の `errMsgSaveUserMessage` 等と同じ命名パターン（`errMsg` プレフィックス）に揃えること。

### テスト要件（必須）
`tests/unit/response_streamer_test.go` の既存アサーションが `SendError` の引数を `mock.Anything` で受けている箇所を、`errMsgGenerateResponse`（固定文言）を期待する形に変更する。**内部エラー文字列（`"llm generate:"` 等）がSendErrorの引数に含まれないこと**を検証するテストケースを最低1件追加する。既存15件が無改変で通ることを確認する。

---

## ③ 防壁2のミューテーション要件（20回検証）を満たす

### 原因
`tests/unit/sse_conn_test.go` の `W-03-24_Done後は何度WriteEventしてもErrConnClosed`（539行目付近）が `for i := 0; i < 3; i++` になっている。仕様書§2.4.1は「1テスト内で20回」を要求しており、3回では見逃し確率が理論値・実測値ともに無視できない大きさ（そらの実測: 30回中1回すり抜け）。

### 対処方針（確定・1行修正）
```go
for i := 0; i < 20; i++ {
```
に変更するだけ。ループ回数以外は変更しない。

### 確認要件
修正後、二段構えselectを1つのselectに戻す改変（そらが③の実測で使った改変と同じ）を行い、20回中1回以上は必ずFAILすることを実測してから元に戻す（ミューテーション実測、複数回試行して見逃しがないか確認）。

---

## 共通の完了条件

- `go build ./...` / `go vet ./...` / `gofmt -l .`（無出力）
- `go test ./tests/... -count=1 -v` 全PASS・0 FAIL・0 SKIP（使用したDBを`ZUNCHA_TEST_DB_OWNER`で明記）
- `go test ./tests/... -race -count=1` もクリーン
- ①②③それぞれのRED確認（対処前に赤くなることの実測）を報告に含める
- 完了後、`tasks/todo.md` 末尾に対応内容を追記する
