// The UI against the real Go backend on demo data, in headless Chrome. The
// same cases as gogo-messages' ui_tests.rs. Run with `just test` (it builds
// bin/gomessages first) or `bun test --timeout 60000 ./e2e` from frontend/.
import { after, before, beforeEach, test } from "node:test";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { chromium } from "playwright-core";
import { chromePath } from "./chrome.mjs";

const BIN = new URL("../../bin/gomessages", import.meta.url).pathname;
const ADDR = `127.0.0.1:${9300 + Math.floor(Math.random() * 500)}`;

let server, home, browser, page;

before(async () => {
  // A throwaway home: fresh demo data, nothing of the real app touched.
  home = mkdtempSync(join(tmpdir(), "gomessages-e2e-"));
  server = spawn(BIN, ["--demo", "--serve", ADDR], { env: { ...process.env, HOME: home }, stdio: "ignore" });
  for (let i = 0; ; i++) {
    try {
      await fetch(`http://${ADDR}/`);
      break;
    } catch (e) {
      if (i > 100) throw e;
      await new Promise((r) => setTimeout(r, 100));
    }
  }
  browser = await chromium.launch({ executablePath: chromePath() });
});

after(async () => {
  await browser?.close();
  server?.kill();
  rmSync(home, { recursive: true, force: true });
});

beforeEach(async () => {
  await page?.close();
  page = await browser.newPage({ viewport: { width: 1180, height: 760 } });
  page.on("pageerror", (e) => assert.fail(`page error: ${e.message}`));
  await page.goto(`http://${ADDR}/?http`);
  await page.getByRole("button", { name: "JB Bouhier" }).first().waitFor();
});

const state = (fn, arg) => page.evaluate(fn, arg);
const act = (name) => page.evaluate((n) => window.gm.act(n), name);
const until = (what, fn, arg) =>
  page.waitForFunction(fn, arg, { timeout: 5000 }).catch(() => assert.fail(`timed out waiting for ${what}`));
const distanceFromEnd = () =>
  state(() => {
    const l = document.querySelector(".thread .list");
    return l.scrollHeight - l.scrollTop - l.clientHeight;
  });

async function open(name, id) {
  await page.getByRole("button", { name }).first().click();
  await until(`${name}'s messages`, (id) => window.gm.threads[id]?.loaded, id);
  await page.waitForTimeout(300);
  assert.ok((await distanceFromEnd()) < 2, `${name} opens at the newest message`);
}

test("lists the conversations by name", async () => {
  for (const name of ["JB Bouhier", "Oliver Bennett", "Haruto Sato", "Lukas Weber"]) {
    assert.ok(await page.getByRole("button", { name }).first().isVisible(), `${name} in the list`);
  }
});

// Regression: switching chats showed an empty pane or stopped short of the
// newest message.
test("switching chats lands on the newest message every time", async () => {
  await open("Oliver Bennett", "4");
  await open("JB Bouhier", "1");
  await open("Oliver Bennett", "4");
});

// Regression: reaching the top loaded older pages but jumped the view.
test("scrolling up loads older messages and keeps the place", async () => {
  await open("Oliver Bennett", "4");
  const before = await state(() => window.gm.threads["4"].msgs.length);
  await page.locator(".thread .list").evaluate((l) => (l.scrollTop = 0));
  await until("an older page", (n) => window.gm.threads["4"].msgs.length > n && !window.gm.threads["4"].loading, before);
  await page.waitForTimeout(200);
  const top = await page.locator(".thread .list").evaluate((l) => l.scrollTop);
  assert.ok(top > 100, `the view was put back in place (scrollTop ${top})`);
  assert.equal(await state(() => window.gm.atBottom), false, "still reading older messages");
});

test("Page Up leaves the end and End returns to it", async () => {
  await open("Oliver Bennett", "4");
  await page.locator(".thread .list").click({ position: { x: 600, y: 10 } });
  await page.keyboard.press("PageUp");
  await until("Page Up to leave the end", () => !window.gm.atBottom);
  await page.keyboard.press("End");
  await until("End to reach the newest message", () => window.gm.atBottom);
  await page.waitForTimeout(600);
  assert.ok((await distanceFromEnd()) < 2);
});

