<script lang="ts">
  import { getStoredInputMode, setStoredInputMode } from '$lib/inputMode';
  import type { InputMode } from '$lib/inputMode';
  import { isSendButtonDisabled } from '$lib/sendButton';
  import type { InputState } from '$lib/sendButton';
  import { routeSSEEvent } from '$lib/sseEventRouter';
  import type { Emotion, SSEEvent } from '$lib/sseEventRouter';

  import MessageBubble from '$components/MessageBubble.svelte';
  import ModeToggle from '$components/ModeToggle.svelte';
  import SendButton from '$components/SendButton.svelte';
  import Toast from '$components/Toast.svelte';

  // 会話ログの表示上限。バックエンド GetRecentMessages（直近20件・古い順）と一致させる。
  const MAX_MESSAGES = 20;
  // テキスト入力欄の aria-label（テストの参照点）。
  const TEXT_INPUT_LABEL = 'メッセージ';
  // STT失敗系のリトライ発話。文言は 01_screen_design.md 9-1 / 9-2 の「テキスト例」どおり。
  const STT_FAILURE_MESSAGE = 'うまく聞き取れなかったのだ。もう一度話しかけてほしいのだ';
  const STT_TIMEOUT_MESSAGE = 'あれ、聞こえなくなってしまったのだ。もう一度話しかけてほしいのだ';
  // 認識失敗は自責でなく困惑（9-1 感情ラベル）。
  const STT_FAILURE_EMOTION: Emotion = '困惑';

  type SttFailureReason = 'unrecognized' | 'timeout';

  interface AssistantMessage {
    id: number;
    text: string;
    emotion: Emotion;
  }

  // 状態の単一の真実の源泉。各ハンドラは部分更新ではなく一貫した完全な状態を生成する。
  let mode: InputMode = getStoredInputMode();
  let text = '';
  // 実録音パイプラインの配線は本フェーズ対象外のため、初期状態は 'editable'。
  let inputState: InputState = 'editable';
  let toastMessage: string | null = null;
  // {#key} 用の単調増加カウンタ。error 受信ごとに進め、同一文言でも再マウントを起こす（乱数・時刻に依存させない）。
  let toastSeq = 0;
  let messages: AssistantMessage[] = [];
  // {#each} のキー用の単調増加ID（乱数・時刻に依存させない）。
  let nextMessageId = 0;

  // 古い順を保ちつつ、上限を超えた分は最古から溢れさせる。
  function appendMessage(messageText: string, emotion: Emotion): void {
    messages = [...messages, { id: nextMessageId++, text: messageText, emotion }].slice(
      -MAX_MESSAGES,
    );
  }

  // モード切替でクリアするのは旧モードの入力＝テキスト欄の文字列のみ（設計書§4）。
  function handleModeChange(next: InputMode): void {
    // INV-1b: 同一モードの再クリックは「切替」ではない。旧モードが存在しない以上 text を
    // 消す根拠がなく、入力中の誤タップが全消しに直結するため何もしない。
    if (next === mode) return;
    setStoredInputMode(next);
    mode = next;
    text = '';
    // INV-6b: 送信中は飛行中リクエストの占有状態を保持する。
    // mode の更新・永続化は行う（ModeToggle が内部で楽観的更新するため無視すると表示が乖離する）。
    if (inputState !== 'sending') {
      inputState = 'editable';
    }
  }

  function handleSubmit(): void {
    // INV-6a: SendButton の disabled は DOM 更新が非同期のため同一フレームの二連打を防げない。
    // 二重送信の最終防衛線としてコンテナ側で状態を検査する。
    if (inputState !== 'editable') return;
    inputState = 'sending';
  }

  // 以降は外部（実STT/実SSE配線・テスト）からの受口。Svelte 4 のコンポーネントメソッドとして呼ぶ。
  export function startRecording(): void {
    inputState = 'recording';
  }

  export function startTranscribing(): void {
    inputState = 'transcribing';
  }

  export function handleSttSuccess(transcript: string): void {
    // INV-6c: 送信中に届いた STT 結果は送信済み旧インタラクションの stale 結果。破棄する。
    if (inputState === 'sending') return;
    text = transcript;
    inputState = 'editable';
  }

  // STT失敗は応答バブルで表現し、トーストには一切触れない（経路分離・設計書§4）。
  export function handleSttFailure(reason: SttFailureReason = 'unrecognized'): void {
    // INV-6c: 遅延到着の失敗通知はバブルも追加しない（会話ログを汚さない）。
    if (inputState === 'sending') return;
    text = '';
    inputState = 'editable';
    appendMessage(
      reason === 'timeout' ? STT_TIMEOUT_MESSAGE : STT_FAILURE_MESSAGE,
      STT_FAILURE_EMOTION,
    );
  }

  // error はトーストのみ、message はバブルのみ（routeSSEEvent が排他を保証する）。
  export function receiveSSEEvent(event: SSEEvent): void {
    routeSSEEvent(event, {
      onError: (message: string): void => {
        toastMessage = message;
        // 同値代入では何も伝播しないため、受信ごとに必ず進めて再表示のトリガとする。
        toastSeq += 1;
        // INV-3b: 中間状態で固着させず再入力可へ戻す。transcribing は自力の出口
        // （STT成功/失敗コールバック）が来ないと ModeToggle も disabled で操作不能になる。
        if (inputState === 'sending' || inputState === 'transcribing') {
          inputState = 'editable';
        }
      },
      onMessage: appendMessage,
    });
  }

  // SSE done 相当。done は sseEventRouter の契約を壊さないようコンテナ側で表現する。
  export function completeSubmission(): void {
    // INV-6d: 旧リクエストの遅延 done が、ユーザーが既に打ち始めた新規入力を消してはならない。
    if (inputState !== 'sending') return;
    inputState = 'editable';
    text = '';
  }
</script>

<section class="conversation-view">
  <div class="conversation-view__log">
    {#each messages as message (message.id)}
      <MessageBubble text={message.text} emotion={message.emotion} />
    {/each}
  </div>

  <ModeToggle
    {mode}
    isRecording={inputState === 'recording'}
    isTranscribing={inputState === 'transcribing'}
    onModeChange={handleModeChange}
  />

  <textarea aria-label={TEXT_INPUT_LABEL} bind:value={text}></textarea>

  <SendButton disabled={isSendButtonDisabled({ mode, text, inputState })} onSubmit={handleSubmit} />

  <!-- 常に単一インスタンス（INV-4）。toastSeq をキーにし、同一文言の再来でも再マウントして再表示する。 -->
  {#if toastMessage !== null}
    {#key toastSeq}
      <Toast message={toastMessage} />
    {/key}
  {/if}
</section>
