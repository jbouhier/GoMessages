// Message list bookkeeping: matching the phone's echo of a send to the
// optimistic bubble shown at once. Pure, so it runs under `node --test`.
import type { Message } from "./types";

/** Local placeholder for a send the phone hasn't echoed yet. */
export const isPending = (x: Message) => x.id.startsWith("tmp_");

/** A tmp id shaped like the native app's (a bare UUID). */
export function newTmpID(): string {
  if (typeof crypto.randomUUID === "function") return crypto.randomUUID();
  return "10000000-1000-4000-8000-100000000000".replace(/[018]/g, (c) =>
    (+c ^ (crypto.getRandomValues(new Uint8Array(1))[0] & (15 >> (+c / 4)))).toString(16),
  );
}

/** Index of the stored copy of m: same id, same tmp id, or (when the phone
 *  echoes our send with a rewritten or missing tmp id) the oldest pending
 *  placeholder in the conversation with the same text. -1 if none. */
export function findMessage(msgs: Message[], m: Message): number {
  const i = msgs.findIndex((x) => x.id === m.id || (!!m.tmp_id && (x.id === m.tmp_id || x.tmp_id === m.tmp_id)));
  if (i >= 0 || !m.from_me || isPending(m)) return i;
  return msgs.findIndex(
    (x) =>
      isPending(x) &&
      x.conversation_id === m.conversation_id &&
      (x.text === m.text || (x.text.startsWith("📎 ") && !!m.media?.length)),
  );
}

/** Insert or replace a message, keeping the list in timestamp order. */
export function upsertMessage(msgs: Message[], m: Message): void {
  const i = findMessage(msgs, m);
  if (i >= 0) {
    msgs[i] = m;
    return;
  }
  let at = msgs.length;
  while (at > 0 && msgs[at - 1].timestamp > m.timestamp) at--;
  msgs.splice(at, 0, m);
}