test("⌘B hides and shows the list", async () => {
  await act("toggle-list");
  await until("the list to hide", () => !window.gm.settings.sidebar_visible);
  assert.ok(await page.getByRole("button", { name: "Show list" }).isVisible(), "the toggle offers to show it again");
  await act("toggle-list");
  await until("the list to show", () => window.gm.settings.sidebar_visible);
});

test("⌘K switches to the best match", async () => {
  await act("switch");
  await page.getByPlaceholder("Jump to a conversation…").fill("ob");
  await page.keyboard.press("Enter");
  await until("Oliver's chat", () => window.gm.selected === "4" && !window.gm.switcher);
});

const switcherRows = () => page.locator(".switcher .row");
const activeRow = () => switcherRows().evaluateAll((rs) => rs.findIndex((r) => r.classList.contains("active")));

// Regression: opening ⌘K under a resting pointer selected the row that
// rendered beneath it (the 4th) instead of the first.
test("⌘K opens on the first row even with the pointer resting over another", async () => {
  await act("switch");
  const box = await switcherRows().nth(3).boundingBox();
  await page.keyboard.press("Escape");
  await until("the switcher to close", () => !window.gm.switcher);
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await act("switch");
  await switcherRows().nth(3).waitFor();
  await page.waitForTimeout(100);
  assert.equal(await activeRow(), 0, "first row selected");
  await page.keyboard.press("ArrowDown");
  assert.equal(await activeRow(), 1, "arrows move from the first row");
});

test("⌘K: moving the pointer over a row selects it", async () => {
  await act("switch");
  const box = await switcherRows().nth(2).boundingBox();
  await page.mouse.move(box.x + 20, box.y + box.height / 2);
  await page.mouse.move(box.x + 40, box.y + box.height / 2);
  assert.equal(await activeRow(), 2);
});

test("settings open and Escape goes back", async () => {
  await open("JB Bouhier", "1");
  await page.getByPlaceholder("RCS message").focus();
  await act("settings");
  await until("settings", () => window.gm.settingsOpen);
  await page.keyboard.press("Escape");
  await until("Esc to close settings", () => !window.gm.settingsOpen);
});

test("new chat starts a conversation with a number", async () => {
  await act("new-chat");
  await page.getByPlaceholder("Name or phone number").fill("+15550009999");
  await page.keyboard.press("Enter");
  await until("the new chat to open", () => !window.gm.newChat && window.gm.selected?.startsWith("new-"));
});

// Regression: the viewer showed the same photo whatever was picked.
test("the viewer walks the photos and Escape closes it", async () => {
  await open("Mei Tanaka", "5");
  await page.locator(".photo").first().click();
  await until("the viewer", () => window.gm.viewer);
  const first = await state(() => ({ ...window.gm.viewer }));
  await page.keyboard.press("ArrowRight");
  await until("the second photo", (f) => window.gm.viewer && window.gm.viewer.message !== f.message, first);
  await page.keyboard.press("ArrowLeft");
  await until("the first photo again", (f) => window.gm.viewer?.message === f.message && window.gm.viewer.index === f.index, first);
  await page.keyboard.press("Escape");
  await until("the viewer to close", () => !window.gm.viewer);
});

test("a sent message goes from sending to read", async () => {
  await open("JB Bouhier", "1");
  const box = page.getByPlaceholder("RCS message");
  await box.fill("hello from a test");
  await box.press("Enter");
  const status = () => window.gm.threads["1"].msgs.findLast((m) => m.text === "hello from a test")?.status;
  assert.ok(await state(status), "shown at once");
  await until("the read receipt", () => window.gm.threads["1"].msgs.findLast((m) => m.text === "hello from a test")?.status === "read");
  await page.waitForTimeout(300);
  assert.ok((await distanceFromEnd()) < 2, "our own message keeps the view at the end");
  assert.equal(await page.locator(".thread .row", { hasText: "hello from a test" }).count(), 1, "shown once");
});

