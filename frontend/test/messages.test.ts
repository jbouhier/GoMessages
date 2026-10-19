// Unit tests for matching sends to the phone's echo. Run with
// `just unit`, or from frontend/:
// `node --test --experimental-strip-types test/*.test.ts` (Node 22.6+).
import { test } from "node:test";
import assert from "node:assert/strict";
import { findMessage, isPending, newTmpID, upsertMessage } from "../src/lib/messages.ts";
import type { Message } from "../src/lib/types.ts";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const msg = (over: Partial<Message>): Message => ({
  id: "1",
  conversation_id: "c1",
  timestamp: 1000,
  from_me: false,
  sender: "",
  text: "",
  status: "received",
  ...over,
});

/** The bubble sendDraft shows before the phone answers. */
const pending = (tmp: string, text: string, over: Partial<Message> = {}) =>
  msg({ id: `tmp_${tmp}`, tmp_id: tmp, from_me: true, text, status: "sending", ...over });

/** The phone's copy of a sent message. */
const echo = (id: string, text: string, over: Partial<Message> = {}) =>
  msg({ id, from_me: true, text, status: "sent", ...over });

test("newTmpID is a v4 UUID like the native app's", () => {
  const a = newTmpID();
  assert.match(a, UUID);
  assert.notEqual(a, newTmpID());
});

test("newTmpID falls back when randomUUID is missing (insecure context)", () => {
  const orig = crypto.randomUUID;
  Object.defineProperty(crypto, "randomUUID", { value: undefined, configurable: true });
  try {
    assert.match(newTmpID(), UUID);
  } finally {
    Object.defineProperty(crypto, "randomUUID", { value: orig, configurable: true });
  }
});

test("echo with the same tmp id replaces the placeholder", () => {
  const msgs = [pending("u1", "hi")];
  upsertMessage(msgs, echo("100", "hi", { tmp_id: "u1" }));
  assert.equal(msgs.length, 1);
  assert.equal(msgs[0].id, "100");
});

// The bug: the phone echoed our send with a tmp id we didn't send, so the
// placeholder stayed and the message showed twice.
test("echo with a rewritten tmp id still replaces the placeholder", () => {
  const msgs = [pending("u1", "hi")];
  upsertMessage(msgs, echo("100", "hi", { tmp_id: "phone-made-this-up" }));
  assert.equal(msgs.length, 1);
  assert.equal(msgs[0].id, "100");
  assert.ok(!msgs.some(isPending));
});

test("echo without a tmp id still replaces the placeholder", () => {
  const msgs = [pending("u1", "hi")];
  upsertMessage(msgs, echo("100", "hi"));
  assert.equal(msgs.length, 1);
  assert.equal(msgs[0].id, "100");
});

test("a fetched page (no tmp ids) doesn't duplicate a pending send", () => {
  const msgs = [msg({ id: "50", text: "earlier" }), pending("u1", "hi", { timestamp: 2000 })];
  for (const m of [msg({ id: "50", text: "earlier" }), echo("100", "hi", { timestamp: 2000 })]) upsertMessage(msgs, m);
  assert.deepEqual(
    msgs.map((m) => m.id),
    ["50", "100"],
  );
});

test("status updates after the swap land on the same bubble", () => {
  const msgs = [pending("u1", "hi")];
  upsertMessage(msgs, echo("100", "hi", { tmp_id: "x" }));
  upsertMessage(msgs, echo("100", "hi", { status: "delivered" }));
  upsertMessage(msgs, echo("100", "hi", { status: "read" }));
  assert.equal(msgs.length, 1);
  assert.equal(msgs[0].status, "read");
});

test("two identical texts sent quickly match in order, one each", () => {
  const msgs = [pending("u1", "ok", { timestamp: 1 }), pending("u2", "ok", { timestamp: 2 })];
  upsertMessage(msgs, echo("100", "ok", { tmp_id: "x", timestamp: 1 }));
  assert.deepEqual(
    msgs.map((m) => m.id),
    ["100", "tmp_u2"],
  );
  upsertMessage(msgs, echo("101", "ok", { tmp_id: "y", timestamp: 2 }));
  assert.deepEqual(
    msgs.map((m) => m.id),
    ["100", "101"],
  );
});

test("an attachment echo replaces the 📎 placeholder", () => {
  const msgs = [pending("u1", "📎 cat.jpg")];
  upsertMessage(msgs, echo("100", "", { media: [{ id: "m1" } as any] }));
  assert.equal(msgs.length, 1);
  assert.equal(msgs[0].id, "100");
});

test("an incoming message never takes a pending placeholder", () => {
  const msgs = [pending("u1", "hi")];
  upsertMessage(msgs, msg({ id: "100", text: "hi", timestamp: 2000 }));
  assert.equal(msgs.length, 2);
  assert.ok(isPending(msgs[0]));
});

test("a send in another conversation doesn't take the placeholder", () => {
  const msgs = [pending("u1", "hi")];
  upsertMessage(msgs, echo("100", "hi", { conversation_id: "c2" }));
  assert.equal(msgs.length, 2);
});

test("a different text doesn't take the placeholder", () => {
  const msgs = [pending("u1", "hi")];
  upsertMessage(msgs, echo("100", "sent from the phone itself", { timestamp: 2000 }));
  assert.equal(msgs.length, 2);
});

test("a failed send can still be matched if the phone sends it after all", () => {
  const msgs = [pending("u1", "hi", { status: "failed" })];
  upsertMessage(msgs, echo("100", "hi"));
  assert.equal(msgs.length, 1);
  assert.equal(msgs[0].status, "sent");
});

test("new messages insert in timestamp order", () => {
  const msgs = [msg({ id: "1", timestamp: 1 }), msg({ id: "3", timestamp: 3 })];
  upsertMessage(msgs, msg({ id: "2", timestamp: 2 }));
  assert.deepEqual(
    msgs.map((m) => m.id),
    ["1", "2", "3"],
  );
});

test("findMessage reports -1 for a new message", () => {
  assert.equal(findMessage([pending("u1", "hi")], msg({ id: "9", text: "new" })), -1);
});
