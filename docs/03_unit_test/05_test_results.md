# 単体テスト結果記録 — フェーズ1・フェーズ2・フェーズ3・フェーズ4

| 項目 | 内容 |
|------|------|
| バージョン | 1.3 |
| 作成日 | 2026-07-08 |
| 更新日 | 2026-07-09（フェーズ4追記） |
| 作成者 | WhiteCUL（テスト仕様書・テストコード作成担当） |
| 対象 | `04_test_specification.md` フェーズ1（計77件）、`08_test_specification.md` フェーズ2（計43件）、`11_test_specification.md` フェーズ3（計52件）、`14_test_specification.md` フェーズ4（計60件、TC-4-1-26は欠番）。合計232件 |
| 実施状況 | 全件未実施（GREEN phase未着手のため） |

---

## 使い方

- 実装（GREEN phase）が完了した関数から順に、フェーズ1は`go test ./tests/unit/... -v`、フェーズ2はオーケストレーション層/純粋ロジック層を`go test ./tests/unit/... -v`、I/O層を`go test ./tests/integration/... -v`（要`ZUNCHA_TEST_DATABASE_URL`）、フェーズ3は`go test ./tests/unit/... -v`、フェーズ4は`npm test`（`tests/unit/frontend/`配下、Vitest）で実行し、結果を該当行に反映する。
- 「結果」列は `未実施` / `PASS` / `FAIL` のいずれかを記入する。
- `FAIL` の場合は「備考」列に失敗内容の要約（アサーション内容、想定と実際の差分など）を記載する。
- 実装修正後に再実行した場合は「実施日」を更新し、再実施の旨を「備考」に残す。

---

# フェーズ1（77件）

## 1-1. `IsValidULID`（21件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-1-1-01 | IsValidULID | 未実施 | | |
| TC-1-1-02 | IsValidULID | 未実施 | | |
| TC-1-1-03 | IsValidULID | 未実施 | | |
| TC-1-1-04 | IsValidULID | 未実施 | | |
| TC-1-1-05 | IsValidULID | 未実施 | | |
| TC-1-1-06 | IsValidULID | 未実施 | | |
| TC-1-1-07 | IsValidULID | 未実施 | | |
| TC-1-1-08 | IsValidULID | 未実施 | | |
| TC-1-1-09 | IsValidULID | 未実施 | | |
| TC-1-1-10 | IsValidULID | 未実施 | | |
| TC-1-1-11 | IsValidULID | 未実施 | | |
| TC-1-1-12 | IsValidULID | 未実施 | | |
| TC-1-1-13 | IsValidULID | 未実施 | | |
| TC-1-1-14 | IsValidULID | 未実施 | | |
| TC-1-1-15 | IsValidULID | 未実施 | | |
| TC-1-1-16 | IsValidULID | 未実施 | | |
| TC-1-1-17 | IsValidULID | 未実施 | | |
| TC-1-1-18 | IsValidULID | 未実施 | | |
| TC-1-1-19 | IsValidULID | 未実施 | | |
| TC-1-1-20 | IsValidULID | 未実施 | | |
| TC-1-1-21 | IsValidULID | 未実施 | | |

## 1-2. `TruncateFirstText`（11件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-1-2-01 | TruncateFirstText | 未実施 | | |
| TC-1-2-02 | TruncateFirstText | 未実施 | | |
| TC-1-2-03 | TruncateFirstText | 未実施 | | |
| TC-1-2-04 | TruncateFirstText | 未実施 | | |
| TC-1-2-05 | TruncateFirstText | 未実施 | | |
| TC-1-2-06 | TruncateFirstText | 未実施 | | |
| TC-1-2-07 | TruncateFirstText | 未実施 | | |
| TC-1-2-08 | TruncateFirstText | 未実施 | | |
| TC-1-2-09 | TruncateFirstText | 未実施 | | |
| TC-1-2-10 | TruncateFirstText | 未実施 | | |
| TC-1-2-11 | TruncateFirstText | 未実施 | | |

## 1-3. role・emotionバリデーション（3関数・24件）

