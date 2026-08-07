// 対応仕様: docs/04_implementation/02_parent_container_design.md（INV-1〜5）
// 実装指示: tasks/instructions_zundamon_conversation_view.md（TC-P-01〜TC-P-19b）
// 文言根拠: docs/02_functional_design/01_screen_design.md 9-1（486行）/ 9-2（540行）/ 9-3
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent } from '@testing-library/svelte';
import { tick } from 'svelte';

import { DEFAULT_ERROR_MESSAGE } from '../../../src/lib/sseEventRouter';
import type { SSEEvent, Emotion } from '../../../src/lib/sseEventRouter';
import { INPUT_MODE_STORAGE_KEY } from '../../../src/lib/inputMode';
import { DEFAULT_TOAST_DURATION_MS } from '../../../src/lib/toast';
import ConversationView from '../../../src/components/ConversationView.svelte';
// TC-P-21b で「親のガードが最終防衛線である」根拠を固定するために単体で描画する。
import SendButton from '../../../src/components/SendButton.svelte';

// 仕様値をテスト側にも直書きし、実装の定数と一致することを検証する（仕様の二重記述＝回帰検知）。
const VOICE_LABEL = 'マイク';
const TEXT_LABEL = 'テキスト';
const SEND_BUTTON_LABEL = '送信';
const TEXT_INPUT_LABEL = 'メッセージ';
const STT_FAILURE_MESSAGE = 'うまく聞き取れなかったのだ。もう一度話しかけてほしいのだ';
const STT_TIMEOUT_MESSAGE = 'あれ、聞こえなくなってしまったのだ。もう一度話しかけてほしいのだ';
const STT_FAILURE_EMOTION = '困惑';
const MAX_MESSAGES = 20;

type SttFailureReason = 'unrecognized' | 'timeout';

// Svelte 4 の ambient 型（`*.svelte` の default export = SvelteComponent）は
// `export function` を型として露出しないため、コンテナの外部受口の契約をここで宣言する。
interface ConversationViewExports {
  startRecording(): void;
  startTranscribing(): void;
  handleSttSuccess(transcript: string): void;
  handleSttFailure(reason?: SttFailureReason): void;
  receiveSSEEvent(event: SSEEvent): void;
  completeSubmission(): void;
}

const renderView = (): ConversationViewExports => {
  const { component } = render(ConversationView, {});
  return component as unknown as ConversationViewExports;
};

const textInput = (): HTMLTextAreaElement =>
  screen.getByLabelText<HTMLTextAreaElement>(TEXT_INPUT_LABEL);

const buttonByName = (name: string): HTMLButtonElement =>
  screen.getByRole<HTMLButtonElement>('button', { name });

// MessageBubble のルート要素（data-role="assistant"）を古い順に取得する。
const bubbles = (): HTMLElement[] =>
  Array.from(document.querySelectorAll<HTMLElement>('[data-role="assistant"]'));

const bubbleTexts = (): string[] => bubbles().map((bubble) => bubble.textContent ?? '');

const sendButtonDisabled = (): boolean => buttonByName(SEND_BUTTON_LABEL).disabled;

const typeText = async (value: string): Promise<void> => {
  await fireEvent.input(textInput(), { target: { value } });
};

const errorEvent = (message?: string): SSEEvent => ({
  type: 'error',
  payload: message === undefined ? {} : { message },
});

const messageEvent = (text: string, emotion: Emotion): SSEEvent => ({
  type: 'message',
  payload: { text, emotion },
});

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  localStorage.clear();
});

describe('ConversationView 初期状態', () => {
  it('TC-P-01: localStorage未設定なら音声モードで起動し、ログ・トースト無し・送信可能', () => {
    renderView();

    expect(buttonByName(VOICE_LABEL).getAttribute('aria-pressed')).toBe('true');
    expect(buttonByName(TEXT_LABEL).getAttribute('aria-pressed')).toBe('false');
    expect(textInput().value).toBe('');
    expect(bubbles()).toHaveLength(0);
    expect(screen.queryAllByRole('alert')).toHaveLength(0);
    // voice + editable は空でも送信可能（isSendButtonDisabled 契約）。
    expect(sendButtonDisabled()).toBe(false);
  });

  it('TC-P-02: localStorageに text が保存済みならテキストモードで起動し、空入力では送信不可', () => {
    localStorage.setItem(INPUT_MODE_STORAGE_KEY, 'text');

    renderView();

    expect(buttonByName(TEXT_LABEL).getAttribute('aria-pressed')).toBe('true');
    expect(buttonByName(VOICE_LABEL).getAttribute('aria-pressed')).toBe('false');
    expect(sendButtonDisabled()).toBe(true);
  });
});