test("receipts: one grey check when sent, two when delivered, two in the accent when read", async () => {
  await open("JB Bouhier", "1");
  const box = page.getByPlaceholder("RCS message");
  const text = "checking the checks";
  await box.fill(text);
  await box.press("Enter");
  const receipt = () => page.locator(".thread .row", { hasText: text }).last().locator(".receipt");
  const look = () =>
    receipt().evaluate((r) => ({
      status: r.dataset.status,
      paths: r.querySelectorAll("path").length,
      color: getComputedStyle(r).color,
      label: r.getAttribute("aria-label"),
    }));
  const accent = await state(() => {
    const p = document.createElement("div");
    p.style.color = "var(--accent)";
    document.body.append(p);
    const c = getComputedStyle(p).color;
    p.remove();
    return c;
  });
  const seen = {};
  for (let i = 0; i < 60 && !seen.read; i++) {
    const r = await look().catch(() => null);
    if (r && !seen[r.status]) seen[r.status] = r;
    await page.waitForTimeout(50);
  }
  assert.equal(seen.sent?.paths, 1, "sent: one check");
  assert.equal(seen.delivered?.paths, 2, "delivered: two checks");
  assert.notEqual(seen.delivered?.color, accent, "delivered: grey");
  assert.equal(seen.read?.paths, 2, "read: two checks");
  await page.waitForTimeout(300);
  assert.equal((await look()).color, accent, "read: accent colour");
  assert.equal((await look()).label, "Read", "named for screen readers");
});

// Regression: the phone echoed a send with a tmp id other than ours, the
// placeholder never got replaced, and the message showed twice.
test("a send the phone echoes with a different tmp id shows once", async () => {
  await open("JB Bouhier", "1");
  await state(() => {
    const call = window.gm.call.bind(window.gm);
    window.gm.call = (cmd, args) => call(cmd, cmd === "send" ? { ...args, tmp_id: "rewritten-by-phone" } : args);
  });
  const box = page.getByPlaceholder("RCS message");
  await box.fill("echo with a new tmp id");
  await box.press("Enter");
  const text = "echo with a new tmp id";
  await until("the read receipt", (t) => window.gm.threads["1"].msgs.some((m) => m.text === t && m.status === "read"), text);
  assert.deepEqual(
    await state((t) => window.gm.threads["1"].msgs.filter((m) => m.text === t).map((m) => [m.id.startsWith("tmp_"), m.status]), text),
    [[false, "read"]],
    "one bubble, the phone's copy",
  );
  assert.equal(await page.locator(".thread .row", { hasText: "echo with a new tmp id" }).count(), 1, "rendered once");
});

// Regression: zooming in (page zoom shrinks the viewport) brought up
// horizontal and vertical scroll bars.
test("zoomed in to 300%, nothing overflows the window", async () => {
  await page.setViewportSize({ width: Math.round(1120 / 3), height: Math.round(740 / 3) });
  for (const screen of [null, "settings", "new-chat"]) {
    if (screen) await act(screen);
    await page.waitForTimeout(300);
    const over = await state(() => {
      const vw = document.documentElement.clientWidth;
      return [...document.querySelectorAll("main *")]
        .filter((e) => e.getBoundingClientRect().right > vw + 1 || e.scrollWidth > e.clientWidth + 1 && getComputedStyle(e).overflowX !== "hidden" && e.clientWidth > 0 && getComputedStyle(e).overflowX !== "visible")
        .map((e) => `${e.tagName}.${e.className}`);
    });
    assert.deepEqual(over, [], `${screen ?? "thread"}: elements wider than the window`);
  }
});

// Regression: a reaction badge covered the time and read receipt.
test("a reaction doesn't cover the time under the bubble", async () => {
  await open("JB Bouhier", "1");
  const row = page.locator(".row", { hasText: "owns the couch" }).first();
  await row.scrollIntoViewIfNeeded();
  const badge = await row.locator(".reactions").boundingBox();
  const meta = await row.locator(".meta").boundingBox();
  assert.ok(badge.y + badge.height <= meta.y + 1, `badge bottom ${badge.y + badge.height} over the time at ${meta.y}`);
});

// Regression: photos in the viewer were stretched to the previous photo's
// shape, or shown at half size instead of filling the space.
test("the viewer fills the space with each photo, in its own shape", async () => {
  await open("Mei Tanaka", "5");
  await page.locator(".photo").first().click();
  for (const step of ["first", "ArrowRight", "ArrowLeft"]) {
    if (step !== "first") await page.keyboard.press(step);
    await page.waitForTimeout(400);
    const r = await page.locator(".viewer .stage img").evaluate((img) => {
      const s = img.parentElement;
      return { w: img.clientWidth, h: img.clientHeight, nw: img.naturalWidth, nh: img.naturalHeight, sw: s.clientWidth - 48, sh: s.clientHeight - 32 };
    });
    assert.ok(Math.abs(r.w / r.h - r.nw / r.nh) < 0.02, `${step}: shown ${r.w}×${r.h}, picture ${r.nw}×${r.nh}`);
    assert.ok(Math.abs(r.w - r.sw) < 2 || Math.abs(r.h - r.sh) < 2, `${step}: ${r.w}×${r.h} doesn't fill ${r.sw}×${r.sh}`);
  }
});

