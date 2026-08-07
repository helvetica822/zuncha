// Package middleware は HTTP ハンドラを横断的に装飾するミドルウェアを提供する。
package middleware

import "net/http"

const (
	corsAllowMethods = "GET, POST, OPTIONS"
	corsAllowHeaders = "Content-Type"
)

// CORS は allowedOrigins に含まれる Origin からのリクエストにのみ CORS ヘッダを付与する
// ミドルウェアを返す。credentials（Access-Control-Allow-Credentials）は対象外（NF-SEC-02）。
//
// 挙動:
//   - Origin が allowlist 一致: Access-Control-Allow-Origin と Vary: Origin を付与。
//   - OPTIONS（プリフライト）: 許可 Origin なら Allow-Methods/Headers も付与し、
//     許可有無に関わらず 204 で終了して next を呼ばない。
//   - Origin 無し / 非許可 Origin の非OPTIONS: CORS ヘッダを付けず next へ素通し。
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			_, isAllowed := allowed[origin]
			originAllowed := origin != "" && isAllowed

			if originAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				if originAllowed {
					w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
					w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
