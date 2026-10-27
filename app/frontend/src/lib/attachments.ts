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

const IMAGE = new Set(["jpg", "jpeg", "png", "gif", "webp", "bmp", "heic", "heif", "avif"]);
const VIDEO = new Set(["mp4", "m4v", "mov", "webm", "3gp"]);

/** How a file previews in the message box: a picture, a video's first
 *  frame, or its name. By extension: the file isn't read for it. */
export function attachmentKind(path: string): "image" | "video" | "file" {
  const name = fileName(path);
  if (!name.includes(".")) return "file";
  const ext = name.split(".").pop()!.toLowerCase();
  return IMAGE.has(ext) ? "image" : VIDEO.has(ext) ? "video" : "file";
}

/** Where the UI loads an attached file to preview it. Go serves only files
 *  the user picked or dropped. */
export function attachmentURL(path: string): string {
  return `/attachment?path=${encodeURIComponent(path)}`;
}
