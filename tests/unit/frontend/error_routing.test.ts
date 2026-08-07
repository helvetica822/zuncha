// 対応仕様: docs/03_unit_test/14_test_specification.md 4.3（観点4-3、TC-4-3-01〜15）
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';

import { routeSSEEvent, DEFAULT_ERROR_MESSAGE } from '../../../src/lib/sseEventRouter';
import type { SSEEvent, Emotion } from '../../../src/lib/sseEventRouter';
import Toast from '../../../src/components/Toast.svelte';
import MessageBubble from '../../../src/components/MessageBubble.svelte';

afterEach(() => {
  cleanup();
});

describe('routeSSEEvent', () => {
  it('TC-4-3-01: errorイベントはonErrorのみを呼ぶ', () => {
    const onError = vi.fn();
    const onMessage = vi.fn();
    const event: SSEEvent = { type: 'error', payload: { message: 'サーバーエラーが発生しました' } };

    routeSSEEvent(event, { onError, onMessage });

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError).toHaveBeenCalledWith('サーバーエラーが発生しました');
    expect(onMessage).not.toHaveBeenCalled();
  });

  it('TC-4-3-02: 通常応答はonMessageのみを呼ぶ', () => {
    const onError = vi.fn();
    const onMessage = vi.fn();
    const event: SSEEvent = { type: 'message', payload: { text: 'こんにちは', emotion: '喜び' as Emotion } };

    routeSSEEvent(event, { onError, onMessage });

    expect(onMessage).toHaveBeenCalledWith('こんにちは', '喜び');
    expect(onError).not.toHaveBeenCalled();
  });

  it('TC-4-3-03: 困惑・リトライ文言も通常応答と同一パスで処理される（ネガティブテスト）', () => {
    const onMessageNormal = vi.fn();
    const onMessageRetry = vi.fn();

    routeSSEEvent(
      { type: 'message', payload: { text: 'こんにちは', emotion: '喜び' as Emotion } },
      { onError: vi.fn(), onMessage: onMessageNormal },
    );
    routeSSEEvent(
      { type: 'message', payload: { text: 'もう一度お話しください', emotion: '困惑' as Emotion } },
      { onError: vi.fn(), onMessage: onMessageRetry },
    );

    expect(onMessageNormal).toHaveBeenCalledWith('こんにちは', '喜び');
    expect(onMessageRetry).toHaveBeenCalledWith('もう一度お話しください', '困惑');
  });

  it('TC-4-3-04: messageフィールド欠落時はデフォルトメッセージにフォールバックする', () => {
    const onError = vi.fn();
    const event: SSEEvent = { type: 'error', payload: {} };

    routeSSEEvent(event, { onError, onMessage: vi.fn() });

    expect(onError).toHaveBeenCalledWith(DEFAULT_ERROR_MESSAGE);
  });

  it('TC-4-3-05: messageが空文字列でもデフォルトメッセージにフォールバックする', () => {
    const onError = vi.fn();
    const event: SSEEvent = { type: 'error', payload: { message: '' } };

    routeSSEEvent(event, { onError, onMessage: vi.fn() });

    expect(onError).toHaveBeenCalledWith(DEFAULT_ERROR_MESSAGE);
  });

  it('TC-4-3-06: errorイベントでonMessageは呼ばれない（排他性）', () => {
    const onMessage = vi.fn();
    const event: SSEEvent = { type: 'error', payload: { message: 'サーバーエラーが発生しました' } };

    routeSSEEvent(event, { onError: vi.fn(), onMessage });

    expect(onMessage).not.toHaveBeenCalled();
  });

  it('TC-4-3-07: 通常応答でonErrorは呼ばれない（逆方向排他性）', () => {
    const onError = vi.fn();
    const event: SSEEvent = { type: 'message', payload: { text: 'こんにちは', emotion: '喜び' as Emotion } };

    routeSSEEvent(event, { onError, onMessage: vi.fn() });

    expect(onError).not.toHaveBeenCalled();
  });
});

describe('Toast', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('TC-4-3-08: propsのmessageがalertロールに表示される', () => {
    render(Toast, { message: 'サーバーエラーが発生しました' });

    expect(screen.getByRole('alert')).toHaveTextContent('サーバーエラーが発生しました');
  });

  it('TC-4-3-09: 3秒後にDOMから消滅する', async () => {
    render(Toast, { message: 'サーバーエラーが発生しました' });

    await vi.advanceTimersByTimeAsync(3000);

    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('TC-4-3-10: アンマウント時にタイマーがクリアされ警告が出ない', () => {
    const { unmount } = render(Toast, { message: 'サーバーエラーが発生しました' });
    const consoleError = vi.spyOn(console, 'error');

    unmount();
    vi.advanceTimersByTime(3000);

    expect(consoleError).not.toHaveBeenCalled();
  });

  it('TC-4-3-11: 2999msの時点ではまだDOMに存在する', () => {
    render(Toast, { message: 'サーバーエラーが発生しました' });

    vi.advanceTimersByTime(2999);

    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('TC-4-3-12: ちょうど3000msの時点で消滅している', async () => {
    render(Toast, { message: 'サーバーエラーが発生しました' });

    await vi.advanceTimersByTimeAsync(3000);

    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('TC-4-3-13: 連続表示時は上書きされ多重表示にならない（暫定仕様、7章BB確定）', async () => {
    const { rerender } = render(Toast, { message: '1件目のエラー' });

    await vi.advanceTimersByTimeAsync(100);
    await rerender({ message: '2件目のエラー' });

    expect(screen.getAllByRole('alert')).toHaveLength(1);
    expect(screen.getByRole('alert')).toHaveTextContent('2件目のエラー');
  });
});

describe('MessageBubble', () => {
  it('TC-4-3-14: 通常応答がずんだもん発話として表示される', () => {
    render(MessageBubble, { text: 'こんにちは', emotion: '喜び' as Emotion });

    expect(screen.getByText('こんにちは')).toBeInTheDocument();
  });

  it('TC-4-3-15: 困惑・リトライ文言も通常応答と同一コンポーネントパスで表示される', () => {
    // 本文と data-emotion 以外が完全一致＝同一コンポーネントパス。
    // data-emotion は感情の反映点なので比較から除外し、別途値を検証する。
    const stripVolatile = (html: string, text: string): string =>
      html.replace(text, '').replace(/ data-emotion="[^"]*"/, '');

    const normal = render(MessageBubble, { text: 'こんにちは', emotion: '喜び' as Emotion });
    const normalHtml = stripVolatile(normal.container.innerHTML, 'こんにちは');
    normal.unmount();

    const retry = render(MessageBubble, { text: 'もう一度お願いします', emotion: '困惑' as Emotion });
    const retryHtml = stripVolatile(retry.container.innerHTML, 'もう一度お願いします');

    expect(screen.getByText('もう一度お願いします')).toBeInTheDocument();
    expect(retryHtml).toBe(normalHtml);
    // 除外した data-emotion 自体は感情を反映する（画面設計 9-1 の感情ラベル）。
    expect(retry.container.querySelector('[data-role="assistant"]')?.getAttribute('data-emotion')).toBe(
      '困惑',
    );
  });
});
