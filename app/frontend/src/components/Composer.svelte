<script lang="ts">
  import { app } from "../lib/app.svelte";
  import { keys, commandKey } from "../lib/platform";
  import type { Conversation } from "../lib/types";
  import Icon from "./Icon.svelte";
  import { popup } from "../lib/popup";
  import { attachmentKind, attachmentURL, fileName } from "../lib/attachments";

  let { conv }: { conv: Conversation } = $props();
  let input = $state<HTMLTextAreaElement>();
  let simMenu = $state(false);
  /** Previews that didn't load (a format the web view can't show, or a file
   *  gone since): those show their name instead. */
  let unpreviewable = $state<Record<string, boolean>>({});

  $effect(() => {
    if (app.focusComposer) input?.focus();
  });

  const draft = $derived(app.drafts[conv.id] ?? "");
  const replying = $derived(app.replyTo[conv.id]);
  const canSend = $derived(!!draft.trim() || app.attachments.length > 0);
  const currentSim = $derived(app.selectedSims[conv.id] ?? conv.sim_number);
  const simLabel = $derived(app.sims.find((s) => s.number === currentSim)?.label);

  // Grow with the text up to a few lines.
  $effect(() => {
    void draft;
    if (!input) return;
    input.style.height = "auto";
    input.style.height = `${Math.min(input.scrollHeight, 160)}px`;
    // Scrolling only once the text outgrows the box: a pixel of rounding
    // otherwise brings up a scroll bar on a single line.
    input.style.overflowY = input.scrollHeight > 160 ? "auto" : "hidden";
  });

  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape" && replying) {
      // First Escape drops the reply, the next one is the app's.
      e.preventDefault();
      e.stopPropagation();
      delete app.replyTo[conv.id];
      return;
    }
    if (e.key !== "Enter" || e.isComposing) return;
    // Enter sends (Shift+Enter for a new line), or ⌘Enter when the setting
    // is off and Enter writes new lines.
    const send = app.settings.enter_sends ? !e.shiftKey && !e.altKey && !commandKey(e) : commandKey(e);
    if (send) {
      e.preventDefault();
      void app.sendDraft();
    }
  }

  function onInput(e: Event) {
    app.drafts[conv.id] = (e.target as HTMLTextAreaElement).value;
    app.typed();
  }
</script>

<svelte:window onclick={() => (simMenu = false)} />

