// グレースフルシャットダウンの結合テスト。対応仕様: NF-SCALE-02（単一インスタンス動作）、結合テスト phase IT-2。
// 「in-flightリクエストの完遂」と「停止後の新規接続遮断」を決定的（channel同期・sleep非依存）に検証する。
// DB非依存（httptest不使用・実ポート起動）＝skipなし。
package integration

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

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
	select {
	case resp := <-respCh:
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "in-flightリクエストは完遂する")
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
