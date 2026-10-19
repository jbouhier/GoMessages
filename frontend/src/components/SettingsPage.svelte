<script lang="ts">
  import { app, zoomStep } from "../lib/app.svelte";
  import { keys } from "../lib/platform";
  import { Go } from "../lib/go";
  import { THEMES } from "../lib/themes";
  import Icon from "./Icon.svelte";

  const s = $derived(app.settings);


  function set<K extends keyof typeof app.settings>(key: K, value: (typeof app.settings)[K]) {
    app.settings[key] = value;
    app.saveSettings();
  }

  /** A setting that changes the colors: crossfades to them. */
  function restyle<K extends "mode" | "theme" | "background">(key: K, value: (typeof app.settings)[K]) {
    if (app.settings[key] === value) return;
    app.restyle(() => set(key, value));
  }

  function back() {
    app.settingsOpen = false;
    app.confirmUnpair = false;
  }

  function testSound() {
    void Go.Notify("GoMessages", "This is how new messages sound.", true);
  }
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && back()} />

<section class="page">
  <header class="drag">
    <button class="icon-button" title="Back  Esc" onclick={back}><Icon name="arrow-left" size={19} /></button>
    <h1>Settings</h1>
  </header>
  <div class="scroll">
    <div class="column">
      <h2>Appearance</h2>
      <div class="card glass">
        <div class="row">
          <div><div>Mode</div><div class="hint">Every theme has a light and a dark side.</div></div>
          <div class="segmented" role="radiogroup" aria-label="Mode">
            {#each [["system", "System"], ["light", "Light"], ["dark", "Dark"]] as [value, label] (value)}
              <button class:on={s.mode === value} role="radio" aria-checked={s.mode === value} onclick={() => restyle("mode", value)}>{label}</button>
            {/each}
          </div>
        </div>
        <div class="row">
          <div><div>Theme</div><div class="hint">Colors for both sides.</div></div>
          <div class="swatches" role="radiogroup" aria-label="Theme">
            {#each THEMES as t (t.id)}
              <button
                class="swatch"
                class:on={app.theme === t.id}
                role="radio"
                aria-checked={app.theme === t.id}
                aria-label={t.name}
                title={t.credit ? `${t.name}: ${t.credit}` : t.name}
                style:--light={t.light.accent}
                style:--dark={t.dark.accent}
                onclick={() => restyle("theme", t.id)}>
                <span class="dot"></span>
                <span class="name">{t.name}</span>
              </button>
            {/each}
          </div>
        </div>
        <div class="row">
          <div><div>Background</div><div class="hint">The theme's gradient, or plain color.</div></div>
          <div class="segmented" role="radiogroup" aria-label="Background">
            {#each [["gradient", "Gradient"], ["solid", "Solid"]] as [value, label] (value)}
              <button class:on={(s.background || "gradient") === value} role="radio" aria-checked={(s.background || "gradient") === value} onclick={() => restyle("background", value)}>{label}</button>
            {/each}
          </div>
        </div>
        <div class="row">
          <div>
            <div>Transparency</div>
            <div class="hint">
              {app.blurSupported ? "Shows what's behind the window, blurred." : "Fades the theme's background."}
            </div>
          </div>
          <div class="slider">
            <input
              type="range"
              min="0"
              max="100"
              step="1"
              aria-label="Transparency"
              value={s.transparency}
              oninput={(e) => set("transparency", Number(e.currentTarget.value))} />
            <span>{Math.round(s.transparency)}%</span>
          </div>
        </div>
        <div class="row">
          <div><div>Text size</div><div class="hint">{keys("Also ⌘+ / ⌘− / ⌘0.")}</div></div>
          <div class="zoom">
            <button class="secondary" onclick={() => app.setZoom(zoomStep(s.zoom, false))}>−</button>
            <span>{Math.round(s.zoom * 100)}%</span>
            <button class="secondary" onclick={() => app.setZoom(zoomStep(s.zoom, true))}>+</button>
            <button class="secondary" title="Reset" onclick={() => app.setZoom(1)}>↺</button>
          </div>
        </div>
      </div>

      <h2>Notifications</h2>
      <div class="card glass">
        <div class="row">
          <div><div>Desktop notifications</div><div class="hint">Show a banner when a message arrives.</div></div>
          <button class="switch" class:on={s.notifications} aria-label="Desktop notifications" onclick={() => set("notifications", !s.notifications)}></button>
        </div>
        <div class="row" class:disabled={!s.notifications}>
          <div><div>Show message preview</div><div class="hint">Include the sender's text in the banner.</div></div>
          <button class="switch" class:on={s.notification_preview} aria-label="Show message preview" onclick={() => set("notification_preview", !s.notification_preview)}></button>
        </div>
        <div class="row">
          <div><div>Notification sound</div><div class="hint">Play a sound for new messages.</div></div>
          <div class="pair">
            <button class="secondary small" onclick={testSound}>Test</button>
            <button class="switch" class:on={s.notification_sound} aria-label="Notification sound" onclick={() => set("notification_sound", !s.notification_sound)}></button>
          </div>
        </div>
      </div>

      <h2>Chats</h2>
      <div class="card glass">
        <div class="row">
          <div><div>Enter key sends</div><div class="hint">{keys("Off: Enter adds a new line and ⌘Enter sends.")}</div></div>
          <button class="switch" class:on={s.enter_sends} aria-label="Enter key sends" onclick={() => set("enter_sends", !s.enter_sends)}></button>
        </div>
        <div class="row">
          <div><div>Send typing indicators</div><div class="hint">Let people see when you're typing (RCS).</div></div>
          <button class="switch" class:on={s.send_typing} aria-label="Send typing indicators" onclick={() => set("send_typing", !s.send_typing)}></button>
        </div>
      </div>

      <h2>Account</h2>
      <div class="card glass">
        <div class="row">
          <div>
            <div>{app.account ? `Paired with ${app.account}` : "Paired with your phone"}</div>
            <div class="hint">Messages sync through your phone. It needs to stay online.</div>
          </div>
          {#if app.confirmUnpair}
            <div class="pair">
              <button class="secondary" onclick={() => (app.confirmUnpair = false)}>Cancel</button>
              <button class="secondary danger" onclick={() => app.logout()}>Unpair</button>
            </div>
          {:else}
            <button class="secondary" onclick={() => (app.confirmUnpair = true)}>Unpair…</button>
          {/if}
        </div>
      </div>
    </div>
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
  .scroll {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
  }
  .column {
    width: min(600px, calc(100% - 48px));
    margin: 0 auto 24px;
  }
  h2 {
    margin: 22px 0 6px;
    font-size: 13px;
    font-weight: 600;
    color: var(--accent);
  }
  .card {
    background: var(--sidebar);
    border-radius: 14px;
    padding: 2px 14px;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    min-height: 56px;
    padding: 6px 0;
  }
  .row + .row {
    border-top: 1px solid var(--divider);
  }
  .row.disabled {
    opacity: 0.5;
  }
  .hint {
    font-size: 12px;
    color: var(--subtle);
    margin-top: 2px;
  }
  .swatches {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .swatch {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 5px;
    width: 56px;
    padding: 6px 0 4px;
    border-radius: var(--row-radius);
    font-size: 12px;
    color: var(--subtle);
  }
  .swatch:hover {
    background: var(--hover);
  }
  /* The theme's accent on its light and dark sides. */
  .dot {
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: linear-gradient(135deg, var(--light) 50%, var(--dark) 50%);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--text) 18%, transparent);
  }
  .swatch.on .dot {
    outline: 2px solid var(--text);
    outline-offset: 2px;
  }
  .swatch.on .name {
    color: var(--text);
    font-weight: 600;
  }
  .slider {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
    max-width: 100%;
  }
  .slider input {
    width: 180px;
    min-width: 0;
    flex-shrink: 1;
    accent-color: var(--accent);
    cursor: pointer;
  }
  .slider span {
    min-width: 40px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .segmented {
    display: flex;
    flex-wrap: wrap;
    gap: 2px;
  }
  .segmented button {
    padding: 7px 14px;
    border-radius: 18px;
  }
  .segmented button.on {
    background: var(--selected);
  }
  .zoom,
  .pair {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
  }
  .zoom span {
    min-width: 46px;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }
  .danger {
    color: var(--danger);
  }
</style>
