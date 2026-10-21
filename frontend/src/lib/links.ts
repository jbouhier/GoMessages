// Splits message text into plain runs and links, so bubbles can make URLs
// clickable. Pure, so it runs under `node --test`.

export type Segment = { text: string; href?: string };

// Bare domains only count with a known TLD, so "file.txt" or "v0.4.0" stay text.
const TLDS =
  "com|org|net|io|dev|app|co|ai|me|fr|uk|de|ca|us|eu|gov|edu|info|biz|tv|ly|gg|sh|xyz|so|to|be|nl|es|it|ch|au|jp|in|br|link|page|site|online|store|shop|tech|news|blog";

const URL_RE = new RegExp(
  [
    String.raw`https?:\/\/[^\s<>"]+`,
    String.raw`www\.[^\s<>"]+`,
    String.raw`[a-z0-9.!#$%&'*+\/=?^_\x60{|}~-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)+`,
    String.raw`(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+(?:${TLDS})(?::\d+)?(?:[\/?#][^\s<>"]*)?`,
  ].join("|"),
  "gi",
);

/** Drops sentence punctuation a link swallowed, and a ")" with no "(" in the link. */
function trimEnd(s: string): string {
  for (;;) {
    const last = s.at(-1);
    if (!last) return s;
    if (".,!?;:'\"*_~".includes(last)) s = s.slice(0, -1);
    else if (last === ")" && (s.match(/\(/g)?.length ?? 0) < (s.match(/\)/g)?.length ?? 0)) s = s.slice(0, -1);
    else if (last === "]" && !s.includes("[")) s = s.slice(0, -1);
    else return s;
  }
}

export function linkify(text: string): Segment[] {
  const out: Segment[] = [];
  let at = 0;
  for (const m of text.matchAll(URL_RE)) {
    const start = m.index;
    // Part of a longer word ("foo.bar.com" inside "xfoo") or of an email's tail.
    if (start > 0 && /[\w@.\/-]/.test(text[start - 1])) continue;
    const raw = trimEnd(m[0]);
    if (!raw.includes(".")) continue;
    const href = /^https?:\/\//i.test(raw)
      ? raw
      : raw.includes("@") && !raw.includes("/")
        ? `mailto:${raw}`
        : `https://${raw}`;
    if (start > at) out.push({ text: text.slice(at, start) });
    out.push({ text: raw, href });
    at = start + raw.length;
  }
  if (at < text.length) out.push({ text: text.slice(at) });
  return out;
}

/** A web page a preview can be fetched for (not mailto:). */
export function isWebLink(href: string): boolean {
  return /^https?:\/\//i.test(href);
}

/** "github.com/jb/repo" for a chip with no title yet: no scheme, no www,
 *  no trailing slash. The full address stays in the hover card. */
export function shortURL(href: string): string {
  try {
    const u = new URL(href);
    const host = u.hostname.replace(/^www\./, "");
    const rest = (u.pathname === "/" ? "" : u.pathname.replace(/\/$/, "")) + u.search;
    return host + rest;
  } catch {
    return href;
  }
}

/** A name that's only a phone number or short code: someone not in the
 *  phone's contacts. Their links aren't fetched unless the user says so. */
export function looksLikeNumber(name: string): boolean {
  return !/\p{L}/u.test(name) && /\d{3}/.test(name);
}

export type PreviewPolicy = "contacts" | "everyone" | "off";

/** Whether to fetch a preview for a link in this message. Fetching tells
 *  the site someone opened it, so a stranger's link stays plain by default. */
export function mayPreview(policy: string, fromMe: boolean, sender: string): boolean {
  if (policy === "off") return false;
  if (policy === "everyone" || fromMe) return true;
  return !!sender && !looksLikeNumber(sender);
}
