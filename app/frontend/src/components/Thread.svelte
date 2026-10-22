<script lang="ts">
  import { tick } from "svelte";
  import { app } from "../lib/app.svelte";
  import { dayLabel, sameDay } from "../lib/time";
  import type { BubbleAction, Conversation, Message } from "../lib/types";
  import { isPending } from "../lib/messages";
  import { anchored, atPoint, popup, type Anchor } from "../lib/popup";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import EmojiPicker from "./EmojiPicker.svelte";
  import Avatar from "./Avatar.svelte";
  import Bubble from "./Bubble.svelte";
  import Composer from "./Composer.svelte";
  import Icon from "./Icon.svelte";
  import ThemeButton from "./ThemeButton.svelte";
  import ListButtons from "./ListButtons.svelte";

  const QUICK_REACTIONS = ["❤️", "👍", "😂", "😮", "😢", "🙏"];

  let { conv }: { conv: Conversation } = $props();
  let list = $state<HTMLDivElement>();
  let content = $state<HTMLDivElement>();
  /** Follow the newest message: true at the end, false once scrolled up. */
  let pinned = true;
  /** The thread has reached its newest message since it opened; older pages wait until then. */
  let settled = $state(false);
  /** A message's popup: the quick reactions, every emoji, the ⋮ menu, or
   *  the right-click menu (reactions and actions together). `via` is where
   *  the full emoji picker was opened from. */
  let menu = $state<{ kind: "react" | "emoji" | "more" | "context"; anchor: Anchor; m: Message; via?: "react" | "context" } | null>(
    null,
  );
  /** A message waiting on the delete confirmation. */
  let deleting = $state<Message | null>(null);

  const thread = $derived(app.threads[conv.id]);
  const msgs = $derived(thread?.msgs ?? []);
  const byId = $derived(new Map(msgs.map((x) => [x.id, x])));

  const subtitle = $derived(
    conv.is_group && conv.members?.length ? conv.members.join(", ") : conv.is_rcs ? "RCS chat" : "SMS/MMS",
  );

  // A new conversation opens at its newest message.
  $effect(() => {
    void conv.id;
    pinned = true;
    lastTop = 0;
    lastHeight = 0;
    settled = false;
    menu = null;
    deleting = null;
  });

  function toBottom() {
    if (list) list.scrollTop = list.scrollHeight;
  }

  /** Scroll to the end now and again over the next frames: WebKit can size
   *  a new row (its time and receipts line) after the first layout. */
  function settleBottom() {
    toBottom();
    let frames = 3;
    const again = () => {
      if (!pinned) return;
      toBottom();
      if (--frames > 0) requestAnimationFrame(again);
    };
    requestAnimationFrame(again);
  }

  // Keep the end in view while rows lay out (photos load, pages arrive).
  $effect(() => {
    if (!content) return;
    const ro = new ResizeObserver(() => {
      if (pinned) settleBottom();
    });
    ro.observe(content);
    return () => ro.disconnect();
  });

  // Any change at the end: a new message, or a sent one swapping its
  // temporary id for the real one (a fresh row).
  $effect(() => {
    void msgs.length;
    void msgs[msgs.length - 1]?.id;
    if (thread?.loaded && pinned) {
      tick().then(() => {
        settleBottom();
        settled = true;
      });
    }
  });

  async function loadOlder() {
    if (!list || !thread?.more || thread.loading || !settled) return;
    const height = list.scrollHeight;
    const top = list.scrollTop;
    await app.loadMessages(conv.id, true);
    await tick();
    // Keep what was on screen in place (WebKit has no scroll anchoring).
    list.scrollTop = top + (list.scrollHeight - height);
  }

  let lastTop = 0;
  let lastHeight = 0;
  /** Our own smooth scroll to the end is running: it isn't the user leaving. */
  let autoScrolling = false;
  function onScroll() {
    if (!list) return;
    const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 32;
    // Only scrolling up lets go of the end. Rows growing below (photos,
    // content-visibility laying out) also leave us short of the bottom for a
    // moment, and the ResizeObserver catches up with those.
    if (atBottom) pinned = true;
    // A height change (a row resized or replaced) can pull scrollTop up by
    // itself; that isn't the user leaving either.
    else if (list.scrollTop < lastTop - 1 && list.scrollHeight === lastHeight && !autoScrolling) pinned = false;
    lastTop = list.scrollTop;
    lastHeight = list.scrollHeight;
    app.atBottom = pinned;
    if (pinned) app.unseenBelow = 0;
    if (list.scrollTop < 400) void loadOlder();
    menu = null;
  }

  // Page Up/Down, Home and End.
  $effect(() => {
    const cmd = app.threadScroll;
    if (!cmd || !list) return;
    const h = list.clientHeight * 0.85;
    if (cmd.kind === "page-up") list.scrollBy({ top: -h, behavior: "smooth" });
    if (cmd.kind === "page-down") list.scrollBy({ top: h, behavior: "smooth" });
    if (cmd.kind === "top") list.scrollTo({ top: 0, behavior: "smooth" });
    if (cmd.kind === "bottom") {
      // Rows growing on the way move the end; the smooth scroll still heads
      // for the old one, so land on the real end once it settles.
      pinned = true;
      autoScrolling = true;
      const el = list;
      const land = () => {
        if (!autoScrolling) return;
        autoScrolling = false;
        if (pinned) settleBottom();
      };
      el.addEventListener("scrollend", land, { once: true });
      setTimeout(land, 1000);
      el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
    }
  });

  // Reading the open conversation marks it read.
  $effect(() => {
    if (conv.unread && thread?.loaded) app.markReadIfVisible();
  });

  function openMenu(e: MouseEvent, m: Message) {
    // Leave the browser's own menu for selected text.
    if (window.getSelection()?.toString()) return;
    e.preventDefault();
    e.stopPropagation();
    menu = { kind: "context", anchor: atPoint(e.clientX, e.clientY), m };
  }

  function onAction(kind: BubbleAction, e: MouseEvent, m: Message) {
    e.stopPropagation();
    if (kind === "reply") {
      menu = null;
      app.startReply(m);
      return;
    }
    if (menu?.m.id === m.id && menu.kind === kind) {
      menu = null;
      return;
    }
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    // Reactions above the button; the menu below it, opening away from the bubble.
    menu =
      kind === "react"
        ? { kind, m, anchor: { rect, side: "above", align: "center" } }
        : { kind, m, anchor: { rect, side: "below", align: m.from_me ? "end" : "start" } };
  }

  function react(m: Message, emoji: string, mine: boolean) {
    menu = null;
    app.react(m, emoji, mine);
  }

  /** The last button of the quick reactions: every emoji, in the same place. */
  function moreReactions() {
    if (!menu) return;
    const via = menu.kind === "context" ? "context" : "react";
    menu = { ...menu, kind: "emoji", via };
  }

  function reactAny(m: Message, emoji: string) {
    if (!emoji) return;
    react(m, emoji, (m.reactions ?? []).some((r) => r.emoji === emoji && r.mine));
  }

  function confirmDelete() {
    const m = deleting;
    deleting = null;
    if (m) void app.deleteMessage(m);
  }

  function editAndResend(m: Message) {
    app.drafts[m.conversation_id] = m.text;
    const t = app.threads[m.conversation_id];
    if (t) t.msgs = t.msgs.filter((x) => x.id !== m.id);
    app.focusComposer++;
  }

  const continues = (i: number) => {
    const p = msgs[i - 1];
    const m = msgs[i];
    return (
      !!p &&
      p.from_me === m.from_me &&
      p.sender === m.sender &&
      p.status !== "system" &&
      m.timestamp - p.timestamp < 180000 &&
      sameDay(p.timestamp, m.timestamp)
    );
  };
  const groupEnd = (i: number) => {
    const n = msgs[i + 1];
    const m = msgs[i];
    return !(n && n.from_me === m.from_me && n.sender === m.sender && n.timestamp - m.timestamp < 180000);
  };
