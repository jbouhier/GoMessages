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
