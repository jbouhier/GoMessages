<script lang="ts">
  // The info panel beside a thread: who this is, a few actions, and the
  // photos and links already loaded in the conversation. Everything comes
  // from what the app holds: it never waits on the phone.
  import { albumOf, app } from "../lib/app.svelte";
  import { openURL } from "../lib/go";
  import { isWebLink, linkify, mayPreview, shortURL } from "../lib/links";
  import { mediaURL, type Conversation, type Media, type Person } from "../lib/types";
  import Avatar from "./Avatar.svelte";
  import Icon from "./Icon.svelte";
  import { vcard, vcards } from "../lib/vcard";

  let { conv }: { conv: Conversation } = $props();

  const msgs = $derived(app.threads[conv.id]?.msgs ?? []);
  const people = $derived(conv.people ?? []);
  /** The other person of a one-to-one chat. */
  const person = $derived<Person | undefined>(conv.is_group ? undefined : people[0]);
  const muted = $derived(app.isMuted(conv.id));

  /** Newest first. */
  const photos = $derived(albumOf(msgs).reverse());
  const links = $derived.by(() => {
    const seen = new Set<string>();
    const out: { href: string; fromMe: boolean; sender: string }[] = [];
    for (let i = msgs.length - 1; i >= 0; i--) {
      const m = msgs[i];
      if (!m.text) continue;
      for (const seg of linkify(m.text)) {
        if (!seg.href || !isWebLink(seg.href) || seen.has(seg.href)) continue;
        seen.add(seg.href);
        out.push({ href: seg.href, fromMe: m.from_me, sender: conv.is_group ? m.sender : conv.name });
      }
    }
    return out;
  });

  /** Members with a number: a group's Share sends all of their cards. */
  const shareable = $derived(conv.is_group ? people.filter((p) => p.number) : person?.number ? [person] : []);

  function share(e: MouseEvent) {
    const card = conv.is_group ? vcards(shareable) : vcard({ ...shareable[0], name: conv.name });
    void app.shareCard(conv.name, card, e.currentTarget as HTMLElement);
  }

  function thumb(md: Media) {
    return md.thumb_id ? app.mediaState(md.thumb_id, md.thumb_key ?? "", md.mime) : app.mediaState(md.id, md.key, md.mime);
  }

  /** A title only where the link previews setting allows fetching it. */
  function title(l: { href: string; fromMe: boolean; sender: string }): string | null {
    if (!mayPreview(app.settings.link_previews, l.fromMe, l.sender)) return null;
    const s = app.previewState(l.href);
    return s.state === "ready" ? s.preview.title : null;
  }
</script>

