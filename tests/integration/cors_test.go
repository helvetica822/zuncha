// CORSミドルウェアの結合テスト。対応仕様: NF-SEC-02（社内ドメインのみ許可）、結合テスト phase IT-1。
// DB非依存（httptestのみ）＝skipなし。
package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"zuncha/internal/middleware"
)

const (
	corsAllowedOrigin = "https://app.example.com"
	corsDeniedOrigin  = "https://evil.example.com"
)

// newCORSHandler は allowlist に corsAllowedOrigin のみを含む CORS ミドルウェアで
// スタブ next をラップしたハンドラと、next が実行されたか追跡するフラグを返す。
func newCORSHandler() (http.Handler, *bool) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.CORS([]string{corsAllowedOrigin})(next)
	return handler, &nextCalled
}

func TestCORS_AllowedOriginGET(t *testing.T) {
	handler, nextCalled := newCORSHandler()

	req := httptest.NewRequest(http.MethodGet, "/conversations", nil)
	req.Header.Set("Origin", corsAllowedOrigin)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, corsAllowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"),
		"許可Originはそのまま Access-Control-Allow-Origin に反映される")
	assert.Equal(t, "Origin", rec.Header().Get("Vary"),
		"オリジン別キャッシュ制御のため Vary: Origin を付与する")
	assert.True(t, *nextCalled, "通常リクエストは next へ通す")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCORS_DeniedOriginGET(t *testing.T) {
	handler, nextCalled := newCORSHandler()

	req := httptest.NewRequest(http.MethodGet, "/conversations", nil)
	req.Header.Set("Origin", corsDeniedOrigin)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"非許可Originには Access-Control-Allow-Origin を付けない")
	assert.True(t, *nextCalled, "非許可Originでも非OPTIONSは素通しして next を実行する")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCORS_AllowedOriginPreflight(t *testing.T) {
	handler, nextCalled := newCORSHandler()

	req := httptest.NewRequest(http.MethodOptions, "/conversations", nil)
	req.Header.Set("Origin", corsAllowedOrigin)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code, "プリフライトは204で終了する")
	assert.Equal(t, corsAllowedOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "Origin", rec.Header().Get("Vary"))
	assert.False(t, *nextCalled, "プリフライトは next を呼ばない")
}

func TestCORS_DeniedOriginPreflight(t *testing.T) {
	handler, nextCalled := newCORSHandler()

	req := httptest.NewRequest(http.MethodOptions, "/conversations", nil)
	req.Header.Set("Origin", corsDeniedOrigin)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"非許可Originのプリフライトには ACAO を付けない")
	assert.False(t, *nextCalled, "プリフライトは許可有無に関わらず next を呼ばない")
}

func TestCORS_NoOriginHeader(t *testing.T) {
	handler, nextCalled := newCORSHandler()

	req := httptest.NewRequest(http.MethodGet, "/conversations", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"Originヘッダが無ければ CORS ヘッダを付けない")
	assert.True(t, *nextCalled, "Originヘッダ無しは素通しして next を実行する")
	assert.Equal(t, http.StatusOK, rec.Code)
}
