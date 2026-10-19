// Themes. A theme is a light and a dark palette, each with optional art
// under the panes. The mode (system, light, dark) picks the half; the
// background setting shows the art (a gradient is made for palettes
// without one) or keeps everything solid; the transparency setting fades
// the panes and the art to what's behind the window. Those are the app's
// settings; a theme only brings colors.
//
// Every theme is a file in one format (src/themes/*.json, described by
// theme.schema.json), so themes people bring later load the same way.
// A file with only one half uses Default's for the other.
//
// Mauve is Catppuccin Latte and Mocha (MIT), with Latte's brighter base
// from jenslys/zed-catppuccin-blur (MIT). Sage and Peach are after
// jbouhier.com and notacat.co. Mesh art is paper.design's shader
// (backdrop.ts, Apache-2.0); clouds are a CSS painting.

import type { Backdrop } from "./backdrop";

const COLOR_KEYS = ["base", "mantle", "surface0", "surface1", "text", "subtext", "accent", "onAccent", "red", "yellow"] as const;
const OPTIONAL_KEYS = ["hover", "selected", "bubbleOut", "onBubbleOut", "warn"] as const;

export type Background = { mesh: { colors: string[]; distortion?: number; swirl?: number; frame?: number } } | { clouds: { sky: string[] } };

export type Palette = Record<(typeof COLOR_KEYS)[number], string> &
  Partial<Record<(typeof OPTIONAL_KEYS)[number], string>> & { background?: Background };

export interface Theme {
  id: string;
  name: string;
  credit?: string;
  order: number;
  light: Palette;
  dark: Palette;
}

const HEX = /^#[0-9a-fA-F]{6}$/;
const RGBA = /^rgba\(\s*\d{1,3},\s*\d{1,3},\s*\d{1,3},\s*(0|1|0?\.\d+)\s*\)$/;

/** Checks one palette; returns what's wrong, or nothing. */
function paletteProblem(p: any): string | null {
  if (!p || typeof p !== "object") return "not an object";
  for (const k of COLOR_KEYS) {
    const v = p[k];
    // Dividers may be translucent; everything else is mixed, so hex.
    if (typeof v !== "string" || !(HEX.test(v) || (k === "surface1" && RGBA.test(v)))) return `${k}: not a color`;
  }
  for (const k of OPTIONAL_KEYS) if (k in p && !HEX.test(p[k])) return `${k}: not a #rrggbb color`;
  const b = p.background;
  if (b !== undefined) {
    const colors = b?.mesh?.colors ?? b?.clouds?.sky;
    if (!Array.isArray(colors) || !colors.every((c: unknown) => typeof c === "string" && HEX.test(c)))
      return "background: needs mesh.colors or clouds.sky as #rrggbb colors";
    if (b.clouds && colors.length !== 3) return "background: clouds.sky takes three colors";
  }
  return null;
}

/** Reads a theme file. Throws with what's wrong, for a notice in Settings. */
export function parseTheme(doc: any, fallback: Theme | null): Theme {
  if (doc?.format !== 1) throw new Error("format: expected 1");
  if (typeof doc.id !== "string" || !/^[a-z0-9-]+$/.test(doc.id)) throw new Error("id: lowercase letters, digits, dashes");
  if (typeof doc.name !== "string" || !doc.name) throw new Error("name: missing");
  if (!doc.light && !doc.dark) throw new Error("needs a light or a dark palette");
  for (const half of ["light", "dark"] as const) {
    const problem = doc[half] && paletteProblem(doc[half]);
    if (problem) throw new Error(`${half}.${problem}`);
  }
  const light = doc.light ?? fallback?.light;
  const dark = doc.dark ?? fallback?.dark;
  if (!light || !dark) throw new Error("needs both palettes");
  return { id: doc.id, name: doc.name, credit: doc.credit, order: doc.order ?? 100, light, dark };
}

const files = import.meta.glob<any>("../themes/*.json", { eager: true, import: "default" });
const docs = Object.entries(files)
  .filter(([path]) => !path.endsWith("theme.schema.json"))
  .map(([, doc]) => doc);
const defaultTheme = parseTheme(docs.find((d) => d.id === "default"), null);

/** The built-in themes, in order. */
export const THEMES: Theme[] = docs
  .map((d) => parseTheme(d, defaultTheme))
  .sort((a, b) => a.order - b.order);

const byId = new Map(THEMES.map((t) => [t.id, t]));

/** A theme by id; Default for one that isn't there. */
export function themeOf(id: string): Theme {
  return byId.get(id) ?? defaultTheme;
}

export function paletteOf(id: string, dark: boolean): Palette {
  const t = themeOf(id);
  return dark ? t.dark : t.light;
}

/** Mixes two #rrggbb colors: `amount` of a over b. */
export function mix(a: string, b: string, amount: number): string {
  const ch = (hex: string, i: number) => parseInt(hex.slice(1 + i * 2, 3 + i * 2), 16);
  const out = [0, 1, 2].map((i) => Math.round(ch(a, i) * amount + ch(b, i) * (1 - amount)));
  return "#" + out.map((v) => v.toString(16).padStart(2, "0")).join("");
}

function luminance(hex: string): number {
  return [0, 1, 2]
    .map((i) => parseInt(hex.slice(1 + i * 2, 3 + i * 2), 16) / 255)
    .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
    .reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
}

/** WCAG contrast ratio of two #rrggbb colors. */
export function contrast(a: string, b: string): number {
  const [x, y] = [luminance(a), luminance(b)];
  return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
}