### `ValidateRole`（6件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-1-3-R01 | ValidateRole | 未実施 | | |
| TC-1-3-R02 | ValidateRole | 未実施 | | |
| TC-1-3-R03 | ValidateRole | 未実施 | | |
| TC-1-3-R04 | ValidateRole | 未実施 | | |
| TC-1-3-R05 | ValidateRole | 未実施 | | |
| TC-1-3-R06 | ValidateRole | 未実施 | | |

### `ValidateEmotion`（14件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-1-3-E01 | ValidateEmotion | 未実施 | | |
| TC-1-3-E02 | ValidateEmotion | 未実施 | | |
| TC-1-3-E03 | ValidateEmotion | 未実施 | | |
| TC-1-3-E04 | ValidateEmotion | 未実施 | | |
| TC-1-3-E05 | ValidateEmotion | 未実施 | | |
| TC-1-3-E06 | ValidateEmotion | 未実施 | | |
| TC-1-3-E07 | ValidateEmotion | 未実施 | | |
| TC-1-3-E08 | ValidateEmotion | 未実施 | | |
| TC-1-3-E09 | ValidateEmotion | 未実施 | | |
| TC-1-3-E10 | ValidateEmotion | 未実施 | | |
| TC-1-3-E11 | ValidateEmotion | 未実施 | | |
| TC-1-3-E12 | ValidateEmotion | 未実施 | | |
| TC-1-3-E13 | ValidateEmotion | 未実施 | | |
| TC-1-3-E14 | ValidateEmotion | 未実施 | | |

### `ValidateRoleEmotionConsistency`（4件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-1-3-C01 | ValidateRoleEmotionConsistency | 未実施 | | |
| TC-1-3-C02 | ValidateRoleEmotionConsistency | 未実施 | | |
| TC-1-3-C03 | ValidateRoleEmotionConsistency | 未実施 | | |
| TC-1-3-C04 | ValidateRoleEmotionConsistency | 未実施 | | |

## 1-4. `IsValidInput`（14件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-1-4-01 | IsValidInput | 未実施 | | |
| TC-1-4-02 | IsValidInput | 未実施 | | |
| TC-1-4-03 | IsValidInput | 未実施 | | |
| TC-1-4-04 | IsValidInput | 未実施 | | |
| TC-1-4-05 | IsValidInput | 未実施 | | |
| TC-1-4-06 | IsValidInput | 未実施 | | |
| TC-1-4-07 | IsValidInput | 未実施 | | |
| TC-1-4-08 | IsValidInput | 未実施 | | |
| TC-1-4-09 | IsValidInput | 未実施 | | |
| TC-1-4-10 | IsValidInput | 未実施 | | |
| TC-1-4-11 | IsValidInput | 未実施 | | |
| TC-1-4-12 | IsValidInput | 未実施 | | |
| TC-1-4-13 | IsValidInput | 未実施 | | |
| TC-1-4-14 | IsValidInput | 未実施 | | |

## 1-5. `IsExpired`（7件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-1-5-01 | IsExpired | 未実施 | | |
| TC-1-5-02 | IsExpired | 未実施 | | |
| TC-1-5-03 | IsExpired | 未実施 | | |
| TC-1-5-04 | IsExpired | 未実施 | | |
| TC-1-5-05 | IsExpired | 未実施 | | |
| TC-1-5-06 | IsExpired | 未実施 | | |
| TC-1-5-07 | IsExpired | 未実施 | | |

---

## フェーズ1 集計

| 観点 | 件数 | PASS | FAIL | 未実施 |
|------|------|------|------|--------|
| 1-1 IsValidULID | 21 | 0 | 0 | 21 |
| 1-2 TruncateFirstText | 11 | 0 | 0 | 11 |
| 1-3 ValidateRole | 6 | 0 | 0 | 6 |
| 1-3 ValidateEmotion | 14 | 0 | 0 | 14 |
| 1-3 ValidateRoleEmotionConsistency | 4 | 0 | 0 | 4 |
| 1-4 IsValidInput | 14 | 0 | 0 | 14 |
| 1-5 IsExpired | 7 | 0 | 0 | 7 |
| **フェーズ1合計** | **77** | **0** | **0** | **77** |

