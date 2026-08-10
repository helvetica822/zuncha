CREATE TABLE conversations (
    id         TEXT        PRIMARY KEY NOT NULL,
    started_at TIMESTAMPTZ NOT NULL    DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL    GENERATED ALWAYS AS ((started_at AT TIME ZONE 'UTC' + INTERVAL '30 days') AT TIME ZONE 'UTC') STORED,
    first_text VARCHAR(20)
);

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

-- message_id は messages(id) への FK を持たない（意図的）。
-- TTS合成(audio_filesへの登録)はassistantメッセージ保存(SendDone)より前に完了する必要があり、
-- その時点ではmessage_idはまだ採番済みだがmessagesテーブルには未挿入のため、
-- FK制約があると外部キー違反になる。conversation_idのFK(CASCADE)は残るため、
-- 会話削除時には音声ファイルも連鎖削除される。詳細は
-- docs/04_implementation/04_realtime_wiring_design.md D-4参照。
CREATE TABLE audio_files (
    id              TEXT        PRIMARY KEY NOT NULL,
    conversation_id TEXT        NOT NULL
        REFERENCES conversations(id) ON DELETE CASCADE,
    message_id      TEXT        NOT NULL,
    file_path       TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    fetched_at      TIMESTAMPTZ
);

CREATE INDEX idx_conversations_expires_at
    ON conversations (expires_at);

CREATE INDEX idx_messages_conv_created
    ON messages (conversation_id, created_at DESC);

CREATE INDEX idx_audio_files_fetched_at
    ON audio_files (fetched_at)
    WHERE fetched_at IS NOT NULL;

CREATE INDEX idx_audio_files_conversation_id
    ON audio_files (conversation_id);

CREATE INDEX idx_audio_files_message_id
    ON audio_files (message_id);
