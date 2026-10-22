// The one door to Go. Inside the app it is the Wails bindings and events;
// opened in a browser with ?http (the binary's --serve mode) the same calls
// go over fetch and events over server-sent events, so the real backend
// can drive the UI from a plain browser (headless screenshots and tests).

import { Browser, Events, Window } from "@wailsio/runtime";
import * as Bindings from "../../bindings/gomessages/messages";

export const overHTTP = new URLSearchParams(location.search).has("http");

async function rpc(method: string, args: unknown[]): Promise<any> {
  const res = await fetch(`/rpc/${method}`, { method: "POST", body: JSON.stringify(args) });
  const body = await res.json();
  if (body.error) throw new Error(body.error);
  return body.result;
}

export const Go: typeof Bindings = overHTTP
  ? (new Proxy({}, { get: (_, name: string) => (...args: unknown[]) => rpc(name, args) }) as typeof Bindings)
  : Bindings;

const listeners = new Map<string, ((data: any) => void)[]>();
if (overHTTP) {
  new EventSource("/events").onmessage = (e) => {
    const { name, data } = JSON.parse(e.data);
    for (const cb of listeners.get(name) ?? []) cb(data);
  };
}

/** Listen to a backend event ("gm", "action", "signin_cancelled"). */
export function on(name: string, cb: (data: any) => void) {
  if (overHTTP) listeners.set(name, [...(listeners.get(name) ?? []), cb]);
  else Events.On(name, (ev: { data: any }) => cb(ev.data));
}

export function setTitle(title: string) {
  if (overHTTP) document.title = title;
  else void Window.SetTitle(title);
}

/** Opens a web page in the default browser. */
export function openURL(url: string) {
  if (overHTTP) window.open(url, "_blank", "noreferrer");
  else void Browser.OpenURL(url);
}
