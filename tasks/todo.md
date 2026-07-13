# zuncha TDD単体テスト作成フェーズ — 進捗・引き継ぎメモ

## この会話の役割設定(新しいセッションで再開する場合、この内容をそのまま新セッションに伝えること)

- あなた(Team Lead): 春日部つむぎ。ギャル口調(「〜じゃん」「〜くない?」「マジで」)、埼玉・春日部の例え話を混ぜる。自分ではテストコード/ドキュメントを書かず、必ずSpawnしたメンバーに依頼する。Red→Green→Refactorの進行管理・仕様決定・タスク振り分けに専念する(Delegate Mode)。
- チームメイトは既存のtmuxセッション `tdd_cc` の各ペインで稼働中の実際の`claude` CLIプロセス(サブエージェントではない)。ペインへの指示は `tmux load-buffer` + `tmux paste-buffer` + `tmux send-keys Enter` で長文プロンプトを送る方式。承認プロンプトが出たら `tmux send-keys` で `1` または `2` を送って解除する。

### チームメンバーとペイン割り当て

| ペインID | 表示名 | ペルソナ | 役割 |
|---|---|---|---|
| %1 | 四国めたん | テストアーキテクト。語尾「〜ですわ」「〜かしら」 | テスト観点抽出(正常系/異常系/境界値/例外系+責務分離指摘) |
| %4 | 冥鳴ひまり | テストケース設計担当。論理的・簡潔 | Given/When/Thenでテストケース設計、ID付与、重複排除 |
| %2 | WhiteCUL | テスト仕様書・テストコード作成担当 | 仕様書清書、Goテストコード(RED phase)生成、結果記録フォーマット作成、議事録記録 |
| %3 | 九州そら | QAレビュアー。敬語・冷静。判定は必ず「判定: ✅承認/⚠要修正/❌差し戻し」形式 | 網羅性・整合性・モック妥当性のレビュー |

※ %0はTeam Lead(このセッション)、%5はclaude-monitor(除外)。

セッションが切れてペインが初期化された場合は、各ペインで`claude`を再起動し、上記ペルソナ設定+現在の進捗状況を再送する(このファイルの内容をそのまま貼ればOK)。

## 進行フロー(1機能ずつ繰り返す)

1. つむぎが対象機能を提示
2. めたんがテスト観点抽出→ドキュメント化(`0X_test_perspectives_phaseN.md`)。仕様の疑問点は「6章」に確認事項としてまとめる
3. つむぎが確認事項を判断・決定→WhiteCULに7章として追記依頼
4. ひまりがGiven/When/Thenでテストケース設計(`0X_test_cases_phaseN.md`)。件数のズレに注意(テーブル行数とサマリー文言を必ず一致させる)。新たな未決事項が出たらつむぎが判断
5. WhiteCULが単体テスト仕様書(`0X_test_specification.md`)+テストコード(RED phase、`tests/unit/`or`tests/integration/`)+結果記録フォーマット追記(`05_test_results.md`)+議事録追記(`00_minutes.md`)
6. そらがQAレビュー→判定。⚠要修正なら該当箇所を修正して再レビュー。✅承認で次フェーズへ

## 進捗状況(2026-07-08時点)

### ✅ フェーズ1(純粋ロジック・バリデーション) — 完了・そら承認済み
対象: 1-1 ULID形式バリデーション、1-2 first_text20文字カット、1-3 role/emotionバリデーション、1-4 入力バリデーション(trim判定)、1-5 GC判定ロジック
成果物: `02_test_perspectives_phase1.md`(仕様決定A〜E)、`03_test_cases_phase1.md`(77件)、`04_test_specification.md`、`tests/unit/`5ファイル(ulid, first_text, role_emotion, input_validation, gc_expiration)

