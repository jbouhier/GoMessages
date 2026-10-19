<script lang="ts">
  import { app } from "../lib/app.svelte";
  import { section } from "../lib/match";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";

  let input = $state<HTMLInputElement>();
  let listEl = $state<HTMLDivElement>();
  $effect(() => input?.focus());

  const nc = $derived(app.newChat!);
  const matches = $derived(app.contactMatches(nc.query));
  // Typed digits can always be texted, contact or not.
  const typedNumber = $derived(/^[+\d][\d\s().-]{2,}$/.test(nc.query.trim()) ? nc.query.trim() : null);

  function onKey(e: KeyboardEvent) {
    if (e.key === "ArrowDown") nc.index = Math.min(nc.index + 1, matches.length - 1);
    else if (e.key === "ArrowUp") nc.index = Math.max(nc.index - 1, 0);
    else if (e.key === "Enter") {
      const c = matches[nc.index];
      void app.startChatWithNumber(c && !typedNumber ? c.number : nc.query);
    } else if (e.key === "Escape") {
      app.newChat = null;
      app.focusComposer++;
    } else return;
    e.preventDefault();
    listEl?.querySelector(".row.active")?.scrollIntoView({ block: "nearest" });
  }
</script>

<section class="page">
  <header class="drag">
    <button class="icon-button" title="Back  Esc" onclick={() => (app.newChat = null)}><Icon name="arrow-left" size={19} /></button>
    <h1>New chat</h1>
  </header>
  <div class="top">
    <p class="subtle small">Search contacts or enter a phone number</p>
    <div class="field">
      <input
        bind:this={input}
        bind:value={nc.query}
        disabled={nc.busy}
        oninput={() => (nc.index = 0)}
        onkeydown={onKey}
        placeholder="Name or phone number" />
    </div>
    {#if nc.error}<p class="error small">{nc.error}</p>{/if}
    {#if nc.busy}<div class="spinner"></div>{/if}
  </div>
  <div class="list" bind:this={listEl}>
    {#if typedNumber}
      <button class="row" onclick={() => app.startChatWithNumber(typedNumber)}>
        <span class="num-icon"><Icon name="new-chat" /></span>
        <div><div>Text {typedNumber}</div></div>
      </button>
    {/if}
    {#if app.contactsLoading}
      <p class="center subtle">Loading phone contacts…</p>
    {:else if app.contactsError}
      <p class="center error">{app.contactsError}</p>
    {:else if !matches.length}
      <p class="center subtle">{nc.query ? "No matching contacts" : "No contacts available"}</p>
    {/if}
    {#each matches as c, i (c.number + c.name)}
      {#if !nc.query && (i === 0 || section(matches[i - 1].name) !== section(c.name))}
        {#if i > 0}<hr />{/if}
        <h3>{section(c.name)}</h3>
      {/if}
      <button
        class="row"
        class:active={i === nc.index}
        aria-label="{c.name} · {c.formatted_number}"
        onmouseenter={() => (nc.index = i)}
        onclick={() => app.startChatWithNumber(c.number)}>
        <Avatar name={c.name} id={c.avatar_id} size={40} />
        <div>
          <div>{c.name}</div>
          <div class="subtle small">{c.formatted_number || c.number}</div>
        </div>
      </button>
    {/each}
  </div>
</section>

<style>
  .page {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    background: var(--pane);
  }
  header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: calc(var(--top-inset) + 12px) 16px 10px 12px;
    border-bottom: 1px solid var(--divider);
  }
  h1 {
    margin: 0;
    font-size: 22px;
    font-weight: 400;
  }
  .top,
  .list {
    width: min(900px, calc(100% - 64px));
    margin: 0 auto;
  }
  .top {
    padding-top: 18px;
  }
  .field {
    background: var(--composer);
    border-radius: 10px;
    padding: 11px 14px;
    margin: 10px 0;
  }
  .field input {
    width: 100%;
    font-size: 16px;
  }
  .list {
    flex: 1;
    overflow-y: auto;
    padding-bottom: 20px;
  }
  h3 {
    margin: 12px 16px 4px;
    font-size: 14px;
  }
  hr {
    border: none;
    border-top: 1px solid var(--divider);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 14px;
    width: 100%;
    padding: 8px 16px;
    border-radius: var(--row-radius);
    text-align: left;
  }
  .row.active,
  .row:hover {
    background: var(--selected);
  }
  .num-icon {
    width: 40px;
    display: grid;
    place-items: center;
    color: var(--accent);
  }
  .center {
    text-align: center;
  }
  .error {
    color: var(--danger);
  }
</style>
