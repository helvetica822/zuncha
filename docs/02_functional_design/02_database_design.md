# データベース設計書

| 項目 | 内容 |
|------|------|
| バージョン | 1.0 |
| 作成日 | 2026-06-16 |
| 作成者 | めたん（システムアーキテクト） / 記録: WhiteCUL |
| DBMS | PostgreSQL |

---

## 目次

1. [テーブル一覧](#1-テーブル一覧)
2. [テーブル定義](#2-テーブル定義)
   - [conversations](#21-conversations-テーブル)
   - [messages](#22-messages-テーブル)
   - [audio_files](#23-audio_files-テーブル)
3. [インデックス](#3-インデックス)
4. [ER図（テキスト表現）](#4-er図テキスト表現)
5. [補足・設計方針](#5-補足設計方針)

---

## 1. テーブル一覧

3テーブルのシンプルな構成で、会話セッション・メッセージ履歴・音声ファイルのライフサイクルをすべてカバーします。

| テーブル名 | 概要 |
|-----------|------|
| `conversations` | 会話セッション管理（ULID・30日自動削除起点） |
| `messages` | メッセージ履歴（user/assistant・感情ラベル） |
| `audio_files` | TTS音声一時ファイル管理（再生後削除） |

---

## 2. テーブル定義

### 2.1 conversations テーブル

会話セッションを管理するテーブル。削除されると関連する `messages` と `audio_files` はCASCADEで連鎖削除される。

| カラム名 | 型 | 制約 | 説明 |
|---------|-----|------|------|
| id | TEXT | PRIMARY KEY, NOT NULL | ULID（アプリ層で生成） |
| started_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 会話開始日時 |
| expires_at | TIMESTAMPTZ | NOT NULL, GENERATED ALWAYS AS (started_at + INTERVAL '30 days') STORED | 自動削除判定日時（started_at + 30日） |
| first_text | VARCHAR(20) | NULL可 | 最初のユーザー発話の冒頭20文字（ルーン単位） |

**DDL:**

```sql
CREATE TABLE conversations (
    id         TEXT        PRIMARY KEY NOT NULL,
    started_at TIMESTAMPTZ NOT NULL    DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL    GENERATED ALWAYS AS (started_at + INTERVAL '30 days') STORED,
    first_text VARCHAR(20)
);
```

---

### 2.2 messages テーブル

会話内のメッセージ履歴を管理するテーブル。`role` カラムで発話者を区別し、assistantの発話には感情ラベルを付与する。

| カラム名 | 型 | 制約 | 説明 |
|---------|-----|------|------|
| id | TEXT | PRIMARY KEY, NOT NULL | ULID（アプリ層で生成） |
| conversation_id | TEXT | NOT NULL, FK → conversations.id ON DELETE CASCADE | 親会話のID |
| role | TEXT | NOT NULL, CHECK (role IN ('user', 'assistant')) | 発話者 |
| content | TEXT | NOT NULL | 発話テキスト全文 |
| emotion | TEXT | NULL可, CHECK（7種） | 感情ラベル（assistantのみ使用） |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | 発話日時 |

**感情ラベル一覧（`emotion` カラムの許容値）:**

| 値 | 説明 |
|----|------|
| 喜び | うれしい・ポジティブな反応 |
| 怒り | 怒り・強い否定 |
| 悲しみ | 悲しい・落ち込んだ反応 |
| 楽しい | 楽しそう・弾んだ反応 |
| 照れ | 恥ずかしい・はにかみ |
| 困惑 | 戸惑い・困り顔 |
| ドヤ顔 | 自信満々・誇らしい反応 |

> `emotion` は `role = 'assistant'` の場合のみ設定する。`role = 'user'` のレコードでは NULL とする。

**DDL:**

```sql
CREATE TABLE messages (
    id              TEXT        PRIMARY KEY NOT NULL,
    conversation_id TEXT        NOT NULL
        REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT        NOT NULL
        CHECK (role IN ('user', 'assistant')),
    content         TEXT        NOT NULL,
    emotion         TEXT        CHECK (
                        emotion IS NULL OR
                        emotion IN ('喜び', '怒り', '悲しみ', '楽しい', '照れ', '困惑', 'ドヤ顔')
                    ),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

### 2.3 audio_files テーブル

TTS（Text-to-Speech）で生成した音声ファイルの一時管理テーブル。フロントが `GET /audio/{id}` でファイルを取得した後、サーバーはファイルとレコードを即削除する。

| カラム名 | 型 | 制約 | 説明 |
|---------|-----|------|------|
| id | TEXT | PRIMARY KEY, NOT NULL | ULID（URL `/audio/{id}` のキー） |
| conversation_id | TEXT | NOT NULL, FK → conversations.id ON DELETE CASCADE | 紐づく会話ID |
| message_id | TEXT | NOT NULL, FK → messages.id ON DELETE CASCADE | 紐づくメッセージID |
| file_path | TEXT | NOT NULL | サーバー上の一時ファイルパス |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | ファイル生成日時 |
| fetched_at | TIMESTAMPTZ | NULL可 | フロントがGETした日時（NULL = 未取得） |

**DDL:**

```sql
CREATE TABLE audio_files (
    id              TEXT        PRIMARY KEY NOT NULL,
    conversation_id TEXT        NOT NULL
        REFERENCES conversations(id) ON DELETE CASCADE,
    message_id      TEXT        NOT NULL
        REFERENCES messages(id) ON DELETE CASCADE,
    file_path       TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    fetched_at      TIMESTAMPTZ
);
```

---

## 3. インデックス

| インデックス名 | テーブル | 対象カラム | 条件 | 用途 |
|--------------|---------|-----------|------|------|
| idx_conversations_expires_at | conversations | expires_at | — | 30日期限切れ会話の検索（GC） |
| idx_messages_conv_created | messages | (conversation_id, created_at DESC) | — | 直近20件メッセージの高速取得 |
| idx_audio_files_fetched_at | audio_files | fetched_at | WHERE fetched_at IS NOT NULL | 取得済みファイルのGC用（部分インデックス） |
| idx_audio_files_conversation_id | audio_files | conversation_id | — | 外部キー補助 |
| idx_audio_files_message_id | audio_files | message_id | — | 外部キー補助 |

**DDL:**

```sql
-- 30日期限切れ検索
CREATE INDEX idx_conversations_expires_at
    ON conversations (expires_at);

-- 直近20件メッセージ取得
CREATE INDEX idx_messages_conv_created
    ON messages (conversation_id, created_at DESC);

-- 取得済み音声ファイルのGC（部分インデックス）
CREATE INDEX idx_audio_files_fetched_at
    ON audio_files (fetched_at)
    WHERE fetched_at IS NOT NULL;

-- FK補助
CREATE INDEX idx_audio_files_conversation_id
    ON audio_files (conversation_id);

CREATE INDEX idx_audio_files_message_id
    ON audio_files (message_id);
```

---

## 4. ER図（テキスト表現）

```
conversations
─────────────────────────────────
PK  id              TEXT
    started_at      TIMESTAMPTZ
    expires_at      TIMESTAMPTZ  (generated)
    first_text      VARCHAR(20)
        │
        │ 1:N (ON DELETE CASCADE)
        │
        ├──────────────────────────────────────────────────┐
        │                                                  │
        ▼                                                  │
messages                                                   │
─────────────────────────────────                          │
PK  id              TEXT                                   │
FK  conversation_id TEXT ──→ conversations.id              │
    role            TEXT                                   │
    content         TEXT                                   │
    emotion         TEXT                                   │
    created_at      TIMESTAMPTZ                            │
        │                                                  │
        │ 1:1 (ON DELETE CASCADE)                          │
        ▼                                                  │
audio_files                                                │
─────────────────────────────────                          │
PK  id              TEXT                                   │
FK  conversation_id TEXT ──→ conversations.id ─────────────┘
FK  message_id      TEXT ──→ messages.id
    file_path       TEXT
    created_at      TIMESTAMPTZ
    fetched_at      TIMESTAMPTZ
```

---

## 5. 補足・設計方針

### 5.1 ULID採用理由

- UUIDに比べてソート可能であり、`created_at` インデックスとの親和性が高い。
- 生成はアプリ層（Go）で行う。推奨ライブラリ: `github.com/oklog/ulid/v2`
- DBへの依存がなく、マイクロサービス移行時にも対応しやすい。

### 5.2 30日自動削除（GC）の仕組み

バックグラウンドバッチは使用せず、新規会話開始時にGCを実行する方式を採用。

**GCトリガータイミング:** `POST /conversations` リクエスト受信時

**実行クエリ:**
```sql
DELETE FROM conversations WHERE expires_at < NOW();
```

CASCADE設定により、削除された `conversations` に紐づく `messages` と `audio_files` レコードも自動削除される。

**`expires_at` の生成:** `generated column` として定義しているため、アプリ側での計算が不要。

### 5.3 音声ファイルの削除フロー

1. SSEで `audio_url` イベントを受信したフロントが `GET /audio/{id}` をリクエスト
2. サーバーが `file_path` からファイルを読み込む
3. `fetched_at` を現在時刻で更新
4. フロントへレスポンスを送信
5. サーバーが物理ファイルを削除
6. `audio_files` レコードを削除

**GC補助（fetched_at部分インデックス）:** 万が一削除が漏れた場合のために、`fetched_at IS NOT NULL` の部分インデックスを用いて未削除の取得済みレコードを高速検索できる。

### 5.4 first_text のカット仕様

- ルーン単位で20文字カット（バイト単位ではない）。
- Goでの実装例:
  ```go
  runes := []rune(text)
  if len(runes) > 20 {
      runes = runes[:20]
  }
  firstText := string(runes)
  ```

### 5.5 messages.emotion の使用条件

- `role = 'assistant'` のレコードにのみ設定する。
- `role = 'user'` のレコードでは NULL とする。
- アプリ層でのバリデーションを推奨。DBレベルの CHECK 制約は任意。

### 5.6 直近20件メッセージの取得クエリ例

```sql
SELECT *
FROM messages
WHERE conversation_id = $1
ORDER BY created_at DESC
LIMIT 20;
```

`idx_messages_conv_created` インデックスにより効率的に検索可能。
