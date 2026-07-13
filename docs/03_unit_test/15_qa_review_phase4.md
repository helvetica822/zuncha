# フェーズ4 QAレビュー結果

| 項目 | 内容 |
|------|------|
| バージョン | 1.1 |
| 作成日 | 2026-07-09 |
| 更新日 | 2026-07-09（再レビュー結果を追記） |
| 作成者 | そら（QAレビュアー） |
| 対象 | `12_test_perspectives_phase4.md`、`13_test_cases_phase4.md`、`14_test_specification.md`、`tests/unit/frontend/`配下3ファイル、`package.json`・`tsconfig.json`・`vitest.config.ts`、`05_test_results.md` |
| 次工程 | （初回）WhiteCULによる指摘①・②の修正 → そらによる再レビュー（本追記で完了） |

---

## 判定（初回レビュー、2026-07-09）

**⚠要修正**

設計全体の方向性（責務分離・モック方針・境界値網羅）は妥当であり、大きな手戻りは不要と判断していますが、以下2点は修正のうえ再レビューをお願いします。

---

## 指摘①：`@testing-library/jest-dom`のマッチャーが未登録で、コンポーネント層のテストがGREEN phase後も実行不能

### 該当ファイル

- `vitest.config.ts`
- `package.json`
- `tests/unit/frontend/input_mode.test.ts`
- `tests/unit/frontend/send_button.test.ts`
- `tests/unit/frontend/error_routing.test.ts`

### 該当箇所

以下のカスタムマッチャー呼び出し（計15箇所前後）が対象。

- `input_mode.test.ts`
  - `toHaveAttribute('aria-pressed', ...)`（TC-4-1-21・22・23）
  - `toBeDisabled()`（TC-4-1-24・25）
  - `toHaveValue('')`（TC-4-1-26）
- `send_button.test.ts`
  - `not.toBeDisabled()`（TC-4-2-17）
  - `toBeDisabled()`（TC-4-2-18）
- `error_routing.test.ts`
  - `toHaveTextContent(...)`（TC-4-3-08・13）
  - `toBeInTheDocument()`（TC-4-3-11・15）

### 問題点

`package.json`には`@testing-library/jest-dom`が`devDependencies`として記載されているが、これをVitestの`expect`に実際に登録する処理がどこにも存在しない。

- `vitest.config.ts`の`test`ブロックに`setupFiles`の指定がない
- 3つのテストファイルのいずれにも`import '@testing-library/jest-dom'`（または`@testing-library/jest-dom/vitest`）の記述がない

このままでは、GREEN phaseで`src/lib`・`src/components`の実装が完了した後でも、上記マッチャー呼び出しが「そのようなメソッドは存在しない」旨の実行時エラーで失敗する。これはRED phaseが本来意図している「未実装によるインポートエラー」とは異なる種類の失敗であり、**実装が正しくてもテストが機能しない**、スカフォールド自体の欠陥である。

### 推奨対応

1. リポジトリルートに`vitest-setup.ts`を新規作成し、以下を記述する。
   ```typescript
   import '@testing-library/jest-dom/vitest';
   ```
2. `vitest.config.ts`の`test`ブロックに`setupFiles`を追加する。
   ```typescript
   test: {
     environment: 'jsdom',
     include: ['tests/unit/frontend/**/*.test.ts'],
     globals: true,
     setupFiles: ['./vitest-setup.ts'],
   },
   ```

---

## 指摘②：TC-4-1-26が`ModeToggle`のProps仕様（`14_test_specification.md` §3.4）と矛盾している

### 該当ファイル

- `docs/03_unit_test/14_test_specification.md`（§3.4 `ModeToggleProps`定義）
- `tests/unit/frontend/input_mode.test.ts`（TC-4-1-26、229〜241行目付近）

### 該当箇所

`14_test_specification.md` §3.4の`ModeToggleProps`インターフェース定義：

```typescript
interface ModeToggleProps {
  mode: InputMode;
  isRecording: boolean;
  isTranscribing: boolean;
  onModeChange: (mode: InputMode) => void; // 変更時、旧モードのテキスト入力欄をクリアする責務もここで担う（7章U確定）
}
```

テキスト入力欄に関するpropは`mode`／`isRecording`／`isTranscribing`／`onModeChange`のいずれにも存在しない。

一方、`input_mode.test.ts`のTC-4-1-26：

```typescript
render(ModeToggle, {
  mode: 'text' as InputMode,
  isRecording: false,
  isTranscribing: false,
  onModeChange: vi.fn(),
  text: 'こんにちは',   // ← Props定義に存在しないprop
});

await fireEvent.click(screen.getByRole('button', { name: 'マイク' }));

expect(screen.getByRole('textbox')).toHaveValue('');  // ← ModeToggle自身がtextboxを持つ前提
```

### 問題点

このテストは、`ModeToggle`コンポーネント単体に未定義の`text`propを渡し、かつ`ModeToggle`自身がテキスト入力欄（`textbox`ロール）を内包していることを前提にしている。

しかし§3.4のProps定義にはそのような入出力の受け皿が一切なく、コメント「変更時、旧モードのテキスト入力欄をクリアする責務もここで担う」は`onModeChange`コールバック側（＝呼び出し元の親コンポーネント）の責務と読むのが自然である。

