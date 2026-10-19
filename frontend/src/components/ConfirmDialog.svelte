<script lang="ts">
  // A confirmation for something that can't be undone. Cancel has focus when
  // it opens, so Enter or Space on a stray keypress keeps things as they are;
  // Tab or the arrows reach the destructive button.
  import { popup } from "../lib/popup";

  let {
    title,
    body,
    confirmLabel,
    onconfirm,
    oncancel,
  }: {
    title: string;
    body: string;
    confirmLabel: string;
    onconfirm: () => void;
    oncancel: () => void;
  } = $props();

  const id = `confirm-${Math.random().toString(36).slice(2, 8)}`;
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="backdrop" onclick={oncancel}></div>
<div
  class="confirm-sheet"
  role="alertdialog"
  aria-modal="true"
  aria-labelledby="{id}-title"
  aria-describedby="{id}-body"
  use:popup={{ onclose: oncancel, kind: "dialog" }}>
  <h2 id="{id}-title">{title}</h2>
  <p id="{id}-body" class="subtle">{body}</p>
  <div class="confirm-buttons">
    <button class="secondary" data-autofocus onclick={oncancel}>Cancel</button>
    <button class="secondary danger-fill" onclick={onconfirm}>{confirmLabel}</button>
  </div>
</div>

<style>
  .confirm-sheet {
    position: fixed;
    z-index: 41;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: min(400px, calc(100vw - 40px));
    padding: 22px 22px 18px;
    background: var(--bg);
    border: 1px solid var(--divider);
    border-radius: 16px;
    box-shadow: 0 12px 40px var(--shadow);
  }
  h2 {
    margin: 0 0 8px;
    font-size: 19px;
    font-weight: 400;
    overflow-wrap: anywhere;
  }
  p {
    margin: 0;
    font-size: 14px;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }
  .confirm-buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 20px;
  }
  .danger-fill {
    background: var(--danger);
    color: var(--bg);
  }
  .danger-fill:hover {
    background: color-mix(in srgb, var(--danger) 85%, black);
  }
</style>
