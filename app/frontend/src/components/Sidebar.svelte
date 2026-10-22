<script lang="ts">
  // The conversation list: full, or photos only when dragged narrow. Its
  // right edge resizes it; past the narrow end it snaps to photos only.
  import { app, MIN_WIDE_WIDTH, snapWidth } from "../lib/app.svelte";
  import { shortTime } from "../lib/time";
  import type { Conversation } from "../lib/types";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";
  import ListButtons from "./ListButtons.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import { anchored, atPoint, popup, type Anchor } from "../lib/popup";

  let dragWidth = $state<number | null>(null);
  const width = $derived(dragWidth !== null ? snapWidth(dragWidth) : app.settings.sidebar_width);
  const compact = $derived(width < MIN_WIDE_WIDTH);
  let searchInput = $state<HTMLInputElement>();
  /** A conversation's ⋮ menu. */
  let menu = $state<{ anchor: Anchor; c: Conversation } | null>(null);
  /** Trash or block waiting on the confirmation popup. */
  let confirm = $state<{ c: Conversation; action: "delete" | "block_spam" } | null>(null);

  const MENU_W = 236;
  function openMenu(e: MouseEvent, c: Conversation) {
    e.preventDefault();
    e.stopPropagation();
    if (menu?.c.id === c.id && e.type === "click") {
      menu = null;
      return;
    }
    // From the ⋮ button: under it, right edges lined up. Right-click: at the pointer.
    menu =
      e.type === "click"
        ? { anchor: { rect: (e.currentTarget as HTMLElement).getBoundingClientRect(), align: "end" }, c }
        : { anchor: atPoint(e.clientX, e.clientY), c };
  }

  function run(action: Parameters<typeof app.convAction>[1]) {
    if (!menu) return;
    const c = menu.c;
    menu = null;
    if (action === "delete" || action === "block_spam") confirm = { c, action };
    else void app.convAction(c, action);
  }

  function confirmed() {
    if (!confirm) return;
    const { c, action } = confirm;
    confirm = null;
    void app.convAction(c, action);
  }

  $effect(() => {
    if (app.focusSearch) searchInput?.focus();
  });

  function startDrag(e: PointerEvent) {
    const startX = e.clientX;
    const startW = app.settings.sidebar_width;
    dragWidth = startW;
    const move = (ev: PointerEvent) => (dragWidth = startW + ev.clientX - startX);
    const up = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      const w = snapWidth(dragWidth ?? startW);
      app.settings.sidebar_width = w;
      if (w >= MIN_WIDE_WIDTH) app.settings.sidebar_wide_width = w;
      dragWidth = null;
      app.saveSettings();
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  }

  function toggleCompact() {
    app.act("photos-only");
  }

  function searchKeys(e: KeyboardEvent) {
    if (e.key === "Escape") {
      app.search = "";
      app.focusComposer++;
    } else if (e.key === "Enter" && app.visible[0]) {
      app.select(app.visible[0].id);
      app.search = "";
    }
  }

  const banner = $derived(
    app.link === "connecting"
      ? "Connecting to your phone…"
      : app.link === "phone_offline"
        ? "Your phone isn't responding. Check that it's on and online."
        : null,
  );
</script>

<svelte:window onclick={() => (menu = null)} onblur={() => (menu = null)} />