test("a one-line message has no scroll bar in the message box", async () => {
  await open("JB Bouhier", "1");
  const box = page.getByPlaceholder("RCS message");
  await box.fill("hi");
  assert.equal(await box.evaluate((t) => getComputedStyle(t).overflowY), "hidden");
  assert.equal(await box.evaluate((t) => getComputedStyle(t).overflowX), "hidden", "no sideways scroll bar");
  for (const text of ["", "hi"]) {
    await box.fill(text);
    const [inner, outer] = await box.evaluate((t) => [t.clientHeight, t.offsetHeight]);
    assert.equal(inner, outer, `${text ? "text" : "placeholder"}: no scroll bar takes room from the line`);
  }
  await box.fill("line\n".repeat(20));
  assert.equal(await box.evaluate((t) => getComputedStyle(t).overflowY), "auto", "a long draft scrolls");
});

// Every theme keeps text readable on both sides: WCAG AA (4.5:1) for each
// pair of text and background the app draws, read from the rendered page,
// and over the art under the see-through panes.
const THEME_IDS = ["default", "graphite", "sky", "mauve", "aurora", "peach", "amber", "sage"];
const use = (theme, mode, background = "gradient") =>
  page.evaluate(([t, m, b]) => Object.assign(window.gm.settings, { theme: t, mode: m, background: b }), [theme, mode, background]);

test("every theme's text meets WCAG AA contrast, light and dark, gradient and solid", async () => {
  const pairs = [
    ["--text", "--bg"],
    ["--text", "--sidebar"],
    ["--text", "--selected"],
    ["--text", "--hover"],
    ["--subtle", "--bg"],
    ["--subtle", "--sidebar"],
    ["--text", "--bubble-in"],
    ["--on-bubble-out", "--bubble-out"],
    ["--on-accent", "--accent"],
    ["--accent", "--bg"],
    ["--accent", "--sidebar"],
    ["--text", "--composer"],
  ];
  const failures = [];
  for (const id of THEME_IDS) {
    for (const [mode, background] of [["light", "gradient"], ["dark", "gradient"], ["light", "solid"], ["dark", "solid"]]) {
      await use(id, mode, background);
      await until(`${id} ${mode}`, ([t, m]) => document.documentElement.dataset.theme === t && document.documentElement.dataset.mode === m, [id, mode]);
      const found = await page.evaluate(([pairs, id, dark, gradient]) => {
        const css = getComputedStyle(document.documentElement);
        const probe = document.createElement("div");
        document.body.append(probe);
        // Resolve any CSS color to sRGB through the browser.
        const rgb = (v) => {
          probe.style.color = css.getPropertyValue(v);
          return getComputedStyle(probe).color.match(/[\d.]+/g).slice(0, 3).map(Number);
        };
        const hex = (h) => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16));
        const mix = (a, c, k) => a.map((x, i) => x * k + c[i] * (1 - k));
        const lum = (c) =>
          c
            .map((v) => v / 255)
            .map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
            .reduce((s, v, i) => s + v * [0.2126, 0.7152, 0.0722][i], 0);
        const ratio = (a, b) => {
          const [x, y] = [lum(a), lum(b)];
          return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
        };
        const out = pairs.map(([fg, bg]) => [`${fg} on ${bg}`, ratio(rgb(fg), rgb(bg))]);
        // Over the art: the see-through list and pane over each of its colors.
        const under = window.gmUnder(id, dark, gradient);
        for (const c of under.colors.map(hex)) {
          const list = mix(rgb("--sidebar"), c, under.list);
          const pane = mix(rgb("--bg"), c, under.pane);
          for (const fg of ["--text", "--subtle", "--accent"]) out.push([`${fg} on the list over ${c}`, ratio(rgb(fg), list)]);
          for (const fg of ["--text", "--subtle"]) out.push([`${fg} on the pane over ${c}`, ratio(rgb(fg), pane)]);
        }
        probe.remove();
        return out;
      }, [pairs, id, mode === "dark", background === "gradient"]);
      for (const [what, r] of found) if (r < 4.5) failures.push(`${id} ${mode} ${background}: ${what} is ${r.toFixed(2)}:1`);
    }
  }
  await use("default", "system");
  assert.deepEqual(failures, []);
});

