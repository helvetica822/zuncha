export type InputMode = 'voice' | 'text';

export const INPUT_MODE_STORAGE_KEY = 'zuncha:inputMode';

// 無効値・例外時のフォールバックとなる既定の入力モード。
export const DEFAULT_INPUT_MODE: InputMode = 'voice';

// 正規化しない厳密判定。'voice'/'text'のみtrue、それ以外(null/undefined/数値/大文字違い/空)はfalse。
export const isInputMode = (value: unknown): value is InputMode =>
  value === 'voice' || value === 'text';

// localStorageから読み、無効値・例外時は 'voice' にフォールバックする。
export const getStoredInputMode = (): InputMode => {
  try {
    const stored = localStorage.getItem(INPUT_MODE_STORAGE_KEY);
    return isInputMode(stored) ? stored : DEFAULT_INPUT_MODE;
  } catch {
    return DEFAULT_INPUT_MODE;
  }
};

// 保存。例外(QuotaExceededError等)は握り潰して伝播させない。
export const setStoredInputMode = (mode: InputMode): void => {
  try {
    localStorage.setItem(INPUT_MODE_STORAGE_KEY, mode);
  } catch {
    // 永続化失敗は致命的でないため無視する
  }
};
