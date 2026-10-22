const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

export function sameDay(a: number, b: number): boolean {
  return day(new Date(a)) === day(new Date(b));
}

export function formatTime(ts: number): string {
  return new Date(ts).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}

/** Time in the conversation list: Now, 5 min, 3:42 PM, Tue, Oct 7, Oct 7, 2024. */
export function shortTime(ts: number, now = Date.now()): string {
  if (!ts) return "";
  const t = new Date(ts);
  const n = new Date(now);
  const minutes = Math.floor((now - ts) / 60000);
  if (minutes < 1) return "Now";
  if (minutes < 60) return `${minutes} min`;
  if (sameDay(ts, now)) return formatTime(ts);
  if (now - ts < 7 * 86400000) return t.toLocaleDateString([], { weekday: "short" });
  if (t.getFullYear() === n.getFullYear()) return t.toLocaleDateString([], { month: "short", day: "numeric" });
  return t.toLocaleDateString([], { month: "short", day: "numeric", year: "numeric" });
}

/** Day separators in a thread: Today, Yesterday, Tuesday, Monday, October 5… */
export function dayLabel(ts: number, now = Date.now()): string {
  const diff = Math.round((day(new Date(now)) - day(new Date(ts))) / 86400000);
  const t = new Date(ts);
  if (diff === 0) return "Today";
  if (diff === 1) return "Yesterday";
  if (diff < 7) return t.toLocaleDateString([], { weekday: "long" });
  if (t.getFullYear() === new Date(now).getFullYear()) {
    return t.toLocaleDateString([], { weekday: "long", month: "long", day: "numeric" });
  }
  return t.toLocaleDateString([], { month: "long", day: "numeric", year: "numeric" });
}
