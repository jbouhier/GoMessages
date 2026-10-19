/** How well `name` matches `query`: prefix beats word start beats substring
 *  beats letters in order ("mch" finds "Maya Chen"). null when it doesn't. */
export function score(name: string, query: string): number | null {
  const n = name.toLowerCase();
  const q = query.trim().toLowerCase();
  if (!q) return 0;
  if (n.startsWith(q)) return 1000 - n.length;
  if (n.split(/[^\p{L}\p{N}]+/u).some((w) => w.startsWith(q))) return 800 - n.length;
  if (n.includes(q)) return 600 - n.length;
  let gaps = 0;
  let i = 0;
  for (const qc of q) {
    while (i < n.length && n[i] !== qc) {
      i++;
      gaps++;
    }
    if (i >= n.length) return null;
    i++;
  }
  return 400 - gaps;
}

/** Number of emoji when a message is nothing but emoji (and spaces). */
export function emojiOnly(text: string): number | null {
  const seg = new Intl.Segmenter(undefined, { granularity: "grapheme" });
  let n = 0;
  for (const { segment } of seg.segment(text)) {
    if (!segment.trim()) continue;
    if (!/\p{Extended_Pictographic}|\p{Regional_Indicator}/u.test(segment)) return null;
    n++;
  }
  return n > 0 ? n : null;
}

/** Section letter for a contact list: A–Z, or # for the rest. */
export function section(name: string): string {
  const first = name.normalize("NFD").replace(/\p{M}/gu, "").trim().charAt(0).toUpperCase();
  return /[A-Z]/.test(first) ? first : "#";
}

/** Stable colour for someone without one, from their name. */
export function fallbackColor(seed: string): string {
  const colors = ["#1a73e8", "#d93025", "#188038", "#e37400", "#a142f4", "#007b83", "#c5221f", "#5f6368"];
  let h = 7;
  for (const ch of seed) h = (Math.imul(h, 31) + ch.charCodeAt(0)) >>> 0;
  return colors[h % colors.length];
}
