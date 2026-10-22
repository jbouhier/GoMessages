<script lang="ts">
  import { app } from "../lib/app.svelte";

  const svgURL = (emoji: string) =>
    `https://fonts.gstatic.com/s/e/notoemoji/latest/${[...emoji]
      .filter((c) => c !== "️")
      .map((c) => c.codePointAt(0)!.toString(16))
      .join("_")}/emoji.svg`;
</script>

<section class="pairing drag">
  <div class="column">
    <svg class="logo" viewBox="0 0 64 64" aria-hidden="true">
      <rect x="4" y="9" width="56" height="37" rx="18" fill="var(--accent)" />
      <polygon points="13,42 9,56 22,45" fill="var(--accent)" />
      <rect x="15" y="20" width="34" height="15" rx="7.5" fill="var(--on-accent)" />
    </svg>
    <h1>Messages for desktop</h1>
    <p class="subtle">
      Text from your computer with your phone's number. Messages stay end-to-end encrypted between this app and your
      phone.
    </p>

    {#if app.pairing.kind === "idle"}
      <button class="primary" onclick={() => app.signIn()}>Sign in with Google</button>
      <p class="subtle small">Your phone needs Google Messages and an internet connection.</p>
    {:else if app.pairing.kind === "signing-in"}
      <div class="spinner"></div>
      <p class="subtle">Finish signing in to Google in the window that opened…</p>
      <button class="secondary" onclick={() => app.cancelPairing()}>Cancel</button>
    {:else}
      <p class="big">On your phone, tap this emoji</p>
      <img class="emoji" src={svgURL(app.pairing.emoji)} alt={app.pairing.emoji} />
      {#if app.pairing.account}<p class="subtle">Pairing with {app.pairing.account}</p>{/if}
      <p class="subtle small">Google Messages shows a pairing prompt. Pick the same emoji there.</p>
      <div class="row"><div class="spinner"></div><button class="secondary" onclick={() => app.cancelPairing()}>Cancel</button></div>
    {/if}

    {#if app.pairError}<p class="error">{app.pairError}</p>{/if}
  </div>
</section>

<style>
  .pairing {
    flex: 1;
    background: var(--pane);
    display: flex;
    justify-content: center;
    overflow-y: auto;
  }
  .column {
    width: min(440px, calc(100% - 32px));
    margin-top: max(14vh, 60px);
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 10px;
  }
  .column > * {
    --wails-draggable: no-drag;
  }
  .logo {
    width: 72px;
    height: 72px;
  }
  h1 {
    font-size: 26px;
    font-weight: 400;
    margin: 14px 0 0;
  }
  .big {
    font-size: 18px;
    margin-top: 18px;
  }
  .emoji {
    width: 120px;
    height: 120px;
  }
  .row {
    display: flex;
    gap: 12px;
    align-items: center;
  }
  .error {
    color: var(--danger);
    margin-top: 16px;
  }
</style>
