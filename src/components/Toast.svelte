<script lang="ts">
  import { onDestroy } from 'svelte';
  import { DEFAULT_TOAST_DURATION_MS } from '$lib/toast';

  export let message: string;
  export let durationMs = DEFAULT_TOAST_DURATION_MS;

  let visible = true;
  let timer: ReturnType<typeof setTimeout> | undefined;

  // message が変わるたびにタイマーを張り直す（上書き表示・多重化しない）。
  function startTimer(): void {
    if (timer !== undefined) clearTimeout(timer);
    visible = true;
    timer = setTimeout(() => {
      visible = false;
    }, durationMs);
  }

  $: message, startTimer();

  // アンマウント時にタイマーを破棄し、破棄後の状態更新警告を防ぐ（R-7）。
  onDestroy(() => {
    if (timer !== undefined) clearTimeout(timer);
  });
</script>

{#if visible}
  <div role="alert">{message}</div>
{/if}
