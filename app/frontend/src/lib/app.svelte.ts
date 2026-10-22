// Application state and everything that talks to the backend: a port of
// gogo-messages' app.rs and actions.rs onto Svelte runes.

import { flushSync } from "svelte";
import { Go, on, overHTTP } from "./go";
import type { Settings as GoSettings } from "../../bindings/gomessages/models";
import type { Contact, Conversation, LinkPreview, Media, Message, SimChoice } from "./types";
import { isImage } from "./types";
import { findMessage, isPending, newTmpID, upsertMessage } from "./messages";
import { score } from "./match";
import { themeOf } from "./themes";

export type Link = "starting" | "unpaired" | "pairing" | "connecting" | "connected" | "phone_offline" | "error";
export type Pairing = { kind: "idle" } | { kind: "signing-in" } | { kind: "emoji"; emoji: string; account: string };
export type MediaState = { state: "loading" } | { state: "ready"; path: string } | { state: "failed" };
export type PreviewState = { state: "loading" } | { state: "ready"; preview: LinkPreview } | { state: "failed" };
/** The hover card over a link chip: which link, and where the chip is. */
export type LinkCard = { href: string; rect: { left: number; top: number; bottom: number; width: number } };
export type ThreadScroll = "page-up" | "page-down" | "top" | "bottom";

export interface Thread {
  msgs: Message[];
  more: boolean;
  loading: boolean;
  /** The request in flight pages older messages (the only load the UI shows). */
  older: boolean;
  loaded: boolean;
  loadingSince: number;
  /** Bumped per request so a stale answer can't clear a newer one's state. */
  seq: number;
  /** No older-page retry before this time, after one failed. */
  olderRetryAt: number;
}

export type Settings = Omit<GoSettings, "window" | "maximized"> & { window?: unknown; maximized?: boolean };

export const COMPACT_WIDTH = 76;
export const MIN_WIDE_WIDTH = 240;
export const MAX_WIDTH = 560;
export const ZOOM_STEPS = [0.67, 0.75, 0.8, 0.9, 1.0, 1.1, 1.25, 1.5, 1.75, 2.0, 2.5];

export function zoomStep(zoom: number, up: boolean): number {
  if (up) return ZOOM_STEPS.find((z) => z > zoom + 0.01) ?? zoom;
  return [...ZOOM_STEPS].reverse().find((z) => z < zoom - 0.01) ?? zoom;
}

/** Width for a divider position: below the narrow end, photos only. */
export function snapWidth(raw: number): number {
  if (raw < (COMPACT_WIDTH + MIN_WIDE_WIDTH) / 2) return COMPACT_WIDTH;
  return Math.min(Math.max(raw, MIN_WIDE_WIDTH), MAX_WIDTH);
}

/** Every photo in a thread, oldest first, keyed by message and position
 *  (media IDs can repeat or be empty, so they can't tell photos apart). */
export function albumOf(msgs: Message[]): { message: Message; media: Media; index: number }[] {
  return msgs.flatMap((message) =>
    (message.media ?? []).filter(isImage).map((media, index) => ({ message, media, index })),
  );
}

const sortConvs = (list: Conversation[]) =>
  list.sort((a, b) => Number(b.pinned) - Number(a.pinned) || b.timestamp - a.timestamp);

export class AppState {
  demo = $state(false);
  /** The window can blur what's behind it (macOS, Windows). */
  blurSupported = $state(false);
  link = $state<Link>("starting");
  account = $state("");
  pairing = $state<Pairing>({ kind: "idle" });
  pairError = $state<string | null>(null);

  convs = $state<Conversation[]>([]);
  convsLoading = $state(false);
  selected = $state<string | null>(null);
  threads = $state<Record<string, Thread>>({});
  drafts = $state<Record<string, string>>({});
  /** The message each conversation's next send replies to. */
  replyTo = $state<Record<string, Message>>({});
  search = $state("");
  typing = $state<Record<string, number>>({});
  media = $state<Record<string, MediaState>>({});
  previews = $state<Record<string, PreviewState>>({});
  linkCard = $state<LinkCard | null>(null);
  avatars = $state<Record<string, string | null>>({});
  attachment = $state<string | null>(null);
  toast = $state<string | null>(null);

