# 議事録 — 機能設計ディスカッション

| 項目 | 内容 |
|------|------|
| 日時 | 2026-06-16 |
| 参加者 | めたん（システムアーキテクト）、小夜（UI/UX設計担当）、WhiteCUL（ドキュメント作成） |
| テーマ | ZUNCHA — DB設計・画面設計の合意形成 |
| 成果物 | `01_screen_design.md`、`02_database_design.md` |

---

## 議題 1: データベース設計（担当: めたん）

### 決定事項

#### テーブル構成
3テーブル構成で合意。

| テーブル名 | 役割 |
|-----------|------|
| `conversations` | 会話セッション管理（ULID・30日自動削除起点） |
| `messages` | メッセージ履歴（user/assistant・感情ラベル） |
| `audio_files` | TTS音声一時ファイル管理（再生後削除） |

#### 主要な設計判断

- **主キーはULID**を採用。生成はアプリ層（Go）で行い、`github.com/oklog/ulid/v2` を推奨。
- **30日自動削除**はバックグラウンドバッチではなく、新規会話開始時にGCをトリガーする方式を採用。`expires_at` カラムを generated column（`started_at + 30日`）として定義し、起動コスト低減と実装シンプルさを両立。
- **CASCADE削除**を採用。`conversations` 削除時に `messages` と `audio_files` を連鎖削除。
- **音声ファイルの削除フロー**: `GET /audio/{id}` 時にファイルを返し、レスポンス送信後に物理ファイルとレコードを即削除する方式。`fetched_at` カラムで取得済み判定を行うGC補助インデックスも用意。
- **感情ラベル**は `messages.emotion` に格納。assistantのみ使用し、7種類（喜び/怒り/悲しみ/楽しい/照れ/困惑/ドヤ顔）を定義。
- **`first_text`**はルーン単位で20文字カット。Go側で `[]rune` にキャストして切り出す。

#### インデックス設計

| インデックス名 | 対象 | 用途 |
|--------------|------|------|
| idx_conversations_expires_at | conversations(expires_at) | 30日期限切れ検索 |
| idx_messages_conv_created | messages(conversation_id, created_at DESC) | 直近20件取得 |
| idx_audio_files_fetched_at | audio_files(fetched_at) WHERE NOT NULL | GC用（部分インデックス） |
| idx_audio_files_conversation_id | audio_files(conversation_id) | FK補助 |
| idx_audio_files_message_id | audio_files(message_id) | FK補助 |

---

## 議題 2: 画面設計（担当: 小夜）

### 決定事項

#### 画面構成

| 画面ID | 画面名 | URL |
|--------|--------|-----|
| S-01 | ホーム/会話一覧 | `/` |
| S-02 | 会話画面 | `/c/{conversationID}` |
| S-03 | 過去会話閲覧 | `/c/{conversationID}?view=history` |

#### 主要な設計判断

- **ずんだもんの感情表現**: S-02においてSSEの `emotion` イベントをトリガーに画像を切替。opacity transition 0.33s cubic-bezier で滑らかに。
- **S-03はS-02の派生**: S-02と同構成で、音声入力バーのみ非表示とする。全メッセージ表示（最大20件）でページネーションなし。
- **不正URLのリダイレクト**: conversationIDがULID形式（26文字 `[0-9A-HJKMNP-TV-Z]`）でない場合は `/` にリダイレクト。
- **デザイントークン統一**: CTA色 `#3E6AE1`（Electric Blue）、遷移 `0.33s cubic-bezier(0.5,0,0,0.75)`、ボタン角丸 `4px` で統一。
- **新規会話起点**: S-01の「新しい会話を始める」ボタン押下時にサーバー側でULID採番＋GC実行。クライアントは `/c/{新規ULID}` にリダイレクト。

#### SSEイベント仕様（S-02）

| イベント | フロント処理 |
|---------|------------|
| emotion | ずんだもん画像切替 |
| text | 会話ログへのストリーミング表示 |
| audio_url | GET /audio/{ulid} で音声取得・再生。再生後サーバーが削除 |
| done | ステータスを「待機中」に戻す |
| error | エラートースト表示（3秒自動消滅） |

---

## 未決定事項・次回への持ち越し

| # | 内容 | 担当 |
|---|------|------|
| 1 | API設計（エンドポイント定義、リクエスト/レスポンス仕様） | 未定 |
| 2 | 音声入力の技術選定（Web Speech API vs. whisper等） | 未定 |
| 3 | エラー設計の詳細（タイムアウト値、リトライ方針） | 未定 |

---

## 変更履歴

### 2026-06-17

変更2件追加: ①ずんだもん画像の大型化（左寄せ/S-01は2列グリッド・S-02は45vh）、②入力モード切り替えUI（音声モード/テキストモードのSegmented Control）

*記録: WhiteCUL*
