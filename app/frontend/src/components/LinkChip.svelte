<script lang="ts">
  // A link in a message, as one line: the site's icon, the page's title and
  // the site. Until the title arrives (or when there is none) it shows the
  // short address in the same shape, so the bubble never changes height.
  // Hovering or focusing it opens the bigger card (LinkCard.svelte).
  import { app } from "../lib/app.svelte";
  import { openURL } from "../lib/go";
  import { shortURL } from "../lib/links";
  import { mediaURL } from "../lib/types";
  import Icon from "./Icon.svelte";

  let { href }: { href: string } = $props();

  const s = $derived(app.previewState(href));
  const p = $derived(s.state === "ready" ? s.preview : null);
  let iconFailed = $state(false);
  let el: HTMLAnchorElement | undefined = $state();
  let timer = 0;

  function show(delay: number) {
    window.clearTimeout(timer);
    timer = window.setTimeout(() => {
      if (!el) return;
      const r = el.getBoundingClientRect();
      app.linkCard = { href, rect: { left: r.left, top: r.top, bottom: r.bottom, width: r.width } };
    }, delay);
  }

  function hide() {
    window.clearTimeout(timer);
    if (app.linkCard?.href === href) app.linkCard = null;
  }

  function open(e: MouseEvent) {
    // The web view must never navigate away from the app.
    e.preventDefault();
    hide();
    openURL(href);
  }
</script>

<a
  bind:this={el}
  class="chip"
  class:ready={!!p}
  {href}
  rel="noreferrer"
  draggable="false"
  aria-label={p ? `${p.title}, ${p.site}` : undefined}
  onclick={open}
  onmouseenter={() => show(400)}
  onmouseleave={hide}
  onfocus={() => show(0)}
  onblur={hide}
  >{#if p?.icon && !iconFailed}<img class="fav" src={mediaURL(p.icon)} alt="" draggable="false" onerror={() => (iconFailed = true)} />{:else}<span
      class="fav"><Icon name="link" size={13} /></span
    >{/if}{#if p}<span class="title">{p.title}</span><span class="site">{p.site}</span>{:else}<span class="title"
      >{shortURL(href)}</span
    >{/if}</a
>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    max-width: 100%;
    box-sizing: border-box;
    vertical-align: bottom;
    padding: 0 8px 0 3px;
    margin: 1px 0;
    border-radius: 7px;
    background: color-mix(in srgb, currentColor 9%, transparent);
    color: inherit;
    text-decoration: none;
    white-space: nowrap;
    cursor: pointer;
    transition: background 0.12s ease-out;
  }
  .chip:hover,
  .chip:focus-visible {
    background: color-mix(in srgb, currentColor 16%, transparent);
  }
  .fav {
    flex: none;
    display: grid;
    place-items: center;
    width: 16px;
    height: 16px;
    border-radius: 4px;
    object-fit: contain;
    opacity: 0.85;
  }
  img.fav {
    opacity: 1;
  }
  .title {
    min-width: 0;
    max-width: 44ch;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .ready .title {
    font-weight: 500;
  }
  .site {
    flex: none;
    max-width: 18ch;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 0.92em;
    opacity: 0.7;
  }
</style>