</script>

<svelte:window onfocus={() => app.markReadIfVisible()} onclick={() => (menu = null)} />

<section class="thread">
  <header class="head drag">
    {#if !app.settings.sidebar_visible}<ListButtons />{/if}
    <Avatar name={conv.name} color={conv.color} group={conv.is_group} id={conv.avatar_id} size={38} />
    <div class="title">
      <div class="name">{conv.name}</div>
      <div class="subtle small sub">{subtitle}</div>
    </div>
    <div class="theme" class:toolbar={app.settings.sidebar_visible}><ThemeButton /></div>
  </header>

  <div class="list" bind:this={list} onscroll={onScroll}>
    <div bind:this={content}>
      {#if !thread?.loaded}
        <div class="center"><div class="spinner"></div></div>
      {:else}
        <div class="top">
          {#if thread.loading && thread.older}
            <div class="spinner"></div>
          {:else if !thread.more && msgs.length}
            <span class="subtle small">Beginning of conversation</span>
          {/if}
        </div>
        {#each msgs as m, i (m.id)}
          <div class="item" class:tail={i >= msgs.length - 10}>
            {#if i === 0 || !sameDay(msgs[i - 1].timestamp, m.timestamp)}
              <p class="day subtle small">{dayLabel(m.timestamp)}</p>
            {/if}
            <Bubble
              {conv}
              {m}
              continues={continues(i)}
              groupEnd={groupEnd(i)}
              quoted={m.reply_to ? byId.get(m.reply_to) : undefined}
              active={menu?.m.id === m.id ? (menu.kind === "emoji" ? (menu.via === "react" ? "react" : null) : menu.kind === "context" ? null : menu.kind) : null}
              onmenu={openMenu}
              onaction={onAction} />
          </div>
        {/each}
        {#if app.typing[conv.id]}
          <div class="typing" class:group={conv.is_group}><span></span><span></span><span></span></div>
        {/if}
        <div class="end"></div>
      {/if}
    </div>
  </div>

  {#if !app.atBottom && thread?.loaded}
    <button class="to-latest" title={app.unseenBelow ? "New messages  End" : "Jump to latest  End"} onclick={() => app.scrollThread("bottom")}>
      <Icon name="arrow-down" size={20} />
      {#if app.unseenBelow}<span class="badge">{Math.min(app.unseenBelow, 99)}</span>{/if}
    </button>
  {/if}

  <Composer {conv} />

  {#if menu?.kind === "emoji"}
    {@const m = menu.m}
    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
    <div class="emoji-popup" use:anchored={menu.anchor} onclick={(e) => e.stopPropagation()}>
      <EmojiPicker onpick={(e) => reactAny(m, e)} onclose={() => (menu = null)} />
    </div>
  {:else if menu}
    {@const m = menu.m}
    {@const live = !isPending(m)}
    {@const reactions = menu.kind !== "more" && live && !conv.read_only}
    {@const actions = menu.kind !== "react"}
    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
    <div
      class="menu msg-menu"
      class:picker={!actions}
      role="menu"
      tabindex="-1"
      aria-label={actions ? "Message options" : "React"}
      use:anchored={menu.anchor}
      use:popup={{ onclose: () => (menu = null) }}
      onclick={(e) => e.stopPropagation()}>
      {#if reactions}
        <div class="reactions" role="group" aria-label="Reactions">
          {#each QUICK_REACTIONS as e (e)}
            {@const mine = (m.reactions ?? []).some((r) => r.emoji === e && r.mine)}
            <button class:mine role="menuitemcheckbox" aria-checked={mine} aria-label="React {e}" onclick={() => react(m, e, mine)}>{e}</button>
          {/each}
          <button class="any" role="menuitem" aria-haspopup="dialog" aria-label="More reactions" title="More reactions" onclick={moreReactions}>
            <Icon name="smile-plus" size={22} />
          </button>
        </div>
        {#if actions}<hr />{/if}
      {/if}
      {#if actions}
        {#if menu.kind === "context" && live && !conv.read_only}
          <button class="item" role="menuitem" onclick={() => { app.startReply(m); menu = null; }}><Icon name="reply" />Reply</button>
        {/if}
        {#if m.text}
          <button class="item" role="menuitem" onclick={() => { void app.copyMessage(m); menu = null; }}><Icon name="copy" />Copy</button>
        {/if}
        {#if m.status === "failed"}
          <button class="item" role="menuitem" onclick={() => { editAndResend(m); menu = null; }}><Icon name="new-chat" />Edit and resend</button>
        {/if}
        <button class="item danger" role="menuitem" onclick={() => { deleting = m; menu = null; }}><Icon name="trash" />Delete</button>
      {/if}
    </div>
  {/if}

  {#if deleting}
    <ConfirmDialog
      title="Delete this message?"
      body={isPending(deleting)
        ? "It wasn't sent, so it only leaves this window."
        : "It's deleted from your phone. Others in the conversation keep their copy."}
      confirmLabel="Delete"
      onconfirm={confirmDelete}
      oncancel={() => (deleting = null)} />
  {/if}
</section>

<style>
  .thread {
    position: relative;
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    background: var(--pane);
  }
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: calc(var(--top-inset) + 10px) 16px 10px 18px;
    border-bottom: 1px solid var(--divider);
  }
  .title {
    min-width: 0;
  }
  .theme {
    display: flex;
    margin-left: auto;
  }
  /* On the line of the list's buttons (its header pads 14px, this one 10px). */
  .theme.toolbar {
    align-self: flex-start;
    margin-top: 4px;
  }
  .name {
    font-size: 16.5px;
  }
  .sub {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .list {
    /* Bubble.svelte moves the hover buttons when this gets narrow. */
    container: thread / inline-size;
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    overflow-anchor: none;
    /* The scrollbar takes 10px on the right; reserve the same on the left so
       both sides of the conversation get equal margins. */
    scrollbar-gutter: stable both-edges;
  }
  .item {
    /* Rows off screen skip layout and paint: long threads stay fast. */
    content-visibility: auto;
    contain-intrinsic-size: auto 64px;
  }
  /* The newest rows always lay out for real: a placeholder height there
     leaves the end of the thread short when a message arrives. */
  .item.tail {
    content-visibility: visible;
  }
  .center,
  .top {
    display: grid;
    place-items: center;
    min-height: 44px;
    padding-top: 12px;
  }
  .center {
    height: 100%;
    min-height: 300px;
  }
  .day {
    text-align: center;
    margin: 16px 0 6px;
  }
  .end {
    height: 12px;
  }
  .typing {
    display: inline-flex;
    gap: 5px;
    margin: 10px 20px;
    padding: 12px 16px;
    background: var(--bubble-in);
    border-radius: 18px;
  }
  .typing.group {
    margin-left: 60px;
  }
  .typing span {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--subtle);
    animation: bounce 1.2s infinite;
  }
  .typing span:nth-child(2) {
    animation-delay: 0.15s;
  }
  .typing span:nth-child(3) {
    animation-delay: 0.3s;
  }
  @keyframes bounce {
    0%,
    60%,
    100% {
      transform: translateY(0);
      opacity: 0.5;
    }
    30% {
      transform: translateY(-3px);
      opacity: 1;
    }
  }
  .to-latest {
    position: absolute;
    right: 22px;
    bottom: 92px;
    width: 42px;
    height: 42px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--bg);
    border: 1px solid var(--divider);
    box-shadow: 0 3px 12px var(--shadow);
    z-index: 3;
  }
  .to-latest:hover {
    background: var(--hover);
  }
  .badge {
    position: absolute;
    top: -6px;
    right: -6px;
    min-width: 19px;
    height: 19px;
    padding: 0 5px;
    border-radius: 10px;
    background: var(--accent);
    color: white;
    font-size: 11px;
    line-height: 19px;
    border: 2px solid var(--bg);
  }
  .emoji-popup {
    position: fixed;
    z-index: 50;
  }
  .reactions button.any {
    display: grid;
    place-items: center;
    color: var(--subtle);
  }
  .reactions button.any:hover,
  .reactions button.any:focus-visible {
    color: var(--text);
  }
  .msg-menu.picker {
    min-width: 0;
    padding: 4px;
    border-radius: 999px;
  }
  .reactions {
    display: flex;
    gap: 2px;
    padding: 2px;
  }
  .reactions button {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    font-size: 20px;
  }
  .reactions button:hover,
  .reactions button:focus-visible {
    background: var(--hover);
  }
  .reactions button.mine {
    background: var(--selected);
  }
</style>
