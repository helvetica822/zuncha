// 対応仕様: docs/03_unit_test/14_test_specification.md 4.2（観点4-2、TC-4-2-01〜19）
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/svelte';

import { isSendButtonDisabled } from '../../../src/lib/sendButton';
import type { InputState } from '../../../src/lib/sendButton';
import type { InputMode } from '../../../src/lib/inputMode';
import SendButton from '../../../src/components/SendButton.svelte';

afterEach(() => {
  cleanup();
});

describe('isSendButtonDisabled', () => {
  it('TC-4-2-01: 音声モードで録音完了後は有効', () => {
    expect(
      isSendButtonDisabled({ mode: 'voice' as InputMode, text: '', inputState: 'editable' as InputState }),
    ).toBe(false);
  });

  it('TC-4-2-02: テキストモードで非空文字列は有効', () => {
    expect(
      isSendButtonDisabled({ mode: 'text' as InputMode, text: 'こんにちは', inputState: 'editable' as InputState }),
    ).toBe(false);
  });

  it('TC-4-2-03: STT反映後の非空文字列は有効', () => {
    expect(
      isSendButtonDisabled({
        mode: 'text' as InputMode,
        text: 'STTにより反映された文字列',
        inputState: 'editable' as InputState,
      }),
    ).toBe(false);
  });

  it('TC-4-2-04: 音声モードで録音前待機中は無効', () => {
    expect(
      isSendButtonDisabled({ mode: 'voice' as InputMode, text: '', inputState: 'idle' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-05: 空文字列は無効', () => {
    expect(
      isSendButtonDisabled({ mode: 'text' as InputMode, text: '', inputState: 'editable' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-06: 半角スペースのみは無効', () => {
    expect(
      isSendButtonDisabled({ mode: 'text' as InputMode, text: '   ', inputState: 'editable' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-07: 変換中は入力内容に関わらず無効', () => {
    expect(
      isSendButtonDisabled({
        mode: 'text' as InputMode,
        text: '変換中の暫定文字列',
        inputState: 'transcribing' as InputState,
      }),
    ).toBe(true);
  });

  it('TC-4-2-08: 録音中は無効（7章V確定）', () => {
    expect(
      isSendButtonDisabled({ mode: 'voice' as InputMode, text: '', inputState: 'recording' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-09: 送信中は有効な文字列があっても無効（7章W確定）', () => {
    expect(
      isSendButtonDisabled({ mode: 'text' as InputMode, text: 'こんにちは', inputState: 'sending' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-10: 送信中はモードを問わず無効', () => {
    expect(
      isSendButtonDisabled({ mode: 'voice' as InputMode, text: '', inputState: 'sending' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-11: 1文字のみでも有効（空でない最小境界）', () => {
    expect(
      isSendButtonDisabled({ mode: 'text' as InputMode, text: 'あ', inputState: 'editable' as InputState }),
    ).toBe(false);
  });

  it('TC-4-2-12: タブのみは無効', () => {
    expect(
      isSendButtonDisabled({ mode: 'text' as InputMode, text: '\t', inputState: 'editable' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-13: 改行のみは無効', () => {
    expect(
      isSendButtonDisabled({ mode: 'text' as InputMode, text: '\n', inputState: 'editable' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-14: 全角スペースのみは無効', () => {
    expect(
      isSendButtonDisabled({ mode: 'text' as InputMode, text: '　', inputState: 'editable' as InputState }),
    ).toBe(true);
  });

  it('TC-4-2-15: 空白文字混在は無効', () => {
    expect(
      isSendButtonDisabled({
        mode: 'text' as InputMode,
        text: ' \t\n　 ',
        inputState: 'editable' as InputState,
      }),
    ).toBe(true);
  });

  it('TC-4-2-16: 1000文字でも文字数上限による無効化はない', () => {
    expect(
      isSendButtonDisabled({
        mode: 'text' as InputMode,
        text: 'あ'.repeat(1000),
        inputState: 'editable' as InputState,
      }),
    ).toBe(false);
  });
});

describe('SendButton', () => {
  it('TC-4-2-17: disabled=falseのときボタンは有効', () => {
    render(SendButton, { disabled: false, onSubmit: vi.fn() });

    expect(screen.getByRole('button', { name: '送信' })).not.toBeDisabled();
  });

  it('TC-4-2-18: disabled=trueのときボタンは無効', () => {
    render(SendButton, { disabled: true, onSubmit: vi.fn() });

    expect(screen.getByRole('button', { name: '送信' })).toBeDisabled();
  });

  it('TC-4-2-19: disabled状態でクリックしてもonSubmitは呼ばれない', async () => {
    const onSubmit = vi.fn();
    render(SendButton, { disabled: true, onSubmit });

    await fireEvent.click(screen.getByRole('button', { name: '送信' }));

    expect(onSubmit).not.toHaveBeenCalled();
  });
});
