// Package main は zuncha API サーバのエントリポイント。
package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/oklog/ulid/v2"

	"zuncha/internal/anthropic"
	"zuncha/internal/handler"
	"zuncha/internal/httpserver"
	"zuncha/internal/llm"
	"zuncha/internal/localfs"
	"zuncha/internal/middleware"
	"zuncha/internal/postgres"
	"zuncha/internal/service"
	"zuncha/internal/sse"
	"zuncha/internal/tts"
)

const (
	defaultPort        = "8080"
	envPort            = "PORT"
	envDatabaseURL     = "ZUNCHA_DATABASE_URL"
	envAllowedOrigins  = "ZUNCHA_ALLOWED_ORIGINS"
	envAnthropicAPIKey = "ANTHROPIC_API_KEY"
	readHeaderTimeout  = 10 * time.Second
	shutdownTimeout    = 10 * time.Second
	allowedOriginsSep  = ","
)

// config は環境変数から読み込むサーバ設定。
type config struct {
	port            string
	databaseURL     string
	allowedOrigins  []string
	anthropicAPIKey string
}

func loadConfig() config {
	c := config{
		port:            os.Getenv(envPort),
		databaseURL:     os.Getenv(envDatabaseURL),
		allowedOrigins:  parseAllowedOrigins(os.Getenv(envAllowedOrigins)),
		anthropicAPIKey: os.Getenv(envAnthropicAPIKey),
	}
	if c.port == "" {
		c.port = defaultPort
	}
	return c
}

// parseAllowedOrigins はカンマ区切りの許可オリジン文字列を分割する。
// 各要素は trim し、空要素は無視する。
func parseAllowedOrigins(raw string) []string {
	parts := strings.Split(raw, allowedOriginsSep)
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

func main() {
	cfg := loadConfig()
	if cfg.databaseURL == "" {
		log.Fatal("ZUNCHA_DATABASE_URL が未設定です")
	}
	// 実行時に初めて落ちるより起動時に落とす（DB URL と同じ流儀）。
	if cfg.anthropicAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY が未設定です")
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
	msgRepo := postgres.NewMessageRepository(db)
	files := localfs.NewFileStore()
	hub := sse.NewHub()

	llmClient, err := anthropic.NewClient(cfg.anthropicAPIKey)
	if err != nil {
		log.Fatalf("Claude APIクライアントの生成に失敗: %v", err)
	}
	parser := llm.NewDefaultParser()

	// TODO(W-09): TTSClient(VOICEVOX) の実装が揃い次第ここへ差し替える。
	// ResponseStreamer と ChatService の配線自体は確定済み。
	var ttsClient tts.TTSClient
	streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, sse.NewSentenceChunker())
	chat := service.NewChatService(
		msgRepo, convRepo, streamer, hub,
		func() string { return ulid.Make().String() },
		time.Now,
	)

	h := handler.NewHandler(
		service.NewCreateConversationService(convRepo),
		service.NewFetchAudioService(audioRepo, files),
		chat, convRepo, hub,
	)

	handler := middleware.CORS(cfg.allowedOrigins)(h.Routes())

	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ln, err := net.Listen("tcp", ":"+cfg.port)
	if err != nil {
		log.Fatalf("リッスンに失敗: %v", err)
	}

	// SIGINT/SIGTERM でキャンセルされる ctx を用意し、グレースフル停止のトリガにする。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("zuncha API サーバを :%s で起動", cfg.port)
	if err := httpserver.Run(ctx, httpServer, ln, shutdownTimeout); err != nil {
		log.Fatalf("サーバ実行エラー: %v", err)
	}
	log.Println("サーバを停止しました")
}