test("the theme picker and the mode", async () => {
  await act("settings");
  const root = () => page.evaluate(() => ({ ...document.documentElement.dataset }));
  const accent = () => page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--accent").trim());
  await page.getByRole("radio", { name: "Light", exact: true }).click();
  await page.getByRole("radio", { name: "Sage" }).click();
  await until("Sage", () => document.documentElement.dataset.theme === "sage");
  assert.equal((await root()).mode, "light");
  assert.equal(await accent(), "#255a4c", "Sage's light side");
  await page.getByRole("radio", { name: "Dark", exact: true }).click();
  await until("dark", () => document.documentElement.dataset.mode === "dark");
  assert.equal((await root()).theme, "sage", "the theme stays with the mode");
  assert.equal(await accent(), "#8fc7b5", "Sage's dark side");
  // Following the system.
  await page.getByRole("radio", { name: "System", exact: true }).click();
  await page.evaluate(() => (window.gm.systemDark = false));
  await until("system light", () => document.documentElement.dataset.mode === "light");
  await page.evaluate(() => (window.gm.systemDark = true));
  await until("system dark", () => document.documentElement.dataset.mode === "dark");
  assert.equal(await page.getByRole("radio", { name: "Sage" }).getAttribute("aria-checked"), "true");
  // Back to System after a fixed mode: the system's own mode applies, and
  // its changes are followed.
  await page.emulateMedia({ colorScheme: "light" });
  await page.getByRole("radio", { name: "Dark", exact: true }).click();
  await until("forced dark", () => document.documentElement.dataset.mode === "dark");
  await page.getByRole("radio", { name: "System", exact: true }).click();
  await until("system's light", () => document.documentElement.dataset.mode === "light");
  await page.emulateMedia({ colorScheme: "dark" });
  await until("system turns dark", () => document.documentElement.dataset.mode === "dark");
  await page.emulateMedia({ colorScheme: "light" });
  await until("system turns light", () => document.documentElement.dataset.mode === "light");
  // Background: the theme's art, or none.
  await page.getByRole("radio", { name: "Sage" }).click();
  await page.getByRole("radio", { name: "Solid", exact: true }).click();
  await until("solid", () => !document.querySelector("div.backdrop"));
  await page.getByRole("radio", { name: "Gradient", exact: true }).click();
  await until("gradient", () => !!document.querySelector("div.backdrop"));
  // An unknown theme falls back to Default.
  await page.evaluate(() => (window.gm.settings.theme = "gone"));
  await until("default", () => document.documentElement.dataset.theme === "default");
  await use("default", "system");
});

