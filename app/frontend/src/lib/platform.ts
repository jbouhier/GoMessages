export const isMac = /Mac|iPhone|iPad/.test(navigator.platform) || navigator.userAgent.includes("Mac OS");

/** A shortcut written with macOS symbols, as this platform shows it:
 *  ⌘B stays ⌘B on macOS and becomes Ctrl+B on Linux and Windows. */
export function keys(mac: string): string {
  if (isMac) return mac;
  return mac.replace(/⌘/g, "Ctrl+").replace(/⌥/g, "Alt+").replace(/⇧/g, "Shift+").replace(/⌃/g, "Ctrl+");
}

/** ⌘ on macOS, Ctrl elsewhere. */
export function commandKey(e: KeyboardEvent): boolean {
  return isMac ? e.metaKey : e.ctrlKey;
}

/** Windows and Linux: no system title bar, the UI draws its own (see
 *  customTitleBar in window.go). ?titlebar shows it in a browser too, for
 *  screenshots and tests on a Mac. */
export const customTitleBar = !isMac || new URLSearchParams(location.search).has("titlebar");