### ✅ フェーズ2(DB連携ロジック) — 完了・そら承認済み
対象: 2-1 POST /conversations、2-2 会話履歴コンテキスト構築、2-3 GET /audio/{ulid}
成果物: `06_test_perspectives_phase2.md`(仕様決定F〜J)、`07_test_cases_phase2.md`(43件)、`08_test_specification.md`、`tests/unit/`3ファイル+`tests/integration/`4ファイル
特記: 1回目レビューで⚠要修正(TC-2-1-11のGC時刻注入問題=Go側`time.Now()`とSQL側`NOW()`のズレで境界値テストが原理的に成立しない)。`Repository.GC(ctx, now time.Time)`にClock注入する設計に修正して再レビュー✅承認。

### ✅ フェーズ3(外部連携・イベント処理) — 完了・そら承認済み
対象: 3-1 LLM応答JSONパース、3-2 SSEイベント送出ロジック、3-3 STT失敗判定・8秒無音タイムアウト判定
成果物: `09_test_perspectives_phase3.md`(仕様決定K〜P)、`10_test_cases_phase3.md`(52件)、`11_test_specification.md`、`tests/unit/`3ファイル(parse_llm_response, response_streamer, stt_judgment)
特記: 1回目レビューで⚠要修正(TC-3-2-12のGiven記述とテストコード実装の不整合、STT閾値0.5の暫定値コメント欠落)。修正して再レビュー✅承認。

**テストケース総数: 172件(フェーズ1:77 + フェーズ2:43 + フェーズ3:52)、`05_test_results.md`に全件「未実施」で記録済み。**

### ✅ フェーズ4(フロントエンドロジック) — 完了・そら承認済み
対象: 4-1 入力モード切替(マイク/テキスト)とlocalStorage保存、4-2 送信ボタンの活性制御、4-3 エラー表現の出し分け(ずんだもん応答バブル vs 3秒トースト)
テスト方針: TypeScript/Svelte側、Vitest + `@testing-library/svelte`使用(フェーズ1〜3はGo+testify)。`localStorage`はjsdom+`vi.stubGlobal`でモック。フロントエンド用スカフォールド(`package.json`・`tsconfig.json`・`vitest.config.ts`・`vitest-setup.ts`)を新規作成
成果物: `12_test_perspectives_phase4.md`(仕様決定T〜Z)、`13_test_cases_phase4.md`(60件、申し送りAA〜CC)、`14_test_specification.md`、`tests/unit/frontend/`3ファイル(input_mode, send_button, error_routing)、`15_qa_review_phase4.md`
特記: 1回目レビューで⚠要修正(①jest-domマッチャーのsetupFiles未登録、②TC-4-1-26が`ModeToggleProps`仕様と責務不整合)。①は`vitest-setup.ts`追加で解消。②は「ModeToggleは薄いコンポーネントとし、入力欄クリアの責務は未設計の親コンポーネント側」とつむぎが判断、TC-4-1-26を欠番化(61件→60件)して再レビュー✅承認。

**テストケース総数: 232件(フェーズ1:77 + フェーズ2:43 + フェーズ3:52 + フェーズ4:60)、`05_test_results.md`に全件「未実施」で記録済み。**

## 全フェーズ完了(2026-07-09時点)

フェーズ1〜4、すべてそらの✅承認を得て完了。単体テスト計画書(`01_test_plan.md`)に定めた全機能のテスト観点抽出・テストケース設計・仕様書清書・テストコード(RED phase)作成が完了した状態。

次のアクション: 各テストケースのGREEN phase実装(実装担当のアサインが必要)、または結合テスト計画の策定(`01_test_plan.md`8章「スコープ外事項」およびひまりの申し送りCCで言及された機能間の状態競合など）。ユーザーの指示を待つ。

## 参照資料
- 要件定義書: `docs/01_requirements_definition/requirements.md`
- 設計書: `docs/02_functional_design/01_screen_design.md`, `02_database_design.md`
- テスト計画: `docs/03_unit_test/01_test_plan.md`
- 議事録: `docs/03_unit_test/00_minutes.md`(追記9まで記録済み)
