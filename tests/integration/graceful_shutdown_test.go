// グレースフルシャットダウンの結合テスト。対応仕様: NF-SCALE-02（単一インスタンス動作）、結合テスト phase IT-2。
// 「in-flightリクエストの扱い」と「停止後の新規接続遮断」を決定的（channel同期・sleep非依存）に検証する。
// 原則としてDB非依存（httptest不使用・実ポート起動）＝skipなし。
// ただし TestGracefulShutdown_SSE接続中でも停止がエラーなく完了する は実ハンドラ経由でDBに依存するため例外で、
// ZUNCHA_TEST_DATABASE_URL 未設定だと skip される。同じ性質（r.Context() 依存ハンドラが停止トリガで
// 打ち切られること）は TestGracefulShutdown_CtxDependentHandlerIsCanceled がDB非依存で担保する。
package integration

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuncha/internal/httpserver"
)

func TestGracefulShutdown_InFlightCompletesAndNewConnRefused(t *testing.T) {
	// ハンドラは「開始」を通知し release を待ってから 200 を返すスタブ。
	started := make(chan struct{})
	release := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))
	})

	srv := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- httpserver.Run(ctx, srv, ln, 5*time.Second)
	}()

	client := &http.Client{}
	defer client.CloseIdleConnections()

	respCh := make(chan *http.Response, 1)
	reqErrCh := make(chan error, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/slow", nil)
		resp, err := client.Do(req)
		if err != nil {
			reqErrCh <- err
			return
		}
		respCh <- resp
	}()

	// ハンドラの処理開始を待つ（in-flight状態を確立）。
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("ハンドラが開始されなかった")
	}

	// in-flight中に停止をトリガし、その後リクエストを完了させる。
	cancel()
	close(release)

	// in-flightリクエストは 200 で完遂する。
	// 補足: このスタブハンドラは r.Context() を参照しないため、停止トリガ後も打ち切られず完遂する。
	// 停止トリガ時に in-flight の r.Context() は能動的にキャンセルされるので、r.Context() に
	// 依存する処理は逆に打ち切られる。その挙動は
	// TestGracefulShutdown_CtxDependentHandlerIsCanceled を参照。
	select {
	case resp := <-respCh:
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "r.Context()を参照しないin-flightリクエストは完遂する")
	case err := <-reqErrCh:
		t.Fatalf("in-flightリクエストが失敗した: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("in-flightリクエストのレスポンスが得られなかった")
	}

	// グレースフル停止がエラーなく完了する。
	select {
	case err := <-runDone:
		assert.NoError(t, err, "グレースフル停止はエラーなく完了する")
	case <-time.After(3 * time.Second):
		t.Fatal("Runが完了しなかった")
	}

	// 停止後の新規接続は拒否される。
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("停止後も新規接続が受理された")
	}
	assert.Error(t, err, "停止後の新規接続は拒否される")
}

// そら指摘①の性質をDB非依存で担保する回帰テスト。
// SSE版（下記 TestGracefulShutdown_SSE接続中でも停止がエラーなく完了する）は実ハンドラ経由で
// DBを要求するため ZUNCHA_TEST_DATABASE_URL 未設定だと skip されてしまう。ここでは
// 「r.Context() が閉じるまで戻らない」という Events ハンドラと同じ性質だけを持つスタブを使い、
// SSE・DBに一切依存せずに以下を検証する:
//   - 停止トリガで in-flight の r.Context() が能動的にキャンセルされ、ハンドラが打ち切られる
//   - その結果 httpserver.Run は shutdownTimeout 満了を待たず nil を返す
//
// BaseContext の付与または baseCancel() の呼び出しを外すと、ハンドラは戻らず Shutdown が
// shutdownTimeout 満了までブロックして赤くなる（ミューテーション検知可能）。
func TestGracefulShutdown_CtxDependentHandlerIsCanceled(t *testing.T) {
	// ハンドラは「開始」を通知したあと r.Context() が閉じるまでブロックするだけのスタブ
	// （internal/handler/events.go の conn.Run(r.Context()) と同じ性質）。
	started := make(chan struct{})
	ctxErrCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/blocking", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		ctxErrCh <- r.Context().Err()
	})

	srv := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	const shutdownTimeout = 3 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- httpserver.Run(ctx, srv, ln, shutdownTimeout)
	}()

	client := &http.Client{}
	defer client.CloseIdleConnections()

	reqDone := make(chan struct{})
	go func() {
		defer close(reqDone)
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/blocking", nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	// ハンドラの処理開始を待つ（in-flight状態を確立）。
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("ハンドラが開始されなかった")
	}

	// in-flight中に停止をトリガする。
	start := time.Now()
	cancel()

	// r.Context() に依存するハンドラは停止トリガで打ち切られる。
	select {
	case ctxErr := <-ctxErrCh:
		assert.ErrorIs(t, ctxErr, context.Canceled,
			"in-flightの r.Context() は停止トリガでキャンセルされる")
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("r.Context() 依存ハンドラが打ち切られなかった")
	}

	// グレースフル停止が shutdownTimeout 満了を待たずエラーなく完了する。
	select {
	case err := <-runDone:
		elapsed := time.Since(start)
		assert.NoError(t, err, "グレースフル停止はエラーなく完了する")
		assert.Less(t, elapsed, shutdownTimeout/2,
			"shutdownTimeout満了を待たずに戻ること（実測 %v / 上限 %v）", elapsed, shutdownTimeout)
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("Runが完了しなかった")
	}

	<-reqDone
}

// そら指摘①の回帰テスト。会話画面はSSEを常設する設計（仕様書§2.3）なので、
// SSE接続が張られたままの停止は「例外的な状況」ではなく通常運用の既定経路である。
// Events ハンドラは conn.Run(r.Context()) でブロックするが、http.Server.Shutdown は
// in-flight ハンドラの r.Context() をキャンセルしないため、BaseContext による能動的な
// キャンセルが無いと Shutdown は必ず shutdownTimeout 満了までブロックして失敗する。
func TestGracefulShutdown_SSE接続中でも停止がエラーなく完了する(t *testing.T) {
	h, db, hub, _ := newTestHandler(t)
	convID := ulid.Make().String()
	insertConversation(t, db, convID, time.Now())

	srv := &http.Server{Handler: h.Routes()}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()

	const shutdownTimeout = 3 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		runDone <- httpserver.Run(ctx, srv, ln, shutdownTimeout)
	}()

	// SSE接続を張り、読み取り可能な状態のまま維持する。
	reqCtx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet,
		"http://"+addr+"/conversations/"+convID+"/events", nil)
	require.NoError(t, err)

	client := &http.Client{}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// 最初のフレーム（retry: 3000）が読めた時点で「接続確立」とみなす。
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	require.NoError(t, err, "SSEの初回フレームが読めなかった")
	require.Equal(t, "retry: 3000\n", line)
	waitConnCount(t, hub, convID, 1)

	// 接続を維持したまま停止をトリガする。
	start := time.Now()
	cancel()

	select {
	case err := <-runDone:
		elapsed := time.Since(start)
		assert.NoError(t, err, "SSE接続が1本あってもグレースフル停止はエラーなく完了する")
		assert.Less(t, elapsed, shutdownTimeout/2,
			"shutdownTimeout満了を待たずに戻ること（実測 %v / 上限 %v）", elapsed, shutdownTimeout)
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("Runが完了しなかった")
	}

	assert.Equal(t, 0, hub.ConnCount(convID), "停止後は接続がHubから解除されている")
}
