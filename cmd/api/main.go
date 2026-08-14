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
	"zuncha/internal/audioconv"
	"zuncha/internal/handler"
	"zuncha/internal/httpserver"
	"zuncha/internal/llm"
	"zuncha/internal/localfs"
	"zuncha/internal/middleware"
	"zuncha/internal/postgres"
	"zuncha/internal/service"
	"zuncha/internal/sse"
	"zuncha/internal/voicevox"
	"zuncha/internal/whispercpp"
)

const (
	defaultPort        = "8080"
	envPort            = "PORT"
	envDatabaseURL     = "ZUNCHA_DATABASE_URL"
	envAllowedOrigins  = "ZUNCHA_ALLOWED_ORIGINS"
	envAnthropicAPIKey = "ANTHROPIC_API_KEY"
	envVoicevoxBaseURL = "VOICEVOX_BASE_URL"
	envWhisperBaseURL  = "WHISPER_SERVER_BASE_URL"
	// defaultVoicevoxBaseURL は VOICEVOX ENGINE の標準ポート。ANTHROPIC_API_KEY と違い
	// 秘密情報でも環境ごとに変わる値でもなく、開発時は既定値でそのまま動くため、
	// 未設定を起動時エラーにせずデフォルトを与える（W-11 の Compose ではサービス名で上書きする）。
	defaultVoicevoxBaseURL = "http://localhost:50021"
	readHeaderTimeout      = 10 * time.Second
	shutdownTimeout        = 10 * time.Second
	allowedOriginsSep      = ","
)

// config は環境変数から読み込むサーバ設定。
type config struct {
	port            string
	databaseURL     string
	allowedOrigins  []string
	anthropicAPIKey string
	voicevoxBaseURL string
	whisperBaseURL  string
}

func loadConfig() config {
	c := config{
		port:            os.Getenv(envPort),
		databaseURL:     os.Getenv(envDatabaseURL),
		allowedOrigins:  parseAllowedOrigins(os.Getenv(envAllowedOrigins)),
		anthropicAPIKey: os.Getenv(envAnthropicAPIKey),
		voicevoxBaseURL: os.Getenv(envVoicevoxBaseURL),
		whisperBaseURL:  os.Getenv(envWhisperBaseURL),
	}
	if c.port == "" {
		c.port = defaultPort
	}
	if c.voicevoxBaseURL == "" {
		c.voicevoxBaseURL = defaultVoicevoxBaseURL
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
	// VOICEVOX と違いデフォルトを持たせない。whisper-server の既定ポートは 8080 で
	// 本API の defaultPort と衝突するため、既定値を補うと「自分自身へ POST して 404」
	// という原因の見えない失敗になる。未設定は起動時に落として気づかせる。
	if cfg.whisperBaseURL == "" {
		log.Fatal("WHISPER_SERVER_BASE_URL が未設定です（例: http://whisper-server:8080）")
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

	newULID := func() string { return ulid.Make().String() }

	// TTSClient が nil のままだと最初のユーザー発話で nil ポインタ参照になりプロセスが
	// 落ちる（そらの申し送り）。ここで実装を注入することで解消する。
	ttsClient, err := voicevox.NewClient(cfg.voicevoxBaseURL, audioRepo, files, newULID, time.Now)
	if err != nil {
		// loadConfig がデフォルトURLを補うため、現状この分岐は到達しない（多層防御として残す）。
		// 将来 defaultVoicevoxBaseURL を撤廃して必須設定にした場合にここが効く。
		log.Fatalf("VOICEVOXクライアントの生成に失敗: %v", err)
	}

	streamer := service.NewResponseStreamer(llmClient, parser, ttsClient, sse.NewSentenceChunker())
	chat := service.NewChatService(
		msgRepo, convRepo, streamer, hub,
		newULID,
		time.Now,
	)

	whisperClient, err := whispercpp.NewClient(cfg.whisperBaseURL)
	if err != nil {
		log.Fatalf("whisper-serverクライアントの生成に失敗: %v", err)
	}
	speechToText := service.NewSpeechToTextService(audioconv.NewConverter(), whisperClient)

	h := handler.NewHandler(
		service.NewCreateConversationService(convRepo),
		service.NewFetchAudioService(audioRepo, files),
		chat, speechToText, convRepo, hub,
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