  settings = $state<Settings>({
    mode: "system",
    theme: "default",
    background: "gradient",
    transparency: 0,
    zoom: 1,
    sidebar_visible: true,
    sidebar_width: 340,
    sidebar_wide_width: 340,
    notifications: true,
    notification_preview: true,
    notification_sound: true,
    enter_sends: true,
    send_typing: true,
    link_previews: "contacts",
    muted: [],
  });
  settingsOpen = $state(false);
  confirmUnpair = $state(false);
  showArchived = $state(false);
  switcher = $state<{ query: string; index: number } | null>(null);
  showShortcuts = $state(false);
  viewer = $state<{ conv: string; message: string; index: number } | null>(null);
  newChat = $state<{ query: string; index: number; busy: boolean; error: string | null } | null>(null);
  contacts = $state<Contact[]>([]);
  contactsLoading = $state(false);
  contactsError = $state<string | null>(null);
  sims = $state<SimChoice[]>([]);
  selectedSims = $state<Record<string, number>>({});

  /** The open thread is at its newest message. */
  atBottom = $state(true);
  /** New messages that arrived below while reading older ones. */
  unseenBelow = $state(0);
  /** A keyboard scroll for the thread to apply. */
  threadScroll = $state<{ kind: ThreadScroll; seq: number } | null>(null);
  /** Bumped to ask the search field / message box for focus. */
  focusSearch = $state(0);
  focusComposer = $state(0);

  private avatarQueue = new Set<string>();
  private mediaRequested = new Set<string>();
  private previewRequested = new Set<string>();
  private lastTypingSent = 0;
  private notificationsAsked = false;
  private toastTimer: number | undefined;
  private settingsTimer: number | undefined;

  visible = $derived.by(() => {
    const q = this.search.trim().toLowerCase();
    return this.convs.filter(
      (c) =>
        c.archived === this.showArchived &&
        (!q ||
          c.name.toLowerCase().includes(q) ||
          c.snippet.toLowerCase().includes(q) ||
          (c.members ?? []).some((m) => m.toLowerCase().includes(q))),
    );
  });

  /** The switcher's rows: best match first, ties in list (most recent
   *  first) order. ⌘1–9 pick from these while the switcher is open. */
  switcherMatches = $derived.by(() => {
    const q = this.switcher?.query;
    if (q === undefined) return [];
    return this.convs
      .filter((c) => !c.archived)
      .map((c, order) => {
        const best = Math.max(...[c.name, ...(c.members ?? [])].map((n) => score(n, q) ?? -Infinity));
        return { c, best, order };
      })
      .filter((x) => x.best > -Infinity)
      .sort((a, b) => b.best - a.best || a.order - b.order)
      .slice(0, 9)
      .map((x) => x.c);
  });

  /** The system is in dark mode. */
  systemDark = $state(matchMedia("(prefers-color-scheme: dark)").matches);
  /** Dark is in use: by the mode, or the system's when following it. */
  dark = $derived(this.settings.mode === "dark" || (this.settings.mode !== "light" && this.systemDark));
  /** The theme's art shows (or everything is solid). */
  gradient = $derived(this.settings.background !== "solid");
  /** A theme shown while its swatch is hovered or focused; never saved. */
  previewTheme = $state<string | null>(null);
  /** The theme's id (or the one previewed); Default for one that isn't there. */
  theme = $derived(themeOf(this.previewTheme ?? this.settings.theme).id);

  compact = $derived(this.settings.sidebar_width < MIN_WIDE_WIDTH);
  unreadCount = $derived(this.convs.filter((c) => c.unread && !c.archived).length);
  current = $derived(this.convs.find((c) => c.id === this.selected));

  // ---- start ----

  async start() {
    const dark = matchMedia("(prefers-color-scheme: dark)");
    dark.addEventListener("change", () => this.readSystemDark());
    on("gm", (d: { event: string; data: any }) => this.onEvent(d.event, d.data));
    on("action", (name: string) => this.act(name));
    // In a browser there is no menu bar; tests send its actions and read
    // the state through this.
    if (overHTTP) (window as any).gm = this;
    on("signin_cancelled", () => {
      if (this.pairing.kind === "signing-in") this.pairing = { kind: "idle" };
    });
    this.demo = await Go.IsDemo();
    this.blurSupported = await Go.BlurSupported();
    const s = await Go.GetSettings();
    this.settings = { ...this.settings, ...s };
    // Events emitted before we listened are replayed by asking for the state.
    const state = await this.call("state");
    if (state) this.onEvent("state", state);
    window.setInterval(() => this.tick(), 1000);
  }

