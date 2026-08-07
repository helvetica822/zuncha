<script lang="ts">
  import type { InputMode } from '$lib/inputMode';

  // モード切替ボタンの表示ラベル（テストの getByRole name 参照点）。
  const VOICE_LABEL = 'マイク';
  const TEXT_LABEL = 'テキスト';

  export let mode: InputMode;
  export let isRecording = false;
  export let isTranscribing = false;
  export let onModeChange: (mode: InputMode) => void;

  // 初期表示はmode propを反映し、クリックで内部選択状態を即時更新する（楽観的更新）。
  let selected: InputMode = mode;
  $: isDisabled = isRecording || isTranscribing;

  function selectMode(next: InputMode): void {
    selected = next;
    onModeChange(next);
  }
</script>

<div role="group" class="mode-toggle">
  <button
    type="button"
    aria-pressed={selected === 'voice'}
    disabled={isDisabled}
    on:click={() => selectMode('voice')}
  >{VOICE_LABEL}</button>
  <button
    type="button"
    aria-pressed={selected === 'text'}
    disabled={isDisabled}
    on:click={() => selectMode('text')}
  >{TEXT_LABEL}</button>
</div>

<style>
  .mode-toggle {
    display: inline-flex;
  }
</style>