---

# フェーズ2（43件）

## 2-1. POST /conversations（15件）

### オーケストレーション層（7件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-2-1-01 | CreateConversationService | 未実施 | | |
| TC-2-1-02 | CreateConversationService | 未実施 | | |
| TC-2-1-03 | CreateConversationService | 未実施 | | |
| TC-2-1-04 | CreateConversationService | 未実施 | | |
| TC-2-1-05 | CreateConversationService | 未実施 | | |
| TC-2-1-06 | CreateConversationService | 未実施 | | |
| TC-2-1-07 | CreateConversationService | 未実施 | | |

### I/O層（8件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-2-1-08 | ConversationRepository | 未実施 | | |
| TC-2-1-09 | ConversationRepository | 未実施 | | |
| TC-2-1-10 | ConversationRepository | 未実施 | | |
| TC-2-1-11 | ConversationRepository | 未実施 | | |
| TC-2-1-12 | ConversationRepository | 未実施 | | |
| TC-2-1-13 | ConversationRepository | 未実施 | | |
| TC-2-1-14 | ConversationRepository | 未実施 | | |
| TC-2-1-15 | ConversationRepository | 未実施 | | |

## 2-2. 会話履歴コンテキスト構築（15件）

### 純粋ロジック層（4件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-2-2-01 | ReverseMessages | 未実施 | | |
| TC-2-2-02 | ReverseMessages | 未実施 | | |
| TC-2-2-03 | ReverseMessages | 未実施 | | |
| TC-2-2-04 | ReverseMessages | 未実施 | | |

### I/O層（11件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-2-2-05 | MessageRepository | 未実施 | | |
| TC-2-2-06 | MessageRepository | 未実施 | | |
| TC-2-2-07 | MessageRepository | 未実施 | | |
| TC-2-2-08 | MessageRepository | 未実施 | | |
| TC-2-2-09 | MessageRepository | 未実施 | | |
| TC-2-2-10 | MessageRepository | 未実施 | | |
| TC-2-2-11 | MessageRepository | 未実施 | | |
| TC-2-2-12 | MessageRepository | 未実施 | | |
| TC-2-2-13 | MessageRepository | 未実施 | | |
| TC-2-2-14 | MessageRepository | 未実施 | | |
| TC-2-2-15 | MessageRepository | 未実施 | | |

## 2-3. GET /audio/{ulid}（13件）

### オーケストレーション層（8件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-2-3-01 | FetchAudioService | 未実施 | | |
| TC-2-3-02 | FetchAudioService | 未実施 | | |
| TC-2-3-03 | FetchAudioService | 未実施 | | |
| TC-2-3-04 | FetchAudioService | 未実施 | | |
| TC-2-3-05 | FetchAudioService | 未実施 | | |
| TC-2-3-06 | FetchAudioService | 未実施 | | |
| TC-2-3-07 | FetchAudioService | 未実施 | | |
| TC-2-3-08 | FetchAudioService | 未実施 | | |

### I/O層（5件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-2-3-09 | FetchAudioService（実DB・実FS） | 未実施 | | |
| TC-2-3-10 | FetchAudioService（実DB・実FS） | 未実施 | | |
| TC-2-3-11 | FetchAudioService（実DB・実FS） | 未実施 | | |
| TC-2-3-12 | FetchAudioService（実DB・実FS） | 未実施 | | |
| TC-2-3-13 | FetchAudioService（実DB・実FS） | 未実施 | | |

---

## フェーズ2 集計

| 観点 | 件数 | PASS | FAIL | 未実施 |
|------|------|------|------|--------|
| 2-1 オーケストレーション層 | 7 | 0 | 0 | 7 |
| 2-1 I/O層 | 8 | 0 | 0 | 8 |
| 2-2 純粋ロジック層 | 4 | 0 | 0 | 4 |
| 2-2 I/O層 | 11 | 0 | 0 | 11 |
| 2-3 オーケストレーション層 | 8 | 0 | 0 | 8 |
| 2-3 I/O層 | 5 | 0 | 0 | 5 |
| **フェーズ2合計** | **43** | **0** | **0** | **43** |