describe('ConversationView モード切替 (INV-1/2/5)', () => {
  it('TC-P-03: モード切替でテキスト欄がクリアされ、切替後のモードが永続化される', async () => {
    renderView();
    await typeText('切替前に入力した文字');

    await fireEvent.click(buttonByName(TEXT_LABEL));

    expect(textInput().value).toBe('');
    expect(localStorage.getItem(INPUT_MODE_STORAGE_KEY)).toBe('text');
  });

  it('TC-P-04: voice→text 切替直後は送信不可、文字を入力すると送信可能になる', async () => {
    renderView();

    await fireEvent.click(buttonByName(TEXT_LABEL));
    expect(sendButtonDisabled()).toBe(true);

    await typeText('こんにちは');
    expect(sendButtonDisabled()).toBe(false);
  });
});

describe('ConversationView STT (INV-1/3/4b・経路分離)', () => {
  it('TC-P-05: STT成功で認識結果がテキスト欄に反映され送信可能になる', async () => {
    const view = renderView();

    view.handleSttSuccess('こんにちは');
    await tick();

    expect(textInput().value).toBe('こんにちは');
    expect(sendButtonDisabled()).toBe(false);
  });

  it('TC-P-06: STT失敗は応答バブルで表現し、トーストを出さず transcribing に固着しない', async () => {
    const view = renderView();

    view.startTranscribing();
    await tick();
    view.handleSttFailure();
    await tick();

    expect(textInput().value).toBe('');
    expect(bubbles()).toHaveLength(1);
    expect(bubbles()[0].textContent).toBe(STT_FAILURE_MESSAGE);
    expect(bubbles()[0].getAttribute('data-emotion')).toBe(STT_FAILURE_EMOTION);
    // 経路分離: STT失敗はトーストを一切使わない（設計書§4）。
    expect(screen.queryAllByRole('alert')).toHaveLength(0);
    // INV-3: transcribing 固着なし → トグルが操作可能に戻る。
    expect(buttonByName(VOICE_LABEL).disabled).toBe(false);
    expect(buttonByName(TEXT_LABEL).disabled).toBe(false);
    // INV-5: voice + editable なので空でも送信可能に整合する。
    expect(sendButtonDisabled()).toBe(false);
  });

  it('TC-P-07: タイムアウトはタイムアウト専用文言のバブルになる', async () => {
    const view = renderView();

    view.handleSttFailure('timeout');
    await tick();

    expect(bubbles()).toHaveLength(1);
    expect(bubbles()[0].textContent).toBe(STT_TIMEOUT_MESSAGE);
  });

  it('TC-P-08: STT失敗直後にモード切替してもバブルは残り、text空・mode切替後・送信不可で整合する', async () => {
    const view = renderView();
    await typeText('録音前に入力した文字');

    view.startRecording();
    await tick();
    view.startTranscribing();
    await tick();
    view.handleSttFailure();
    await tick();

    await fireEvent.click(buttonByName(TEXT_LABEL));

    expect(textInput().value).toBe('');
    expect(buttonByName(TEXT_LABEL).getAttribute('aria-pressed')).toBe('true');
    // INV-4b: モード切替でも会話ログのバブルは消えない。
    expect(bubbles()).toHaveLength(1);
    expect(bubbles()[0].textContent).toBe(STT_FAILURE_MESSAGE);
    expect(sendButtonDisabled()).toBe(true);
  });

  it('TC-P-09: モード切替直後のSTT失敗はモードを巻き戻さず、バブルのみ追加する', async () => {
    const view = renderView();

    await fireEvent.click(buttonByName(TEXT_LABEL));
    view.handleSttFailure();
    await tick();

    expect(textInput().value).toBe('');
    // INV-2: STT失敗は mode を変えない。
    expect(buttonByName(TEXT_LABEL).getAttribute('aria-pressed')).toBe('true');
    expect(localStorage.getItem(INPUT_MODE_STORAGE_KEY)).toBe('text');
    expect(bubbles()).toHaveLength(1);
    expect(screen.queryAllByRole('alert')).toHaveLength(0);
  });

  it('TC-P-10: STT失敗が連続すると古い順にバブルが積まれる', async () => {
    const view = renderView();

    view.handleSttFailure();
    await tick();
    view.handleSttFailure('timeout');
    await tick();

    expect(bubbleTexts()).toEqual([STT_FAILURE_MESSAGE, STT_TIMEOUT_MESSAGE]);
  });
});