/** A palette as the CSS variables app.css and the components read. */
function variables(p: Palette, dark: boolean): Record<string, string> {
  return {
    "--bg": p.base,
    "--sidebar": p.mantle,
    "--selected": p.selected ?? mix(p.accent, p.mantle, dark ? 0.28 : 0.2),
    "--hover": p.hover ?? p.surface0,
    "--text": p.text,
    "--subtle": p.subtext,
    "--accent": p.accent,
    "--on-accent": p.onAccent,
    "--bubble-out": p.bubbleOut ?? mix(p.accent, p.base, dark ? 0.3 : 0.18),
    "--on-bubble-out": p.onBubbleOut ?? p.text,
    "--bubble-in": p.surface0,
    "--composer": p.surface0,
    "--divider": p.surface1,
    "--danger": p.red,
    "--warn-bg": p.warn ?? mix(p.yellow, p.base, 0.25),
    "--shadow": dark ? "rgba(0, 0, 0, 0.5)" : "rgba(76, 79, 105, 0.2)",
  };
}

/** CSS for every theme's two halves, added once at startup after app.css.
 *  :root:root outranks app.css's own (pre-theme) defaults. */
export function themeCSS(themes: Theme[] = THEMES): string {
  return themes
    .flatMap((t) =>
      (["light", "dark"] as const).map((mode) => {
        const vars = Object.entries(variables(t[mode], mode === "dark"))
          .map(([k, v]) => `  ${k}: ${v};`)
          .join("\n");
        return `:root:root[data-theme="${t.id}"][data-mode="${mode}"] {\n${vars}\n  color-scheme: ${mode};\n}`;
      }),
    )
    .join("\n");
}

/** A mesh's CSS stand-in: the preview, and the art without WebGL. */
function meshCSS(c: string[]): string {
  const at = (i: number) => c[i % c.length];
  return `radial-gradient(120% 90% at 20% 15%, ${at(0)}, transparent 60%), radial-gradient(90% 80% at 85% 85%, ${at(2)}, transparent 65%), ${at(1)}`;
}

/** Clouds: soft white puffs (under 60% white, so text on the panes keeps
 *  AA) over a gradient of the three sky colors. */
function cloudsCSS([top, middle, bottom]: string[]): string {
  const puff = (w: number, h: number, x: number, y: number, a: number) =>
    `radial-gradient(${w}% ${h}% at ${x}% ${y}%, rgba(255,255,255,${a}), transparent 70%)`;
  return [
    puff(38, 26, 88, 8, 0.6),
    puff(30, 22, 70, 18, 0.4),
    puff(34, 24, 8, 88, 0.55),
    puff(28, 18, 30, 96, 0.45),
    puff(24, 16, 96, 70, 0.4),
    puff(20, 14, 4, 34, 0.35),
    `linear-gradient(160deg, ${top}, ${middle} 55%, ${bottom})`,
  ].join(", ");
}

/** A gradient for a palette without art: its own background colors with a
 *  hint of its accent, so text keeps the palette's contrast. */
function madeGradient(id: string, p: Palette, dark: boolean): Background {
  const frame = [...id].reduce((n, c) => n + c.charCodeAt(0), 0) * 997;
  return {
    mesh: {
      colors: [p.base, mix(p.accent, p.base, dark ? 0.18 : 0.12), p.mantle, mix(p.accent, p.mantle, dark ? 0.1 : 0.06)],
      distortion: 0.8,
      swirl: 0.4,
      frame,
    },
  };
}

/** The art a palette shows with the gradient background: its own, or one
 *  made from its colors. Null with the solid background. */
function artOf(id: string, dark: boolean, gradient: boolean): Background | null {
  if (!gradient) return null;
  const p = paletteOf(id, dark);
  return p.background ?? madeGradient(themeOf(id).id, p, dark);
}

/** A palette's art under the panes: CSS, with a shader over it for meshes.
 *  Null with the solid background. */
export function backgroundOf(id: string, dark: boolean, gradient: boolean): { css: string; shader: Backdrop | null } | null {
  const b = artOf(id, dark, gradient);
  if (!b) return null;
  if ("clouds" in b) return { css: cloudsCSS(b.clouds.sky), shader: null };
  const css = meshCSS(b.mesh.colors);
  const { colors, distortion = 0.8, swirl = 0.4, frame = 0 } = b.mesh;
  return { css, shader: { colors, distortion, swirl, frame, css } };
}

/** How opaque the conversation pane and the list are at no transparency
 *  (%): see-through over art, more so for meshes than for clouds. */
export function paneOpacity(id: string, dark: boolean, gradient: boolean): [number, number] {
  const b = artOf(id, dark, gradient);
  if (!b) return [100, 100];
  return "clouds" in b ? [84, 82] : [72, 70];
}

/** What the panes sit over at no transparency, for checking contrast: the
 *  art's colors (for clouds, also the brightest puff) and the panes'
 *  opacity. Empty for a solid color. */
export function underOf(id: string, dark: boolean, gradient: boolean): { colors: string[]; pane: number; list: number } {
  const b = artOf(id, dark, gradient);
  const [pane, list] = paneOpacity(id, dark, gradient);
  if (!b) return { colors: [], pane, list };
  const colors = "clouds" in b ? [...b.clouds.sky, mix("#ffffff", b.clouds.sky[2], 0.6)] : b.mesh.colors;
  return { colors, pane: pane / 100, list: list / 100 };
}
