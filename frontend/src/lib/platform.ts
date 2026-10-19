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