describe('ConversationView SSE (INV-4・経路分離)', () => {
  it('TC-P-11: SSE error はトーストのみで、会話ログにバブルを追加しない', async () => {
    const view = renderView();

    view.receiveSSEEvent(errorEvent('サーバーエラー'));
    await tick();

    const alerts = screen.queryAllByRole('alert');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toBe('サーバーエラー');
    expect(bubbles()).toHaveLength(0);
  });

  it('TC-P-12: 異なる文言のerrorが連続してもトーストは1件・最新の文言が勝つ', async () => {
    const view = renderView();

    view.receiveSSEEvent(errorEvent('1回目のエラー'));
    await tick();
    view.receiveSSEEvent(errorEvent('2回目のエラー'));
    await tick();

    const alerts = screen.queryAllByRole('alert');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toBe('2回目のエラー');
  });

  it('TC-P-13: 同一文言のerrorが連続してもトーストは多重化しない', async () => {
    const view = renderView();

    view.receiveSSEEvent(errorEvent('同じエラー'));
    await tick();
    view.receiveSSEEvent(errorEvent('同じエラー'));
    await tick();

    const alerts = screen.queryAllByRole('alert');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toBe('同じエラー');
  });

  it('TC-P-14: error の message 欠落時はデフォルト文言のトーストになる', async () => {
    const view = renderView();

    view.receiveSSEEvent(errorEvent());
    await tick();

    expect(screen.queryAllByRole('alert')[0].textContent).toBe(DEFAULT_ERROR_MESSAGE);
  });

  it('TC-P-15: message イベントは感情付きバブルのみを追加し、トーストを出さない', async () => {
    const view = renderView();

    view.receiveSSEEvent(messageEvent('元気だったのだ', '喜び'));
    await tick();

    expect(bubbles()).toHaveLength(1);
    expect(bubbles()[0].textContent).toBe('元気だったのだ');
    expect(bubbles()[0].getAttribute('data-emotion')).toBe('喜び');
    expect(screen.queryAllByRole('alert')).toHaveLength(0);
  });

  it('TC-P-20a: 同一文言のerrorが自動消滅後に再来すると、トーストが再表示される', async () => {
    vi.useFakeTimers();
    const view = renderView();

    view.receiveSSEEvent(errorEvent('同じエラー'));
    await tick();
    expect(screen.queryAllByRole('alert')).toHaveLength(1);

    // 自動消滅させたうえで、まったく同じ文言のerrorを再受信する。
    vi.advanceTimersByTime(DEFAULT_TOAST_DURATION_MS);
    await tick();
    expect(screen.queryAllByRole('alert')).toHaveLength(0);

    view.receiveSSEEvent(errorEvent('同じエラー'));
    await tick();

    const alerts = screen.queryAllByRole('alert');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toBe('同じエラー');
  });

  it('TC-P-20b: 異なる文言のerrorが自動消滅後に再来しても、トーストが再表示される', async () => {
    vi.useFakeTimers();
    const view = renderView();

    view.receiveSSEEvent(errorEvent('1回目のエラー'));
    await tick();

    vi.advanceTimersByTime(DEFAULT_TOAST_DURATION_MS);
    await tick();
    expect(screen.queryAllByRole('alert')).toHaveLength(0);

    view.receiveSSEEvent(errorEvent('2回目のエラー'));
    await tick();

    const alerts = screen.queryAllByRole('alert');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toBe('2回目のエラー');
  });

  it('TC-P-20c: 表示中に同一文言のerrorが再来してもトーストは1件のみ（INV-4）', async () => {
    vi.useFakeTimers();
    const view = renderView();

    view.receiveSSEEvent(errorEvent('同じエラー'));
    await tick();
    // 消滅前（duration未満）に同一文言を再受信しても多重化しない。
    vi.advanceTimersByTime(DEFAULT_TOAST_DURATION_MS - 1);
    view.receiveSSEEvent(errorEvent('同じエラー'));
    await tick();

    const alerts = screen.queryAllByRole('alert');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toBe('同じエラー');
  });

  it('TC-P-20d: 表示中に同一文言のerrorが再来すると、タイマーが張り直され表示が延長される', async () => {
    vi.useFakeTimers();
    const view = renderView();

    view.receiveSSEEvent(errorEvent('同じエラー'));
    await tick();
    expect(screen.queryAllByRole('alert')).toHaveLength(1);

    // 消滅の直前（残り1ms）まで進めたところで、同一文言を再受信する。
    vi.advanceTimersByTime(DEFAULT_TOAST_DURATION_MS - 1);
    await tick();
    expect(screen.queryAllByRole('alert')).toHaveLength(1);

    view.receiveSSEEvent(errorEvent('同じエラー'));
    await tick();

    // 旧タイマーが破棄されている証明: 元の満了時刻を過ぎても消えない。
    vi.advanceTimersByTime(1);
    await tick();
    expect(screen.queryAllByRole('alert')).toHaveLength(1);

    // 再来時点を起点に満了する証明: そこから残りの duration で消える。
    vi.advanceTimersByTime(DEFAULT_TOAST_DURATION_MS - 1);
    await tick();
    expect(screen.queryAllByRole('alert')).toHaveLength(0);
  });

  it('TC-P-16: トーストは既定時間の経過で自動的に消える', async () => {
    vi.useFakeTimers();
    const view = renderView();

    view.receiveSSEEvent(errorEvent('消えるエラー'));
    await tick();
    expect(screen.queryAllByRole('alert')).toHaveLength(1);

    vi.advanceTimersByTime(DEFAULT_TOAST_DURATION_MS);
    await tick();

    expect(screen.queryAllByRole('alert')).toHaveLength(0);
  });
});