つまり「入力欄クリア」の責務が`ModeToggle`自身にあるのか、それを利用する親コンポーネントにあるのか、Props仕様書とテストコードの間で解釈が割れている。この不整合を残したままGREEN phaseの実装に着手すると、実装者が本来スコープ外であるはずの入力欄を`ModeToggle.svelte`に持たせてしまい、責務分離（純粋関数層・薄いコンポーネント層という設計方針）が崩れる恐れがある。

### 推奨対応

次のいずれかで解消する。

1. `ModeToggleProps`に`text`・`onTextChange`相当のpropを正式に追加し、仕様とテストコードを一致させる。
2. あるいはTC-4-1-26を「`ModeToggle`と入力欄を持つ親コンポーネント（現状未設計）」を組み合わせた結合的なテストとして別ファイル・別仕様に切り出し、`ModeToggle`単体のテストからは外す。

いずれを採用するかは、つむぎ・WhiteCULで責務の所在を確定させたうえで反映すること。

---

## 設計全体（責務分離・モック方針・境界値網羅）への所感

- **責務分離**: 純粋関数層（`isInputMode`・`getStoredInputMode`／`setStoredInputMode`・`isSendButtonDisabled`・`routeSSEEvent`）とコンポーネント層の分離方針は明確であり、異常系・境界値を純粋関数層に厚く配置し、コンポーネント層は結線確認に絞るというテストピラミッドの比率も適切に守られている。指摘②はこの方針自体の問題ではなく、`ModeToggle`一箇所における仕様書とテストコードの記述の齟齬にとどまる。
- **モック方針**: `localStorage`の`vi.stubGlobal`による完全置換（7章Z決定）は全テストで一貫して守られており、キー・値のスパイ検証も適切。過剰なモック化・モック不足のいずれも見られない。
- **境界値網羅**: trim対象文字種（半角・タブ・改行・全角スペース・混在）、1,000文字での上限なし確認、トースト3秒消滅の2999ms/3000ms境界など、`tdd-comprehensive.md`のエッジケース・カタログに沿った網羅ができている。
- **`vi.useFakeTimers()`の決定性**: `describe('Toast')`ブロック内で`beforeEach`／`afterEach`によりフェイクタイマーのON/OFFが適切にスコープされており、他のテスト（`routeSSEEvent`・`MessageBubble`）に影響を与えない設計になっている。ただしTC-4-3-10（アンマウント時のタイマークリア確認）は`console.error`が呼ばれないことを根拠にしているが、Svelteはdestroyed componentへの後続更新に対してReactのような警告を標準では出さないため、タイマー未クリアの実装でも本テストが偽陽性でパスする可能性がある。必須の修正ではないが、`vi.spyOn(global, 'clearTimeout')`等でタイマークリア自体を直接検証する形への改善を推奨する（次回以降の申し送り事項として記録）。

以上、指摘①・②の修正完了後、再レビューをお願いします。

---

## 再レビュー結果（2026-07-09）

### 判定

**✅承認**

### 確認した修正内容

**指摘①への対応（確認OK）**
- リポジトリルートに`vitest-setup.ts`が新規作成され、`import '@testing-library/jest-dom/vitest';`が記述されていることを確認した。
- `vitest.config.ts`の`test`ブロックに`setupFiles: ['./vitest-setup.ts']`が追加されていることを確認した。
- `@testing-library/jest-dom`はpackage.json記載のバージョン（`^6.4.0`）で`/vitest`サブパスエクスポートに対応しており、整合性の問題はない。これにより`toBeInTheDocument`・`toBeDisabled`・`toHaveAttribute`・`toHaveTextContent`・`toHaveValue`等のマッチャーがGREEN phase後に正しく機能する状態になった。

**指摘②への対応（確認OK）**
- `input_mode.test.ts`からTC-4-1-26が削除され、該当箇所（229〜233行目）に欠番の理由と「入力欄クリアは親コンポーネント側の責務」である旨のコメントが残されていることを確認した。ファイル冒頭の対応仕様コメントも「TC-4-1-01〜27、TC-4-1-26は欠番」に更新されている。
- `14_test_specification.md` §3.4の`ModeToggleProps`定義に、「`ModeToggle`自身はテキスト入力欄を持たない」「クリア責務は親コンポーネント（未設計）側にある」という責務の所在を明記する注記が追加されていることを確認した。§4.1.3の該当行も取り消し線付きで欠番として明示されている。
- `13_test_cases_phase4.md`にも同様の注記・取り消し線があり、フェーズ4の実施対象件数が61件→**60件**（4-1: 26件）に整合的に更新されていることを確認した。
- `05_test_results.md`のフェーズ4集計・総合集計が60件／**232件**に正しく修正されていることを確認した。
- 「モード切替時に旧モードの入力をクリアする」という仕様（7章U確定）自体は取り下げられておらず、将来の親コンポーネント設計時に別途テストケース化する旨が申し送りとして残っている点も適切である。

**TC-4-3-10の改善提案について**
- 今回対応せず`00_minutes.md`議題11への申し送りとした判断は妥当。`vi.spyOn(clearTimeout)`への変更は必須の指摘ではなく、次フェーズ以降の改善提案として扱って問題ない。

### 総合所感

指摘①・②とも該当ファイルの修正内容を実際に確認し、ドキュメント間（`13_test_cases_phase4.md`・`14_test_specification.md`・`05_test_results.md`・テストコード）の件数・記述に矛盾がないことを突合済み。責務分離・モック方針・境界値網羅を含めフェーズ4全体として問題なく、承認します。

*記録: そら（2026-07-09、再レビュー）*