  private tick() {
    const now = Date.now();
    for (const [id, at] of Object.entries(this.typing)) if (now - at > 8000) delete this.typing[id];
  }

  async call(cmd: string, args: Record<string, unknown> = {}): Promise<any> {
    return Go.Call(cmd, args);
  }

  showToast(msg: string) {
    this.toast = msg;
    window.clearTimeout(this.toastTimer);
    this.toastTimer = window.setTimeout(() => (this.toast = null), 4000);
  }

  /** Shows or hides the conversation list. The window keeps its size. */
  async setListVisible(visible: boolean) {
    if (visible === this.settings.sidebar_visible) return;
    this.settings.sidebar_visible = visible;
    this.saveSettings();
  }

  saveSettings() {
    window.clearTimeout(this.settingsTimer);
    this.settingsTimer = window.setTimeout(() => {
      void Go.UpdateSettings($state.snapshot(this.settings) as GoSettings);
    }, 300);
  }

  /** Reads the system's light or dark (the page's, while the window
   *  follows the system). */
  readSystemDark() {
    const dark = matchMedia("(prefers-color-scheme: dark)").matches;
    if (dark === this.systemDark) return;
    // Forcing light or dark flips the window's appearance, and with it the
    // page's prefers-color-scheme. That flip changes nothing on screen, so
    // it skips the crossfade: a second transition would cut the mode's short.
    if (this.settings.mode === "system") this.restyle(() => (this.systemDark = dark));
    else this.systemDark = dark;
  }

  /** Applies a change to the look (theme, mode, background) with a
   *  crossfade of the whole window. Instant where the webview has no view
   *  transitions or the system asks for reduced motion. */
  restyle(change: () => void) {
    const doc = document as Document & { startViewTransition?: (update: () => void) => { ready?: Promise<unknown>; finished?: Promise<unknown> } };
    if (!doc.startViewTransition || matchMedia("(prefers-reduced-motion: reduce)").matches) return change();
    // The old frame is captured first; flushSync lands the new one (state,
    // DOM and the root's data-theme effect) before the fade starts.
    const vt = doc.startViewTransition(() => flushSync(change));
    // A second restyle before this fade ends skips it: the change still
    // lands, only the animation is dropped, and its promises reject with
    // "Transition was skipped". Expected, so not a page error. A change that
    // throws still surfaces through updateCallbackDone.
    vt?.ready?.catch(() => {});
    vt?.finished?.catch(() => {});
  }

  /** Flips to the other side of what's on screen. Leaves System behind:
   *  Settings or the theme menu bring it back. */
  toggleDark() {
    const mode = this.dark ? "light" : "dark";
    this.restyle(() => {
      this.settings.mode = mode;
      this.saveSettings();
    });
  }

  setZoom(zoom: number) {
    this.settings.zoom = zoom;
    void Go.SetZoom(zoom);
  }

  // ---- inbound ----

  onEvent(name: string, data: any) {
    switch (name) {
      case "state": {
        if (data?.account) this.account = data.account;
        const was = this.link;
        const map: Record<string, Link> = {
          unpaired: "unpaired",
          pairing: "pairing",
          connecting: "connecting",
          connected: "connected",
          phone_offline: "phone_offline",
        };
        this.link = map[data?.state] ?? this.link;
        if (this.link === "connected" && was !== "connected") {
          this.pairing = { kind: "idle" };
          this.pairError = null;
          void this.refreshConversations();
          void this.loadSims();
          if (this.selected) void this.loadMessages(this.selected, false);
        }
        break;
      }
      case "pair_emoji":
        this.pairError = null;
        this.pairing = { kind: "emoji", emoji: data.emoji ?? "", account: data.account ?? "" };
        break;
      case "paired":
        this.pairing = { kind: "idle" };
        this.link = "connecting";
        break;
      case "pair_error":
        this.pairing = { kind: "idle" };
        this.pairError = data?.message ?? "Pairing failed.";
        break;
      case "logged_out":
        this.convs = [];
        this.threads = {};
        this.selected = null;
        this.sims = [];
        this.selectedSims = {};
        this.pairError = data?.reason ?? null;
        break;
      case "conversation":
        this.upsertConversation(data as Conversation);
        break;
      case "message":
        this.onMessage(data as Message);
        break;
      case "typing":
        if (data.typing) this.typing[data.conversation_id] = Date.now();
        else delete this.typing[data.conversation_id];
        break;
      case "sims":
        this.sims = data ?? [];
        for (const [conv, n] of Object.entries(this.selectedSims)) {
          if (!this.sims.some((s) => s.number === n)) delete this.selectedSims[conv];
        }
        break;
    }
  }

