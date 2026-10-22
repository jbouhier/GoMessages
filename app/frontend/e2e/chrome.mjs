import { readdirSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

/** Chrome for Testing from Playwright's cache, or $CHROME. */
export function chromePath() {
  if (process.env.CHROME) return process.env.CHROME;
  const base = join(homedir(), "Library/Caches/ms-playwright");
  const dir = readdirSync(base).filter((d) => d.startsWith("chromium-")).sort().pop();
  return join(base, dir, "chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing");
}