describe('ConversationView 送信・境界・エッジ', () => {
  it('TC-P-17: 送信中は送信ボタンが無効になり、完了で有効に戻りテキスト欄が空になる', async () => {
    const view = renderView();
    view.handleSttSuccess('送信する文字');
    await tick();

    await fireEvent.click(buttonByName(SEND_BUTTON_LABEL));
    expect(sendButtonDisabled()).toBe(true);

    view.completeSubmission();
    await tick();

    expect(sendButtonDisabled()).toBe(false);
    expect(textInput().value).toBe('');
  });

  it('TC-P-18: テキストモードで空白のみの入力は送信不可（trim判定）', async () => {
    renderView();

    await fireEvent.click(buttonByName(TEXT_LABEL));
    await typeText('   ');

    expect(sendButtonDisabled()).toBe(true);
  });

  it('TC-P-19a: 1000文字の入力もモード切替でクリアされる', async () => {
    renderView();
    const longText = 'あ'.repeat(1000);

    await typeText(longText);
    expect(textInput().value).toBe(longText);

    await fireEvent.click(buttonByName(TEXT_LABEL));

    expect(textInput().value).toBe('');
  });

  it('TC-P-19b: バブルは最大20件で、最古から溢れ古い順を維持する', async () => {
    const view = renderView();

    for (let index = 1; index <= MAX_MESSAGES + 1; index += 1) {
      view.receiveSSEEvent(messageEvent(`メッセージ${index}`, '楽しい'));
    }
    await tick();

    const texts = bubbleTexts();
    expect(texts).toHaveLength(MAX_MESSAGES);
    expect(texts[0]).toBe('メッセージ2');
    expect(texts[MAX_MESSAGES - 1]).toBe(`メッセージ${MAX_MESSAGES + 1}`);
  });
});