  private upsertConversation(c: Conversation) {
    if (c.removed) return this.dropConversation(c.id);
    if (c.avatar_id) this.wantAvatar(c.avatar_id);
    const i = this.convs.findIndex((x) => x.id === c.id);
    if (i >= 0) this.convs[i] = c;
    else this.convs.push(c);
    sortConvs(this.convs);
  }

  private onMessage(m: Message) {
    const conv = m.conversation_id;
    // Deleted on the phone (or here): it leaves the thread.
    if (m.status === "deleted") return this.dropMessage(conv, m.id);
    const incoming = !m.from_me && !m.is_old && m.status === "received";
    if (incoming) {
      delete this.typing[conv];
      const viewing = this.selected === conv && document.hasFocus();
      if (!viewing) void this.notify(m);
    }
    const thread = this.threads[conv];
    if (thread) {
      const atEnd = !thread.msgs.length || m.timestamp >= thread.msgs[thread.msgs.length - 1].timestamp;
      const isNew = findMessage(thread.msgs, m) < 0;
      upsertMessage(thread.msgs, m);
      // Follow new messages only at the bottom (or our own); otherwise
      // count them on the ↓ button.
      if (atEnd && this.selected === conv && !this.atBottom && isNew && !m.from_me) this.unseenBelow++;
    }
    const c = this.convs.find((x) => x.id === conv);
    if (c && m.timestamp >= c.timestamp && m.text) {
      c.snippet = m.text;
      c.snippet_from_me = m.from_me;
      c.timestamp = m.timestamp;
      c.latest_message_id = m.id;
      if (incoming) c.unread = true;
      sortConvs(this.convs);
    }
  }

  private async notify(m: Message) {
    const s = this.settings;
    if (!s.notifications || this.isMuted(m.conversation_id)) return;
    if (!this.notificationsAsked) {
      this.notificationsAsked = true;
      await Go.RequestNotifications();
    }
    const c = this.convs.find((x) => x.id === m.conversation_id);
    const title = c ? (c.is_group && m.sender ? `${m.sender} · ${c.name}` : c.name) : m.sender || "New message";
    const body = !s.notification_preview ? "New message" : m.text || "📎 Attachment";
    void Go.Notify(title, body, s.notification_sound);
  }

  // ---- conversations and threads ----

  async refreshConversations() {
    this.convsLoading = true;
    try {
      const list: Conversation[] = await this.call("conversations", { count: 100 });
      for (const c of list) this.upsertConversation(c);
      if (!this.selected) {
        const first = this.visible[0];
        if (first) this.select(first.id);
      }
    } catch (e) {
      this.showToast(`Couldn't load conversations: ${e}`);
    } finally {
      this.convsLoading = false;
    }
  }

  async setArchivedView(archived: boolean) {
    this.showArchived = archived;
    this.search = "";
    void this.setListVisible(true);
    if (archived) {
      this.convsLoading = true;
      try {
        const list: Conversation[] = await this.call("conversations", { count: 100, folder: "archive" });
        for (const c of list) this.upsertConversation(c);
      } finally {
        this.convsLoading = false;
      }
    }
  }

  private dropConversation(id: string) {
    this.convs = this.convs.filter((x) => x.id !== id);
    delete this.threads[id];
    if (this.selected === id) {
      this.selected = null;
      const next = this.visible[0];
      if (next) this.select(next.id);
    }
  }

  isMuted(id: string) {
    return (this.settings.muted ?? []).includes(id);
  }

  /** Held unread after "Mark as unread", until it's opened again. */
  private keepUnread: string | null = null;

