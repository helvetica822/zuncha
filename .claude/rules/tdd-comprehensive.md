# TDD 詳細ガイドライン

> **配置場所**: `~/.claude/rules/tdd-comprehensive.md`
> このファイルは `CLAUDE.md` の Implementation Policy から参照される。

---

## 目次

1. [TDDの本質](#tddの本質)
2. [TDDサイクル詳細](#tddサイクル詳細)
3. [テスト設計の原則](#テスト設計の原則)
4. [テスト種別ごとの要件チェックリスト](#テスト種別ごとの要件チェックリスト)
5. [エッジケース・カタログ](#エッジケースカタログ)
6. [テストコード記述パターン](#テストコード記述パターン)
7. [よくある失敗と対策](#よくある失敗と対策)
8. [レビューチェックリスト](#レビューチェックリスト)

---

## TDDの本質

**「仕様をテストコードとして記述すること」**

テストは「動作の確認」ではなく「仕様の文書化」である。テストコードを読めば、そのモジュールが何をすべきか・何をすべきでないかが明確にわかる状態を目指す。

```
❌ 誤解: テストは実装後に書くもの（確認作業）
✅ 正解: テストは実装前に書くもの（設計作業）
```

### なぜ先にテストを書くのか

| 理由 | 説明 |
|------|------|
| **仕様の明確化** | テストを書く行為が、曖昧な仕様の矛盾や抜け漏れを露わにする |
| **設計の改善** | テストしにくいコードは設計が悪い。テスト先行で設計が自然に改善される |
| **回帰防止** | 変更のたびに仕様が壊れていないか即座に検証できる |
| **ドキュメント化** | テストコードが生きた仕様書になる |

---

## TDDサイクル詳細

### 🔴 RED — 失敗するテストを書く

**目的**: 実装すべき仕様を、具体的な値で表現する。

```
やること:
- 1つの振る舞いに対して1つのテストを書く
- 期待値は具体的な値を使う（"正しく表示される" ではなく expect(width).toBe(48)）
- テストが失敗することを必ず確認する（偽陽性の排除）
- エッジケースも最初から計画する（後回しにしない）

やらないこと:
- 複数の振る舞いを1テストに詰め込む
- 実装を先に書いてからテストを合わせる
- 「だいたい動く」を確認するだけのテスト
```

**失敗確認の意味**: テストが最初から通ってしまう場合、テスト自体が間違っているか、実装が既に存在している。必ず赤を確認してから次へ。

---

### 🟢 GREEN — 最小限のコードでテストをパス

**目的**: テストを通すだけのコードを、できるだけシンプルに書く。

```
やること:
- テストをパスするための最小限の実装のみ
- ハードコードでも構わない（次のステップで改善する）
- "動く" 状態を素早く作る

やらないこと:
- 先回りして汎用的・最適なコードを書く
- まだテストのない機能を実装する
- リファクタリングをこのフェーズでやる
```

---

### 🔵 REFACTOR — コードを改善する

**目的**: テストが通ったまま、コードの質を上げる。

```
やること:
- マジックナンバーを定数化する
- 命名を意図が伝わるものにする
- 重複を排除する
- 責務を適切に分離する

確認事項:
- すべてのテストが引き続きパスしていること
- テストコード自体も読みやすいか（テストのリファクタリングも対象）
```

---

### 💡 FEEDBACK — 設計への気づきを反映

**目的**: テストを書いたことで気づいた設計上の問題を記録・対処する。

```
問いかけ:
- このテストを書くのが辛かった場合、なぜか？（依存が多い、責務が大きいなど）
- モックが多すぎないか？（結合度が高いサイン）
- テスト名が長くなりすぎていないか？（振る舞いが複雑すぎるサイン）
- 同じセットアップが繰り返されていないか？（共通の前提を抽出すべきサイン）
```

---

## テスト設計の原則

### F.I.R.S.T 原則

| 原則 | 意味 | 実践 |
|------|------|------|
| **Fast** | 高速 | 1テスト < 100ms を目安。外部依存はモック化 |
| **Isolated** | 独立 | テスト間で状態を共有しない。順序依存NG |
| **Repeatable** | 再現可能 | 環境・時刻・乱数に依存しない |
| **Self-Validating** | 自己検証 | PASS/FAIL が自動で判定される。目視確認不要 |
| **Timely** | タイムリー | 実装の直前に書く。後から書くのは TDD ではない |

---

### テスト粒度の選び方

```
Unit Test（単体テスト）
  対象: 関数・クラス・コンポーネント単体
  速度: 高速
  用途: ロジック・計算・変換・状態管理

Integration Test（結合テスト）
  対象: 複数モジュールの連携
  速度: 中速
  用途: API呼び出し・DB操作・コンポーネント間連携

E2E Test（エンドツーエンド）
  対象: ユーザー操作のシナリオ全体
  速度: 低速
  用途: クリティカルなユーザーフロー

割合の目安（テストピラミッド）:
  Unit      : 70%
  Integration: 20%
  E2E        : 10%
```

---

## テスト種別ごとの要件チェックリスト

### ✅ 正常系

- [ ] **具体的な値を検証する**
  ```typescript
  // ❌ 曖昧
  expect(button).toBeVisible();

  // ✅ 具体的
  expect(button.style.width).toBe('48px');
  expect(button.style.backgroundColor).toBe('#1a73e8');
  expect(button.textContent).toBe('送信');
  ```

- [ ] **仕様の数値・色・フォント・間隔をそのままテスト化**
  ```typescript
  // デザイン仕様が「アイコンサイズ 24px、余白 16px」なら
  expect(icon.getAttribute('width')).toBe('24');
  expect(container.style.padding).toBe('16px');
  ```

- [ ] **デフォルト値の確認**
  ```typescript
  // 引数なしで生成したとき、期待されるデフォルト状態を検証
  const button = new Button();
  expect(button.variant).toBe('primary');
  expect(button.size).toBe('medium');
  expect(button.disabled).toBe(false);
  ```

- [ ] **戻り値の型・構造まで検証**
  ```typescript
  const result = parseUser(raw);
  expect(result).toEqual({
    id: 'u-001',
    name: '大橋太郎',
    role: 'admin',
    createdAt: expect.any(Date),
  });
  ```

---

### ✅ エッジケース（詳細は次章）

- [ ] 空の値（空文字列 `''`、`undefined`、`null`、`0`、`[]`、`{}`）
- [ ] 非常に長い値（1000文字、10000件など）
- [ ] 境界値（最小値、最大値、その前後）
- [ ] 矛盾する状態の組み合わせ（`loading + disabled`、`error + success` など）
- [ ] 連続操作・二重送信
- [ ] タイムアウト・遅延レスポンス

---

### ✅ 異常系

- [ ] **無効な入力に対してエラーを投げるか**
  ```typescript
  expect(() => parseDate('not-a-date')).toThrow(InvalidDateError);
  expect(() => parseDate('not-a-date')).toThrow('日付の形式が不正です');
  ```

- [ ] **エラー時の状態変化を検証**
  ```typescript
  await act(() => submitForm(invalidData));
  expect(screen.getByRole('alert')).toHaveTextContent('入力内容を確認してください');
  expect(form.querySelector('[aria-invalid="true"]')).not.toBeNull();
  ```

- [ ] **エラー後の回復動線**
  ```typescript
  // エラー後に正しい値を入力すると、エラーが解消されること
  await userEvent.type(input, 'invalid');
  await userEvent.clear(input);
  await userEvent.type(input, 'valid@email.com');
  expect(screen.queryByRole('alert')).toBeNull();
  ```

---

### ✅ 副作用・非同期

- [ ] **APIコールの引数を検証**
  ```typescript
  await submitOrder(order);
  expect(mockApiClient.post).toHaveBeenCalledWith('/orders', {
    items: order.items,
    userId: 'u-001',
    // timestamp は除外（テストごとに変わるため）
  });
  ```

- [ ] **コールバック・イベントの発火タイミング**
  ```typescript
  const onSuccess = jest.fn();
  await submitForm(validData, { onSuccess });
  expect(onSuccess).toHaveBeenCalledTimes(1);
  expect(onSuccess).toHaveBeenCalledWith(expect.objectContaining({ id: expect.any(String) }));
  ```

- [ ] **状態の連鎖変化（ローディング → 完了 → リセット）**
  ```typescript
  const { result } = renderHook(() => useSubmit());

  act(() => result.current.submit(data));
  expect(result.current.status).toBe('loading');

  await waitFor(() => expect(result.current.status).toBe('success'));
  expect(result.current.data).toEqual(expectedData);
  ```

---

## エッジケース・カタログ

### 文字列

| ケース | 値の例 | 確認ポイント |
|--------|--------|-------------|
| 空文字列 | `''` | クラッシュしないか、デフォルト表示になるか |
| 空白のみ | `'   '` | trim されるか、バリデーションエラーになるか |
| 非常に長い | `'a'.repeat(1000)` | レイアウト崩れ、切り捨て、エラーになるか |
| 特殊文字 | `'<script>alert(1)</script>'` | エスケープされるか |
| 絵文字 | `'😀🎉'` | 文字数カウント、表示が正しいか |
| 多言語 | `'日本語テスト'`、`'مرحبا'` | RTL 対応、エンコードの問題 |
| 改行含む | `'line1\nline2'` | 改行が保持・除去・変換されるか |

### 数値

| ケース | 値の例 | 確認ポイント |
|--------|--------|-------------|
| ゼロ | `0` | 除算・パーセント計算でクラッシュしないか |
| 負の値 | `-1` | バリデーション、表示が正しいか |
| 最大値 | `Number.MAX_SAFE_INTEGER` | オーバーフローしないか |
| 小数 | `0.1 + 0.2` | 浮動小数点誤差の扱い |
| NaN | `NaN` | フォールバックが機能するか |
| Infinity | `Infinity` | 表示・計算がクラッシュしないか |

### 配列・コレクション

| ケース | 値の例 | 確認ポイント |
|--------|--------|-------------|
| 空配列 | `[]` | 「データなし」表示になるか |
| 1件 | `[item]` | 複数件前提のロジックが破綻しないか |
| 重複あり | `[a, a, b]` | 重複の扱いが仕様通りか |
| 大量件数 | 10,000件 | パフォーマンス、メモリ、表示 |
| undefined 含む | `[1, undefined, 3]` | フィルタ・マップ処理でクラッシュしないか |

### 状態の組み合わせ

```typescript
// UI コンポーネントで起きがちな矛盾状態の組み合わせ
describe('Button 状態の組み合わせ', () => {
  it('loading 中は disabled 状態と同様に動作する', () => { ... });
  it('disabled かつ loading の場合、loading アイコンは表示されない', () => { ... });
  it('error かつ loading の場合、error 表示が優先される', () => { ... });
});
```

### 連続操作・競合

```typescript
it('ボタンを素早く2回クリックしても、APIは1回しか呼ばれない', async () => {
  await userEvent.dblClick(submitButton);
  expect(mockApi.post).toHaveBeenCalledTimes(1);
});

it('前のリクエストが完了する前に新しいリクエストが来た場合、前者はキャンセルされる', async () => {
  const first = search('abc');
  const second = search('abcd');
  await Promise.all([first, second]);
  expect(mockApi.get).toHaveBeenCalledTimes(2);
  // 表示されるのは最新リクエストの結果のみ
  expect(screen.getAllByRole('listitem')).toHaveLength(secondResults.length);
});
```

---

## テストコード記述パターン

### Arrange-Act-Assert（AAA）

```typescript
it('ユーザーが送信ボタンを押すと、フォームデータが API に送信される', async () => {
  // Arrange（準備）
  const mockPost = jest.fn().mockResolvedValue({ id: 'order-001' });
  render(<OrderForm onSubmit={mockPost} />);

  await userEvent.type(screen.getByLabelText('商品名'), 'ノートPC');
  await userEvent.type(screen.getByLabelText('数量'), '2');

  // Act（実行）
  await userEvent.click(screen.getByRole('button', { name: '注文する' }));

  // Assert（検証）
  expect(mockPost).toHaveBeenCalledWith({
    productName: 'ノートPC',
    quantity: 2,
  });
});
```

### Given-When-Then（振る舞い記述）

```typescript
describe('Given: ユーザーが認証済みの場合', () => {
  describe('When: ダッシュボードにアクセスする', () => {
    it('Then: ユーザー名が表示される', () => { ... });
    it('Then: ログアウトボタンが表示される', () => { ... });
  });

  describe('When: ログアウトボタンを押す', () => {
    it('Then: ログインページにリダイレクトされる', () => { ... });
    it('Then: セッションが削除される', () => { ... });
  });
});
```

### テスト名の書き方

```typescript
// ❌ 何を確認しているかわからない
it('works correctly');
it('renders');
it('test1');

// ❌ 実装に依存した名前（実装が変わるとテスト名が嘘になる）
it('useState を使って disabled 状態を管理する');

// ✅ 振る舞いで命名（誰が・何をしたとき・どうなるか）
it('送信ボタンは、フォームが空のとき無効化される');
it('エラーメッセージは、入力を修正すると消える');
it('ローディング中は、送信ボタンのテキストが「送信中...」になる');
```

### パラメータ化テスト

```typescript
// 複数のケースを同じロジックで検証する
describe.each([
  { input: '',        expected: 'メールアドレスを入力してください' },
  { input: 'abc',     expected: 'メールアドレスの形式が正しくありません' },
  { input: 'a@b',     expected: 'メールアドレスの形式が正しくありません' },
  { input: 'a@b.com', expected: null },
])('validateEmail($input)', ({ input, expected }) => {
  it(`エラーメッセージが "${expected ?? 'なし'}" であること`, () => {
    expect(validateEmail(input)).toBe(expected);
  });
});
```

---

## よくある失敗と対策

### ❌ 実装の詳細をテストしている

```typescript
// ❌ 内部状態（useState の値）を直接検証
expect(component.state('isOpen')).toBe(true);

// ✅ ユーザーが見る振る舞いを検証
expect(screen.getByRole('dialog')).toBeVisible();
```

### ❌ テストが実装に依存して壊れやすい

```typescript
// ❌ DOM 構造に依存（リファクタリングで壊れる）
expect(wrapper.find('div').at(2).find('span').text()).toBe('送信');

// ✅ セマンティクス・ロールで検証（リファクタリングに強い）
expect(screen.getByRole('button', { name: '送信' })).toBeInTheDocument();
```

### ❌ テストが遅い（実際の時間に依存）

```typescript
// ❌ 実際に3秒待つ
await new Promise(r => setTimeout(r, 3000));
expect(isExpired(token)).toBe(true);

// ✅ 時刻をモックする
jest.useFakeTimers();
jest.setSystemTime(new Date('2026-01-01'));
expect(isExpired(token)).toBe(true);
jest.useRealTimers();
```

### ❌ 1テストで複数の振る舞いを検証

```typescript
// ❌ 何が失敗したのかわからない
it('フォームが正しく動作する', () => {
  // バリデーションのチェック
  expect(validate('')).toBe(false);
  // 送信のチェック
  expect(mockApi).toHaveBeenCalled();
  // リセットのチェック
  expect(input.value).toBe('');
});

// ✅ 1つのテストで1つの振る舞い
it('空のメールアドレスはバリデーションエラーになる', () => { ... });
it('有効な入力で送信すると API が呼ばれる', () => { ... });
it('送信後にフォームがリセットされる', () => { ... });
```

### ❌ モックしすぎて何もテストしていない

```typescript
// ❌ モックだらけで実質的なテストがない
jest.mock('../api');
jest.mock('../validator');
jest.mock('../formatter');
// すべてモックされているので、実際の統合動作は未検証
```

### ❌ テストのセットアップが重複している

```typescript
// ❌ 各テストで同じセットアップ
it('test1', () => {
  const user = createUser({ name: '大橋' });
  const session = createSession(user);
  // ...
});
it('test2', () => {
  const user = createUser({ name: '大橋' }); // 重複
  const session = createSession(user);       // 重複
  // ...
});

// ✅ beforeEach でまとめる
let user: User;
let session: Session;
beforeEach(() => {
  user = createUser({ name: '大橋' });
  session = createSession(user);
});
```

---

## レビューチェックリスト

### テストを書く前

- [ ] 何の仕様をテストするか言語化できているか
- [ ] エッジケースを洗い出したか
- [ ] テスト名（振る舞いの説明）を先に書いたか

### テストを書いた後（RED確認）

- [ ] テストが実際に失敗することを確認したか
- [ ] 失敗メッセージは何が問題かを明確に示しているか

### 実装後（GREEN確認）

- [ ] すべてのテストがパスするか
- [ ] テストをパスするための余分な実装をしていないか

### リファクタリング後

- [ ] すべてのテストが引き続きパスするか
- [ ] テストコード自体も読みやすいか
- [ ] テスト名と実際のテスト内容が一致しているか

### 全体レビュー

- [ ] 「どのように動作するか」が具体的な値で検証されているか
- [ ] エッジケースが少なくとも3つ以上カバーされているか
- [ ] 異常系・エラーパスのテストがあるか
- [ ] テストが独立していて（他のテストに依存せず）単独で実行できるか
- [ ] テストが外部環境（時刻・ランダム・ネットワーク）に依存していないか

---

## 参考：テストファイルのディレクトリ構成

```
src/
├── components/
│   ├── Button/
│   │   ├── Button.tsx
│   │   ├── Button.test.tsx       # 単体テスト（同階層に置く）
│   │   └── Button.stories.tsx
├── hooks/
│   ├── useSubmit.ts
│   └── useSubmit.test.ts
├── utils/
│   ├── validator.ts
│   └── validator.test.ts
└── __tests__/
    └── integration/
        └── orderFlow.test.tsx    # 結合テスト（シナリオ単位）
```

---

*最終更新: 2026-03 / このファイルは `CLAUDE.md` の Implementation Policy と連動して機能します。*
