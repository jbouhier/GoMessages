// Renders stage.html frame by frame into an MP4 (or stills with --stills t1,t2,...).
import { chromium } from "playwright-core";
import fs from "node:fs";
import { spawn } from "node:child_process";
import { chromePath } from "../chrome.mjs";

const FPS = 30, DUR = 38.5;
const stills = process.argv.includes("--stills") ? process.argv[process.argv.indexOf("--stills") + 1].split(",").map(Number) : null;
const clips = {};
const C = ".work/clips";
for (const d of fs.readdirSync(C)) if (fs.existsSync(`${C}/${d}/meta.json`)) clips[d] = JSON.parse(fs.readFileSync(`${C}/${d}/meta.json`)).frames;

const b = await chromium.launch({ executablePath: process.platform === "linux" ? process.env.CHROME : chromePath(), args: ["--allow-file-access-from-files"] });
const p = await b.newPage({ viewport: { width: 1920, height: 1080 } });
p.on("pageerror", (e) => console.log("pageerror", e.message));
await p.goto("file://" + process.cwd() + "/stage.html");
await p.evaluate((c) => (window.CLIPS = c), clips);
await p.evaluate(() => document.fonts.ready);

if (stills) {
  for (const t of stills) {
    await p.evaluate((t) => window.render(t), t);
    await p.screenshot({ path: `.work/still-${t}.jpg`, type: "jpeg", quality: 85 });
  }
} else {
  const ff = spawn("ffmpeg", ["-y", "-f", "image2pipe", "-framerate", String(FPS), "-c:v", "mjpeg", "-i", "-",
    "-c:v", "libx264", "-preset", "slow", "-crf", "17", "-pix_fmt", "yuv420p", "-movflags", "+faststart", ".work/silent.mp4"], { stdio: ["pipe", "inherit", "inherit"] });
  const n = Math.round(DUR * FPS);
  const t0 = Date.now();
  for (let f = 0; f < n; f++) {
    await p.evaluate((t) => window.render(t), f / FPS);
    const buf = await p.screenshot({ type: "jpeg", quality: 95 });
    if (!ff.stdin.write(buf)) await new Promise((r) => ff.stdin.once("drain", r));
    if (f % 150 === 0) console.log(`frame ${f}/${n} ${((Date.now() - t0) / 1000).toFixed(0)}s`);
  }
  ff.stdin.end();
  await new Promise((r) => ff.on("close", r));
}
await b.close();