  /** A conversation's ⋮ menu: archive, block, trash, mute, unread. */
  async convAction(c: Conversation, action: "archive" | "unarchive" | "block_spam" | "delete" | "mute" | "unmute" | "unread" | "read") {
    if (action === "unread" || action === "read") {
      // The phone has no "mark unread": it's held here until opened.
      c.unread = action === "unread";
      if (action === "unread") this.keepUnread = c.id;
      else {
        if (this.keepUnread === c.id) this.keepUnread = null;
        const last = c.latest_message_id;
        if (last) void this.call("mark_read", { conversation_id: c.id, message_id: last });
      }
      return;
    }
    if (action === "mute" || action === "unmute") {
      const muted = (this.settings.muted ?? []).filter((x) => x !== c.id);
      if (action === "mute") muted.push(c.id);
      this.settings.muted = muted;
      this.saveSettings();
    }
    try {
      await this.call("update_conversation", { conversation_id: c.id, action });
    } catch (e) {
      // Mute is kept here either way: it's what silences notifications.
      if (action !== "mute" && action !== "unmute") this.showToast(`Couldn't update the conversation: ${e}`);
      return;
    }
    if (action === "archive" || action === "unarchive") {
      const wasOpen = this.selected === c.id;
      c.archived = action === "archive";
      if (wasOpen) {
        this.selected = null;
        const next = this.visible[0];
        if (next) this.select(next.id);
      }
      this.showToast(action === "archive" ? "Conversation archived" : "Conversation unarchived");
    } else if (action === "delete" || action === "block_spam") {
      this.dropConversation(c.id);
      this.showToast(action === "delete" ? "Conversation moved to trash" : "Blocked and reported as spam");
    }
  }

  select(id: string) {
    this.settingsOpen = false;
    this.newChat = null;
    const held = this.keepUnread === id;
    this.keepUnread = null;
    if (this.selected === id) {
      if (held) this.markReadIfVisible();
      return;
    }
    this.selected = id;
    this.attachment = null;
    this.unseenBelow = 0;
    this.atBottom = true;
    this.focusComposer++;
    // Always fetch the newest page: a first visit loads it, a return visit
    // catches anything missed.
    void this.loadMessages(id, false);
  }

  selectRelative(delta: number) {
    const ids = this.visible.map((c) => c.id);
    if (!ids.length) return;
    const cur = this.selected ? ids.indexOf(this.selected) : -1;
    const next = cur < 0 ? 0 : (((cur + delta) % ids.length) + ids.length) % ids.length;
    this.select(ids[next]);
  }

  selectUnread(dir: number) {
    const list = this.visible;
    if (!list.length) return;
    const start = this.selected ? list.findIndex((c) => c.id === this.selected) : -1;
    for (let step = 1; step <= list.length; step++) {
      const i = (((start + dir * step) % list.length) + list.length) % list.length;
      if (list[i].unread) return this.select(list[i].id);
    }
  }

  async loadMessages(conv: string, older: boolean) {
    const thread = (this.threads[conv] ??= { msgs: [], more: false, loading: false, older: false, loaded: false, loadingSince: 0, seq: 0, olderRetryAt: 0 });
    // A request that never got an answer doesn't block the thread forever.
    if (thread.loading && Date.now() - thread.loadingSince < 12000) return;
    const args: Record<string, unknown> = { conversation_id: conv, count: 40 };
    if (older) {
      const oldest = thread.msgs.find((m) => !m.id.startsWith("tmp_"));
      if (!oldest || !thread.more) return;
      args.before_id = oldest.id;
      args.before_ts = oldest.timestamp;
    }
    if (older && Date.now() < thread.olderRetryAt) return;
    const seq = ++thread.seq;
    thread.loading = true;
    thread.older = older;
    thread.loadingSince = Date.now();
    const t = this.threads[conv];
    try {
      const v = await this.call("messages", args);
      const msgs: Message[] = v.messages ?? [];
      if (older || !t.loaded) t.more = !!v.more;
      t.loaded = true;
      for (const m of msgs) upsertMessage(t.msgs, m);
    } catch (e) {
      // A background refresh failing (the phone was slow) costs nothing:
      // the thread still has what it showed. Say so only for what the user asked for.
      if (older) t.olderRetryAt = Date.now() + 15000;
      if (older || !t.loaded) this.showToast(`Couldn't load messages: ${e}`);
      if (!t.loaded) t.loaded = true;
    } finally {
      if (t.seq === seq) {
        t.loading = false;
        t.older = false;
      }
    }
  }

  /** Mark the open conversation read once the user can see it. */
  markReadIfVisible() {
    const sel = this.selected;
    if (!sel || !document.hasFocus()) return;
    const c = this.convs.find((x) => x.id === sel);
    if (!c?.unread || this.keepUnread === sel) return;
    c.unread = false;
    const last = [...(this.threads[sel]?.msgs ?? [])].reverse().find((m) => !m.from_me)?.id ?? c.latest_message_id;
    if (last) void this.call("mark_read", { conversation_id: sel, message_id: last });
  }

