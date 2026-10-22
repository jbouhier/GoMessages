// The demo backend (demo.go + server.go) in Node: same protocol and sample data,
// plus delays that scale with SLOW, a reaction echo, and a scripted reply for the video.
import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Usage: SLOW=8 node mock.mjs   (SLOW stretches the scripted delays to match record.mjs)
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const DIST = path.join(ROOT, "frontend/dist");
const ASSETS = path.join(ROOT, "assets/demo");
const PORT = 9250;
const MIN = 60e3, H = 60 * MIN, D = 24 * H;

let nextID = 0, state = "connected";
const convs = {}, msgs = {};
const clients = new Set();
const emit = (event, data) => {
  const line = `data: ${JSON.stringify({ name: "gm", data: { event, data } })}\n\n`;
  for (const c of clients) c.write(line);
};

function seed() {
  const now = Date.now();
  const S = [
    ["1", "JB Bouhier", "#e8710a"], ["2", "Haruto Sato", "#1a73e8"], ["3", "Lukas Weber", "#188038"],
    ["4", "Oliver Bennett", "#a142f4"], ["5", "Mei Tanaka", "#5f6368"], ["6", "Lucía Fernández", "#d93025"],
  ];
  const L = {
    1: [[0, 31 * MIN, "Big news. Someone moved in with me."], [1, 30 * MIN, "Since when??"], [1, 4 * MIN, "He's tiny."],
        [0, 3 * MIN, "He's 3 months old and already owns the couch."], [1, 2 * MIN, "Who pays rent?"],
        [0, 1 * MIN, "Not him. Dinner at 7 is on me, apparently 🐟"]],
    2: [[0, 50 * MIN, "Are you free tonight?"], [0, 40 * MIN, "Found a new ramen place 🍜"]],
    3: [[0, 50 * H, "Kannst du mir das Dokument schicken?"], [1, 49 * H, "Sending it tonight"]],
    4: [[1, 6 * D, "Thanks for the book tip"], [0, 6 * D, "Anytime! Tell me what you think"]],
    5: [[0, 8 * D, "Sent you the photos from Kyoto 📸"]],
    6: [[0, 12 * D, "¿Nos vemos en Madrid el mes que viene?"], [1, 12 * D, "¡Claro que sí!"]],
  };
  for (const [id, name, color] of S) {
    const list = L[id].map(([me, ago, t]) => ({
      id: String(++nextID), conversation_id: id, timestamp: now - ago, from_me: !!me, text: t,
      status: me ? "read" : "received", sender: me ? "" : name,
    }));
    msgs[id] = list;
    const last = list[list.length - 1];
    convs[id] = { id, name, color, is_group: false, is_rcs: true, avatar_id: "p" + id, sim_number: 1,
      snippet: last.text, snippet_from_me: last.from_me, timestamp: last.timestamp, unread: id === "1" || id === "2",
      latest_message_id: last.id, pinned: false, archived: false, read_only: false, members: [] };
  }
  const jb = msgs["1"];
  jb[3].reactions = [{ emoji: "😂", count: 1, mine: true }];
  const photo = { id: String(++nextID), conversation_id: "1", sender: "JB Bouhier", status: "received", from_me: false,
    timestamp: now - 29 * MIN, text: "Since 8:14 this morning. Meet Kevin 🐾",
    media: [{ id: "demo-photo-1", thumb_id: "demo-photo-1-thumb", mime: "image/jpeg", name: "photo.jpg", width: 1000, height: 1000, key: "", size: 0 }] };
  msgs["1"] = [...jb.slice(0, 2), photo, ...jb.slice(2)];
  [[1500, 1000], [1600, 1200]].forEach(([w, h], i) => {
    const id = `demo-photo-${i + 2}`;
    msgs["5"].splice(i, 0, { id: String(++nextID), conversation_id: "5", sender: "Mei Tanaka", status: "received", from_me: false,
      timestamp: now - 8 * D - (2 - i) * MIN, text: "",
      media: [{ id, thumb_id: id + "-thumb", mime: "image/jpeg", name: id + ".jpg", width: w, height: h, key: "", size: 0 }] });
  });
  const lines = ["Did you see the game last night?", "Lunch tomorrow?", "Running late, 10 min", "Haha that's amazing 😂",
    "Can you send me the address?", "Sure, here it is: 42 Main St", "Thanks! See you there 👍", "How was the trip?",
    "Long story, tell you later", "This one is a longer message to check that wrapping works across several lines when the bubble gets narrow."];
  const hist = [];
  for (let i = 0; i < 320; i++) {
    const me = i % 3 === 0;
    hist.push({ id: String(++nextID), conversation_id: "4", from_me: me, status: me ? "read" : "received",
      timestamp: now - (7 * 24 * 60 + (320 - i) * 47) * MIN, text: `#${i + 1} ${lines[i % lines.length]}`, sender: me ? "" : "Oliver Bennett" });
  }
  msgs["4"] = [...hist, ...msgs["4"]];
}
seed();

