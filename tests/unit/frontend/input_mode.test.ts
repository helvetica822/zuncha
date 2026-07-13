// 対応仕様: docs/03_unit_test/14_test_specification.md 4.1（観点4-1、TC-4-1-01〜27、TC-4-1-26は欠番）
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';

import {
  isInputMode,
  getStoredInputMode,
  setStoredInputMode,
  INPUT_MODE_STORAGE_KEY,
} from '../../../src/lib/inputMode';
import type { InputMode } from '../../../src/lib/inputMode';
import ModeToggle from '../../../src/components/ModeToggle.svelte';

function createMockStorage(overrides: Partial<Storage> = {}): Storage {
  return {
    getItem: vi.fn().mockReturnValue(null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
    key: vi.fn(),
    length: 0,
    ...overrides,
  } as unknown as Storage;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('isInputMode', () => {
  it('TC-4-1-01: "voice"はtrueを返す', () => {
    expect(isInputMode('voice')).toBe(true);
  });

  it('TC-4-1-02: "text"はtrueを返す', () => {
    expect(isInputMode('text')).toBe(true);
  });

  it('TC-4-1-03: Union型にない文字列はfalseを返す', () => {
    expect(isInputMode('foo')).toBe(false);
  });

  it('TC-4-1-04: 数字文字列はfalseを返す', () => {
    expect(isInputMode('1')).toBe(false);
  });

  it('TC-4-1-05: 空文字列はfalseを返す', () => {
    expect(isInputMode('')).toBe(false);
  });

  it('TC-4-1-06: nullはfalseを返す', () => {
    expect(isInputMode(null)).toBe(false);
  });

  it('TC-4-1-07: undefinedはfalseを返す', () => {
    expect(isInputMode(undefined)).toBe(false);
  });

  it('TC-4-1-08: 数値型はfalseを返す', () => {
    expect(isInputMode(123)).toBe(false);
  });

  it('TC-4-1-09: 大文字小文字違いはfalseを返す（正規化しない）', () => {
    expect(isInputMode('Voice')).toBe(false);
  });

  it('TC-4-1-10: 大文字小文字違い（TEXT）はfalseを返す', () => {
    expect(isInputMode('TEXT')).toBe(false);
  });
});

describe('getStoredInputMode', () => {
  it('TC-4-1-11: キー未存在時はvoiceを返す', () => {
    const mockStorage = createMockStorage({ getItem: vi.fn().mockReturnValue(null) });
    vi.stubGlobal('localStorage', mockStorage);

    expect(getStoredInputMode()).toBe('voice');
  });

  it('TC-4-1-12: 保存値がtextならtextを返す', () => {
    const mockStorage = createMockStorage({ getItem: vi.fn().mockReturnValue('text') });
    vi.stubGlobal('localStorage', mockStorage);

    expect(getStoredInputMode()).toBe('text');
  });

  it('TC-4-1-13: 保存値がvoiceならvoiceを返す', () => {
    const mockStorage = createMockStorage({ getItem: vi.fn().mockReturnValue('voice') });
    vi.stubGlobal('localStorage', mockStorage);

    expect(getStoredInputMode()).toBe('voice');
  });

  it('TC-4-1-16: 無効値はvoiceにフォールバックする', () => {
    const mockStorage = createMockStorage({ getItem: vi.fn().mockReturnValue('foo') });
    vi.stubGlobal('localStorage', mockStorage);

    expect(getStoredInputMode()).toBe('voice');
  });

  it('TC-4-1-17: 空文字列はvoiceにフォールバックする', () => {
    const mockStorage = createMockStorage({ getItem: vi.fn().mockReturnValue('') });
    vi.stubGlobal('localStorage', mockStorage);

    expect(getStoredInputMode()).toBe('voice');
  });

  it('TC-4-1-18: getItemが例外を投げてもvoiceを返す', () => {
    const mockStorage = createMockStorage({
      getItem: vi.fn().mockImplementation(() => {
        throw new Error('SecurityError');
      }),
    });
    vi.stubGlobal('localStorage', mockStorage);

    expect(() => getStoredInputMode()).not.toThrow();
    expect(getStoredInputMode()).toBe('voice');
  });

  it('TC-4-1-20: 大文字小文字違いもvoiceにフォールバックする（正規化しない）', () => {
    const mockStorage = createMockStorage({ getItem: vi.fn().mockReturnValue('Voice') });
    vi.stubGlobal('localStorage', mockStorage);

    expect(getStoredInputMode()).toBe('voice');
  });
});

describe('setStoredInputMode', () => {
  it('TC-4-1-14: textを渡すとsetItemがキーと値で呼ばれる', () => {
    const mockStorage = createMockStorage();
    vi.stubGlobal('localStorage', mockStorage);

    setStoredInputMode('text');

    expect(mockStorage.setItem).toHaveBeenCalledWith(INPUT_MODE_STORAGE_KEY, 'text');
  });

  it('TC-4-1-15: voiceを渡すとsetItemがキーと値で呼ばれる', () => {
    const mockStorage = createMockStorage();
    vi.stubGlobal('localStorage', mockStorage);

    setStoredInputMode('voice');

    expect(mockStorage.setItem).toHaveBeenCalledWith(INPUT_MODE_STORAGE_KEY, 'voice');
  });

  it('TC-4-1-19: setItemが例外を投げても伝播しない', () => {
    const mockStorage = createMockStorage({
      setItem: vi.fn().mockImplementation(() => {
        throw new Error('QuotaExceededError');
      }),
    });
    vi.stubGlobal('localStorage', mockStorage);

    expect(() => setStoredInputMode('text')).not.toThrow();
  });
});

describe('ModeToggle', () => {
  it('TC-4-1-21: テキストをクリックするとtextで変更コールバックが呼ばれる', async () => {
    const onModeChange = vi.fn();
    render(ModeToggle, {
      mode: 'voice' as InputMode,
      isRecording: false,
      isTranscribing: false,
      onModeChange,
    });

    await fireEvent.click(screen.getByRole('button', { name: 'テキスト' }));

    expect(onModeChange).toHaveBeenCalledWith('text');
    expect(screen.getByRole('button', { name: 'テキスト' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('TC-4-1-22: マイクをクリックするとvoiceで変更コールバックが呼ばれる', async () => {
    const onModeChange = vi.fn();
    render(ModeToggle, {
      mode: 'text' as InputMode,
      isRecording: false,
      isTranscribing: false,
      onModeChange,
    });

    await fireEvent.click(screen.getByRole('button', { name: 'マイク' }));

    expect(onModeChange).toHaveBeenCalledWith('voice');
    expect(screen.getByRole('button', { name: 'マイク' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('TC-4-1-23: 保存済みの値が初期値としてvoiceより優先される', () => {
    const mockStorage = createMockStorage({ getItem: vi.fn().mockReturnValue('text') });
    vi.stubGlobal('localStorage', mockStorage);

    render(ModeToggle, {
      mode: getStoredInputMode(),
      isRecording: false,
      isTranscribing: false,
      onModeChange: vi.fn(),
    });

    expect(screen.getByRole('button', { name: 'テキスト' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('TC-4-1-24: 録音中は両ボタンがdisabledになる', () => {
    render(ModeToggle, {
      mode: 'voice' as InputMode,
      isRecording: true,
      isTranscribing: false,
      onModeChange: vi.fn(),
    });

    expect(screen.getByRole('button', { name: 'マイク' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'テキスト' })).toBeDisabled();
  });

  it('TC-4-1-25: 変換中は両ボタンがdisabledになる', () => {
    render(ModeToggle, {
      mode: 'voice' as InputMode,
      isRecording: false,
      isTranscribing: true,
      onModeChange: vi.fn(),
    });

    expect(screen.getByRole('button', { name: 'マイク' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'テキスト' })).toBeDisabled();
  });

  // TC-4-1-26は欠番（そらのQAレビュー指摘反映）。
  // ModeToggleはテキスト入力欄propを持たない薄いコンポーネントであり、本ケースが前提とする
  // 「ModeToggle自身が入力欄をクリアする」という想定はProps定義（14_test_specification.md §3.4）と矛盾するため削除した。
  // 「モード切替時に旧モードの入力をクリアする」仕様（7章U確定）自体は取り下げず、
  // 入力欄を保持する親コンポーネント（未設計）側の責務として、その設計時に別途テストケース化する。

  it('TC-4-1-27: 高速連続操作でも最終的な変更コールバックの順序・保存値が一致する', async () => {
    const mockStorage = createMockStorage();
    vi.stubGlobal('localStorage', mockStorage);
    const onModeChange = vi.fn((mode: InputMode) => setStoredInputMode(mode));
    render(ModeToggle, {
      mode: 'voice' as InputMode,
      isRecording: false,
      isTranscribing: false,
      onModeChange,
    });

    await fireEvent.click(screen.getByRole('button', { name: 'テキスト' }));
    await fireEvent.click(screen.getByRole('button', { name: 'マイク' }));
    await fireEvent.click(screen.getByRole('button', { name: 'テキスト' }));
    await fireEvent.click(screen.getByRole('button', { name: 'マイク' }));

    expect(onModeChange.mock.calls.map((call) => call[0])).toEqual(['text', 'voice', 'text', 'voice']);
    expect(mockStorage.setItem).toHaveBeenLastCalledWith(INPUT_MODE_STORAGE_KEY, 'voice');
  });
});