  async sendDraft() {
    const conv = this.selected;
    if (!conv) return;
    const currentSim = this.convs.find((c) => c.id === conv)?.sim_number;
    const selectedSim = this.selectedSims[conv] ?? currentSim;
    if (this.sims.length > 1 && !this.sims.some((s) => s.number === selectedSim)) {
      this.showToast("Choose an available SIM before sending.");
      return;
    }
    const text = (this.drafts[conv] ?? "").trim();
    const attachment = this.attachment;
    if (!text && !attachment) return;
    const replyTo = this.replyTo[conv]?.id;
    this.attachment = null;
    delete this.drafts[conv];
    delete this.replyTo[conv];
    // Same tmp id shape as the native app (a bare UUID); the local
    // placeholder id keeps the tmp_ prefix so it's never paged from.
    const tmpID = newTmpID();
    const tmp = `tmp_${tmpID}`;
    const name = attachment?.split(/[\\/]/).pop() ?? "";
    const shown = attachment ? (text ? `📎 ${name}\n${text}` : `📎 ${name}`) : text;
    const thread = (this.threads[conv] ??= { msgs: [], more: false, loading: false, older: false, loaded: true, loadingSince: 0, seq: 0, olderRetryAt: 0 });
    thread.msgs.push({
      id: tmp,
      tmp_id: tmpID,
      conversation_id: conv,
      timestamp: Date.now(),
      from_me: true,
      sender: "",
      text: shown,
      status: "sending",
      reply_to: replyTo,
    });
    this.scrollThread("bottom");
    const args: Record<string, unknown> = { conversation_id: conv, text, tmp_id: tmpID };
    if (replyTo) args.reply_to = replyTo;
    if (attachment) args.path = attachment;
    const sim =
      this.selectedSims[conv] ??
      (this.sims.length === 1 && currentSim !== this.sims[0].number ? this.sims[0].number : undefined);
    if (sim !== undefined) args.sim_number = sim;
    try {
      await this.call("send", args);
    } catch (e) {
      const m = this.threads[conv]?.msgs.find((x) => x.id === tmp);
      if (m) m.status = "failed";
      this.showToast(`Not sent: ${e}`);
    }
  }

  typed() {
    if (!this.settings.send_typing || !this.selected) return;
    if (Date.now() - this.lastTypingSent < 5000) return;
    this.lastTypingSent = Date.now();
    void this.call("typing", { conversation_id: this.selected });
  }

  /** Reply to a message: it shows above the message box until sent. */
  startReply(m: Message) {
    this.replyTo[m.conversation_id] = m;
    this.focusComposer++;
  }

  async copyMessage(m: Message) {
    try {
      await navigator.clipboard.writeText(m.text);
      this.showToast("Copied");
    } catch (e) {
      this.showToast(`Couldn't copy: ${e}`);
    }
  }

  /** Delete a message from the phone. One that never went out (still
   *  sending, or failed) only leaves the window. */
  async deleteMessage(m: Message) {
    const conv = m.conversation_id;
    const thread = this.threads[conv];
    const at = thread ? thread.msgs.findIndex((x) => x.id === m.id) : -1;
    this.dropMessage(conv, m.id);
    if (isPending(m)) return;
    try {
      await this.call("delete_message", { conversation_id: conv, message_id: m.id });
    } catch (e) {
      if (thread && at >= 0 && !thread.msgs.some((x) => x.id === m.id)) thread.msgs.splice(Math.min(at, thread.msgs.length), 0, m);
      this.showToast(`Couldn't delete the message: ${e}`);
    }
  }

  private dropMessage(conv: string, id: string) {
    const thread = this.threads[conv];
    if (thread) thread.msgs = thread.msgs.filter((x) => x.id !== id);
    if (this.replyTo[conv]?.id === id) delete this.replyTo[conv];
    // The list's snippet falls back to the message before it.
    const c = this.convs.find((x) => x.id === conv);
    if (c && c.latest_message_id === id && thread) {
      const last = [...thread.msgs].reverse().find((x) => x.status !== "system");
      c.snippet = last?.text ?? "";
      c.snippet_from_me = last?.from_me ?? false;
      c.latest_message_id = last?.id ?? "";
    }
  }

