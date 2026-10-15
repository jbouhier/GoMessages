// INIT_JS (src/bin/gomessages/webview.rs) in a fake page: notification shim,
// unread poller, keybinds, splash.

import { test, describe } from "node:test";
import assert from "node:assert/strict";
import { makePage, NOTIFY_FIXTURE } from "./harness.mjs";

describe("notification shim: happy path", () => {
  test("reports permission as granted", async () => {
    const p = makePage();
    assert.equal(p.run("Notification.permission"), "granted");
    assert.equal(await p.run("Notification.requestPermission()"), "granted");
    let cb = null;
    await p.run("Notification.requestPermission")((v) => (cb = v));
    assert.equal(cb, "granted", "legacy callback form");
  });

  test("forwards a notification to Rust", () => {
    const p = makePage();
    p.run("new Notification('Ana', {body: 'hey', tag: 'conv1'})");
    assert.deepEqual(p.notifyPosts(), [{ id: "1", title: "Ana", body: "hey", tag: "conv1" }]);
  });

  test("contract fixture: shim output is exactly what Rust parses", () => {
    for (const c of NOTIFY_FIXTURE.cases) {
      const p = makePage();
      p.ctx.__case = c;
      p.run("new Notification(__case.title, __case.options)");
      const notify = p.posts.filter((m) => m.startsWith("notify:"));
      assert.deepEqual(notify, [c.ipc], c.name);
    }
  });

  test("ids are unique and increasing", () => {
    const p = makePage();
    p.run("new Notification('a'); new Notification('b'); new Notification('c')");
    assert.deepEqual(
      p.notifyPosts().map((n) => n.id),
      ["1", "2", "3"],
    );
  });

  test("exposes the Notification fields page code reads", () => {
    const p = makePage();
    const n = p.run("new Notification('T', {body: 'B', tag: 'G', data: {x: 1}, silent: 1})");
    assert.equal(n.title, "T");
    assert.equal(n.body, "B");
    assert.equal(n.tag, "G");
    assert.equal(JSON.stringify(n.data), '{"x":1}');
    assert.equal(n.silent, true);
    assert.equal(p.run("Notification.maxActions"), 0);
  });

  test("banner click runs the page's onclick and listeners, and focuses", () => {
    const p = makePage();
    const n = p.run("window.__n = new Notification('Ana'); __n");
    const seen = [];
    n.onclick = (ev) => seen.push(["onclick", ev.type, ev.target === n]);
    n.addEventListener("click", (ev) => seen.push(["listener", ev.type]));
    p.run("__gomsgNotifClick('1')");
    assert.deepEqual(seen, [
      ["onclick", "click", true],
      ["listener", "click"],
    ]);
    assert.equal(p.focusCalls, 1);
  });

  test("onshow fires asynchronously, like a browser", () => {
    const p = makePage();
    let shown = 0;
    p.run("window.__n = new Notification('x')").onshow = () => shown++;
    assert.equal(shown, 0);
    p.timers.tick(0);
    assert.equal(shown, 1);
  });
});

describe("notification shim: failures", () => {
  test("no window.ipc: constructing still works", () => {
    const p = makePage({ ipc: false });
    assert.doesNotThrow(() => p.run("new Notification('x', {body: 'y'})"));
  });

  test("throwing handlers don't break the others", () => {
    const p = makePage();
    const n = p.run("new Notification('x')");
    let reached = 0;
    n.onclick = () => {
      throw new Error("page bug");
    };
    n.addEventListener("click", () => {
      throw new Error("page bug 2");
    });
    n.addEventListener("click", () => reached++);
    assert.doesNotThrow(() => p.run("__gomsgNotifClick('1')"));
    assert.equal(reached, 1);
  });

  test("requestPermission callback that throws still resolves", async () => {
    const p = makePage();
    const r = p.run("Notification.requestPermission")(() => {
      throw new Error("x");
    });
    assert.equal(await r, "granted");
  });

  test("click for an unknown or malformed id is a no-op", () => {
    const p = makePage();
    for (const id of ["999", "", "preview", null, undefined, "__proto__", "constructor"]) {
      p.ctx.__id = id;
      assert.doesNotThrow(() => p.run("__gomsgNotifClick(__id)"), String(id));
    }
  });

  test("not installed outside messages.google.com", () => {
    const p = makePage({ host: "accounts.google.com" });
    assert.equal(p.run("typeof window.Notification"), "undefined");
    assert.equal(p.run("typeof window.__gomsgNotifClick"), "undefined");
    p.timers.tick(10_000);
    assert.deepEqual(
      p.posts.filter((m) => m.startsWith("unread")),
      [],
      "no unread poller on sign-in pages",
    );
  });
});

