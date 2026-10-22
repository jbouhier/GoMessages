// Shapes sent by the backend (model.go): the same JSON gogo-messages reads.

export interface Conversation {
  id: string;
  name: string;
  snippet: string;
  snippet_from_me: boolean;
  timestamp: number; // unix millis
  unread: boolean;
  is_group: boolean;
  pinned: boolean;
  archived: boolean;
  /** Deleted, trashed, spam or blocked: dropped from the list. */
  removed?: boolean;
  color: string;
  read_only: boolean;
  is_rcs: boolean;
  latest_message_id: string;
  members: string[] | null;
  avatar_id?: string;
  sim_number?: number;
}

export interface Media {
  id: string;
  key: string;
  thumb_id?: string;
  thumb_key?: string;
  mime: string;
  name: string;
  size: number;
  width?: number;
  height?: number;
}

export interface Reaction {
  emoji: string;
  count: number;
  mine: boolean;
}

export interface Message {
  id: string;
  tmp_id?: string;
  conversation_id: string;
  timestamp: number;
  from_me: boolean;
  sender: string;
  sender_color?: string;
  sender_id?: string;
  text: string;
  media?: Media[] | null;
  status: string; // sending, sent, delivered, read, failed, received, system, deleted
  reply_to?: string;
  reactions?: Reaction[] | null;
  is_old?: boolean;
}

/** A web page's title and picture, fetched by Go (linkpreview.go). */
export interface LinkPreview {
  url: string;
  title: string;
  description?: string;
  site: string;
  /** Files in the media folder: show them with mediaURL. */
  image?: string;
  icon?: string;
}

/** The buttons that show beside a message on hover. */
export type BubbleAction = "react" | "reply" | "more";

export interface Contact {
  name: string;
  number: string;
  formatted_number: string;
  top: boolean;
  avatar_id?: string;
}

export interface SimChoice {
  number: number;
  label: string;
  phone: string;
}

/** Formats the image decoder (the web view) shows inline. */
export function isImage(m: Media): boolean {
  const mime = m.mime.toLowerCase().split(";")[0].trim();
  return ["image/jpeg", "image/jpg", "image/pjpeg", "image/png", "image/gif", "image/webp", "image/heic", "image/heif"].includes(mime);
}

/** URL of a file the backend downloaded into its media folder. */
export function mediaURL(path: string): string {
  const name = path.split(/[\\/]/).pop() ?? "";
  return `/media/${encodeURIComponent(name)}`;
}
