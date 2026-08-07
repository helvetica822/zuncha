<script lang="ts">
  // 送信ボタンの表示ラベル（テストの getByRole name 参照点）。
  const SEND_BUTTON_LABEL = '送信';

  export let disabled = false;
  export let onSubmit: () => void;

  // disabled時はネイティブにクリックが発火しないが、防御的にもガードする。
  function handleClick(): void {
    if (disabled) return;
    onSubmit();
  }
</script>

<button type="button" {disabled} on:click={handleClick}>{SEND_BUTTON_LABEL}</button>