// Transparency fades the panes and the theme's art to what's behind the
// window; bubbles stay solid. (The native blur itself needs the app.)
test("transparency fades every theme the same way", async () => {
  await open("JB Bouhier", "1");
  const alpha = (sel) =>
    page.locator(sel).first().evaluate((e) => {
      const m = getComputedStyle(e).backgroundColor.match(/[\d.]+/g).map(Number);
      return m.length > 3 ? m[3] : 1;
    });
  const set = (v) => page.evaluate((v) => Object.assign(window.gm.settings, v), v);
  await page.evaluate(() => (window.gm.blurSupported = true));
  await use("mauve", "dark", "solid");
  await until("mauve", () => document.documentElement.dataset.theme === "mauve");
  assert.equal(await alpha(".thread"), 1, "solid at 0");
  assert.equal(await page.locator("div.backdrop").count(), 0, "no art with the solid background");
  assert.ok(!(await page.evaluate(() => document.documentElement.classList.contains("blur"))), "no blur at 0");
  await set({ transparency: 40 });
  await until("blur", () => document.documentElement.classList.contains("blur"));
  assert.equal(await alpha("body"), 0, "the window is clear");
  assert.ok(Math.abs((await alpha(".thread")) - 0.6) < 0.01, "the conversation pane is 60% at 40");
  assert.ok(Math.abs((await alpha(".sidebar")) - 0.6) < 0.01, "so is the list");
  assert.equal(await alpha(".bubble"), 1, "bubbles stay solid");
  await set({ transparency: 100 });
  await page.waitForTimeout(50);
  assert.equal(await alpha(".thread"), 0, "everything behind shows at 100");
  // A theme without art of its own gets a made gradient.
  await set({ transparency: 0, background: "gradient" });
  await until("mauve's gradient", () => !!document.querySelector("div.backdrop"));
  assert.ok(Math.abs((await alpha(".thread")) - 0.72) < 0.01, "see-through over its gradient");
  // A theme with art: its panes and the art fade together.
  await use("sage", "light");
  await set({ transparency: 50 });
  await until("sage", () => document.documentElement.dataset.theme === "sage");
  assert.ok(Math.abs((await alpha(".thread")) - 0.36) < 0.01, "Sage's 72% pane, halved");
  const faded = await page.locator("div.backdrop").evaluate((e) => getComputedStyle(e).opacity);
  assert.equal(Number(faded), 0.5, "the art fades with it");
  await set({ transparency: 0 });
  await until("no blur", () => !document.documentElement.classList.contains("blur"));
  assert.ok(Math.abs((await alpha(".thread")) - 0.72) < 0.01);
  await use("default", "system");
});

// ---- message hover buttons and popups by keyboard ----

const focusedName = () => state(() => document.activeElement?.getAttribute("aria-label") || document.activeElement?.textContent.trim());

test("hovering a message shows React, Reply and More on the side toward the middle", async () => {
  await open("JB Bouhier", "1");
  const theirs = page.locator(".thread .row", { hasText: "owns the couch" }).first();
  const mine = page.locator(".thread .row", { hasText: "He's tiny" }).first();
  await theirs.hover();
  await until("the buttons to show", () => getComputedStyle(document.querySelector(".row:hover .actions")).opacity === "1");
  const names = await theirs.locator(".actions button").evaluateAll((bs) => bs.map((b) => b.getAttribute("aria-label")));
  assert.deepEqual(names, ["React", "Reply", "More"]);
  const t = await theirs.evaluate((r) => [r.querySelector(".bubble").getBoundingClientRect().right, r.querySelector(".actions").getBoundingClientRect().left]);
  assert.ok(t[1] >= t[0] - 1, "right of their bubble");
  await mine.hover();
  const m = await mine.evaluate((r) => [r.querySelector(".bubble").getBoundingClientRect().left, r.querySelector(".actions").getBoundingClientRect().right]);
  assert.ok(m[1] <= m[0] + 1, "left of my bubble");
});

test("the More menu: arrows and Tab move, Escape closes and gives focus back", async () => {
  await open("JB Bouhier", "1");
  const row = page.locator(".thread .row", { hasText: "owns the couch" }).first();
  await row.hover();
  await row.getByRole("button", { name: "More" }).click();
  const items = await page.locator(".msg-menu [role=menuitem]").evaluateAll((bs) => bs.map((b) => b.textContent.trim()));
  assert.deepEqual(items, ["Copy", "Delete"]);
  assert.equal(await focusedName(), "Copy", "the first item has focus");
  await page.keyboard.press("ArrowDown");
  assert.equal(await focusedName(), "Delete");
  await page.keyboard.press("ArrowDown");
  assert.equal(await focusedName(), "Copy", "wraps around");
  await page.keyboard.press("Tab");
  assert.equal(await focusedName(), "Delete", "Tab stays in the menu");
  await page.keyboard.press("Escape");
  assert.equal(await page.locator(".msg-menu").count(), 0);
  assert.equal(await focusedName(), "More", "focus back on the button");
});

