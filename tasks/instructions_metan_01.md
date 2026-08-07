# めたんへの依頼: GREEN phase 実装方針・タスク分割の策定

依頼者: 春日部つむぎ (PM)
依頼日: 2026-07-17

## 背景

- 本プロジェクトはTDDで進行中。RED phase(テストコード作成)は完了済み。
- テストケース総数232件(フェーズ1:77 / フェーズ2:43 / フェーズ3:52 / フェーズ4:60)、すべて「未実施」。
- テストコード: バックエンドは `tests/unit/`・`tests/integration/`(Go+testify)、フロントエンドは `tests/unit/frontend/`(Vitest+@testing-library/svelte)。
- これからGREEN phase(プロダクションコード実装)に入る。今回の依頼は**実装計画の策定のみ**。実装はまだ開始しない。

## 読むべき資料

1. `docs/01_requirements_definition/requirements.md` — 要件定義
2. `docs/02_functional_design/01_screen_design.md` — 画面設計
3. `docs/02_functional_design/02_database_design.md` — DB設計
4. `docs/03_unit_test/01_test_plan.md` — テスト計画(対象機能一覧)
5. `docs/03_unit_test/04_test_specification.md`, `08_...`, `11_...`, `14_...` — 各フェーズのテスト仕様書(実装対象のインターフェース・型・関数シグネチャがここに定義されているはず)
6. 実際のテストコード(`tests/` 配下) — 実装が満たすべきインポートパス・パッケージ構成の確認
7. `tasks/lessons.md` — 過去の教訓(特にClock注入の設計決定など)

## 成果物

`docs/04_implementation/01_implementation_plan.md` を新規作成し、以下を記載すること。

### 1. 実装方針
- バックエンド(Go)のレイヤー構成・パッケージ構成(`src/backend/` 配下のディレクトリ設計)
- フロントエンド(Svelte/TS)の構成(`src/frontend/` 配下のディレクトリ設計)
- テストコードが期待するインポートパス/モジュール解決との整合方法(go.mod のmodule名、vitest のalias等)
- 使用ライブラリとバージョン方針
- コーディング規約は `.claude/rules/golang-coding-guideline.md`・`ts-svelte-coding-guideline.md` に準拠する旨

### 2. タスク分割表
各タスクに以下を含めること:
- タスクID(例: T-01)
- タスク名・作業内容
- 対象ディレクトリ/ファイル
- 対応するテストケース群(フェーズ・機能ID単位でOK。例: フェーズ1の1-1 ULIDバリデーション 等)
- 依存関係(先行タスクのID)
- 推奨実装順序と理由
- 規模感(S/M/L)

### 3. リスク・懸念事項
- 技術的リスク、仕様の曖昧点、事前に決めておくべき事項があれば列挙

## 制約

- このタスクでは `docs/04_implementation/01_implementation_plan.md` 以外のファイルを作成・変更しないこと(src配下への実装は禁止)。
- 完成したらファイル末尾に一行 `<!-- PLAN_COMPLETE -->` と書くこと(つむぎが完成検知に使う)。
- 完成後、つむぎへSendMessageで完了報告(200文字以内)をすること。