  react(msg: Message, emoji: string, remove: boolean) {
    void this.call("react", { conversation_id: msg.conversation_id, message_id: msg.id, emoji, remove });
  }

  /** A downloaded file, fetching it on first sight. Called while
   *  rendering, so the download starts just after (state can't change
   *  mid-render). */
  mediaState(id: string, key: string, mime: string): MediaState | undefined {
    if (!id) return undefined;
    const known = this.media[id];
    if (known) return known;
    if (!this.mediaRequested.has(id)) {
      this.mediaRequested.add(id);
      queueMicrotask(() => this.fetchMedia(id, key, mime));
    }
    return { state: "loading" };
  }

  private fetchMedia(id: string, key: string, mime: string) {
    this.media[id] = { state: "loading" };
    this.call("download", { media_id: id, key, mime }).then(
      (v) => (this.media[id] = v?.path ? { state: "ready", path: v.path } : { state: "failed" }),
      () => (this.media[id] = { state: "failed" }),
    );
  }

  /** A link's preview, fetched on first sight like media. */
  previewState(href: string): PreviewState {
    const known = this.previews[href];
    if (known) return known;
    if (!this.previewRequested.has(href)) {
      this.previewRequested.add(href);
      queueMicrotask(() => this.fetchPreview(href));
    }
    return { state: "loading" };
  }

  private fetchPreview(href: string) {
    this.previews[href] = { state: "loading" };
    this.call("link_preview", { url: href }).then(
      (p: LinkPreview | null) => (this.previews[href] = p?.title ? { state: "ready", preview: p } : { state: "failed" }),
      () => (this.previews[href] = { state: "failed" }),
    );
  }

  // ---- contact photos ----

  wantAvatar(id: string | undefined) {
    if (!id || id in this.avatars || this.avatarQueue.has(id)) return;
    this.avatarQueue.add(id);
    if (this.avatarQueue.size === 1) window.setTimeout(() => this.flushAvatars(), 50);
  }

  private async flushAvatars() {
    if (this.link !== "connected" && this.link !== "phone_offline") {
      window.setTimeout(() => this.flushAvatars(), 1000);
      return;
    }
    const ids = [...this.avatarQueue];
    this.avatarQueue.clear();
    for (const id of ids) this.avatars[id] = null;
    try {
      const map: Record<string, string> = await this.call("avatars", { ids });
      for (const [id, path] of Object.entries(map ?? {})) this.avatars[id] = path || null;
    } catch {
      /* initials stay */
    }
  }

  avatarPath(id: string | undefined): string | null {
    return id ? (this.avatars[id] ?? null) : null;
  }

  // ---- pairing and account ----

  async signIn() {
    this.pairError = null;
    this.pairing = { kind: "signing-in" };
    try {
      await Go.SignIn();
    } catch (e) {
      this.pairing = { kind: "idle" };
      this.pairError = String(e);
    }
  }

  cancelPairing() {
    this.pairing = { kind: "idle" };
    void this.call("cancel_pair");
  }

  logout() {
    void this.call("logout");
    this.convs = [];
    this.contacts = [];
    this.threads = {};
    this.selected = null;
    this.settingsOpen = false;
  }

  private async loadSims() {
    try {
      this.sims = (await this.call("sims")) ?? [];
    } catch {
      this.sims = [];
    }
  }

  // ---- new chat ----

  openNewChat() {
    this.settingsOpen = false;
    this.newChat = { query: "", index: 0, busy: false, error: null };
    // Show the last list from disk right away: the phone can take a while
    // (or never answer) and typing a number must not wait on it.
    if (!this.contacts.length) {
      this.call("contacts_cached").then(
        (list: Contact[] | null) => {
          if (!this.contacts.length && list?.length) this.setContacts(list);
        },
        () => {},
      );
    }
    this.refreshContacts();
  }

  refreshContacts() {
    if (this.contactsLoading) return;
    this.contactsLoading = true;
    this.contactsError = null;
    this.call("contacts").then(
      (list: Contact[]) => {
        this.setContacts(list ?? []);
        this.contactsLoading = false;
      },
      (e) => {
        this.contactsLoading = false;
        this.contactsError = `Couldn't load contacts: ${e}`;
      },
    );
  }

