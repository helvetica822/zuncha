# cmd/api 配線 実装仕様 (T-20相当・非テスト対象)

- 作成者: 四国めたん (テックリード)
- 対象: `cmd/api/main.go`(#24で空mainを作成済み。これを実配線に置換)。
- スコープ判断: 実LLM/STT/TTS実装は後続フェーズ(R-3/R-4、SDK追加を伴う)ゆえ、**現存する実装済みコンポーネントのみを配線**する最小HTTPサーバとする。すなわち:
  - **配線する**: DB接続 → `postgres.NewConversationRepository/NewAudioRepository` + `localfs.NewFileStore` → `service.NewCreateConversationService`・`service.NewFetchAudioService`。
  - **エンドポイント**: `POST /conversations`(会話作成)、`GET /audio/{id}`(音声取得)。
  - **据え置き(TODO)**: チャット/SSE(`ResponseStreamer`)は `LLMClient`/`TTSClient`/`ResponseParser`/`TextChunker`/`EventSink` の実装が未整備のため本フェーズでは配線しない。コメントで明示する。
- ルーティング: Go 1.22 標準 `net/http.ServeMux`(メソッド+パスパターン `POST /conversations`・`GET /audio/{id}`、`r.PathValue("id")`)。外部ルータ不要。
- テスト: cmd配下はテスト対象外。検証は `go build ./...` と手動スモーク(サーバ起動→curl)で行う。

## cmd/api/main.go
```go
// Package main は zuncha API サーバのエントリポイント。
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"zuncha/internal/localfs"
	"zuncha/internal/postgres"
	"zuncha/internal/repository"
	"zuncha/internal/service"
)

// config は環境変数から読み込むサーバ設定。
type config struct {
	port        string
	databaseURL string
}

func loadConfig() config {
	c := config{
		port:        os.Getenv("PORT"),
		databaseURL: os.Getenv("ZUNCHA_DATABASE_URL"),
	}
	if c.port == "" {
		c.port = "8080"
	}
	return c
}

// server はハンドラが依存するサービスを保持する。
type server struct {
	createConv *service.CreateConversationService
	fetchAudio *service.FetchAudioService
}

func main() {
	cfg := loadConfig()
	if cfg.databaseURL == "" {
		log.Fatal("ZUNCHA_DATABASE_URL が未設定です")
	}

	db, err := sql.Open("postgres", cfg.databaseURL)
	if err != nil {
		log.Fatalf("DB接続の初期化に失敗: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("DBへのpingに失敗: %v", err)
	}

	convRepo := postgres.NewConversationRepository(db)
	audioRepo := postgres.NewAudioRepository(db)
	files := localfs.NewFileStore()

	srv := &server{
		createConv: service.NewCreateConversationService(convRepo),
		fetchAudio: service.NewFetchAudioService(audioRepo, files),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /conversations", srv.handleCreateConversation)
	mux.HandleFunc("GET /audio/{id}", srv.handleGetAudio)
	// TODO(後続フェーズ): POST /chat 等の SSE エンドポイントは
	// LLMClient/TTSClient/ResponseParser/TextChunker/EventSink の実装整備後に配線する。

	httpServer := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// グレースフルシャットダウン。
	go func() {
		log.Printf("zuncha API サーバを :%s で起動", cfg.port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("サーバ起動に失敗: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("シャットダウン中にエラー: %v", err)
	}
	log.Println("サーバを停止しました")
}

// handleCreateConversation は POST /conversations を処理する。
func (s *server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	conv, err := s.createConv.CreateConversation(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "会話の作成に失敗しました")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"id": conv.ID})
}

// handleGetAudio は GET /audio/{id} を処理する。
func (s *server) handleGetAudio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := s.fetchAudio.FetchAudio(r.Context(), id)
	if errors.Is(err, repository.ErrAudioNotFound) {
		respondError(w, http.StatusNotFound, "音声が見つかりません")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "音声の取得に失敗しました")
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("レスポンスのエンコードに失敗: %v", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
```

## 完了検証(必須・ログ報告)
`export PATH="$HOME/.local/bin:$PATH"` 後:
1. `gofmt -l cmd` 差分なし、`go vet ./cmd/...` 成功、`go build ./...` 成功。
2. スモークテスト(テストDBを流用): `source scripts/test_env.sh && ZUNCHA_DATABASE_URL="$ZUNCHA_TEST_DATABASE_URL" PORT=8090 go run ./cmd/api &` で起動 →
   - `curl -s -XPOST localhost:8090/conversations` が 201 で `{"id":"..."}`(26桁ULID)を返す。
   - `curl -s -o /dev/null -w "%{http_code}" localhost:8090/audio/01ARZ3NDEKTSV4RRFFQ69G5FAV` が 404 を返す(未登録IDゆえ)。
   - 確認後サーバを停止(kill)。作成した会話レコードは `TRUNCATE` で掃除。
3. `go test ./tests/...` に退行がないこと。
in_progress維持で検証ログ付き報告。cmd配下はテスト対象外ゆえレビューは軽微(そら任意)。
