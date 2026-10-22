<script lang="ts">
  import { app } from "../lib/app.svelte";
  import { emojiOnly } from "../lib/match";
  import { formatTime } from "../lib/time";
  import { isImage, mediaURL, type BubbleAction, type Conversation, type Media, type Message } from "../lib/types";
  import { Go, openURL } from "../lib/go";
  import { isWebLink, linkify, mayPreview } from "../lib/links";
  import LinkChip from "./LinkChip.svelte";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";
  import { isPending } from "../lib/messages";

  let {
    conv,
    m,
    continues,
    groupEnd,
    quoted,
    active = null,
    onmenu,
    onaction,
  }: {
    conv: Conversation;
    m: Message;
    continues: boolean;
    groupEnd: boolean;
    /** The message this one replies to, when it's loaded. */
    quoted?: Message;
    /** Which of this message's popups is open: its hover buttons stay up. */
    active?: BubbleAction | null;
    onmenu: (e: MouseEvent, m: Message) => void;
    onaction: (kind: BubbleAction, e: MouseEvent, m: Message) => void;
  } = $props();

  // A send the phone hasn't taken yet has no id to react or reply to.
  const live = $derived(!isPending(m));

  const images = $derived((m.media ?? []).filter(isImage));
  const files = $derived((m.media ?? []).filter((md) => !isImage(md)));
  const big = $derived(!m.media?.length && (emojiOnly(m.text) ?? 99) <= 3);
  const failed = $derived(m.status === "failed");
  const showSender = $derived(conv.is_group && !m.from_me && !continues && !!m.sender);
  const statusLabel = $derived(
    ({ sending: "Sending…", sent: "Sent", delivered: "Delivered", read: "Read", failed: "Not sent" } as Record<string, string>)[
      m.status
    ] ?? "",
  );

  // Receipts on my messages: one grey check once sent, two once delivered,
  // two in the accent colour once read.
  const receipt = $derived(m.from_me && ["sending", "sent", "delivered", "read"].includes(m.status));
  const double = $derived(m.status === "delivered" || m.status === "read");

  function thumb(md: Media) {
    return md.thumb_id ? app.mediaState(md.thumb_id, md.thumb_key ?? "", md.mime) : app.mediaState(md.id, md.key, md.mime);
  }

  const segments = $derived(m.text ? linkify(m.text) : []);
  // A one-to-one chat is named after the other person; in a group, each
  // message has its sender.
  const previews = $derived(mayPreview(app.settings.link_previews, m.from_me, conv.is_group ? m.sender : conv.name));

  function openLink(e: MouseEvent, href: string) {
    // The webview must never navigate away from the app.
    e.preventDefault();
    openURL(href);
  }

  async function openFile(md: Media) {
    const s = app.mediaState(md.id, md.key, md.mime);
    if (s?.state === "ready") await Go.OpenPath(s.path);
    else app.showToast("Downloading…");
  }
</script>

