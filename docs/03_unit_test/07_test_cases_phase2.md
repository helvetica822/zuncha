# フェーズ2 単体テストケース一覧

| 項目 | 内容 |
|------|------|
| バージョン | 1.0 |
| 作成日 | 2026-07-08 |
| 作成者 | ひまり（テストケース設計担当） |
| 入力 | `06_test_perspectives_phase2.md`（めたん作成） |
| 前提 | 6章確認事項F〜J：つむぎ仮決定（本書内で反映） |
| 次工程 | WhiteCULが単体テスト仕様書・テストコードへ清書 |

---

## 0. 前提：確認事項F〜Jの決定内容（本書での反映方針）

| # | 対象 | 決定内容 | 本書での反映 |
|---|------|---------|-------------|
| F | 2-1 | ULID採番（INSERT）とGC実行（DELETE）は別処理とし、GCはbest-effortとして扱う。GCが失敗しても新規会話作成は必ず成功させる | GC失敗時も新規会話作成が成功することをオーケストレーション層のケースで明示的に固定 |
| G | 2-2 | 並び替え（古い→新しい）処理はRepository層の責務とする。「直近10往復」は単純な20件上限（role交互は問わない） | 並び替えロジックを純粋関数として抽出しつつ、Repository層の契約として組み込む。role連続データでも単純20件上限で取得することを明示 |
| H | 2-3 | `fetched_at`更新後の物理ファイル削除失敗時、自動回復（再削除バッチ等）は用意しない。検出のみに留める | 削除失敗後の自動再試行が発生しないことをオーケストレーション層で確認するケースを追加 |
| I | 2-3 | 「物理ファイル削除→レコード削除」（現行DB設計書5.3の順序）を維持する | 順序保証のケース、およびファイル削除失敗時にレコード削除へ進まないケースとして反映 |
| J | 2-3 | 同一ULIDへの同時多重リクエストに対する排他制御は行わない | 二重リクエスト時、2回目は「レコード削除済み＝404相当」になる挙動をそのまま境界値ケースとして固定 |

---

## 目次

