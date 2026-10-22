<script lang="ts">
  import { albumOf, app } from "../lib/app.svelte";
  import { keys } from "../lib/platform";
  import { mediaURL } from "../lib/types";
  import { Go } from "../lib/go";
  import Icon from "./Icon.svelte";
  import { anchored, atPoint, popup } from "../lib/popup";

  const MIN = 0.1;
  const MAX = 8;
  const STEP = 1.25;

  let stage = $state<HTMLDivElement>();
  /** The image on screen and its pixel size, once it has loaded. */
  let loaded = $state({ photo: "", src: "", w: 0, h: 0 });
  let stageSize = $state({ w: 0, h: 0 });
  let fit = $state(true);
  let zoom = $state(1);
  let menu = $state<{ x: number; y: number } | null>(null);

  const v = $derived(app.viewer!);
  const album = $derived(albumOf(app.threads[v.conv]?.msgs ?? []));
  const index = $derived(album.findIndex((p) => p.message.id === v.message && p.index === v.index));
  const item = $derived(index >= 0 ? album[index] : null);
  const full = $derived(item ? app.mediaState(item.media.id, item.media.key, item.media.mime) : undefined);
  const thumb = $derived(
    item?.media.thumb_id ? app.media[item.media.thumb_id] : undefined,
  );
  const shown = $derived(full?.state === "ready" ? full.path : thumb?.state === "ready" ? thumb.path : null);
  const path = $derived(full?.state === "ready" ? full.path : null);

  // The picture's size. The full file gives it exactly; while only the
  // thumbnail is in, it stands in at the photo's size (from the message, or
  // its own) with its own shape, so nothing is stretched. A size measured
  // for another picture is never used.
  const photo = $derived(item ? `${item.message.id}/${item.index}` : "");
  const natural = $derived.by(() => {
    // The thumbnail's size holds while the full file replaces it (same
    // shape), so the picture doesn't blink.
    if (!shown || loaded.photo !== photo || !loaded.w) return { w: 0, h: 0 };
    if (loaded.src === path) return { w: loaded.w, h: loaded.h };
    const w = Math.max(item?.media.width ?? 0, loaded.w);
    return { w, h: (w * loaded.h) / loaded.w };
  });

  // Fit follows the window as it resizes.
  $effect(() => {
    if (!stage) return;
    const el = stage;
    const ro = new ResizeObserver(() => (stageSize = { w: el.clientWidth, h: el.clientHeight }));
    ro.observe(el);
    return () => ro.disconnect();
  });
  const sender = $derived(
    !item ? "" : item.message.from_me ? "You" : item.message.sender || app.convs.find((c) => c.id === v.conv)?.name || "",
  );
  const when = $derived(
    item
      ? new Date(item.message.timestamp).toLocaleString([], { month: "short", day: "numeric", year: "numeric", hour: "numeric", minute: "2-digit" })
      : "",
  );

  // A photo that left the thread closes the viewer.
  $effect(() => {
    if (index < 0) app.viewer = null;
  });

  // A new photo starts fitted.
  $effect(() => {
    void item?.media.id;
    void item?.index;
    fit = true;
    zoom = 1;
  });

  // Fit fills the space, small pictures included.
  const fitScale = $derived.by(() => {
    if (!stageSize.w || !natural.w) return 1;
    return Math.min((stageSize.w - 48) / natural.w, (stageSize.h - 32) / natural.h);
  });
  const scale = $derived(fit ? fitScale : zoom);

  function zoomBy(factor: number, at?: { x: number; y: number }) {
    if (!stage) return;
    const before = scale;
    const next = Math.min(Math.max(before * factor, MIN), MAX);
    if (next === before) return;
    // Keep the point under the pointer in place.
    const rect = stage.getBoundingClientRect();
    const px = (at?.x ?? rect.left + rect.width / 2) - rect.left;
    const py = (at?.y ?? rect.top + rect.height / 2) - rect.top;
    const fx = (stage.scrollLeft + px) / (natural.w * before || 1);
    const fy = (stage.scrollTop + py) / (natural.h * before || 1);
    fit = false;
    zoom = next;
    requestAnimationFrame(() => {
      if (!stage) return;
      stage.scrollLeft = fx * natural.w * next - px;
      stage.scrollTop = fy * natural.h * next - py;
    });
  }

  function go(dir: number) {
    const target = album[Math.min(Math.max(index + dir, 0), album.length - 1)];
    if (target) app.viewer = { conv: v.conv, message: target.message.id, index: target.index };
  }

  async function copyImage() {
    if (!path) return;
    try {
      const blob = await (await fetch(mediaURL(path))).blob();
      const png = blob.type === "image/png" ? blob : await toPNG(blob);
      await navigator.clipboard.write([new ClipboardItem({ "image/png": png })]);
      app.showToast("Image copied");
    } catch (e) {
      app.showToast(`Couldn't copy the image: ${e}`);
    }
  }

  async function toPNG(blob: Blob): Promise<Blob> {
    const bmp = await createImageBitmap(blob);
    const canvas = document.createElement("canvas");
    canvas.width = bmp.width;
    canvas.height = bmp.height;
    canvas.getContext("2d")!.drawImage(bmp, 0, 0);
    return new Promise((resolve) => canvas.toBlob((b) => resolve(b!), "image/png"));
  }

  function saveAs() {
    if (path) void Go.SaveAs(path, `${sender || "Messages"} photo.${path.split(".").pop()}`);
  }

  function openOutside() {
    if (path) void Go.OpenPath(path);
  }

  function onKey(e: KeyboardEvent) {
    const k = e.key;
    if (k === "Escape") app.viewer = null;
    else if (k === "ArrowLeft") go(-1);
    else if (k === "ArrowRight") go(1);
    else if (k === "+" || k === "=") zoomBy(STEP);
    else if (k === "-") zoomBy(1 / STEP);
    else if (k === "0") fit = true;
    else if (k === "1") {
      fit = false;
      zoom = 1;
    } else if ((e.metaKey || e.ctrlKey) && k === "c") void copyImage();
    else return;
    e.preventDefault();
    e.stopPropagation();
  }

  function onWheel(e: WheelEvent) {
    // Pinch (ctrlKey on trackpads) or ⌘/Ctrl+wheel zooms; plain scrolling pans.
    if (!e.ctrlKey && !e.metaKey) return;
    e.preventDefault();
    zoomBy(Math.exp(-e.deltaY * 0.01), { x: e.clientX, y: e.clientY });
  }

  let drag: { x: number; y: number; left: number; top: number } | null = null;
  function onDown(e: PointerEvent) {
    if (!stage || e.button !== 0) return;
    drag = { x: e.clientX, y: e.clientY, left: stage.scrollLeft, top: stage.scrollTop };
  }
  function onMove(e: PointerEvent) {
    if (!drag || !stage) return;
    stage.scrollLeft = drag.left - (e.clientX - drag.x);
    stage.scrollTop = drag.top - (e.clientY - drag.y);
  }

  function onDouble(e: MouseEvent) {
    if (fit) {
      zoomBy(1 / scale, { x: e.clientX, y: e.clientY });
      zoom = 1;
      fit = false;
    } else fit = true;
  }