{#if m.status === "system"}
  <p class="system subtle small">{m.text}</p>
{:else}
  <div class="row" class:mine={m.from_me} class:continues class:group={conv.is_group && !m.from_me}>
    {#if showSender}<div class="sender subtle small">{m.sender}</div>{/if}
    <div class="line">
      {#if conv.is_group && !m.from_me}
        <div class="avatar-slot">
          {#if groupEnd}<Avatar name={m.sender} color={m.sender_color} id={m.sender_id} size={30} />{/if}
        </div>
      {/if}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="bubble"
        class:big
        class:failed
        class:first={!continues}
        class:last={groupEnd}
        oncontextmenu={(e) => onmenu(e, m)}>
        {#each images as md, index (index)}
          {@const s = thumb(md)}
          {@const ratio = md.width && md.height ? md.height / md.width : 0.75}
          <button
            class="photo"
            style:aspect-ratio="1 / {Math.min(Math.max(ratio, 0.3), 1.6)}"
            onclick={() => app.openViewer(conv.id, m.id, index)}
            aria-label="Open photo">
            {#if s?.state === "ready"}
              <img src={mediaURL(s.path)} alt="" draggable="false" />
            {:else if s?.state === "failed"}
              <span class="subtle small">Photo unavailable</span>
            {:else}
              <div class="spinner"></div>
            {/if}
          </button>
        {/each}
        {#each files as md (md.id)}
          <button class="file" onclick={() => openFile(md)}>
            <Icon name="file" />
            <span>{md.name || md.mime}</span>
          </button>
        {/each}
        {#if m.reply_to && quoted}
          <div class="quote">
            <div class="who">{quoted.from_me ? "You" : quoted.sender || conv.name}</div>
            <div class="what">{quoted.text || (quoted.media?.length ? "📷 Photo" : "Message")}</div>
          </div>
        {/if}
        {#if m.text}
          <div class="text">{#each segments as seg, i (i)}{#if seg.href && previews && isWebLink(seg.href)}<LinkChip
                  href={seg.href} />{:else if seg.href}<a
                  href={seg.href}
                  title={seg.href}
                  rel="noreferrer"
                  draggable="false"
                  onclick={(e) => openLink(e, seg.href!)}>{seg.text}</a
                >{:else}{seg.text}{/if}{/each}</div>
        {/if}
        {#if m.reactions?.length}
          <div class="reactions">
            {#each m.reactions as r (r.emoji)}
              <span class:mine={r.mine}>{r.emoji}{r.count > 1 ? ` ${r.count}` : ""}</span>
            {/each}
          </div>
        {/if}
      </div>
      <!-- Toward the middle of the window: right of their bubbles, left of mine. -->
      <div class="actions" class:open={!!active}>
        {#if live && !conv.read_only}
          <button
            class="icon-button act"
            tabindex="-1"
            title="React"
            aria-label="React"
            aria-haspopup="menu"
            aria-expanded={active === "react"}
            onclick={(e) => onaction("react", e, m)}><Icon name="smile-plus" size={17} /></button>
          <button class="icon-button act" tabindex="-1" title="Reply" aria-label="Reply" onclick={(e) => onaction("reply", e, m)}>
            <Icon name="reply" size={17} />
          </button>
        {/if}
        <button
          class="icon-button act"
          tabindex="-1"
          title="More"
          aria-label="More"
          aria-haspopup="menu"
          aria-expanded={active === "more"}
          onclick={(e) => onaction("more", e, m)}><Icon name="more-vertical" size={17} /></button>
      </div>
    </div>
    {#if groupEnd || failed}
      <div class="meta small" class:danger={failed}>
        <span>{formatTime(m.timestamp)}{failed ? ` · ${statusLabel}` : ""}</span>
        {#if receipt}
          <span
            class="receipt"
            class:pending={m.status === "sending"}
            class:read={m.status === "read"}
            data-status={m.status}
            role="img"
            title={statusLabel}
            aria-label={statusLabel}><Icon name={double ? "receipt-two" : "receipt-one"} size={16} /></span>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<style>
  .system {
    text-align: center;
    margin: 12px 0;
  }
  .row {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    padding: 0 20px;
    margin-top: 10px;
  }
  .row.continues {
    margin-top: 2px;
  }
  .row.mine {
    align-items: flex-end;
  }
  .sender {
    margin: 0 0 3px 52px;
  }
  .line {
    position: relative;
    display: flex;
    align-items: flex-end;
    gap: 10px;
    max-width: min(72%, 600px);
  }
  /* The hover buttons sit beside the bubble, outside the line's width, so
     they never push the bubble around. */
  .actions {
    position: absolute;
    top: 50%;
    left: 100%;
    transform: translateY(-50%);
    display: flex;
    gap: 2px;
    padding: 0 6px;
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.12s ease-out;
  }
  .mine .actions {
    left: auto;
    right: 100%;
    flex-direction: row-reverse;
  }
  /* A narrow conversation has no room beside a wide bubble: the buttons
     float over the bubble's top corner instead, inside the row (rows clip
     what they paint, see .item in Thread.svelte). */
  @container thread (width < 420px) {
    .actions,
    .mine .actions {
      top: 0;
      transform: none;
      padding: 2px;
      border-radius: 10px;
      background: var(--bg);
      border: 1px solid var(--divider);
      box-shadow: 0 3px 12px var(--shadow);
      z-index: 2;
    }
    .actions {
      left: 0;
    }
    .mine .actions {
      left: auto;
      right: 0;
    }
  }
  .row:hover .actions,
  .actions.open {
    opacity: 1;
    pointer-events: auto;
  }
  .act {
    width: 30px;
    height: 30px;
  }
  .actions.open .act[aria-expanded="true"] {
    background: var(--hover);
    color: var(--text);
  }
  .quote {
    margin: 6px 6px 0;
    padding: 5px 10px;
    border-left: 3px solid currentColor;
    border-radius: 10px;
    background: color-mix(in srgb, currentColor 8%, transparent);
    font-size: 13px;
    line-height: 1.35;
  }
  .quote .who {
    font-weight: 600;
  }
  .quote .what {
    opacity: 0.8;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    overflow-wrap: anywhere;
  }
  .avatar-slot {
    width: 30px;
    flex: none;
  }
  .bubble {
    position: relative;
    background: var(--bubble-in);
    color: var(--text);
    border-radius: 18px;
    overflow: hidden;
    min-width: 0;
    user-select: text;
    -webkit-user-select: text;
    cursor: text;
  }
  .mine .bubble {
    background: var(--bubble-out);
    color: var(--on-bubble-out);
  }
  .row:not(.mine) .bubble:not(.first) {
    border-top-left-radius: 6px;
  }
  .row:not(.mine) .bubble:not(.last) {
    border-bottom-left-radius: 6px;
  }
  .mine .bubble:not(.first) {
    border-top-right-radius: 6px;
  }
  .mine .bubble:not(.last) {
    border-bottom-right-radius: 6px;
  }
  .bubble.big {
    background: transparent;
    font-size: 40px;
    line-height: 1.15;
  }
  .bubble.failed {
    opacity: 0.6;
  }
  .text {
    padding: 9px 14px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    line-height: 1.38;
  }
  .text a {
    color: inherit;
    text-decoration: underline;
    text-underline-offset: 2px;
    text-decoration-thickness: 1px;
    cursor: pointer;
  }
  .text a:hover {
    text-decoration-thickness: 2px;
  }
  .big .text {
    padding: 0 2px;
  }
  .photo {
    display: grid;
    place-items: center;
    width: 280px;
    max-width: 100%;
    background: var(--composer);
    cursor: zoom-in;
    margin-bottom: 2px;
  }
  .photo img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .file {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px;
    min-width: 200px;
    text-align: left;
  }
  .reactions {
    position: absolute;
    bottom: -18px;
    right: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 2px;
    /* A circle for one emoji, a pill with round ends for more. */
    box-sizing: border-box;
    height: 24px;
    min-width: 24px;
    padding: 0 4px;
    font-size: 14px;
    line-height: 1;
    background: var(--bg);
    border: 1px solid var(--divider);
    border-radius: 999px;
  }
  .line:has(.reactions) {
    margin-bottom: 22px;
  }
  .bubble:has(.reactions) {
    overflow: visible;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 4px;
    color: var(--subtle);
    margin: 3px 4px 0;
  }
  .receipt {
    display: inline-flex;
    transition: color 0.2s ease-out;
  }
  .receipt.pending {
    opacity: 0.5;
  }
  .receipt.read {
    color: var(--accent);
  }
  .group .meta {
    margin-left: 44px;
  }
  .meta.danger {
    color: var(--danger);
  }
</style>
