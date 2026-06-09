---
trigger: always_on
---

# TypeScript & Svelte コーディング規約

## 目次
1. [TypeScript 規約](#typescript-規約)
2. [Svelte 規約](#svelte-規約)
3. [共通規約](#共通規約)

---

## TypeScript 規約

### 命名規則

#### 変数・関数
- **camelCase** を使用する
- boolean型の変数は `is`, `has`, `should` などの接頭辞を使用する

```typescript
// Good
const userName = 'John';
const isActive = true;
const hasPermission = false;

// Bad
const UserName = 'John';
const active = true;
```

#### クラス・インターフェース・型
- **PascalCase** を使用する
- インターフェースには接頭辞 `I` を付けない
- 型エイリアスには `Type` サフィックスを付けることを推奨

```typescript
// Good
interface User {
  id: number;
  name: string;
}

type UserType = {
  id: number;
  name: string;
};

class UserService {}

// Bad
interface IUser {}
class userService {}
```

#### 定数
- **UPPER_SNAKE_CASE** を使用する

```typescript
const MAX_RETRY_COUNT = 3;
const API_BASE_URL = 'https://api.example.com';
```

### 型定義

#### 明示的な型指定
- 関数の引数と戻り値には必ず型を指定する
- 複雑な型推論が必要な場合は明示的に型を指定する

```typescript
// Good
function calculateTotal(price: number, quantity: number): number {
  return price * quantity;
}

// Bad
function calculateTotal(price, quantity) {
  return price * quantity;
}
```

#### any型の使用禁止
- `any` 型の使用は原則禁止
- どうしても必要な場合は `unknown` を使用し、型ガードで安全に扱う

```typescript
// Good
function processData(data: unknown): void {
  if (typeof data === 'string') {
    console.log(data.toUpperCase());
  }
}

// Bad
function processData(data: any): void {
  console.log(data.toUpperCase());
}
```

#### 型アサーションの使用
- `as` によるアサーションは最小限に抑える
- 型ガードを優先的に使用する

```typescript
// Good
if (typeof value === 'string') {
  console.log(value.toUpperCase());
}

// Bad
console.log((value as string).toUpperCase());
```

### インターフェース vs 型エイリアス

- オブジェクトの形状定義には **インターフェース** を優先
- ユニオン型、インターセクション型、プリミティブ型には **型エイリアス** を使用

```typescript
// Good
interface User {
  id: number;
  name: string;
}

type Status = 'pending' | 'approved' | 'rejected';
type ID = string | number;

// インターフェースの拡張
interface Admin extends User {
  role: string;
}
```

### 配列とオブジェクトの型定義

```typescript
// Good
const numbers: number[] = [1, 2, 3];
const users: User[] = [];

// ジェネリック記法も可（配列の配列などで可読性が上がる場合）
const matrix: Array<Array<number>> = [[1, 2], [3, 4]];

// オブジェクトの型定義
const userMap: Record<string, User> = {};
const config: { [key: string]: string } = {};
```

### 関数

#### アロー関数 vs 通常の関数
- 基本的に **アロー関数** を使用
- メソッド定義やコンストラクタでは通常の関数を使用

```typescript
// Good
const add = (a: number, b: number): number => a + b;

class Calculator {
  multiply(a: number, b: number): number {
    return a * b;
  }
}

// 複数行の場合
const processUser = (user: User): ProcessedUser => {
  const fullName = `${user.firstName} ${user.lastName}`;
  return {
    ...user,
    fullName
  };
};
```

#### オプショナルパラメータとデフォルト値
- デフォルト値がある場合は型推論を活用
- オプショナルパラメータは必須パラメータの後に配置

```typescript
// Good
function greet(name: string, greeting = 'Hello'): string {
  return `${greeting}, ${name}!`;
}

function createUser(name: string, age?: number): User {
  // ...
}

// Bad
function createUser(age?: number, name: string): User {
  // ...
}
```

### 非同期処理

- `async/await` を優先的に使用
- Promise チェーンは複雑になる場合のみ使用

```typescript
// Good
async function fetchUser(id: number): Promise<User> {
  try {
    const response = await fetch(`/api/users/${id}`);
    const user = await response.json();
    return user;
  } catch (error) {
    console.error('Failed to fetch user:', error);
    throw error;
  }
}
```

### Null と Undefined

- `null` よりも `undefined` を優先
- strictNullChecks を有効にする
- オプショナルチェーニング (`?.`) を活用

```typescript
// Good
interface User {
  name: string;
  email?: string; // undefined を許容
}

const userName = user?.profile?.name ?? 'Guest';

// Bad
interface User {
  name: string;
  email: string | null;
}
```

---

## Svelte 規約

### コンポーネント構成

#### ファイル構造
- コンポーネントは **PascalCase** でファイル名を付ける
- 1ファイル1コンポーネントの原則

```
src/
  components/
    UserProfile.svelte
    Button.svelte
    forms/
      LoginForm.svelte
```

#### スクリプト、マークアップ、スタイルの順序

```svelte
<script lang="ts">
  // インポート
  import { onMount } from 'svelte';
  import Button from './Button.svelte';
  
  // Props
  export let userId: number;
  export let showDetails = false;
  
  // State
  let user: User | null = null;
  let isLoading = false;
  
  // リアクティブ宣言
  $: fullName = user ? `${user.firstName} ${user.lastName}` : '';
  
  // 関数
  async function loadUser() {
    isLoading = true;
    user = await fetchUser(userId);
    isLoading = false;
  }
  
  // ライフサイクル
  onMount(() => {
    loadUser();
  });
</script>

<!-- マークアップ -->
<div class="user-profile">
  {#if isLoading}
    <p>Loading...</p>
  {:else if user}
    <h2>{fullName}</h2>
  {/if}
</div>

<!-- スタイル -->
<style>
  .user-profile {
    padding: 1rem;
  }
</style>
```

### Props

#### Props の型定義
- すべての Props に型を定義する
- オプショナルな Props にはデフォルト値を設定

```svelte
<script lang="ts">
  // Good
  export let title: string;
  export let count: number = 0;
  export let isActive: boolean = false;
  export let items: string[] = [];
  
  // オブジェクトの場合
  interface User {
    id: number;
    name: string;
  }
  export let user: User;
</script>
```

#### Props のバリデーション
- 必要に応じてリアクティブステートメントでバリデーション

```svelte
<script lang="ts">
  export let count: number;
  
  $: if (count < 0) {
    console.warn('count should not be negative');
  }
</script>
```

### リアクティブステートメント

#### $: の使用
- シンプルな計算には `$:` を使用
- 複雑なロジックは関数に分離

```svelte
<script lang="ts">
  let firstName = '';
  let lastName = '';
  
  // Good: シンプルな計算
  $: fullName = `${firstName} ${lastName}`.trim();
  
  // Good: 複雑なロジック
  $: processedData = processData(rawData);
  
  function processData(data: RawData): ProcessedData {
    // 複雑な処理
    return result;
  }
  
  // Bad: 複雑な処理を直接記述
  $: complexResult = (() => {
    // 多数の処理...
  })();
</script>
```

### イベントハンドリング

#### イベント定義
- カスタムイベントには型を定義
- `createEventDispatcher` で型安全なイベントを作成

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  
  interface Events {
    submit: { name: string; email: string };
    cancel: never;
  }
  
  const dispatch = createEventDispatcher<Events>();
  
  function handleSubmit() {
    dispatch('submit', { name, email });
  }
</script>

<button on:click={handleSubmit}>Submit</button>
```

#### イベントハンドラの命名
- `handle` または `on` プレフィックスを使用

```svelte
<script lang="ts">
  function handleClick() {
    // ...
  }
  
  function onInputChange(event: Event) {
    // ...
  }
</script>

<button on:click={handleClick}>Click</button>
<input on:input={onInputChange} />
```

### 条件分岐とループ

#### if/else ブロック
- ネストは最小限に抑える
- 早期リターンパターンを活用

```svelte
<!-- Good -->
{#if isLoading}
  <Loader />
{:else if error}
  <Error message={error} />
{:else if data}
  <DataDisplay {data} />
{:else}
  <EmptyState />
{/if}
```

#### each ブロック
- 必ず `key` を指定する
- インデックスよりもユニークなIDを優先

```svelte
<!-- Good -->
{#each items as item (item.id)}
  <ListItem {item} />
{/each}

<!-- Bad -->
{#each items as item, index (index)}
  <ListItem {item} />
{/each}
```

### Store の使用

#### Store の命名
- Writable store: 通常の名詞
- Derived store: 計算を表す名詞
- ファイル名は `stores.ts` または `{機能名}.store.ts`

```typescript
// stores.ts
import { writable, derived } from 'svelte/store';

// Writable store
export const userStore = writable<User | null>(null);
export const cartItems = writable<CartItem[]>([]);

// Derived store
export const cartTotal = derived(
  cartItems,
  ($items) => $items.reduce((sum, item) => sum + item.price, 0)
);
```

#### Store の使用
- コンポーネント内では `$` プレフィックスでアクセス

```svelte
<script lang="ts">
  import { userStore, cartTotal } from './stores';
  
  // 自動購読
  $: userName = $userStore?.name ?? 'Guest';
</script>

<p>User: {userName}</p>
<p>Total: ${$cartTotal}</p>
```

### スタイリング

#### スコープドスタイル
- 基本的にコンポーネントスコープドスタイルを使用
- グローバルスタイルは `:global()` で明示

```svelte
<style>
  /* スコープドスタイル */
  .container {
    padding: 1rem;
  }
  
  /* グローバルスタイル */
  :global(body) {
    margin: 0;
  }
  
  /* 子要素のグローバルスタイル */
  .wrapper :global(.external-class) {
    color: red;
  }
</style>
```

#### クラス名の規則
- BEM記法またはケバブケースを使用
- 一貫性を保つ

```svelte
<div class="user-card">
  <h2 class="user-card__title">{title}</h2>
  <p class="user-card__description">{description}</p>
</div>

<style>
  .user-card {
    border: 1px solid #ccc;
  }
  
  .user-card__title {
    font-size: 1.5rem;
  }
  
  .user-card__description {
    color: #666;
  }
</style>
```

---

## 共通規約

### コードフォーマット

#### インデント
- **2スペース** を使用（タブではなく）

#### 行の長さ
- 1行あたり **80〜100文字** を目安とする

#### セミコロン
- 文末にはセミコロンを使用する

#### クォート
- 文字列には **シングルクォート** を使用
- テンプレートリテラルはバッククォートを使用

```typescript
// Good
const message = 'Hello, World!';
const greeting = `Hello, ${name}!`;

// Bad
const message = "Hello, World!";
```

#### カンマ
- 末尾カンマ（Trailing Comma）を使用

```typescript
const user = {
  name: 'John',
  age: 30,
  email: 'john@example.com', // 末尾カンマ
};
```

### コメント

#### コメントの書き方
- 複雑なロジックには必ずコメントを追加
- 「なぜ」を説明する（「何を」はコードで分かる）

```typescript
// Good
// ユーザーがログインしていない場合はリダイレクト
// （認証トークンの有効期限が切れている可能性があるため）
if (!isAuthenticated) {
  redirect('/login');
}

// Bad
// isAuthenticated が false の場合
if (!isAuthenticated) {
  redirect('/login');
}
```

#### JSDoc
- 公開関数・メソッドには JSDoc を記述

```typescript
/**
 * ユーザー情報を取得する
 * @param userId - ユーザーID
 * @returns ユーザーオブジェクトのPromise
 * @throws {Error} ユーザーが見つからない場合
 */
async function fetchUser(userId: number): Promise<User> {
  // ...
}
```

### エラーハンドリング

- エラーは適切にキャッチして処理する
- ユーザーフレンドリーなエラーメッセージを表示

```typescript
async function saveData(data: FormData): Promise<void> {
  try {
    await api.save(data);
  } catch (error) {
    if (error instanceof ValidationError) {
      showError('入力内容を確認してください');
    } else {
      showError('保存に失敗しました。もう一度お試しください。');
      console.error('Save failed:', error);
    }
  }
}
```

### インポート

#### インポートの順序
1. 外部ライブラリ
2. 内部モジュール
3. 相対パス
4. スタイル・アセット

```typescript
// 1. 外部ライブラリ
import { onMount } from 'svelte';
import { writable } from 'svelte/store';

// 2. 内部モジュール
import { api } from '$lib/api';
import type { User } from '$lib/types';

// 3. 相対パス
import Button from './Button.svelte';
import { formatDate } from '../utils/date';

// 4. スタイル・アセット
import './styles.css';
```

### ファイル・ディレクトリ構造

```
src/
  lib/
    api/
    stores/
    types/
    utils/
  routes/
    +page.svelte
    +layout.svelte
    users/
      +page.svelte
      [id]/
        +page.svelte
  components/
    common/
      Button.svelte
      Input.svelte
    features/
      UserProfile.svelte
```

### ツール設定

#### ESLint
- TypeScript と Svelte 用のルールを設定
- Prettier と併用

#### Prettier
- 自動フォーマットを有効化

#### tsconfig.json
```json
{
  "compilerOptions": {
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true
  }
}
```