// Scripted replies for the video, keyed by conversation.
const replies = { "1": "Deal. Kevin's invited too 🐱", default: "Sounds good 👍" };
const SLOW = Number(process.env.SLOW || 1);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms * SLOW));
async function reply(c, sent) {
  await sleep(500); emit("message", { ...sent, status: "delivered" });
  await sleep(600); emit("message", { ...sent, status: "read" });
  emit("typing", { conversation_id: c.id, typing: true });
  await sleep(1400);
  emit("typing", { conversation_id: c.id, typing: false });
  const m = { id: String(++nextID), conversation_id: c.id, timestamp: Date.now(), sender: c.name, from_me: false,
    text: replies[c.id] ?? replies.default, status: "received" };
  msgs[c.id].push(m);
  Object.assign(convs[c.id], { snippet: m.text, snippet_from_me: false, timestamp: m.timestamp });
  emit("message", m); emit("conversation", convs[c.id]);
}

function handle(cmd, a = {}) {
  switch (cmd) {
    case "state": return { state, account: "demo@gmail.com" };
    case "check_cookies": return { account: "demo@gmail.com" };
    case "contacts": return [
      { name: "Oliver Bennett", number: "+15551230101", formatted_number: "+1 555 123 0101", top: true, avatar_id: "p4" },
      { name: "JB Bouhier", number: "+15551230102", formatted_number: "+1 555 123 0102", top: true, avatar_id: "p1" },
      { name: "Alex Morgan", number: "+15551230103", formatted_number: "+1 555 123 0103", top: false, avatar_id: "palex" }];
    case "sims": return [{ number: 1, label: "Personal", phone: "+1 555 123 0001" }, { number: 2, label: "Work", phone: "+1 555 123 0002" }];
    case "avatars": return Object.fromEntries((a.ids ?? []).map((id) =>
      [id, fs.existsSync(path.join(ASSETS, `avatar-${id}.jpg`)) ? `/x/avatar-${id}.jpg` : ""]));
    case "download": return { path: `/x/${a.media_id.replace(/^demo-/, "")}.jpg` };
    case "conversations": return Object.values(convs).sort((x, y) => y.timestamp - x.timestamp);
    case "messages": {
      let l = msgs[a.conversation_id] ?? [];
      if (a.before_ts) l = l.filter((m) => m.timestamp < a.before_ts);
      l = [...l].sort((x, y) => y.timestamp - x.timestamp);
      const n = a.count || 40;
      return { messages: l.slice(0, n), more: l.length > n };
    }
    case "send": {
      const c = convs[a.conversation_id];
      const m = { id: String(++nextID), tmp_id: a.tmp_id, conversation_id: c.id, timestamp: Date.now(), from_me: true,
        text: a.text, status: "sent", reply_to: a.reply_to, sender: "" };
      msgs[c.id].push(m);
      Object.assign(c, { snippet: m.text, snippet_from_me: true, timestamp: m.timestamp });
      emit("message", m); emit("conversation", c);
      reply(c, m);
      return { tmp_id: a.tmp_id };
    }
    case "react": {
      const m = (msgs[a.conversation_id] ?? []).find((x) => x.id === a.message_id);
      if (m) {
        m.reactions = a.remove ? [] : [{ emoji: a.emoji, count: 1, mine: true }];
        setTimeout(() => emit("message", m), 120 * SLOW);
      }
      return null;
    }
    default: return null;
  }
}

const settings = { mode: "light", theme: "default", background: "gradient", transparency: 0, zoom: 1, sidebar_visible: true,
  sidebar_width: 340, sidebar_wide_width: 340, notifications: false, notification_preview: true, notification_sound: false,
  enter_sends: true, send_typing: true, muted: [] };

const types = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".jpg": "image/jpeg", ".png": "image/png" };
http.createServer(async (req, res) => {
  const url = new URL(req.url, "http://x");
  if (url.pathname === "/events") {
    res.writeHead(200, { "Content-Type": "text/event-stream", "Cache-Control": "no-cache" });
    res.write("\n"); clients.add(res); req.on("close", () => clients.delete(res)); return;
  }
  if (url.pathname.startsWith("/rpc/")) {
    let body = ""; for await (const ch of req) body += ch;
    const args = body ? JSON.parse(body) : [];
    const method = url.pathname.slice(5);
    let result = null;
    if (method === "Call") { if (args[0] === "messages") await sleep(150); result = handle(args[0], args[1]); }
    else if (method === "IsDemo") result = true;
    else if (method === "GetSettings") result = settings;
    else if (method === "UpdateSettings") Object.assign(settings, args[0]);
    else if (["RequestNotifications", "BlurSupported"].includes(method)) result = false;
    else result = "";
    res.writeHead(200, { "Content-Type": "application/json" });
    return res.end(JSON.stringify({ result }));
  }
  if (url.pathname === "/wails/custom.js") { res.writeHead(200, { "Content-Type": "text/javascript" }); return res.end(""); }
  let file;
  if (url.pathname.startsWith("/media/")) file = path.join(ASSETS, decodeURIComponent(url.pathname.slice(7)));
  else file = path.join(DIST, url.pathname === "/" ? "index.html" : url.pathname);
  if (!fs.existsSync(file) || fs.statSync(file).isDirectory()) { res.writeHead(404); return res.end(); }
  res.writeHead(200, { "Content-Type": types[path.extname(file)] ?? "application/octet-stream" });
  fs.createReadStream(file).pipe(res);
}).listen(PORT, "127.0.0.1", () => console.log("mock on", PORT));