// 'sending' は「送信リクエストが飛行中」を表す占有状態であり、これを解けるのは
// completeSubmission()（done）と SSE error の2経路のみ（設計書§5 INV-6）。
describe('ConversationView 送信中の状態保護 (INV-6)', () => {
  // sending へ入るための共通の前提: 認識結果を入力欄に載せてから送信する。
  const submitWith = async (view: ConversationViewExports, value: string): Promise<void> => {
    view.handleSttSuccess(value);
    await tick();
    await fireEvent.click(buttonByName(SEND_BUTTON_LABEL));
  };

  // ⚠ 限界の明示（ミューテーションテストで実測）: 本TCは「二連打後も状態が健全である」ことの
  //    記録であって、INV-6a（handleSubmit の入口ガード）の回帰検知にはならない。handleSubmit の
  //    副作用は現状 inputState='sending' の代入のみで冪等なため、ガードを外しても本TCは緑のまま
  //    通る。区別できる観測点が実装に存在しないので、原理的にここでは書けない。
  //    ガードが実際に効くのは handleSubmit が実送信を行うようになってから。実SSE配線フェーズで
  //    「送信APIの呼び出し回数 = 1」を検証するテストを必ず追加すること。
  //    ガードが必要である根拠自体は TC-P-21b で固定している。
  it('TC-P-21: 送信ボタンを同一フレームで二連打しても送信は1件分として扱われる', async () => {
    const view = renderView();
    view.handleSttSuccess('送信する文字');
    await tick();
    const button = buttonByName(SEND_BUTTON_LABEL);

    // ガードが必要な前提の明示: Svelte の DOM 更新は非同期のため、1回目のクリック直後は
    // まだ disabled が反映されていない＝2回目のクリックは handleSubmit に到達する。
    button.click();
    expect(button.disabled).toBe(false);
    button.click();
    await tick();

    // INV-6a: 二連打後も sending は1件分。done ひとつで完了し、多重送信の残骸を残さない。
    expect(sendButtonDisabled()).toBe(true);
    view.completeSubmission();
    await tick();
    expect(sendButtonDisabled()).toBe(false);
    expect(textInput().value).toBe('');
  });

  it('TC-P-21b: SendButton 単体では同一フレームの二連打を防げない（親ガードが必要な根拠）', async () => {
    const onSubmit = vi.fn();
    render(SendButton, { props: { disabled: false, onSubmit } });
    const button = buttonByName(SEND_BUTTON_LABEL);

    // disabled prop はクリック後も親が更新するまで false のままなので、
    // SendButton 側の防御（native disabled と早期リターン）は二連打を止められない。
    button.click();
    button.click();

    expect(onSubmit).toHaveBeenCalledTimes(2);
  });

  it('TC-P-22a: 送信中のモード切替は mode と text を更新するが、送信ボタンは無効のまま', async () => {
    localStorage.setItem(INPUT_MODE_STORAGE_KEY, 'text');
    renderView();
    await typeText('送信する文字');
    await fireEvent.click(buttonByName(SEND_BUTTON_LABEL));
    expect(sendButtonDisabled()).toBe(true);

    await fireEvent.click(buttonByName(VOICE_LABEL));

    // INV-6b: mode の更新・永続化と text クリアは行う（ModeToggle の楽観的更新と乖離させない）。
    expect(buttonByName(VOICE_LABEL).getAttribute('aria-pressed')).toBe('true');
    expect(localStorage.getItem(INPUT_MODE_STORAGE_KEY)).toBe('voice');
    expect(textInput().value).toBe('');
    // voice + editable なら空でも送信可能になるはずのところ、sending 維持で無効のまま
    // ＝モード切替経由の二重送信が塞がれている証明。
    expect(sendButtonDisabled()).toBe(true);
  });

  it('TC-P-22b: 送信中にモード切替しても、その送信は done で正常に完了できる', async () => {
    localStorage.setItem(INPUT_MODE_STORAGE_KEY, 'text');
    const view = renderView();
    await typeText('送信する文字');
    await fireEvent.click(buttonByName(SEND_BUTTON_LABEL));

    await fireEvent.click(buttonByName(VOICE_LABEL));
    // 切替を挟んでも done が来るまでは sending のまま（所有権が切替に奪われない）。
    expect(sendButtonDisabled()).toBe(true);

    view.completeSubmission();
    await tick();

    // INV-6b: sending の所有権が保たれているので done が効く（切替で迷子にならない）。
    expect(sendButtonDisabled()).toBe(false);
    expect(textInput().value).toBe('');
  });

  it('TC-P-23a: 送信中に遅着したSTT成功結果は破棄され、送信中のテキストを上書きしない', async () => {
    const view = renderView();
    await submitWith(view, '送信する文字');

    view.handleSttSuccess('遅れて届いた認識結果');
    await tick();

    // INV-6c: 旧インタラクションの stale 結果は text も inputState も変えない。
    expect(textInput().value).toBe('送信する文字');
    expect(sendButtonDisabled()).toBe(true);
    expect(bubbles()).toHaveLength(0);
  });

  it.each<[SttFailureReason, string]>([
    ['unrecognized', STT_FAILURE_MESSAGE],
    ['timeout', STT_TIMEOUT_MESSAGE],
  ])(
    'TC-P-23b: 送信中に遅着したSTT失敗(%s)は破棄され、バブルも追加しない',
    async (reason, message) => {
      const view = renderView();
      await submitWith(view, '送信する文字');

      view.handleSttFailure(reason);
      await tick();

      // INV-6c: INV-4b（失敗はバブル1件追加）は sending 中を対象外とする。
      expect(bubbleTexts()).not.toContain(message);
      expect(bubbles()).toHaveLength(0);
      expect(textInput().value).toBe('送信する文字');
      expect(sendButtonDisabled()).toBe(true);
    },
  );

  it('TC-P-24: 送信中でないときの遅延doneは、ユーザーが打ち直した入力を消さない', async () => {
    const view = renderView();
    await submitWith(view, '1回目の送信');

    // SSE error で sending → editable へ復帰し、ユーザーが次の文を打ち始める。
    view.receiveSSEEvent(errorEvent('サーバーエラー'));
    await tick();
    await typeText('2回目に入力した文字');

    // ここで旧リクエストの done が遅れて到着する。
    view.completeSubmission();
    await tick();

    // INV-6d: sending 以外では何もしない＝入力は保持され、状態も editable のまま。
    expect(textInput().value).toBe('2回目に入力した文字');
    expect(sendButtonDisabled()).toBe(false);
  });
});