<aside class="sidebar" style:width="{width}px">
  {#if compact}
    <div class="compact-head drag">
      <ListButtons vertical />
    </div>
    <div class="list">
      {#each app.visible as c (c.id)}
        <button
          class="cell"
          class:selected={app.selected === c.id}
          aria-label={c.name}
          title={c.snippet ? `${c.name}\n${c.snippet.replace(/\n/g, " ")}` : c.name}
          onclick={() => app.select(c.id)}
          oncontextmenu={(e) => openMenu(e, c)}>
          <Avatar name={c.name} color={c.color} group={c.is_group} id={c.avatar_id} size={42} />
          {#if c.unread}<span class="dot"></span>{/if}
        </button>
      {/each}
    </div>
  {:else}
    <header class="head drag">
      <h1>{app.showArchived ? "Archived" : "Messages"}</h1>
      {#if app.convsLoading}<div class="spinner small-spin"></div>{/if}
      <ListButtons />
    </header>
    <div class="search glass">
      <input
        bind:this={searchInput}
        bind:value={app.search}
        placeholder="Search conversations"
        onkeydown={searchKeys} />
    </div>
    {#if banner}
      <div class="banner">
        {#if app.link === "connecting"}<div class="spinner small-spin"></div>{/if}
        <span>{banner}</span>
      </div>
    {/if}
    <div class="list">
      {#if !app.visible.length && !app.convsLoading}
        <p class="empty subtle">
          {app.search ? "No matches" : app.showArchived ? "No archived conversations" : "No conversations yet"}
        </p>
      {/if}
      {#each app.visible as c (c.id)}
        {@const typing = !!app.typing[c.id]}
        <div class="row-wrap" class:menu-open={menu?.c.id === c.id}>
        <button
          class="row"
          class:selected={app.selected === c.id}
          class:unread={c.unread}
          aria-label={c.name}
          onclick={() => app.select(c.id)}
          oncontextmenu={(e) => openMenu(e, c)}>
          <Avatar name={c.name} color={c.color} group={c.is_group} id={c.avatar_id} size={42} />
          <div class="text">
            <div class="line">
              <span class="name">{c.name}</span>
              <span class="time">{shortTime(c.timestamp)}</span>
            </div>
            <div class="line">
              <span class="snippet" class:typing>
                {typing ? "typing…" : (c.snippet_from_me ? "You: " : "") + c.snippet.replace(/\n/g, " ")}
              </span>
              {#if app.isMuted(c.id)}<span class="muted" title="Muted"><Icon name="bell-off" size={14} /></span>{/if}
              {#if c.unread}<span class="dot inline"></span>{/if}
            </div>
          </div>
        </button>
        <button class="more icon-button" title="More options" aria-label="More options for {c.name}" onclick={(e) => openMenu(e, c)}>
          <Icon name="more-vertical" size={18} />
        </button>
        </div>
      {/each}
      <button class="row archived" onclick={() => app.act("archived")}>
        <span class="archived-icon"><Icon name={app.showArchived ? "chats" : "archive"} /></span>
        <span class="subtle">{app.showArchived ? "Back to chats" : "Archived conversations"}</span>
      </button>
    </div>
  {/if}
  {#if menu}
    {@const c = menu.c}
    {@const muted = app.isMuted(c.id)}
    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
    <div
      class="menu conv-menu"
      role="menu"
      tabindex="-1"
      aria-label="Options for {c.name}"
      style:width="{MENU_W}px"
      use:anchored={menu.anchor}
      use:popup={{ onclose: () => (menu = null) }}
      onclick={(e) => e.stopPropagation()}>
      <button class="item" role="menuitem" onclick={() => run(c.archived ? "unarchive" : "archive")}>
        <Icon name="archive" />{c.archived ? "Unarchive" : "Archive"}
      </button>
      <button class="item" role="menuitem" onclick={() => run(muted ? "unmute" : "mute")}>
        <Icon name={muted ? "bell" : "bell-off"} />{muted ? "Unmute" : "Mute"}
      </button>
      <button class="item" role="menuitem" onclick={() => run(c.unread ? "read" : "unread")}>
        <Icon name={c.unread ? "mail-open" : "mail-dot"} />{c.unread ? "Mark as read" : "Mark as unread"}
      </button>
      <hr />
      <button class="item danger" role="menuitem" onclick={() => run("block_spam")}>
        <Icon name="ban" />Block and report spam
      </button>
      <button class="item danger" role="menuitem" onclick={() => run("delete")}>
        <Icon name="trash" />Trash
      </button>
    </div>
  {/if}
  {#if confirm}
    {@const del = confirm.action === "delete"}
    <ConfirmDialog
      title={del ? "Delete this conversation?" : `Block ${confirm.c.name}?`}
      body={del
        ? `Your conversation with ${confirm.c.name} moves to trash on your phone.`
        : "You won't get messages from them. The conversation is reported as spam and moves to spam on your phone."}
      confirmLabel={del ? "Delete" : "Block and report"}
      onconfirm={confirmed}
      oncancel={() => (confirm = null)} />
  {/if}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="divider" onpointerdown={startDrag} ondblclick={toggleCompact} title="Drag to resize · double-click for photos only"></div>
</aside>

<style>
  .sidebar {
    position: relative;
    flex: none;
    display: flex;
    flex-direction: column;
    background: var(--pane-list);
    min-width: 0;
    /* Zoomed in, the list gives way so the conversation still fits. */
    max-width: 45vw;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: calc(var(--top-inset) + 14px) 12px 8px 20px;
  }
  h1 {
    flex: 1;
    margin: 0;
    font-size: 22px;
    font-weight: 400;
  }
  .compact-head {
    padding: calc(var(--top-inset) + 10px) 0 6px;
    display: flex;
    justify-content: center;
  }
  .small-spin {
    width: 16px;
    height: 16px;
    border-width: 2px;
  }
  .search {
    margin: 4px 14px 8px;
    background: var(--bg);
    border-radius: 24px;
    padding: 9px 14px;
  }
  .search input {
    width: 100%;
  }
  .banner {
    display: flex;
    gap: 8px;
    align-items: center;
    margin: 0 12px 8px;
    padding: 8px 12px;
    border-radius: 12px;
    background: var(--warn-bg);
  }
  .list {
    flex: 1;
    overflow-y: auto;
    padding-bottom: 12px;
  }
  .empty {
    text-align: center;
    margin-top: 40px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 14px;
    width: calc(100% - 16px);
    margin: 2px 8px;
    padding: 12px 12px;
    border-radius: var(--row-radius);
    text-align: left;
  }
  .row-wrap {
    position: relative;
  }
  .row:hover,
  .row-wrap.menu-open .row {
    background: var(--hover);
  }
  .more {
    position: absolute;
    top: 50%;
    right: 14px;
    transform: translateY(-50%);
    width: 32px;
    height: 32px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    color: var(--subtle);
    opacity: 0;
    pointer-events: none;
  }
  .row-wrap:hover .more,
  .row-wrap.menu-open .more,
  .more:focus-visible {
    opacity: 1;
    pointer-events: auto;
  }
  .more:hover,
  .row-wrap.menu-open .more {
    background: var(--selected);
    color: var(--text);
  }
  /* Make room for ⋮ so it never covers the time. */
  .row-wrap:hover .text,
  .row-wrap.menu-open .text {
    padding-right: 34px;
  }
  .muted {
    display: grid;
    color: var(--subtle);
  }
  .row.selected {
    background: var(--selected);
  }
  .text {
    flex: 1;
    min-width: 0;
  }
  .line {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .name {
    flex: 1;
    font-size: 15px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .unread .name {
    font-weight: 600;
  }
  .time {
    font-size: 12px;
    color: var(--subtle);
  }
  .unread .time {
    color: var(--accent);
  }
  .snippet {
    flex: 1;
    font-size: 13.5px;
    color: var(--subtle);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .unread .snippet {
    color: var(--text);
  }
  .snippet.typing {
    color: var(--accent);
  }
  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--accent);
    flex: none;
  }
  .cell {
    position: relative;
    display: grid;
    place-items: center;
    width: 56px;
    height: 54px;
    margin: 2px auto;
    border-radius: var(--row-radius);
  }
  .cell:hover {
    background: var(--hover);
  }
  .cell.selected {
    background: var(--selected);
  }
  .cell .dot {
    position: absolute;
    top: 6px;
    right: 8px;
    border: 2px solid var(--sidebar);
    width: 13px;
    height: 13px;
  }
  .archived {
    margin-top: 6px;
  }
  .archived-icon {
    width: 42px;
    display: grid;
    place-items: center;
    color: var(--subtle);
  }
  .divider {
    position: absolute;
    top: 0;
    right: -4px;
    width: 8px;
    height: 100%;
    cursor: col-resize;
    z-index: 5;
  }
  .divider:hover {
    background: linear-gradient(to right, transparent 3px, color-mix(in srgb, var(--accent) 60%, transparent) 3px, color-mix(in srgb, var(--accent) 60%, transparent) 5px, transparent 5px);
  }
</style>
