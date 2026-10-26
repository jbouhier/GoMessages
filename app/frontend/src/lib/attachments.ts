/** Most files the message box holds at once. Each goes out as its own
 *  message, so more is almost always a folder's worth dropped by mistake. */
export const MAX_ATTACHMENTS = 10;

/** The file's name from its path, on any platform. */
export function fileName(path: string): string {
  return path.split(/[\\/]/).pop() ?? "";
}

/** files with paths added in order, skipping ones already there, up to
 *  MAX_ATTACHMENTS. dropped counts the new files that didn't fit. */
export function addAttachments(files: string[], paths: string[]): { files: string[]; dropped: number } {
  const out = [...files];
  let dropped = 0;
  for (const p of paths) {
    if (!p || out.includes(p)) continue;
    if (out.length >= MAX_ATTACHMENTS) dropped++;
    else out.push(p);
  }
  return { files: out, dropped };
}