</script>

<svelte:window
  oncopy={(e) => {
    if (!window.getSelection()?.toString()) {
      e.preventDefault();
      void copyImage();
    }
  }}
  onkeydown={onKey} onpointermove={onMove} onpointerup={() => (drag = null)} onclick={() => (menu = null)} />

<div class="viewer" role="dialog" aria-label="Photo">
  <header class="drag">
    <div class="who">
      <div>{sender}</div>
      <div class="when">{when}{album.length > 1 ? `  ·  ${index + 1} of ${album.length}` : ""}</div>
    </div>
    <div class="actions">
      <button title="Zoom out  −" onclick={() => zoomBy(1 / STEP)}><Icon name="minus" /></button>
      <button class="scale" title={fit ? "Show at actual size  1" : "Fit to the window  0"} onclick={() => (fit ? ((fit = false), (zoom = 1)) : (fit = true))}>
        {fit ? "Fit" : `${Math.round(zoom * 100)}%`}
      </button>
      <button title="Zoom in  +" onclick={() => zoomBy(STEP)}><Icon name="plus" /></button>
      <span class="gap"></span>
      {#if path}
        <button title="Copy image  {keys('⌘C')}" onclick={copyImage}><Icon name="copy" /></button>
        <button title="Save as…" onclick={saveAs}><Icon name="download" /></button>
        <button title="Open in another app" onclick={openOutside}><Icon name="external-link" /></button>
      {/if}
      <button title="Close  Esc" onclick={() => (app.viewer = null)}><Icon name="close" /></button>
    </div>
  </header>

  <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events -->
  <div
    class="stage"
    class:panning={!fit}
    bind:this={stage}
    onwheel={onWheel}
    onpointerdown={onDown}
    onclick={(e) => {
      if (e.target === stage) app.viewer = null;
    }}>
    {#if shown}
      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
      <img
        src={mediaURL(shown)}
        alt=""
        draggable="false"
        class:measuring={!natural.w}
        style:width={natural.w ? `${natural.w * scale}px` : null}
        style:height={natural.h ? `${natural.h * scale}px` : null}
        onload={(e) => {
          const img = e.currentTarget as HTMLImageElement;
          loaded = { photo, src: shown ?? "", w: img.naturalWidth, h: img.naturalHeight };
        }}
        ondblclick={onDouble}
        oncontextmenu={(e) => {
          e.preventDefault();
          if (path) menu = { x: e.clientX, y: e.clientY };
        }} />
      {#if !path}<div class="spinner over"></div>{/if}
    {:else if full?.state === "failed"}
      <p class="missing">This photo isn't available.</p>
    {:else}
      <div class="spinner"></div>
    {/if}
  </div>

  {#if album.length > 1}
    {#if index > 0}
      <button class="nav left" title="Previous photo  ←" onclick={() => go(-1)}><Icon name="chevron-left" size={22} /></button>
    {/if}
    {#if index < album.length - 1}
      <button class="nav right" title="Next photo  →" onclick={() => go(1)}><Icon name="chevron-right" size={22} /></button>
    {/if}
  {/if}

  {#if menu}
    <div class="menu" role="menu" tabindex="-1" aria-label="Photo" use:anchored={atPoint(menu.x, menu.y)} use:popup={{ onclose: () => (menu = null) }}>
      <button class="item" role="menuitem" onclick={() => { menu = null; void copyImage(); }}>Copy image</button>
      <button class="item" role="menuitem" onclick={() => { menu = null; saveAs(); }}>Save as…</button>
      <button class="item" role="menuitem" onclick={() => { menu = null; openOutside(); }}>Open in another app</button>
    </div>
  {/if}
</div>

<style>
  /* A photo viewer is dark in both themes, like every phone's. */
  .viewer {
    position: fixed;
    inset: 0;
    z-index: 60;
    display: flex;
    flex-direction: column;
    background: #0a0a0c;
    color: #f1f3f4;
  }
  header {
    display: flex;
    align-items: center;
    padding: calc(var(--top-inset) + 8px) 14px 8px 18px;
    min-height: 56px;
  }
  :global(.mac) header {
    padding-left: 84px;
  }
  .who {
    flex: 1;
    min-width: 0;
    font-size: 15px;
  }
  .when {
    font-size: 12px;
    color: #9aa0a6;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .actions button {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    color: #9aa0a6;
  }
  .actions button:hover {
    background: rgba(60, 60, 64, 0.8);
    color: #f1f3f4;
  }
  .actions .scale {
    width: auto;
    min-width: 58px;
    padding: 0 12px;
    border-radius: 14px;
    background: rgba(60, 60, 64, 0.8);
    color: #f1f3f4;
    font-size: 13px;
  }
  .gap {
    width: 10px;
  }
  .stage {
    flex: 1;
    overflow: auto;
    display: grid;
    place-items: center;
  }
  .stage.panning {
    cursor: grab;
  }
  .stage.panning:active {
    cursor: grabbing;
  }
  img {
    display: block;
    max-width: none;
  }
  /* Until its size is known the picture stays out of sight, not misshapen. */
  img.measuring {
    visibility: hidden;
    position: absolute;
  }
  .over {
    position: absolute;
  }
  .missing {
    color: #9aa0a6;
  }
  .nav {
    position: absolute;
    top: 50%;
    width: 44px;
    height: 44px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: rgba(60, 60, 64, 0.6);
    color: #f1f3f4;
  }
  .nav:hover {
    background: rgba(60, 60, 64, 0.95);
  }
  .left {
    left: 14px;
  }
  .right {
    right: 14px;
  }
  .menu {
    color: var(--text);
  }
</style>
