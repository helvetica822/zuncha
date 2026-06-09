---
trigger: always_on
---

# Go言語 Webアプリケーション コーディング規約

## 1. 基本方針

### 1.1 公式スタイルガイドの遵守
- [Effective Go](https://go.dev/doc/effective_go)に従う
- `gofmt`でコードを自動フォーマットする
- `go vet`でコードの静的解析を行う
- `golangci-lint`を使用して包括的なリンティングを実施する

### 1.2 可読性と保守性
- シンプルで明確なコードを書く
- 複雑なロジックには適切なコメントを付ける
- 関数は1つの責務のみを持つようにする

## 2. 命名規則

### 2.1 パッケージ名
- 短く、簡潔で、小文字のみを使用する
- アンダースコアやキャメルケースは使用しない
- 例: `http`, `user`, `auth`, `handler`

### 2.2 変数・関数名
- キャメルケースを使用する
- 公開する識別子は大文字で始める（例: `UserHandler`）
- 非公開の識別子は小文字で始める（例: `parseRequest`）
- 略語は統一的に扱う（例: `URL`は`URL`または`url`、`userID`ではなく`userID`）

### 2.3 定数
- キャメルケースを使用する
- すべて大文字のスネークケースは使用しない
```go
// Good
const MaxConnections = 100
const defaultTimeout = 30 * time.Second

// Bad
const MAX_CONNECTIONS = 100
```

### 2.4 インターフェース
- 単一メソッドのインターフェースは`er`で終わる名前を推奨
```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Handler interface {
    Handle(w http.ResponseWriter, r *http.Request)
}
```

## 3. プロジェクト構造

### 3.1 標準的なディレクトリレイアウト
```
project/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   └── model/
├── pkg/
│   └── util/
├── config/
├── migrations/
├── scripts/
├── go.mod
└── go.sum
```

### 3.2 レイヤー分離
- **Handler層**: HTTPリクエストの処理とレスポンスの生成
- **Service層**: ビジネスロジックの実装
- **Repository層**: データアクセス層
- **Model層**: データ構造の定義

## 4. エラーハンドリング

### 4.1 エラーの返却
- エラーは常に最後の戻り値として返す
- エラーをラップして文脈情報を追加する
```go
// Good
func GetUser(id int) (*User, error) {
    user, err := db.FindUser(id)
    if err != nil {
        return nil, fmt.Errorf("failed to get user %d: %w", id, err)
    }
    return user, nil
}
```

### 4.2 エラーチェック
- エラーは必ずチェックする
- `_`でエラーを無視しない（正当な理由がある場合を除く）
```go
// Good
result, err := someFunction()
if err != nil {
    return err
}

// Bad
result, _ := someFunction()
```

### 4.3 カスタムエラー
- ドメイン固有のエラーは独自の型を定義する
```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}
```

## 5. HTTPハンドラー

### 5.1 ハンドラーの構造
```go
type UserHandler struct {
    service UserService
    logger  *log.Logger
}

func NewUserHandler(service UserService, logger *log.Logger) *UserHandler {
    return &UserHandler{
        service: service,
        logger:  logger,
    }
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
    // リクエストの検証
    // サービス層の呼び出し
    // レスポンスの生成
}
```

### 5.2 ステータスコードの使用
- 適切なHTTPステータスコードを返す
- 200: 成功
- 201: 作成成功
- 400: バリデーションエラー
- 401: 認証エラー
- 403: 権限エラー
- 404: リソースが見つからない
- 500: サーバーエラー

### 5.3 JSONレスポンス
```go
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(data); err != nil {
        log.Printf("failed to encode response: %v", err)
    }
}

func respondError(w http.ResponseWriter, status int, message string) {
    respondJSON(w, status, map[string]string{"error": message})
}
```

## 6. コンテキストの使用

### 6.1 コンテキストの伝播
- HTTPリクエストのコンテキストを下位層に伝播させる
```go
func (s *UserService) GetUser(ctx context.Context, id int) (*User, error) {
    // コンテキストをリポジトリ層に渡す
    return s.repo.FindByID(ctx, id)
}
```

### 6.2 タイムアウトとキャンセル
```go
ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
defer cancel()

user, err := h.service.GetUser(ctx, userID)
```

## 7. データベース操作

### 7.1 接続管理
- データベース接続プールを使用する
- `sql.DB`は長寿命オブジェクトとして扱う

### 7.2 トランザクション
```go
func (r *UserRepository) CreateUser(ctx context.Context, user *User) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // データベース操作

    return tx.Commit()
}
```

### 7.3 SQLインジェクション対策
- プリペアドステートメントを使用する
```go
// Good
row := db.QueryRowContext(ctx, "SELECT * FROM users WHERE id = ?", userID)

// Bad
query := fmt.Sprintf("SELECT * FROM users WHERE id = %d", userID)
```

## 8. ロギング

### 8.1 構造化ロギング
- 構造化ロギングライブラリを使用する（例: `zerolog`, `zap`）
```go
logger.Info().
    Str("user_id", userID).
    Str("action", "create_user").
    Msg("user created successfully")
```

### 8.2 ログレベル
- Debug: 開発時のデバッグ情報
- Info: 通常の動作情報
- Warn: 警告（エラーではないが注意が必要）
- Error: エラー情報

## 9. テスト

### 9.1 テストファイル
- テストファイルは`_test.go`で終わる
- テスト対象と同じパッケージに配置する

### 9.2 テーブル駆動テスト
```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "user@example.com", false},
        {"invalid email", "invalid", true},
        {"empty email", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.email)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 9.3 モック
- インターフェースを活用してモックを作成する
- `gomock`などのモックライブラリを使用する

## 10. セキュリティ

### 10.1 入力検証
- すべてのユーザー入力を検証する
- バリデーションライブラリを使用する（例: `validator`）

### 10.2 認証・認可
- JWTトークンの検証を適切に行う
- センシティブな情報はログに出力しない

### 10.3 CORS設定
```go
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

## 11. パフォーマンス

### 11.1 ゴルーチン
- 無制限にゴルーチンを生成しない
- ワーカープールパターンを使用する

### 11.2 メモリ管理
- 不要なメモリアロケーションを避ける
- スライスの容量を事前に確保する
```go
// Good
users := make([]User, 0, expectedSize)

// Bad (頻繁なリアロケーション)
var users []User
```

## 12. 依存性注入

### 12.1 コンストラクタパターン
```go
type UserService struct {
    repo   UserRepository
    cache  Cache
    logger Logger
}

func NewUserService(repo UserRepository, cache Cache, logger Logger) *UserService {
    return &UserService{
        repo:   repo,
        cache:  cache,
        logger: logger,
    }
}
```

## 13. 設定管理

### 13.1 環境変数
- 環境変数を使用して設定を管理する
- `viper`や`envconfig`などのライブラリを使用する

```go
type Config struct {
    Port     int    `env:"PORT" envDefault:"8080"`
    DBHost   string `env:"DB_HOST" envDefault:"localhost"`
    LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}
```

## 14. コメント

### 14.1 パッケージコメント
```go
// Package handler provides HTTP request handlers for the API.
package handler
```

### 14.2 公開API
- すべての公開関数・型にはコメントを付ける
- コメントは対象の名前で始める
```go
// GetUser retrieves a user by ID from the database.
func GetUser(id int) (*User, error) {
    // ...
}
```