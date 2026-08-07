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
func Run(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error {
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
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}
