// Records each scene of the live demo UI as a timestamped JPEG sequence.
// Runs in slow motion (page clock and CSS animations at 1/SLOW) so slow
// machines still capture smooth frames; timestamps are stored in real video time.
import { chromium } from "playwright-core";
import fs from "node:fs";
import { chromePath } from "../chrome.mjs";

// Usage: SLOW=8 node record.mjs   (with mock.mjs running on :9250)
const OUT = ".work/clips";
const SLOW = Number(process.env.SLOW || 8);
fs.rmSync(OUT, { recursive: true, force: true });
const linux = process.platform === "linux";
const b = await chromium.launch({ executablePath: linux ? process.env.CHROME : chromePath(),
  args: ["--hide-scrollbars", ...(linux ? ["--use-gl=angle", "--use-angle=swiftshader", "--enable-unsafe-swiftshader"] : [])] });
const p = await b.newPage({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 1.5, colorScheme: "light" });
await p.addInitScript(() => {
  Object.defineProperty(navigator, "platform", { get: () => "MacIntel" });
  // macOS-style cursor that follows the synthetic mouse.
  addEventListener("DOMContentLoaded", () => {
    const c = document.createElement("div");
    c.id = "fakecursor";
    c.innerHTML = `<svg width="22" height="30" viewBox="0 0 22 30"><path d="M2 2 L2 24 L7.5 18.5 L11.5 27.5 L15 26 L11 17 L19 17 Z" fill="#111" stroke="#fff" stroke-width="1.6" stroke-linejoin="round"/></svg>`;
    Object.assign(c.style, { position: "fixed", left: "0", top: "0", zIndex: 2147483647, pointerEvents: "none",
      transform: "translate(-100px,-100px)", filter: "drop-shadow(0 2px 3px rgba(0,0,0,.35))", transition: "opacity .2s", opacity: "0" });
    document.documentElement.appendChild(c);
    addEventListener("mousemove", (e) => { c.style.transform = `translate(${e.clientX - 2}px,${e.clientY - 2}px)`; c.style.opacity = "1"; }, true);
  });
});
p.on("pageerror", (e) => console.log("pageerror", e.message));
const cdp = await p.context().newCDPSession(p);
let frames = null;
cdp.on("Page.screencastFrame", async (f) => {
  cdp.send("Page.screencastFrameAck", { sessionId: f.sessionId }).catch(() => {});
  if (frames) frames.push({ t: f.metadata.timestamp, data: f.data });
});
const wait = (ms) => p.waitForTimeout(ms * SLOW);
const act = (n) => p.evaluate((n) => window.gm.act(n), n);
const cursor = (show) => p.evaluate((s) => (document.getElementById("fakecursor").style.opacity = s ? "1" : "0"), show);

async function scene(name, fn) {
  frames = [];
  await cdp.send("Page.startScreencast", { format: "jpeg", quality: 92, everyNthFrame: 1 });
  await wait(250);
  const t0 = Date.now() / 1000;
  await fn();
  await wait(250);
  await cdp.send("Page.stopScreencast");
  const list = frames; frames = null;
  fs.mkdirSync(`${OUT}/${name}`, { recursive: true });
  const base = list[0].t;
  const meta = list.map((f, i) => {
    fs.writeFileSync(`${OUT}/${name}/${String(i).padStart(5, "0")}.jpg`, Buffer.from(f.data, "base64"));
    return { i, t: +((f.t - base) / SLOW).toFixed(4) };
  });
  fs.writeFileSync(`${OUT}/${name}/meta.json`, JSON.stringify({ dur: +((Date.now() / 1000 - t0) / SLOW + 0.5).toFixed(3), frames: meta }));
  const dur = meta.at(-1).t;
  console.log(name, list.length, "frames", dur.toFixed(2), "s", (list.length / dur).toFixed(1), "fps");
}

// Smooth eased mouse path to an element's center (or a point).
let mx = 900, my = 700;
async function glide(target, ms = 600, dx = 0, dy = 0) {
  let x, y;
  if (typeof target === "string" || target.boundingBox) {
    const bb = await (typeof target === "string" ? p.locator(target).first() : target).boundingBox();
    x = bb.x + bb.width / 2 + dx; y = bb.y + bb.height / 2 + dy;
  } else ({ x, y } = target);
  const steps = Math.max(8, Math.round(ms / 16));
  const sx = mx, sy = my;
  for (let i = 1; i <= steps; i++) {
    const k = i / steps, e = k < 0.5 ? 4 * k * k * k : 1 - Math.pow(-2 * k + 2, 3) / 2;
    await p.mouse.move(sx + (x - sx) * e, sy + (y - sy) * e);
    await wait(ms / steps);
  }
  mx = x; my = y;
}