describe('ConversationView 中間状態の救済 (INV-3b)', () => {
  it('TC-P-25: transcribing 中のSSEエラーはトーストを出しつつ editable へ復帰させる', async () => {
    const view = renderView();
    view.startTranscribing();
    await tick();
    // 変換中はトグルが操作不能。ここで固着すると自力の出口が無くなる。
    expect(buttonByName(VOICE_LABEL).disabled).toBe(true);
    expect(buttonByName(TEXT_LABEL).disabled).toBe(true);

    view.receiveSSEEvent(errorEvent('バックエンド障害'));
    await tick();

    const alerts = screen.queryAllByRole('alert');
    expect(alerts).toHaveLength(1);
    expect(alerts[0].textContent).toBe('バックエンド障害');
    // INV-3b: 固着が解け、モード切替・送信ともに操作可能へ戻る。
    expect(buttonByName(VOICE_LABEL).disabled).toBe(false);
    expect(buttonByName(TEXT_LABEL).disabled).toBe(false);
    expect(sendButtonDisabled()).toBe(false);
    // 経路分離: error はトーストのみでバブルを増やさない。
    expect(bubbles()).toHaveLength(0);
  });
});

describe('ConversationView 同一モードの再クリック (INV-1b)', () => {
  it('TC-P-26a: テキストモードで「テキスト」を再クリックしても書きかけの入力が消えない', async () => {
    localStorage.setItem(INPUT_MODE_STORAGE_KEY, 'text');
    renderView();
    await typeText('書きかけの文章');

    await fireEvent.click(buttonByName(TEXT_LABEL));

    // INV-1b: モードが変わらない以上「旧モードの入力」は存在せず、クリアの根拠がない。
    expect(textInput().value).toBe('書きかけの文章');
    expect(buttonByName(TEXT_LABEL).getAttribute('aria-pressed')).toBe('true');
    expect(sendButtonDisabled()).toBe(false);
  });

  it('TC-P-26b: 同一モードの再クリックでは localStorage への再書き込みが起きない', async () => {
    const view = renderView();
    view.handleSttSuccess('認識できた文字');
    await tick();
    // 書き込みの有無だけを観測するため、いったんキーを削除してからクリックする。
    localStorage.removeItem(INPUT_MODE_STORAGE_KEY);

    await fireEvent.click(buttonByName(VOICE_LABEL));

    expect(localStorage.getItem(INPUT_MODE_STORAGE_KEY)).toBeNull();
    expect(textInput().value).toBe('認識できた文字');
    expect(buttonByName(VOICE_LABEL).getAttribute('aria-pressed')).toBe('true');
  });
});