<!-- Only shown while the panel floats over the chat: a click beside it closes it. -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
<div class="scrim" onclick={() => (app.info = false)}></div>
<aside class="info" aria-label="Conversation info">
  <header class="bar drag">
    <span class="label">{conv.is_group ? "Group info" : "Contact info"}</span>
    <button class="icon-btn" title="Close  Esc" aria-label="Close info" onclick={() => (app.info = false)}>
      <Icon name="close" size={18} />
    </button>
  </header>

  <div class="scroll">
    <section class="who">
      <Avatar name={conv.name} color={conv.color} group={conv.is_group} id={conv.avatar_id} size={88} />
      <h2>{conv.name}</h2>
      {#if person && person.formatted_number && person.formatted_number !== conv.name}
        <button class="number" title="Copy number" onclick={() => app.copyText(person.number || person.formatted_number)}>
          {person.formatted_number}
        </button>
      {/if}
      {#if conv.is_group}
        <p class="subtle small">{people.length} {people.length === 1 ? "member" : "members"}</p>
      {/if}
      <div class="tags">
        <span class="tag">{conv.is_rcs ? "RCS" : "SMS/MMS"}</span>
        {#if person && !person.saved}<span class="tag warn">Not in your contacts</span>{/if}
      </div>
    </section>

    <section class="actions">
      {#if person?.number}
        <button onclick={() => openURL(`tel:${person.number}`)}><Icon name="phone" size={20} /><span>Call</span></button>
        <button onclick={() => app.copyText(person.number)}><Icon name="copy" size={20} /><span>Copy</span></button>
      {/if}
      {#if shareable.length}
        <button title={app.shareSupported ? "Share contact card" : "Save contact card (.vcf)"} onclick={share}>
          <Icon name="share" size={20} /><span>Share</span>
        </button>
      {/if}
      <button onclick={() => app.convAction(conv, muted ? "unmute" : "mute")}>
        <Icon name={muted ? "bell" : "bell-off"} size={20} /><span>{muted ? "Unmute" : "Mute"}</span>
      </button>
      <button onclick={() => app.convAction(conv, conv.archived ? "unarchive" : "archive")}>
        <Icon name="archive" size={20} /><span>{conv.archived ? "Unarchive" : "Archive"}</span>
      </button>
    </section>

    {#if conv.is_group && people.length}
      <section>
        <h3>Members</h3>
        <ul class="people">
          {#each people as p (p.id)}
            <li>
              <button class="person" title="Message {p.name}" onclick={() => app.chatWith(p.number)}>
                <Avatar name={p.name} color={p.color} id={p.id} size={34} />
                <span class="text">
                  <span class="name">{p.name}</span>
                  {#if p.formatted_number && p.formatted_number !== p.name}
                    <span class="subtle small">{p.formatted_number}</span>
                  {:else if !p.saved}
                    <span class="subtle small">Not in your contacts</span>
                  {/if}
                </span>
              </button>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if photos.length}
      <section>
        <h3>Photos <span class="subtle small">· recent</span></h3>
        <div class="grid">
          {#each photos as ph (`${ph.message.id}:${ph.index}`)}
            {@const s = thumb(ph.media)}
            <button class="tile" aria-label="Open photo" onclick={() => app.openViewer(conv.id, ph.message.id, ph.index)}>
              {#if s?.state === "ready"}<img src={mediaURL(s.path)} alt="" draggable="false" />{/if}
            </button>
          {/each}
        </div>
      </section>
    {/if}

    {#if links.length}
      <section>
        <h3>Links <span class="subtle small">· recent</span></h3>
        <ul class="links">
          {#each links as l (l.href)}
            {@const t = title(l)}
            <li>
              <a href={l.href} title={l.href} rel="noreferrer" draggable="false" onclick={(e) => { e.preventDefault(); openURL(l.href); }}>
                <Icon name="link" size={16} />
                <span class="text">
                  {#if t}<span class="name">{t}</span>{/if}
                  <span class:subtle={!!t} class:small={!!t} class="url">{shortURL(l.href)}</span>
                </span>
              </a>
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    {#if !photos.length && !links.length && app.threads[conv.id]?.loaded}
      <p class="empty subtle small">Photos and links from this chat show up here.</p>
    {/if}
  </div>
</aside>

<style>
  .info {
    flex: none;
    width: 300px;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--pane);
    border-left: 1px solid var(--divider);
  }
  /* A narrow window keeps the thread readable: the panel floats over it. */
  .scrim {
    display: none;
  }
  @media (max-width: 760px) {
    .scrim {
      display: block;
      position: absolute;
      inset: 0;
      z-index: 19;
    }
    .info {
      position: absolute;
      top: 0;
      right: 0;
      bottom: 0;
      z-index: 20;
      background: var(--bg);
      box-shadow: -6px 0 24px var(--shadow);
    }
  }
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: calc(var(--top-inset) + 14px) 10px 10px 18px;
    min-height: 59px;
  }
  .label {
    font-size: 14px;
    color: var(--subtle);
  }
  .icon-btn {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    border-radius: 50%;
    color: var(--subtle);
  }
  .icon-btn:hover,
  .icon-btn:focus-visible {
    background: var(--hover);
    color: var(--text);
  }
  .scroll {
    flex: 1;
    overflow-y: auto;
    padding: 0 16px 24px;
  }
  section {
    padding: 16px 0;
    border-top: 1px solid var(--divider);
  }
  .who {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    text-align: center;
    border-top: none;
    padding-top: 4px;
  }
  h2 {
    user-select: text;
    -webkit-user-select: text;
    cursor: text;
    margin: 8px 0 0;
    font-size: 19px;
    font-weight: 500;
    overflow-wrap: anywhere;
  }
  .who p {
    margin: 0;
  }
  h3 {
    margin: 0 0 10px;
    font-size: 13px;
    font-weight: 500;
  }
  .number {
    color: var(--subtle);
    padding: 2px 8px;
    border-radius: 6px;
  }
  .number:hover {
    background: var(--hover);
    color: var(--text);
  }
  .tags {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 6px;
    margin-top: 4px;
  }
  .tag {
    font-size: 11.5px;
    padding: 2px 8px;
    border-radius: 999px;
    background: var(--hover);
    color: var(--subtle);
  }
  .tag.warn {
    background: var(--warn-bg);
    color: var(--text);
  }
  .actions {
    display: flex;
    justify-content: center;
    gap: 2px;
  }
  .actions button {
    flex: 1;
    max-width: 66px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 5px;
    padding: 9px 4px 7px;
    border-radius: 12px;
    font-size: 11.5px;
    color: var(--accent);
  }
  .actions button span {
    color: var(--text);
  }
  .actions button:hover,
  .actions button:focus-visible {
    background: var(--hover);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .person,
  .links a {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 7px 8px;
    margin: 0 -8px;
    box-sizing: content-box;
    border-radius: var(--row-radius, 8px);
    text-align: left;
    color: inherit;
    text-decoration: none;
  }
  .links a {
    gap: 10px;
    color: var(--subtle);
  }
  .person:hover,
  .person:focus-visible,
  .links a:hover,
  .links a:focus-visible {
    background: var(--hover);
  }
  .text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .text > span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .name,
  .url {
    color: var(--text);
  }
  .url.subtle {
    color: var(--subtle);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 4px;
  }
  .tile {
    aspect-ratio: 1;
    border-radius: 6px;
    overflow: hidden;
    background: var(--hover);
    padding: 0;
  }
  .tile img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
  }
  .tile:hover img {
    opacity: 0.85;
  }
  .empty {
    text-align: center;
    padding: 16px 0;
    border-top: 1px solid var(--divider);
    margin: 0;
  }
</style>
