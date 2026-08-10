# 単一インスタンス動作 手動スモークテスト手順 (NF-SCALE-02 / IT-2)

作成: 四国めたん (テックリード) / 2026-07-23
目的: 単一インスタンスでのグレースフルシャットダウン挙動を、自動結合テスト(`tests/integration/graceful_shutdown_test.go`)で機械化した「in-flightの扱い(`r.Context()`非参照なら完遂・依存する処理は打ち切り)・停止後の新規接続遮断」以外の観点も含め、実プロセスで目視確認する手順を定める。

## 前提

- 開発環境セットアップ済み(`tasks/todo.md`「開発環境セットアップ」参照)。Go 1.22.10 が `~/.local/bin/go` 経由で利用可能。
- テスト用PostgreSQL(`zuncha-test-pg`, ポート55432)が稼働中。
- 環境変数: `ZUNCHA_DATABASE_URL`(必須)、任意で `ZUNCHA_ALLOWED_ORIGINS`、`PORT`(既定8080)。

## 手順

### 1. サーバ起動

```bash
source scripts/test_env.sh   # ZUNCHA_TEST_DATABASE_URL 等を読み込む
export ZUNCHA_DATABASE_URL="$ZUNCHA_TEST_DATABASE_URL"
export PORT=8090
go run ./cmd/api
```

**期待**: ログに `zuncha API サーバを :8090 で起動` が出る。プロセスは単一。

### 2. 通常リクエストの疎通(別ターミナル)

```bash
curl -s -XPOST localhost:8090/conversations   # → {"id":"<26桁ULID>"} / 201
curl -s -o /dev/null -w "%{http_code}\n" localhost:8090/audio/01ARZ3NDEKTSV4RRFFQ69G5FAV  # → 404
```

**期待**: 201(会話作成)・404(未登録音声)。

### 3. グレースフルシャットダウン(in-flightの扱いの目視)

1. 別ターミナルでリクエストを投げつつ、
2. サーバプロセスへ `SIGTERM`(または Ctrl+C=`SIGINT`)を送る。

```bash
# サーバのPIDへ
kill -TERM <server_pid>
```

**期待**:
- サーバログに `サーバを停止しました` が出て正常終了する(パニック・強制終了ではない)。
- 送信中(in-flight)だったリクエストのうち、`r.Context()`を参照しない処理は完遂しレスポンスを受け取れる(`shutdownTimeout`=10秒以内)。
  - 停止トリガ時に in-flight の `r.Context()` は能動的にキャンセルされる(SSEのように`r.Context()`が閉じるまで戻らないハンドラを終わらせるため)。そのため`r.Context()`に依存する処理(DBクエリ・SSE配信など)は打ち切られ、クライアントはエラー応答や接続断を受け取り得る。
- シャットダウン開始後の**新規**接続は拒否される(`curl: (7) Failed to connect`)。

### 4. 単一インスタンスであることの確認

- 同一ポートで2つ目のインスタンスを起動しようとすると `bind: address already in use` で失敗すること(多重起動しない前提の確認)。

```bash
# 1つ目が起動中の状態で
PORT=8090 go run ./cmd/api   # → listen tcp :8090: bind: address already in use
```

## 自動テストとの対応

| 観点 | 手段 |
|---|---|
| in-flightリクエストの扱い(`r.Context()`非参照は完遂 / 依存する処理は打ち切り) | 自動: `tests/integration/graceful_shutdown_test.go`(channel同期で決定的) |
| 停止後の新規接続遮断 | 自動: 同上 |
| グレースフル停止ログ・正常終了 | 手動: 本手順 §3 |
| 多重起動不可(ポート占有) | 手動: 本手順 §4 |
| 実DB接続下での通常疎通 | 手動: 本手順 §2 |

## 備考

- 本アプリは C-08(社内ネットワーク前提・IP制限は運用側)・NF-SCALE-01(最大10人同時=負荷テストで別途検証)の想定であり、単一インスタンス運用が前提。水平スケール(複数インスタンス)は本フェーズのスコープ外。
