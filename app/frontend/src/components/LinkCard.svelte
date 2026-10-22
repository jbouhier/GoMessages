<script lang="ts">
  // The card over a hovered or focused link chip: the page's picture, title
  // and description, and the full address it opens. With no preview (still
  // loading, failed, or not fetched) it shows the address alone. One for the whole app,
  // drawn outside the thread: rows there clip what they paint.
  import { app } from "../lib/app.svelte";
  import { mediaURL } from "../lib/types";

  const WIDTH = 340;
  const GAP = 8;

  const card = $derived(app.linkCard);
  const s = $derived(card ? app.previews[card.href] : undefined);
  const p = $derived(s?.state === "ready" ? s.preview : null);
  let height = $state(0);
  let imageFailed = $state(false);

  $effect(() => {
    void card?.href;
    imageFailed = false;
  });

  // Where the link ends up, when that's another site (a shortener, a
  // tracking redirect): the address in the message isn't the whole story.
  const lands = $derived.by(() => {
    if (!card || !p) return "";
    try {
      const from = new URL(card.href).hostname.replace(/^www\./, "");
      const to = new URL(p.url).hostname.replace(/^www\./, "");
      return from === to ? "" : to;
    } catch {
      return "";
    }
  });

  // Below the chip, or above it when the window has no room underneath;
  // kept inside the window sideways.
  const pos = $derived.by(() => {
    if (!card) return { left: 0, top: 0 };
    const r = card.rect;
    const left = Math.min(Math.max(r.left, GAP), window.innerWidth - WIDTH - GAP);
    const below = r.bottom + GAP;
    const top = below + height <= window.innerHeight - GAP ? below : Math.max(GAP, r.top - GAP - height);
    return { left, top };
  });

  // %20 and friends shown as text; the link still opens the raw address.
  function readable(href: string): string {
    try {
      return decodeURI(href);
    } catch {
      return href;
    }
  }

  function close() {
    app.linkCard = null;
  }
</script>

<svelte:window onscroll={close} onresize={close} onblur={close} onkeydown={(e) => e.key === "Escape" && card && close()} />
<svelte:document onscrollcapture={close} />

{#if card}
  <div
    class="card"
    role="tooltip"
    style:left="{pos.left}px"
    style:top="{pos.top}px"
    style:width="{WIDTH}px"
    style:visibility={height ? "visible" : "hidden"}
    bind:offsetHeight={height}>
    {#if p?.image && !imageFailed}
      <img class="hero" src={mediaURL(p.image)} alt="" draggable="false" onerror={() => (imageFailed = true)} />
    {/if}
    <div class="body">
      {#if p}
        <div class="site subtle">
          {#if p.icon}<img class="fav" src={mediaURL(p.icon)} alt="" draggable="false" />{/if}
          <span>{p.site}</span>
        </div>
        <div class="title">{p.title}</div>
        {#if p.description}<div class="desc">{p.description}</div>{/if}
      {/if}
      <div class="url" class:subtle={!!p}>{readable(card.href)}</div>
      {#if lands}<div class="lands">Opens on <b>{lands}</b></div>{/if}
    </div>
  </div>
{/if}

<style>
  .card {
    position: fixed;
    z-index: 60;
    overflow: hidden;
    pointer-events: none;
    background: var(--bg);
    color: var(--text);
    border: 1px solid var(--divider);
    border-radius: 12px;
    box-shadow: 0 8px 28px var(--shadow);
    animation: appear 0.12s ease-out;
  }
  @keyframes appear {
    from {
      opacity: 0;
      transform: translateY(-2px);
    }
  }
  .hero {
    display: block;
    width: 100%;
    aspect-ratio: 1.91 / 1;
    object-fit: cover;
    background: var(--composer);
  }
  .body {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px 14px 12px;
  }
  .site {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .fav {
    width: 14px;
    height: 14px;
    border-radius: 3px;
    object-fit: contain;
  }
  .title {
    font-weight: 600;
    line-height: 1.3;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .desc {
    font-size: 13px;
    line-height: 1.4;
    color: var(--subtle);
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .url {
    margin-top: 2px;
    font-size: 12px;
    line-height: 1.4;
    overflow-wrap: anywhere;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .lands {
    font-size: 12px;
    color: var(--subtle);
  }
</style>
