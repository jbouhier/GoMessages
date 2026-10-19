// Screenshots of every screen against the demo backend.
// Usage: node e2e/tour.mjs <outdir> [light|dark] [theme]
import { chromium } from "playwright-core";
import { chromePath } from "./chrome.mjs";

const out = process.argv[2] ?? ".";
const scheme = process.argv[3] ?? "light";
const theme = process.argv[4];
const browser = await chromium.launch({ executablePath: chromePath() });
const page = await browser.newPage({ viewport: { width: 1180, height: 760 }, colorScheme: scheme, deviceScaleFactor: 1 });
page.on("pageerror", (e) => console.log("pageerror:", e.message));
page.on("console", (m) => m.type() === "error" && console.log("console:", m.text(), m.location().url));
page.on("response", (r) => r.status() >= 400 && console.log("http", r.status(), r.url()));
const shot = async (name, wait = 600) => {
  await page.waitForTimeout(wait);
  await page.screenshot({ path: `${out}/${name}-${theme ?? scheme}.png` });
};
const act = (name) => page.evaluate((n) => window.gm.act(n), name);

await page.goto("http://127.0.0.1:9250/?http");
await page.getByRole("button", { name: "Maya Chen" }).first().waitFor();
if (theme) await page.evaluate((t) => (window.gm.settings.theme = t), theme);
await shot("a-maya", 2000);
const atEnd = await page.evaluate(() => {
  const l = document.querySelector(".thread .list");
  return l.scrollHeight - l.scrollTop - l.clientHeight;
});
console.log("maya distance from end:", atEnd);
await page.locator(".photo").first().click();
await shot("b-viewer");
await page.keyboard.press("Escape");
await act("switch");
await page.keyboard.type("sam");
await shot("c-switcher");
await page.keyboard.press("Escape");
await act("shortcuts");
await shot("d-shortcuts");
await page.keyboard.press("Escape");
await act("settings");
await shot("e-settings");
await page.keyboard.press("Escape");
await act("new-chat");
await shot("f-newchat");
await page.keyboard.press("Escape");
await act("photos-only");
await shot("g-compact");
await act("photos-only");
await act("toggle-list");
await shot("h-hidden");
await act("toggle-list");
await page.getByRole("button", { name: "Climbing crew" }).first().click();
await shot("i-group", 1200);
await browser.close();