1. [2-1. POST /conversations（ULID採番＋GC実行）](#2-1-post-conversationsulid採番gc実行)
2. [2-2. 会話履歴コンテキスト構築](#2-2-会話履歴コンテキスト構築)
3. [2-3. GET /audio/{ulid}](#2-3-get-audiuulid)
4. [セルフチェック（重複排除・不足確認）](#セルフチェック重複排除不足確認)
5. [決定事項K〜M（つむぎ決定）](#決定事項km・つむぎ決定)

---

## 2-1. POST /conversations（ULID採番＋GC実行）

**設計方針**: 「オーケストレーション層（Service層。Repositoryをモック化し振る舞い・呼び出し順序・呼び出し回数を検証）」と「I/O層（Repository層。テスト用DBに実接続しCASCADE削除・generated column・値域を検証）」の2階層でケースを分離する。

### オーケストレーション層（モックでテスト）

**対象**: `CreateConversationService.CreateConversation(ctx) (*Conversation, error)`。内部で`Repository.GC(ctx)`と`Repository.InsertConversation(ctx, ...)`をモック越しに呼び出す。

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-2-1-01 | `Repository.GC`モックがエラーなし、`Repository.InsertConversation`モックが成功を返すよう設定する | `CreateConversation(ctx)`を呼ぶ | 新規conversationが返る。`GC`・`InsertConversation`がそれぞれ1回ずつ呼ばれる |
| TC-2-1-02 | `Repository.GC`モックが「削除0件」を返すよう設定する | `CreateConversation(ctx)`を呼ぶ | 新規会話作成が成功する（GC対象なしでも正常終了） |
| TC-2-1-03 | `CreateConversation`を連続して3回呼ぶ | 各呼び出しごとの`Repository.GC`呼び出し回数を計測する | 呼び出しのたびに必ず1回ずつ`GC`が呼ばれる（バックグラウンドバッチを使わない設計の担保） |
| TC-2-1-04 | `Repository.GC`モックがエラーを返すよう設定する（確認事項F） | `CreateConversation(ctx)`を呼ぶ | GCのエラーは握りつぶされ、`InsertConversation`が実行され、新規会話作成は成功する |
| TC-2-1-05 | `Repository.InsertConversation`モックがエラー（DB接続断・タイムアウト含む）を返すよう設定する | `CreateConversation(ctx)`を呼ぶ | `CreateConversation`はエラーを返し、レコードは作成されない |
| TC-2-1-06 | `Repository.GC`・`Repository.InsertConversation`の両モックを呼び出し順序記録可能にする | `CreateConversation(ctx)`を呼ぶ | `GC`が`InsertConversation`より先に呼ばれる順序であることを確認する |
| TC-2-1-07 | `Repository.InsertConversation`モックへの引数をキャプチャできる状態にし、`CreateConversation`を連続2回呼ぶ | 2回分のULID採番値を比較する | 1回目と2回目で異なるULIDが`InsertConversation`に渡される |

### I/O層（実DB接続で検証）

**対象**: `Repository.GC(ctx)`（`DELETE FROM conversations WHERE expires_at < NOW()`）、`Repository.InsertConversation(ctx, ...)`。

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-2-1-08 | テスト用DBに期限切れ`conversations`レコードが0件の状態にする | `InsertConversation`を実行する | 新規`conversations`レコードが1件作成される |
| TC-2-1-09 | テスト用DBに期限切れ`conversations`レコードを複数件、紐づく`messages`・`audio_files`とともに用意する | `GC`（DELETE）を実行する | 期限切れレコードが全件削除され、CASCADE先の`messages`・`audio_files`も連鎖削除される |
| TC-2-1-10 | テスト用DBに新規`conversations`レコードを作成する | 作成直後のレコードを取得する | `started_at`≒NOW()、`first_text`はNULL、`expires_at`が`started_at`+30日（generated column）である |
| TC-2-1-11 | `expires_at`がちょうどNOW()と一致する`conversations`レコードを1件用意する | `GC`を実行する | 当該レコードは削除されない（`<`の等号非対称性がSQLレベルでも成立） |
| TC-2-1-12 | 期限切れレコードを1件のみ用意する | `GC`を実行する | 1件が正しく削除される |
| TC-2-1-13 | 期限切れレコードを1,000件用意する | `GC`を実行する | 1,000件全件が削除される |
| TC-2-1-14 | 既存の`conversations`レコードと同一のULIDでINSERTを試みる（PRIMARY KEY制約違反を意図的に発生させる） | `InsertConversation`を実行する | エラーを返す（採番し直しのリトライは行わない。7章K参照） |
| TC-2-1-15 | テスト用DBに対し、`CreateConversation`相当の処理を最大10並列で同時実行する | 全リクエストの完了を待つ | GCの重複実行やULID採番の衝突が起きず、全リクエストが正常に完了する |

---

## 2-2. 会話履歴コンテキスト構築

**設計方針**: 「並び替え（古い→新しい）」を純粋関数として抽出したうえで、確認事項Gの決定に従いRepository層の契約に組み込む。「純粋ロジック層（並び替え関数単体、モック・DB不要）」と「Repository層（取得クエリ＋並び替え済みの戻り値契約、実DB接続で検証）」に分離する。

### 純粋ロジック層（モック・DB不要）

**対象**: `ReverseMessages(messages []Message) []Message`

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-2-2-01 | 新しい→古い順に並んだ5件の`Message`スライスを用意する | `ReverseMessages(messages)`を呼ぶ | 古い→新しい順に反転されたスライスを返す |
| TC-2-2-02 | 空スライス`[]Message{}`を用意する | `ReverseMessages(messages)`を呼ぶ | 空スライスを返す（`nil`ではない） |
| TC-2-2-03 | 1件のみの`Message`スライスを用意する | `ReverseMessages(messages)`を呼ぶ | 同じ1件をそのまま返す |
| TC-2-2-04 | 新しい→古い順に並んだ20件の`Message`スライスを用意する | `ReverseMessages(messages)`を呼ぶ | 古い→新しい順に正しく反転される |

### Repository層（実DB接続で検証）

**対象**: `GetRecentMessages(ctx, conversationID string) ([]Message, error)`（内部で`ORDER BY created_at DESC LIMIT 20`取得後、並び替えまで完了させて返す）

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-2-2-05 | 対象`conversation_id`に紐づく`messages`を5件（20件以下）用意する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | 全5件を古い→新しい順で返す |
| TC-2-2-06 | 対象`conversation_id`に紐づく`messages`を30件（20件超）用意する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | 直近20件のみを古い→新しい順で返す |
| TC-2-2-07 | user/assistantが交互に20件（10往復）存在する`messages`を用意する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | 20件全件が古い→新しい順で返る（10往復として自然に一致する） |
| TC-2-2-08 | 対象`conversation_id`に紐づく`messages`が0件の状態にする | `GetRecentMessages(ctx, conversationID)`を呼ぶ | 空スライスを返す（`nil`ではない） |
| TC-2-2-09 | DBに存在しない`conversation_id`を指定する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | エラーにはせず空配列を返す（7章L参照） |
| TC-2-2-10 | 対象`conversation_id`に紐づく`messages`をちょうど20件用意する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | 全20件が取得され、順序も古い→新しい順で正しい |
| TC-2-2-11 | 対象`conversation_id`に紐づく`messages`を19件用意する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | 全19件が取得される |
| TC-2-2-12 | 対象`conversation_id`に紐づく`messages`を21件用意する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | 最新20件のみ取得され、最も古い1件が除外される |
| TC-2-2-13 | `created_at`が完全に同一時刻（同一ミリ秒）の`messages`を複数件用意する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | ULID等のタイブレーカーにより順序が一意に安定する（フレーキーテスト対策） |
| TC-2-2-14 | user発話が2連続するなどrole非交互の`messages`を22件用意する（確認事項G） | `GetRecentMessages(ctx, conversationID)`を呼ぶ | role交互を問わず単純に直近20件が取得される |
| TC-2-2-15 | 対象`conversation_id`に紐づく`messages`を30件（作成順が判別できる内容で）用意する | `GetRecentMessages(ctx, conversationID)`を呼ぶ | 戻り値の要素が常に古い→新しい順であるという契約が実DB経由でも成立する（並び替え忘れの回帰防止） |

---

## 2-3. GET /audio/{ulid}

**設計方針**: 「ファイル読込」「`fetched_at`更新」「物理ファイル削除」「レコード削除」の各ステップをモック可能なインターフェース（`Repository`・`FileStore`）越しに呼び出す「オーケストレーション層（Service層。モックで順序・失敗時の後続スキップを検証）」と、「I/O層（実DB・実ファイルシステムで最終結果を検証）」に分離する。

### オーケストレーション層（モックでテスト）

**対象**: `FetchAudioService.FetchAudio(ctx, ulid string) ([]byte, error)`。内部で`FileStore.Read`・`Repository.UpdateFetchedAt`・`FileStore.Delete`・`Repository.DeleteRecord`をモック越しに順序通り呼び出す。

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-2-3-01 | `Repository`・`FileStore`の全モックが正常応答するよう設定する | `FetchAudio(ctx, ulid)`を呼ぶ | `Read`→`UpdateFetchedAt`→`Delete`→`DeleteRecord`の順で1回ずつ呼ばれ、ファイル内容が返る |
| TC-2-3-02 | `Repository`が対象ULIDに該当するレコードなし（404相当）を返すよう設定する | `FetchAudio(ctx, ulid)`を呼ぶ | 404相当のエラーを返し、`UpdateFetchedAt`以降は一切呼ばれない |
| TC-2-3-03 | `FileStore.Read`モックがエラーを返すよう設定する | `FetchAudio(ctx, ulid)`を呼ぶ | エラーを返し、`UpdateFetchedAt`・`Delete`・`DeleteRecord`は呼ばれない |
| TC-2-3-04 | `Repository.UpdateFetchedAt`モックがエラー（DB接続断）を返すよう設定する | `FetchAudio(ctx, ulid)`を呼ぶ | エラーを返し、`FileStore.Delete`・`DeleteRecord`は呼ばれない（`fetched_at`未更新のまま音声データが失われる事態を防ぐ） |
| TC-2-3-05 | `FileStore.Delete`モックがエラーを返すよう設定する（確認事項I） | `FetchAudio(ctx, ulid)`を呼ぶ | エラーを返し、`Repository.DeleteRecord`は呼ばれない（現行順序＝ファイル削除→レコード削除を維持） |
| TC-2-3-06 | `FileStore.Delete`モックがエラーを返すよう設定した状態で`FetchAudio`を実行する（確認事項H） | 呼び出し完了後の`FileStore.Delete`呼び出し回数を確認する | 自動での再削除リトライは発生しない（呼び出しは1回のみ。検出のみ・自動回復なしの仕様） |
| TC-2-3-07 | `FileStore.Delete`モックは成功、`Repository.DeleteRecord`モックがエラーを返すよう設定する | `FetchAudio(ctx, ulid)`を呼ぶ | エラーを返す（ファイルは削除済み・レコードが残る「幽霊レコード」状態になることを許容する仕様として明示） |
| TC-2-3-08 | `Repository.UpdateFetchedAt`・`Repository.DeleteRecord`モックが「対象0件」（CASCADE削除で先に消えていた状態）を返すよう設定する | `FetchAudio(ctx, ulid)`を呼ぶ | エラーとせず、削除済みとして冪等に成功扱いとする |

### I/O層（実DB・実ファイルシステムで検証）

| ID | Given（前提条件） | When（操作） | Then（期待結果） |
|----|-------------------|-------------|------------------|
| TC-2-3-09 | テスト用DB・実ファイルシステムに存在するULID・音声ファイルを1件用意する | `FetchAudio(ctx, ulid)`を実行する | 処理完了後、物理ファイルが実際に削除され、`audio_files`レコードも削除されている |
| TC-2-3-10 | TC-2-3-09の処理を完了させた状態にする | 同一ULIDで再度`FetchAudio`を実行する | 404相当のエラーが返る（レコード削除済みのため） |
| TC-2-3-11 | `audio_files`レコードは存在するが、対応する物理ファイルを実際には配置しない状態にする | `FetchAudio(ctx, ulid)`を実行する | エラーを返し、`fetched_at`更新・レコード削除のいずれも実行されない |
| TC-2-3-12 | 実DB・実ファイルシステムに存在するULID・音声ファイルを1件用意し、同一ULIDへ同時に2件のリクエストを送出する（確認事項J） | 2件のリクエストをほぼ同時に実行する | 排他制御は行われず、1回目が処理を完了させ、2回目は「レコード削除済み＝404相当」として扱われる |
| TC-2-3-13 | `audio_files`レコードの`fetched_at`列を意図的に非NULLにしたテストデータを用意する（途中失敗シナリオの再現） | `FetchAudio(ctx, ulid)`を実行する | 実装済みの分岐に従って一貫した挙動（正常処理継続 or エラー）を示すことを確認する |

---

## セルフチェック（重複排除・不足確認）

### 重複排除の実施内容

- 2-1: 「DB接続断・タイムアウト」（めたんの異常系観点）は、Insert失敗の一般化されたケース（TC-2-1-05）に統合し、個別ケースとして重複させていない。
- 2-2: 正常系（5件・30件の例）と境界値（ちょうど20件・19件・21件）は、検証対象の値が異なるため別ケースとして独立させたが、検証観点（件数・順序）自体は重複しないよう役割分担している。
- 2-3: 「ファイル削除成功後にレコード削除が失敗する」ケースと「対象0件で完了する」ケース（CASCADE割り込み）は、いずれも異なる原因・異なる期待挙動のため別ケース（TC-2-3-07 / TC-2-3-08）として残した。

### めたんの観点との突合結果

| 観点（めたん） | 反映状況 |
|---|---|
| 2-1 Service層／Repository層の2階層構成 | オーケストレーション層／I/O層として明確に分離して反映 |
| 2-1 GC呼び出し回数の担保（毎回1回） | TC-2-1-03で反映 |
| 2-1 GC失敗時の新規会話作成継続（確認事項F） | TC-2-1-04で反映 |
| 2-1 ULID一意性（連続呼び出し） | TC-2-1-07で反映 |
| 2-1 CASCADE削除の実DB確認 | TC-2-1-09で反映 |
| 2-1 `<`の等号非対称性のSQLレベル再確認 | TC-2-1-11で反映 |
| 2-2 並び替えの純粋関数抽出 | TC-2-2-01〜04（`ReverseMessages`単体）で反映 |
| 2-2 並び替え忘れの重大リスクへの対応 | TC-2-2-15（契約テスト）で反映 |
| 2-2 空配列と`nil`の区別 | TC-2-2-02、TC-2-2-08で反映 |
| 2-2 同一ミリ秒レコードのタイブレーカー | TC-2-2-13で反映 |
| 2-2 role非交互データでの往復定義（確認事項G） | TC-2-2-14で反映 |
| 2-3 5ステップの順序保証・途中失敗時の後続スキップ | TC-2-3-01〜05で反映 |
| 2-3 ファイル削除失敗時の自動回復要否（確認事項H） | TC-2-3-06で反映 |
| 2-3 ファイル削除→レコード削除の順序維持（確認事項I） | TC-2-3-05で反映 |
| 2-3 幽霊レコード状態の許容 | TC-2-3-07で反映 |
| 2-3 CASCADE割り込み時の冪等性 | TC-2-3-08で反映 |
| 2-3 二重リクエスト時の排他制御なし（確認事項J） | TC-2-3-12で反映 |
| 2-3 `fetched_at`非NULL状態の異常データ | TC-2-3-13で反映 |
| 2-3 ファイルシステム操作のモック化（`FileStore`インターフェース） | オーケストレーション層の対象記述で明示 |

### 抽出した不足ケース（初期観点になかったが追加したもの）

- TC-2-1-06: GC→INSERTの呼び出し順序そのものをモックで明示的に確認するケース（めたんは「自己GC対象化の防御的確認」に言及していたが、その前提となる順序保証自体を独立ケース化した）。
- TC-2-2-07: user/assistant交互データで20件＝10往復が自然に一致することを、確認事項Gの「単純20件上限」決定の裏付けとして明示。
- TC-2-3-10: 削除完了後の再アクセスで404相当になることを、オーケストレーション層のTC-2-3-02（存在しない場合の404）とは別に、I/O層の結果として明示。

---

## 決定事項K〜M（つむぎ決定・2026-07-08）

確認事項F〜Jとは別に、めたんの観点の中でテストケース設計時に暫定判断が必要だった論点。つむぎの判断により、いずれも本書の暫定対応のまま確定した。

| # | 対象 | 論点 | 決定内容 | 本書への反映 |
|---|------|------|---------|-------------|
| K | 2-1 | ULID採番でPRIMARY KEY制約違反（衝突）が発生した場合、即エラーとするか採番し直してリトライするか | **即エラーとする**。ULID（128bit）の衝突確率は天文学的に低く実務上ほぼ発生しない。リトライ機構の実装は過剰（YAGNI）であり、万一発生した場合は重大な異常事態として表面化させる方が、サイレントリトライで問題を隠蔽するより安全なため | TC-2-1-14（即エラー）は変更不要、確定 |
| L | 2-2 | DBに存在しない`conversation_id`を指定した場合、エラーを返すか空配列を返すか | **空配列を返す**。本関数はLLM呼び出し前のコンテキスト取得が目的であり、「メッセージ0件」という結果は「存在しないID」でも「まだ発言のない新規会話」でも同じ扱いでよい。存在有無の区別が必要な場面（画面表示等）は別レイヤー（1-1のULID検証＋DB存在確認）の責務とする | TC-2-2-09（空配列を返す）は変更不要、確定 |
| M | 2-2 | `conversation_id`の形式バリデーションをこの関数内でも防御的に行うか | **行わない**。責務分離の原則通り、形式バリデーションは1-1の`IsValidULID`の責務。この関数に重複させると責務が曖昧になり、テストも冗長になるため | 本書の前提（呼び出し元でバリデーション済みの値を受け取る）は変更不要、確定 |

以上、フェーズ2（3機能・オーケストレーション層/I/O層分割含む）で計 **43件** のテストケースを設計（2-1: 15件、2-2: 15件、2-3: 13件）。決定事項K〜Mも確定したため、本書は確定版としてWhiteCULへの単体テスト仕様書清書・テストコード化に引き継ぐ。
