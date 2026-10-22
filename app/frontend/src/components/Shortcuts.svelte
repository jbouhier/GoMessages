<script lang="ts">
  import { app } from "../lib/app.svelte";
  import { keys } from "../lib/platform";

  const groups: [string, [string, string][]][] = $derived([
    [
      "Navigate",
      [
        ["⌘N", "New chat"],
        ["⌘K", "Switch to a conversation"],
        ["⌘F", "Search conversations"],
        ["⌥↓  ⌥↑", "Next / previous conversation"],
        ["⌃Tab  ⌃⇧Tab", "Same, browser style"],
        ["⌥⇧↓  ⌥⇧↑", "Next / previous unread"],
        ["⌘1 … ⌘9", "Go to conversation 1–9"],
      ],
    ],
    [
      "Read",
      [
        ["PgUp  PgDn", "Scroll the conversation"],
        ["Home  End", "Oldest loaded / newest message"],
      ],
    ],
    [
      "Write",
      [
        app.settings.enter_sends
          ? ["Enter", "Send (Shift+Enter for a new line)"]
          : ["⌘Enter", "Send (Enter for a new line)"],
        ["⌘O", "Attach a file"],
        ["Esc", "Back to the message box (or drop a reply)"],
      ],
    ],
    [
      "Menus and dialogs",
      [
        ["↑ ↓ ← →  Tab", "Move between the choices"],
        ["Enter  Space", "Choose"],
        ["Esc", "Close (a confirmation starts on Cancel)"],
      ],
    ],
    [
      "Photo viewer",
      [
        ["←  →", "Previous / next photo"],
        ["+  −", "Zoom in / out"],
        ["0  1", "Fit / actual size"],
        ["⌘C", "Copy the image"],
        ["Esc", "Close"],
      ],
    ],
    [
      "View",
      [
        ["⌘B", "Show / hide the conversation list"],
        ["⌘⇧\\", "Photos-only list"],
        ["⌘⇧L", "Light / dark"],
        ["⌘+  ⌘−  ⌘0", "Text size"],
        ["⌘R", "Refresh conversations"],
      ],
    ],
    ["App", [["⌘,", "Settings"], ["⌘/", "Keyboard shortcuts"], ["⌘Q", "Quit"]]],
  ]);
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && (app.showShortcuts = false)} />

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="backdrop" onclick={() => (app.showShortcuts = false)}></div>
<div class="sheet" role="dialog" aria-label="Keyboard shortcuts">
  <h2>Keyboard shortcuts</h2>
  {#each groups as [title, items] (title)}
    <h3>{title}</h3>
    {#each items as [k, what] (what)}
      <div class="row"><span>{what}</span><kbd>{keys(k)}</kbd></div>
    {/each}
  {/each}
  <div class="close"><button class="secondary" onclick={() => (app.showShortcuts = false)}>Close</button></div>
</div>

<style>
  .sheet {
    position: fixed;
    z-index: 41;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: min(480px, calc(100vw - 40px));
    max-height: calc(100vh - 60px);
    overflow-y: auto;
    padding: 22px;
    background: var(--bg);
    border: 1px solid var(--divider);
    border-radius: 16px;
    box-shadow: 0 12px 40px var(--shadow);
  }
  h2 {
    margin: 0 0 4px;
    font-size: 20px;
    font-weight: 400;
  }
  h3 {
    margin: 24px 0 4px;
    font-size: 13px;
    color: var(--accent);
  }
  h2 + h3 {
    margin-top: 14px;
  }
  .row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    min-height: 28px;
  }
  .close {
    text-align: center;
    margin-top: 16px;
  }
</style>