---

# フェーズ3（52件）

## 3-1. `ParseLLMResponse`（25件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-3-1-01 | ParseLLMResponse | 未実施 | | |
| TC-3-1-02 | ParseLLMResponse | 未実施 | | |
| TC-3-1-03 | ParseLLMResponse | 未実施 | | |
| TC-3-1-04 | ParseLLMResponse | 未実施 | | |
| TC-3-1-05 | ParseLLMResponse | 未実施 | | |
| TC-3-1-06 | ParseLLMResponse | 未実施 | | |
| TC-3-1-07 | ParseLLMResponse | 未実施 | | |
| TC-3-1-08 | ParseLLMResponse | 未実施 | | |
| TC-3-1-09 | ParseLLMResponse | 未実施 | | |
| TC-3-1-10 | ParseLLMResponse | 未実施 | | |
| TC-3-1-11 | ParseLLMResponse | 未実施 | | |
| TC-3-1-12 | ParseLLMResponse | 未実施 | | |
| TC-3-1-13 | ParseLLMResponse | 未実施 | | |
| TC-3-1-14 | ParseLLMResponse | 未実施 | | |
| TC-3-1-15 | ParseLLMResponse | 未実施 | | |
| TC-3-1-16 | ParseLLMResponse | 未実施 | | |
| TC-3-1-17 | ParseLLMResponse | 未実施 | | |
| TC-3-1-18 | ParseLLMResponse | 未実施 | | |
| TC-3-1-19 | ParseLLMResponse | 未実施 | | |
| TC-3-1-20 | ParseLLMResponse | 未実施 | | |
| TC-3-1-21 | ParseLLMResponse | 未実施 | | |
| TC-3-1-22 | ParseLLMResponse | 未実施 | | |
| TC-3-1-23 | ParseLLMResponse | 未実施 | | |
| TC-3-1-24 | ParseLLMResponse | 未実施 | | |
| TC-3-1-25 | ParseLLMResponse | 未実施 | | |

## 3-2. SSEイベント送出ロジック（`ResponseStreamer`、15件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-3-2-01 | ResponseStreamer | 未実施 | | |
| TC-3-2-02 | ResponseStreamer | 未実施 | | |
| TC-3-2-03 | ResponseStreamer | 未実施 | | |
| TC-3-2-04 | ResponseStreamer | 未実施 | | |
| TC-3-2-05 | ResponseStreamer | 未実施 | | |
| TC-3-2-06 | ResponseStreamer | 未実施 | | |
| TC-3-2-07 | ResponseStreamer | 未実施 | | |
| TC-3-2-08 | ResponseStreamer | 未実施 | | |
| TC-3-2-09 | ResponseStreamer | 未実施 | | |
| TC-3-2-10 | ResponseStreamer | 未実施 | | |
| TC-3-2-11 | ResponseStreamer | 未実施 | | |
| TC-3-2-12 | ResponseStreamer | 未実施 | | |
| TC-3-2-13 | ResponseStreamer | 未実施 | | |
| TC-3-2-14 | ResponseStreamer | 未実施 | | |
| TC-3-2-15 | ResponseStreamer | 未実施 | | |

## 3-3. `IsRecognitionFailed` / `IsTimedOut`（12件）

### `IsRecognitionFailed`（7件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-3-3-R01 | IsRecognitionFailed | 未実施 | | |
| TC-3-3-R02 | IsRecognitionFailed | 未実施 | | |
| TC-3-3-R03 | IsRecognitionFailed | 未実施 | | |
| TC-3-3-R04 | IsRecognitionFailed | 未実施 | | |
| TC-3-3-R05 | IsRecognitionFailed | 未実施 | | |
| TC-3-3-R06 | IsRecognitionFailed | 未実施 | | |
| TC-3-3-R07 | IsRecognitionFailed | 未実施 | | |