  private setContacts(list: Contact[]) {
    this.contacts = list;
    for (const c of list) this.wantAvatar(c.avatar_id);
  }

  contactMatches(query: string): Contact[] {
    const q = query.trim();
    const fold = (s: string) => s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();
    const scored = this.contacts
      .map((c) => {
        const best = Math.max(
          score(c.name, q) ?? -Infinity,
          score(c.number, q) ?? -Infinity,
          score(c.formatted_number, q) ?? -Infinity,
        );
        return best === -Infinity ? null : { c, s: best + (c.top ? 100 : 0) };
      })
      .filter((x): x is { c: Contact; s: number } => x !== null);
    if (!q) scored.sort((a, b) => fold(a.c.name).localeCompare(fold(b.c.name)) || a.c.number.localeCompare(b.c.number));
    else scored.sort((a, b) => b.s - a.s || fold(a.c.name).localeCompare(fold(b.c.name)));
    return scored.map((x) => x.c);
  }

  async startChatWithNumber(raw: string) {
    const nc = this.newChat;
    if (!nc) return;
    const number = [...raw].filter((c) => /[0-9+]/.test(c)).join("");
    if (number.replace(/^\+/, "").length < 3) {
      nc.error = "Enter a phone number or pick a contact.";
      return;
    }
    nc.busy = true;
    nc.error = null;
    try {
      const c: Conversation = await this.call("new_conversation", { number });
      this.newChat = null;
      this.showArchived = false;
      this.upsertConversation(c);
      this.select(c.id);
    } catch (e) {
      if (this.newChat) {
        this.newChat.busy = false;
        this.newChat.error = `Couldn't start the chat: ${e}`;
      }
    }
  }

  // ---- viewer ----

  openViewer(conv: string, message: string, index: number) {
    this.viewer = { conv, message, index };
  }

  // ---- actions (menu bar and keyboard) ----

  scrollThread(kind: ThreadScroll) {
    this.threadScroll = { kind, seq: (this.threadScroll?.seq ?? 0) + 1 };
  }

  async pickAttachment() {
    const path = await Go.PickFile();
    if (path) {
      this.attachment = path;
      this.focusComposer++;
    }
  }

  act(action: string) {
    const inApp = this.link === "connected" || this.link === "phone_offline" || this.link === "connecting";
    switch (action) {
      case "settings":
        this.newChat = null;
        this.settingsOpen = true;
        return;
      case "shortcuts":
        this.showShortcuts = !this.showShortcuts;
        return;
      case "zoom-in":
        return this.setZoom(zoomStep(this.settings.zoom, true));
      case "zoom-out":
        return this.setZoom(zoomStep(this.settings.zoom, false));
      case "zoom-reset":
        return this.setZoom(1);
      case "toggle-dark":
        return this.toggleDark();
    }
    if (!inApp) return;
    switch (action) {
      case "switch":
        this.switcher = this.switcher ? null : { query: "", index: 0 };
        break;
      case "search":
        if (this.compact) this.settings.sidebar_width = Math.max(this.settings.sidebar_wide_width, MIN_WIDE_WIDTH);
        this.saveSettings();
        void this.setListVisible(true).then(() => this.focusSearch++);
        break;
      case "new-chat":
        this.openNewChat();
        break;
      case "next":
        this.selectRelative(1);
        break;
      case "previous":
        this.selectRelative(-1);
        break;
      case "next-unread":
        this.selectUnread(1);
        break;
      case "previous-unread":
        this.selectUnread(-1);
        break;
      case "toggle-list":
        void this.setListVisible(!this.settings.sidebar_visible);
        break;
      case "photos-only":
        this.settings.sidebar_width = this.compact
          ? Math.max(this.settings.sidebar_wide_width, MIN_WIDE_WIDTH)
          : COMPACT_WIDTH;
        this.saveSettings();
        void this.setListVisible(true);
        break;
      case "archived":
        void this.setArchivedView(!this.showArchived);
        break;
      case "refresh":
        void this.refreshConversations();
        break;
      case "attach":
        if (this.selected) void this.pickAttachment();
        break;
      default:
        if (action.startsWith("jump-")) {
          const n = Number(action.slice(5));
          // With the switcher open, the number picks its row, as its hints show.
          const c = this.switcher ? this.switcherMatches[n] : this.visible[n];
          if (!c) return;
          this.switcher = null;
          this.select(c.id);
        }
    }
  }
}

export const app = new AppState();