{#if conv.read_only}
  <p class="readonly subtle">You can't reply to this conversation.</p>
{:else}
  <div class="composer">
    {#if app.sims.length > 1}
      <div class="sim">
        <button
          class="secondary sim-button"
          aria-haspopup="menu"
          aria-expanded={simMenu}
          onclick={(e) => {
            e.stopPropagation();
            simMenu = !simMenu;
          }}>
          <span class="label">{simLabel ? `Sending from ${simLabel}` : "Choose sending SIM"}</span>
          <Icon name="chevron-down" size={14} />
        </button>
        {#if simMenu}
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
          <div class="menu sim-menu" role="menu" tabindex="-1" aria-label="Sending SIM" use:popup={{ onclose: () => (simMenu = false) }} onclick={(e) => e.stopPropagation()}>
            {#each app.sims as s (s.number)}
              <button
                class="item"
                role="menuitemradio"
                aria-checked={s.number === currentSim}
                data-autofocus={s.number === currentSim ? "" : undefined}
                class:current={s.number === currentSim}
                onclick={() => {
                  app.selectedSims[conv.id] = s.number;
                  simMenu = false;
                }}>
                {s.phone ? `${s.label} · ${s.phone}` : s.label}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
    {#if replying}
      <div class="replying">
        <Icon name="reply" size={16} />
        <div class="quote">
          <div class="who">Replying to {replying.from_me ? "yourself" : replying.sender || conv.name}</div>
          <div class="what subtle">{replying.text || (replying.media?.length ? "📷 Photo" : "Message")}</div>
        </div>
        <button class="icon-button tiny" title="Cancel reply  Esc" aria-label="Cancel reply" onclick={() => delete app.replyTo[conv.id]}>
          <Icon name="close" size={14} />
        </button>
      </div>
    {/if}
    {#if app.attachments.length}
      <div class="attachments">
        {#each app.attachments as path (path)}
          {@const kind = unpreviewable[path] ? "file" : attachmentKind(path)}
          <div class="attachment {kind}" title={path}>
            {#if kind === "image"}
              <img src={attachmentURL(path)} alt={fileName(path)} draggable="false" onerror={() => (unpreviewable[path] = true)} />
            {:else if kind === "video"}
              <!-- #t: the first frame shows, not a blank box. -->
              <video src="{attachmentURL(path)}#t=0.1" preload="metadata" muted playsinline aria-label={fileName(path)} onerror={() => (unpreviewable[path] = true)}></video>
              <span class="play" aria-hidden="true"><Icon name="play" size={14} /></span>
            {:else}
              <Icon name="file" size={20} />
              <span class="name">{fileName(path)}</span>
            {/if}
            <button
              class="remove"
              title="Remove attachment"
              aria-label="Remove {fileName(path)}"
              onclick={() => (app.attachments = app.attachments.filter((p) => p !== path))}>
              <Icon name="close" size={12} />
            </button>
          </div>
        {/each}
      </div>
    {/if}
    <div class="bar">
      <button class="icon-button round" title="Attach a file  {keys('⌘O')}" aria-label="Attach a file" onclick={() => app.pickAttachment()}>
        <Icon name="paperclip" size={20} />
      </button>
      <div class="field glass">
        <textarea
          bind:this={input}
          rows="1"
          value={draft}
          placeholder={conv.is_rcs ? "RCS message" : "Text message"}
          oninput={onInput}
          onkeydown={onKey}></textarea>
      </div>
      <button
        class="send"
        class:ready={canSend}
        title={app.settings.enter_sends ? "Send  Enter" : `Send  ${keys("⌘Enter")}`}
        aria-label="Send"
        onclick={() => app.sendDraft()}>
        <Icon name="send" size={20} />
      </button>
    </div>
  </div>
{/if}

<style>
  .replying {
    display: flex;
    align-items: center;
    gap: 10px;
    margin: 0 0 6px 50px;
    padding: 6px 6px 6px 12px;
    border-left: 3px solid var(--accent);
    border-radius: 8px;
    background: var(--composer);
    color: var(--subtle);
  }
  .replying .quote {
    flex: 1;
    min-width: 0;
  }
  .replying .who {
    color: var(--text);
    font-size: 13px;
  }
  .replying .what {
    font-size: 13px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .readonly {
    text-align: center;
    padding: 18px;
  }
  .composer {
    padding: 8px 16px 14px;
  }
  .sim {
    position: relative;
    margin: 0 0 6px 50px;
  }
  .sim-button {
    white-space: nowrap;
    max-width: 100%;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
  }
  .sim-button .label {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .sim-menu {
    position: absolute;
    bottom: 40px;
    left: 0;
  }
  .sim-menu .current {
    background: var(--selected);
  }
  .attachments {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin: 0 0 6px 50px;
  }
  .attachment {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    height: 72px;
    min-width: 0;
    max-width: 100%;
    border-radius: 12px;
    background: var(--composer);
    overflow: hidden;
  }
  .attachment.image,
  .attachment.video {
    width: 72px;
  }
  .attachment.file {
    width: 180px;
    padding: 0 30px 0 12px;
    color: var(--subtle);
  }
  .attachment img,
  .attachment video {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .attachment .name {
    color: var(--text);
    font-size: 13px;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow-wrap: anywhere;
  }
  /* Over the picture, so a dark scrim: readable on any photo, any theme. */
  .play,
  .remove {
    position: absolute;
    display: grid;
    place-items: center;
    border-radius: 50%;
    background: rgba(0, 0, 0, 0.55);
    color: #fff;
  }
  .play {
    left: 50%;
    top: 50%;
    width: 28px;
    height: 28px;
    transform: translate(-50%, -50%);
    pointer-events: none;
  }
  .remove {
    top: 4px;
    right: 4px;
    width: 20px;
    height: 20px;
  }
  .remove:hover {
    background: rgba(0, 0, 0, 0.75);
  }
  .tiny {
    width: 22px;
    height: 22px;
  }
  .bar {
    display: flex;
    align-items: flex-end;
    gap: 6px;
  }
  .round {
    width: 44px;
    height: 44px;
  }
  .field {
    flex: 1;
    min-width: 0;
    background: var(--composer);
    border-radius: 22px;
    padding: 11px 16px;
    display: flex;
  }
  textarea {
    /* Text wraps, so never sideways; a horizontal bar would eat the line. */
    overflow-x: hidden;
    overflow-y: hidden;
    flex: 1;
    min-width: 0;
    resize: none;
    line-height: 1.4;
    max-height: 160px;
    /* Glyphs keep a fixed advance while typing: no kerning, ligatures or
       contextual alternates, so WebKit re-shaping a word never shifts letters. */
    font-kerning: none;
    font-variant-ligatures: none;
    font-feature-settings: "kern" 0, "liga" 0, "clig" 0, "calt" 0;
    text-rendering: optimizeSpeed;
  }
  .send {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    color: var(--subtle);
  }
  .send:hover {
    background: var(--hover);
  }
  .send.ready {
    background: var(--accent);
    color: var(--on-accent);
  }
</style>