### `IsTimedOut`（5件）

| テストケースID | 対象関数 | 結果 | 実施日 | 備考 |
|---------------|---------|------|--------|------|
| TC-3-3-T01 | IsTimedOut | 未実施 | | |
| TC-3-3-T02 | IsTimedOut | 未実施 | | |
| TC-3-3-T03 | IsTimedOut | 未実施 | | |
| TC-3-3-T04 | IsTimedOut | 未実施 | | |
| TC-3-3-T05 | IsTimedOut | 未実施 | | |

---

## フェーズ3 集計

| 観点 | 件数 | PASS | FAIL | 未実施 |
|------|------|------|------|--------|
| 3-1 ParseLLMResponse | 25 | 0 | 0 | 25 |
| 3-2 SSEイベント送出ロジック（ResponseStreamer） | 15 | 0 | 0 | 15 |
| 3-3 IsRecognitionFailed | 7 | 0 | 0 | 7 |
| 3-3 IsTimedOut | 5 | 0 | 0 | 5 |
| **フェーズ3合計** | **52** | **0** | **0** | **52** |

---

# フェーズ4（60件、TC-4-1-26は欠番）

## 4-1. 入力モード切替（マイク/テキスト）とlocalStorage保存（26件、TC-4-1-26は欠番）

### `isInputMode`（10件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-4-1-01 | isInputMode | 未実施 | | |
| TC-4-1-02 | isInputMode | 未実施 | | |
| TC-4-1-03 | isInputMode | 未実施 | | |
| TC-4-1-04 | isInputMode | 未実施 | | |
| TC-4-1-05 | isInputMode | 未実施 | | |
| TC-4-1-06 | isInputMode | 未実施 | | |
| TC-4-1-07 | isInputMode | 未実施 | | |
| TC-4-1-08 | isInputMode | 未実施 | | |
| TC-4-1-09 | isInputMode | 未実施 | | |
| TC-4-1-10 | isInputMode | 未実施 | | |

### `getStoredInputMode` / `setStoredInputMode`（10件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-4-1-11 | getStoredInputMode | 未実施 | | |
| TC-4-1-12 | getStoredInputMode | 未実施 | | |
| TC-4-1-13 | getStoredInputMode | 未実施 | | |
| TC-4-1-14 | setStoredInputMode | 未実施 | | |
| TC-4-1-15 | setStoredInputMode | 未実施 | | |
| TC-4-1-16 | getStoredInputMode | 未実施 | | |
| TC-4-1-17 | getStoredInputMode | 未実施 | | |
| TC-4-1-18 | getStoredInputMode | 未実施 | | |
| TC-4-1-19 | setStoredInputMode | 未実施 | | |
| TC-4-1-20 | getStoredInputMode | 未実施 | | |

### `ModeToggle.svelte`（7件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-4-1-21 | ModeToggle | 未実施 | | |
| TC-4-1-22 | ModeToggle | 未実施 | | |
| TC-4-1-23 | ModeToggle | 未実施 | | |
| TC-4-1-24 | ModeToggle | 未実施 | | |
| TC-4-1-25 | ModeToggle | 未実施 | | |
| ~~TC-4-1-26~~ | ~~ModeToggle~~ | **欠番** | | そらのQAレビュー指摘反映（2026-07-09）。`ModeToggleProps`との責務不整合により削除。詳細は`14_test_specification.md` 7.3節参照 |
| TC-4-1-27 | ModeToggle | 未実施 | | |

## 4-2. 送信ボタンの活性制御（19件）

### `isSendButtonDisabled`（16件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-4-2-01 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-02 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-03 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-04 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-05 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-06 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-07 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-08 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-09 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-10 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-11 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-12 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-13 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-14 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-15 | isSendButtonDisabled | 未実施 | | |
| TC-4-2-16 | isSendButtonDisabled | 未実施 | | |

