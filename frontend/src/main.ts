import { mount } from "svelte";
import "./app.css";
import { themeCSS, underOf } from "./lib/themes";
import { overHTTP } from "./lib/go";
import App from "./App.svelte";

// The palette themes, after app.css so they take over its variables.
const style = document.createElement("style");
style.textContent = themeCSS();
document.head.append(style);
// Tests check text against what's under the see-through panes.
if (overHTTP) (window as any).gmUnder = underOf;

mount(App, { target: document.getElementById("app")! });