test("deleting a message asks first, with Cancel as the default", async () => {
  await open("Lukas Weber", "3");
  const row = () => page.locator(".thread .row", { hasText: "Sending it tonight" });
  await row().hover();
  await row().getByRole("button", { name: "More" }).click();
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await page.getByRole("alertdialog").waitFor();
  assert.equal(await focusedName(), "Cancel");
  await page.keyboard.press("Enter");
  assert.equal(await page.getByRole("alertdialog").count(), 0, "Enter on the default cancels");
  assert.equal(await row().count(), 1, "still there");

  await row().hover();
  await row().getByRole("button", { name: "More" }).click();
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await page.keyboard.press("ArrowRight");
  assert.equal(await focusedName(), "Delete");
  await page.keyboard.press("Tab");
  assert.equal(await focusedName(), "Cancel", "Tab stays in the dialog");
  await page.keyboard.press("Shift+Tab");
  await page.keyboard.press("Enter");
  await until("the message to go", () => !window.gm.threads["3"].msgs.some((m) => m.text === "Sending it tonight"));
});

test("the reaction picker walks with the arrows and reacts on Enter", async () => {
  await open("JB Bouhier", "1");
  const row = page.locator(".thread .row", { hasText: "Dinner at 7" }).first();
  await row.hover();
  await row.getByRole("button", { name: "React" }).click();
  assert.equal(await focusedName(), "React ❤️");
  await page.keyboard.press("ArrowRight");
  assert.equal(await focusedName(), "React 👍");
  await page.keyboard.press("End");
  assert.equal(await focusedName(), "More reactions", "every emoji is last");
  await page.keyboard.press("ArrowLeft");
  assert.equal(await focusedName(), "React 🙏");
  await page.keyboard.press("ArrowRight");
  await page.keyboard.press("ArrowRight");
  assert.equal(await focusedName(), "React ❤️", "wraps around");
  await page.keyboard.press("Enter");
  assert.equal(await page.locator(".msg-menu").count(), 0);
});

test("More reactions opens every emoji: search, arrows, Enter reacts", async () => {
  await open("JB Bouhier", "1");
  const row = page.locator(".thread .row", { hasText: "owns the couch" }).first();
  await row.hover();
  await row.getByRole("button", { name: "React" }).click();
  await page.getByRole("menuitem", { name: "More reactions" }).click();
  const searchFocused = () => document.activeElement?.tagName === "EMOJI-PICKER" && document.activeElement.shadowRoot.activeElement?.id === "search";
  await until("the search box to have focus", searchFocused);
  await page.keyboard.type("taco");
  await until("results", () => document.querySelector("emoji-picker").shadowRoot.querySelector("#search-results [role=option]"));
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await until("the picker to close", () => !document.querySelector(".emoji-popup"));

  await row.hover();
  await row.getByRole("button", { name: "React" }).click();
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await until("the search box to have focus", searchFocused);
  await page.keyboard.press("Escape");
  assert.equal(await page.locator(".emoji-popup").count(), 0);
  assert.equal(await focusedName(), "React", "focus back on React");
});

test("Reply quotes the message above the box and sends it as a reply", async () => {
  await open("Lucía Fernández", "6");
  const row = page.locator(".thread .row", { hasText: "Madrid" }).first();
  await row.hover();
  await row.getByRole("button", { name: "Reply" }).click();
  await page.locator(".replying", { hasText: "Madrid" }).waitFor();
  assert.equal(await state(() => document.activeElement?.tagName), "TEXTAREA", "the box has focus");
  await page.keyboard.press("Escape");
  assert.equal(await page.locator(".replying").count(), 0, "Escape drops the reply");
  await row.hover();
  await row.getByRole("button", { name: "Reply" }).click();
  await page.keyboard.type("Super");
  await page.keyboard.press("Enter");
  await until("the reply", () => window.gm.threads["6"].msgs.some((m) => m.text === "Super" && m.reply_to && !m.id.startsWith("tmp_")));
  await page.locator(".thread .row", { hasText: "Super" }).locator(".quote", { hasText: "Madrid" }).waitFor();
});

test("a conversation's menu and its confirmation work from the keyboard", async () => {
  await page.locator(".row-wrap", { hasText: "Mei Tanaka" }).hover();
  await page.getByRole("button", { name: "More options for Mei Tanaka" }).click();
  assert.equal(await focusedName(), "Archive");
  await page.keyboard.press("End");
  await page.keyboard.press("Enter");
  await page.getByRole("alertdialog").waitFor();
  assert.equal(await focusedName(), "Cancel");
  await page.keyboard.press("Escape");
  assert.equal(await page.getByRole("alertdialog").count(), 0);
  assert.equal(await focusedName(), "More options for Mei Tanaka", "focus back on ⋮");
});
