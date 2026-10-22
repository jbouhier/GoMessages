<script lang="ts">
  import { app } from "../lib/app.svelte";
  import { isMac } from "../lib/platform";
  import Avatar from "./Avatar.svelte";

  let input = $state<HTMLInputElement>();
  let list = $state<HTMLDivElement>();
  $effect(() => input?.focus());

  const sw = $derived(app.switcher!);
  const matches = $derived(app.switcherMatches);

  // Keep the keyboard selection visible when the list scrolls (short windows).
  $effect(() => {
    const i = sw.index;
    (list?.children[i] as HTMLElement | undefined)?.scrollIntoView({ block: "nearest" });
  });

  function pick(id: string) {
    app.switcher = null;
    app.select(id);
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "ArrowDown") sw.index = Math.min(sw.index + 1, matches.length - 1);
    else if (e.key === "ArrowUp") sw.index = Math.max(sw.index - 1, 0);
    else if (e.key === "Enter" && matches[sw.index]) pick(matches[sw.index].id);
    else if (e.key === "Escape") {
      app.switcher = null;
      app.focusComposer++;
    } else return;
    e.preventDefault();
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="backdrop" onclick={() => (app.switcher = null)}></div>
<div class="switcher" role="dialog" aria-label="Switch conversation">
  <div class="field">
    <input
      bind:this={input}
      bind:value={sw.query}
      oninput={() => (sw.index = 0)}
      onkeydown={onKey}
      placeholder="Jump to a conversation…" />
  </div>
  {#if !matches.length}
    <p class="none subtle">No conversation matches</p>
  {/if}
  <div class="list" bind:this={list}>
  {#each matches as c, i (c.id)}
    <button class="row" class:active={i === sw.index} onclick={() => pick(c.id)} onmouseenter={() => (sw.index = i)}>
      <Avatar name={c.name} color={c.color} group={c.is_group} id={c.avatar_id} size={34} />
      <div class="text">
        <div class="name">{c.name}</div>
        <div class="snippet subtle small">{c.snippet.replace(/\n/g, " ")}</div>
      </div>
      {#if c.unread}<span class="dot"></span>{/if}
      {#if !sw.query && i < 9}<span class="hint subtle small">{isMac ? "⌘" : "Ctrl+"}{i + 1}</span>{/if}
    </button>
  {/each}
  </div>
  <p class="foot subtle small">↑↓ to move · Enter to open · Esc to close</p>
</div>

<style>
  .switcher {
    position: fixed;
    z-index: 41;
    --top: clamp(12px, 10vh, 90px);
    top: var(--top);
    left: 50%;
    transform: translateX(-50%);
    width: min(540px, calc(100vw - 40px));
    max-height: calc(100vh - var(--top) - 12px);
    display: flex;
    flex-direction: column;
    padding: 10px;
    background: var(--bg);
    border: 1px solid var(--divider);
    border-radius: 16px;
    box-shadow: 0 12px 40px var(--shadow);
  }
  .field {
    flex-shrink: 0;
    background: var(--composer);
    border-radius: 12px;
    padding: 11px 14px;
    margin-bottom: 6px;
  }
  .field input {
    width: 100%;
    font-size: 17px;
  }
  .list {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 8px 10px;
    border-radius: var(--row-radius);
    text-align: left;
  }
  .row.active {
    background: var(--selected);
  }
  .text {
    flex: 1;
    min-width: 0;
  }
  .name,
  .snippet {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--accent);
  }
  .none {
    text-align: center;
  }
  .foot {
    flex-shrink: 0;
    margin: 6px 8px 2px;
  }
  @media (max-height: 500px) {
    .foot {
      display: none;
    }
  }
</style>
