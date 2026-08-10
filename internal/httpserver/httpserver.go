// Package httpserver は HTTP サーバの起動とグレースフル停止を担う。
// main から分離することで、停止時の挙動を結合テストで検証可能にする。
package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// Run は ln 上で srv を起動し、ctx がキャンセルされたらグレースフルに停止する。
//   - srv.Serve は別 goroutine で起動する（http.ErrServerClosed は正常終了として nil 扱い）。
//   - ctx.Done() で shutdownTimeout を上限に srv.Shutdown を呼び、in-flight リクエストの
//     完遂を待って新規接続を遮断する。
//   - Serve が停止トリガより先にエラー終了した場合は、そのエラーを返す。
//
// BaseContext を配って停止時に能動的にキャンセルするのは、SSE のように r.Context() が
// 閉じるまで戻らないハンドラのため。http.Server.Shutdown は in-flight ハンドラの
// r.Context() をキャンセルしないため、これが無いと SSE 接続が1本でもあるだけで
// Shutdown は必ず shutdownTimeout 満了までブロックしてタイムアウトする。
func Run(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error {
	// Serve 開始前にセットする必要がある（開始後の代入は競合し、既存接続にも届かない）。
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
		// 停止トリガより先に Serve が終了した（起動失敗など）。
		return err
	case <-ctx.Done():
		// in-flight の r.Context() を全て即座にキャンセルする。
		baseCancel()
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}
