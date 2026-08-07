<script lang="ts">
  import type { Emotion } from '$lib/sseEventRouter';

  // アシスタント発話を示す data-role の値（テストの参照点）。
  const ASSISTANT_ROLE = 'assistant';

  export let text: string;
  export let emotion: Emotion;
</script>

<div class="message-bubble" data-role={ASSISTANT_ROLE} data-emotion={emotion}>
  <p class="message-bubble__text">{text}</p>
</div>