await p.clock.install();
await cdp.send("Animation.enable");
await cdp.send("Animation.setPlaybackRate", { playbackRate: 1 / SLOW });
await p.goto("http://127.0.0.1:9250/?http");
await p.getByRole("button", { name: "JB Bouhier" }).first().waitFor();
await p.waitForTimeout(3000);
// Drive the page clock at 1/SLOW of real time, in step with CSS.
const realStart = Date.now();
await p.clock.pauseAt((await p.evaluate(() => Date.now())) + 5);
let virt = 0, driving = true;
(async () => {
  while (driving) {
    const target = (Date.now() - realStart) / SLOW;
    const d = Math.floor(target - virt);
    if (d > 0) { virt += d; await p.clock.runFor(d).catch(() => {}); }
    await new Promise((r) => setTimeout(r, 15));
  }
})();
await wait(800);
await p.mouse.move(mx, my); await cursor(false);

// A: send a message, watch delivery, read, typing, reply.
await scene("send", async () => {
  await wait(300);
  await cursor(true);
  await glide(".composer textarea, textarea", 700, -120, 0);
  await p.mouse.click(mx, my);
  await wait(200);
  await p.keyboard.type("Pizza instead? I'll bring the catnip", { delay: 42 * SLOW });
  await wait(250);
  await p.keyboard.press("Enter");
  await wait(3400);
});

// B: quick switcher.
await cursor(false);
await scene("switcher", async () => {
  await wait(300);
  await act("switch");
  await wait(450);
  for (const ch of "mei") { await p.keyboard.type(ch); await wait(190); }
  await wait(500);
  await p.keyboard.press("Enter");
  await wait(1300);
});

// C: photo viewer, flip with arrows.
await scene("photos", async () => {
  await cursor(true);
  await glide(p.locator(".photo").first(), 650);
  await wait(150);
  await p.mouse.click(mx, my);
  await cursor(false);
  await wait(1300);
  await p.keyboard.press("ArrowRight");
  await wait(1500);
});
await p.keyboard.press("Escape");
await wait(400);

// D: react with a heart.
await p.evaluate(() => window.gm.select("1"));
await wait(1200);
await scene("react", async () => {
  await cursor(true);
  const row = p.locator(".row", { hasText: "owns the couch" });
  await glide(row.locator(".bubble"), 600);
  await wait(250);
  const btn = row.getByRole("button", { name: "React" });
  await glide(btn, 350);
  await wait(150);
  await btn.click();
  await wait(450);
  const heart = p.locator('button[aria-label="React ❤️"]');
  await glide(heart, 450);
  await wait(200);
  await heart.click();
  await wait(300);
  await glide({ x: mx + 160, y: my + 170 }, 400);
  await wait(900);
});

// E: shortcuts sheet.
await cursor(false);
await scene("shortcuts", async () => {
  await wait(250);
  await act("shortcuts");
  await wait(2100);
});
await p.keyboard.press("Escape");
await wait(500);

// F: themes, light and dark.
await scene("themes", async () => {
  const seq = [["default", "dark"], ["graphite", "light"], ["sky", "dark"], ["mauve", "light"], ["aurora", "dark"],
    ["peach", "light"], ["amber", "dark"], ["sage", "light"], ["aurora", "dark"]];
  for (const [theme, mode] of seq) {
    await p.evaluate(([t, m]) => { window.gm.settings.theme = t; window.gm.settings.mode = m; }, [theme, mode]);
    await wait(620);
  }
  await wait(300);
});

// G: a still of the hero for the end card.
await p.evaluate(() => { window.gm.settings.theme = "default"; window.gm.settings.mode = "light"; });
await wait(800);
await p.screenshot({ path: `${OUT}/final-light.jpg`, quality: 92 });
await p.evaluate(() => { window.gm.settings.theme = "aurora"; window.gm.settings.mode = "dark"; });
await wait(800);
await p.screenshot({ path: `${OUT}/final-dark.jpg`, quality: 92 });
driving = false;
await new Promise((r) => setTimeout(r, 100));
await b.close();