describe("notification shim: edge cases", () => {
  test("close() fires close once and forgets the notification", () => {
    const p = makePage();
    const n = p.run("new Notification('x')");
    let closed = 0;
    let clicked = 0;
    n.onclose = () => closed++;
    n.onclick = () => clicked++;
    n.close();
    n.close();
    assert.equal(closed, 1);
    p.run("__gomsgNotifClick('1')");
    assert.equal(clicked, 0, "closed notification can't be clicked");
  });

  test("removeEventListener stops delivery", () => {
    const p = makePage();
    const n = p.run("new Notification('x')");
    let hits = 0;
    const f = () => hits++;
    n.addEventListener("click", f);
    n.removeEventListener("click", f);
    n.removeEventListener("click", () => {}); // unknown: no-op
    p.run("__gomsgNotifClick('1')");
    assert.equal(hits, 0);
  });

  test("keeps only the 50 newest notifications (no leak)", () => {
    const p = makePage();
    const hits = [];
    for (let i = 1; i <= 51; i++) {
      p.run(`new Notification('m${i}')`).onclick = () => hits.push(i);
    }
    p.run("__gomsgNotifClick('1'); __gomsgNotifClick('2'); __gomsgNotifClick('51')");
    assert.deepEqual(hits, [2, 51]);
  });

  test("service worker notifications are forwarded too", async () => {
    const p = makePage();
    const reg = p.run("new ServiceWorkerRegistration()");
    await reg.showNotification("SW", { body: "b", tag: "t" });
    await reg.showNotification("SW2");
    assert.deepEqual(p.notifyPosts(), [
      { id: "sw1", title: "SW", body: "b", tag: "t" },
      { id: "sw2", title: "SW2", body: "", tag: "" },
    ]);
    assert.equal((await reg.getNotifications()).length, 0);
  });

  test("page and SW ids never collide", () => {
    const p = makePage();
    p.run("new Notification('a'); new ServiceWorkerRegistration().showNotification('b')");
    const ids = p.notifyPosts().map((n) => n.id);
    assert.equal(new Set(ids).size, ids.length);
  });

  test("permissions.query: granted for notifications, untouched otherwise", async () => {
    const p = makePage();
    const n = await p.run("navigator.permissions.query({name: 'notifications'})");
    assert.equal(n.state, "granted");
    const other = await p.run("navigator.permissions.query({name: 'geolocation'})");
    assert.equal(other.original, true);
    assert.equal(other.state, "prompt");
  });

  test("no navigator.permissions: shim still installs", () => {
    const p = makePage({ permissions: null });
    assert.equal(p.run("Notification.permission"), "granted");
  });

  test("page can still replace window.Notification (configurable)", () => {
    const p = makePage();
    assert.doesNotThrow(() => p.run("window.Notification = function X(){}"));
  });
});

describe("unread poller", () => {
  test("first reading is a baseline, later changes are real", () => {
    const p = makePage({ title: "Messages (2)" });
    p.timers.tick(2000);
    p.state.title = "Messages (3)";
    p.timers.tick(2000);
    p.timers.tick(2000); // unchanged: no post
    p.state.title = "Messages";
    p.timers.tick(2000);
    assert.deepEqual(
      p.posts.filter((m) => m.startsWith("unread")),
      ["unread-baseline:2", "unread:3", "unread:0"],
    );
  });

  test("uses the larger of title count and unread rows", () => {
    const p = makePage({ title: "Messages (1)", unreadRows: 4 });
    p.timers.tick(2000);
    assert.deepEqual(p.posts, ["unread-baseline:4"]);
  });

  test("DOM query failure falls back to the title", () => {
    const p = makePage({
      title: "(5) Messages",
      unreadRows: () => {
        throw new Error("selector broke");
      },
    });
    p.timers.tick(2000);
    assert.deepEqual(p.posts, ["unread-baseline:5"]);
  });
});

describe("keybinds", () => {
  const cases = [
    ["=", { meta: true }, "zoom-in"],
    ["+", { ctrl: true }, "zoom-in"],
    ["x", { meta: true, code: "NumpadAdd" }, "zoom-in"],
    ["-", { meta: true }, "zoom-out"],
    ["0", { ctrl: true }, "zoom-reset"],
    [",", { meta: true }, "open-settings"],
  ];
  for (const [key, mods, msg] of cases) {
    test(`${JSON.stringify(mods)} + ${key} -> ${msg}`, () => {
      const p = makePage();
      const ev = p.key(key, mods);
      assert.deepEqual(p.posts, [msg]);
      assert.ok(ev.prevented);
    });
  }

  test("plain keys and other shortcuts pass through", () => {
    const p = makePage();
    for (const [k, m] of [["=", {}], ["k", { meta: true }], ["c", { ctrl: true }]]) {
      const ev = p.key(k, m);
      assert.equal(ev.prevented, false, k);
    }
    assert.deepEqual(p.posts, []);
  });
});

describe("splash", () => {
  test("covers the page, then fades out after load", () => {
    const p = makePage();
    assert.equal(p.root.children.length, 1);
    p.fire("load");
    p.timers.tick(400 + 350);
    assert.equal(p.root.children.length, 0);
  });

  test("never sticks if load never fires", () => {
    const p = makePage();
    p.timers.tick(15_000 + 350);
    assert.equal(p.root.children.length, 0);
  });
});
