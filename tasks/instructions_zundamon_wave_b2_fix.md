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

---

## 追補（そら再レビュー: ⚠要修正 新規2件・2026-08-10）— 対象コミット 8a5e934

①②③自体は解消確認済み（ミューテーション実測込み）。①の設計変更（`BaseContext`能動キャンセル）に伴い新たに生じた文書・テストの不整合2件に対応する。

### 【A】in-flight完遂の保証が失われたのに文書・テスト名が古いまま

`baseCancel()` を `srv.Shutdown()` の前に呼ぶため、`r.Context()` を参照するハンドラは停止トリガの瞬間に打ち切られる（そら実測: 変更後は `r.Context().Err()` が `context canceled` になりクライアントは500）。以下3箇所が実態と食い違っている。

1. `internal/httpserver/httpserver.go:15` のdocコメント「in-flight リクエストの完遂を待って新規接続を遮断する」
2. `docs/04_implementation/03_single_instance_smoke.md:46` の「送信中(in-flight)だったリクエストは中断されずレスポンスを受け取れる」
3. `tests/integration/graceful_shutdown_test.go:75` の `"in-flightリクエストは完遂する"` というアサーションコメント

**対処方針（確定・つむぎ判断）**: 猶予時間を挟む案（`time.AfterFunc`でノブを増やす）は採用しない。SSE等の長寿命接続はどのみち`shutdownTimeout`で打ち切られるため効果が薄く、ノブが増える分だけ複雑になるだけ（シンプルさ優先）。**実態に文書を合わせる**方向で確定する。

- `httpserver.go:15` のコメントを「新規接続を遮断し、in-flight の `r.Context()` も能動的にキャンセルする。`r.Context()` を参照しないハンドラの処理は完遂するが、`r.Context()` に依存する処理（キャンセルで中断するもの）は打ち切られる」という趣旨に書き換える
- `03_single_instance_smoke.md:46` も同様の趣旨に修正する（「中断されずレスポンスを受け取れる」→「`r.Context()`を参照しない処理は完遂する」等、実態に合わせた表現に）
- `graceful_shutdown_test.go:75` のコメントは、このテストのスタブハンドラ自体は `r.Context()` を見ないので**テストのアサーション自体は変更不要**。コメントに「（このスタブは `r.Context()` を参照しないため完遂する。`r.Context()` に依存する処理の扱いは別テストを参照）」と一言補足するのみでよい

### 【B】①の回帰テストがDB未設定環境で無言スキップされる

`TestGracefulShutdown_SSE接続中でも停止がエラーなく完了する` は `newTestHandler` 経由で `setupTestDB` を呼ぶため、`ZUNCHA_TEST_DATABASE_URL` 未設定だと `t.Skip` になる。ファイル冒頭の「DB非依存（httptest不使用・実ポート起動）＝skipなし」というコメントが、この1件について偽になっている。

**対処方針（確定）**:
1. ファイル冒頭3行目のコメントを「（`TestGracefulShutdown_SSE接続中でも...` はDBに依存するため例外）」のように実態に合わせて修正する
2. `r.Context()` を参照してブロックするだけのスタブハンドラで、SSEハンドラを使わずに同じ性質（BaseContextの能動キャンセルで打ち切られること）をDB非依存で検証する新規テストを追加する（`TestGracefulShutdown_InFlightCompletesAndNewConnRefused` と同じ構造で、ハンドラを `<-r.Context().Done()` で待つものに差し替えるイメージ）。このテストにより、DB未設定環境でも①の性質（`r.Context()`依存ハンドラが停止時に打ち切られ、`Run`が`shutdownTimeout`満了を待たずに戻ること）が担保される

### 完了条件（追補分）
- 上記3ファイルのコメント修正 + 新規DB非依存テスト1件
- `ZUNCHA_TEST_DATABASE_URL` を意図的に未設定にした状態で `go test ./tests/integration/... -run TestGracefulShutdown -v` を実行し、新規テストはSKIPされず実行されること、既存のSSE版はSKIPされることの両方を確認する
- 通常の完了条件（下記）も満たすこと

## 共通の完了条件

- `go build ./...` / `go vet ./...` / `gofmt -l .`（無出力）
- `go test ./tests/... -count=1 -v` 全PASS・0 FAIL・0 SKIP（使用したDBを`ZUNCHA_TEST_DB_OWNER`で明記）
- `go test ./tests/... -race -count=1` もクリーン
- ①②③それぞれのRED確認（対処前に赤くなることの実測）を報告に含める
- 完了後、`tasks/todo.md` 末尾に対応内容を追記する