### `SendButton.svelte`（3件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-4-2-17 | SendButton | 未実施 | | |
| TC-4-2-18 | SendButton | 未実施 | | |
| TC-4-2-19 | SendButton | 未実施 | | |

## 4-3. エラー表現の出し分け（15件）

### `routeSSEEvent`（7件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-4-3-01 | routeSSEEvent | 未実施 | | |
| TC-4-3-02 | routeSSEEvent | 未実施 | | |
| TC-4-3-03 | routeSSEEvent | 未実施 | | |
| TC-4-3-04 | routeSSEEvent | 未実施 | | |
| TC-4-3-05 | routeSSEEvent | 未実施 | | |
| TC-4-3-06 | routeSSEEvent | 未実施 | | |
| TC-4-3-07 | routeSSEEvent | 未実施 | | |

### `Toast.svelte`（6件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-4-3-08 | Toast | 未実施 | | |
| TC-4-3-09 | Toast | 未実施 | | |
| TC-4-3-10 | Toast | 未実施 | | |
| TC-4-3-11 | Toast | 未実施 | | |
| TC-4-3-12 | Toast | 未実施 | | |
| TC-4-3-13 | Toast | 未実施 | | |

### `MessageBubble.svelte`（2件）

| テストケースID | 対象 | 結果 | 実施日 | 備考 |
|---------------|------|------|--------|------|
| TC-4-3-14 | MessageBubble | 未実施 | | |
| TC-4-3-15 | MessageBubble | 未実施 | | |

---

## フェーズ4 集計

| 観点 | 件数 | PASS | FAIL | 未実施 |
|------|------|------|------|--------|
| 4-1 isInputMode | 10 | 0 | 0 | 10 |
| 4-1 getStoredInputMode/setStoredInputMode | 10 | 0 | 0 | 10 |
| 4-1 ModeToggle | 6 | 0 | 0 | 6 |
| 4-2 isSendButtonDisabled | 16 | 0 | 0 | 16 |
| 4-2 SendButton | 3 | 0 | 0 | 3 |
| 4-3 routeSSEEvent | 7 | 0 | 0 | 7 |
| 4-3 Toast | 6 | 0 | 0 | 6 |
| 4-3 MessageBubble | 2 | 0 | 0 | 2 |
| **フェーズ4合計** | **60** | **0** | **0** | **60** |

> TC-4-1-26は欠番のため件数に含めない（そらのQAレビュー指摘反映、2026-07-09）。

---

## 総合集計（フェーズ1＋フェーズ2＋フェーズ3＋フェーズ4）

| フェーズ | 件数 | PASS | FAIL | 未実施 |
|---------|------|------|------|--------|
| フェーズ1 | 77 | 0 | 0 | 77 |
| フェーズ2 | 43 | 0 | 0 | 43 |
| フェーズ3 | 52 | 0 | 0 | 52 |
| フェーズ4 | 60 | 0 | 0 | 60 |
| **総合計** | **232** | **0** | **0** | **232** |

---

## 変更履歴

### 2026-07-08

初版作成。フェーズ1全77件を「未実施」で記録するテンプレートとして用意。

### 2026-07-08（追記）

フェーズ2の43件（オーケストレーション層19件・I/O層24件）を追記。フェーズ1と合算した総合集計（120件）を追加。

### 2026-07-08（追記2）

フェーズ3の52件（3-1 ParseLLMResponse 25件・3-2 ResponseStreamer 15件・3-3 IsRecognitionFailed/IsTimedOut 12件）を追記。フェーズ1・2と合算した総合集計（172件）を追加。

### 2026-07-09（追記3）

フェーズ4の61件（4-1 入力モード切替 27件・4-2 送信ボタン活性制御 19件・4-3 エラー表現出し分け 15件）を追記。フェーズ1〜3と合算した総合集計（233件）を追加。「使い方」節にフェーズ4の実行コマンド（`npm test`）を追記。

### 2026-07-09（追記4）

そらのQAレビュー指摘②を反映。TC-4-1-26を欠番とし、フェーズ4の件数を61件→60件、総合集計を233件→232件に修正。

*記録: WhiteCUL*
