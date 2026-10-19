# Theming

GoMessages has four appearance settings, and a theme is a small JSON file.
This page covers both: how the settings work, and how to make a theme.

## Appearance settings

Open **Settings → Appearance**.

| Setting | What it does |
|---|---|
| **Mode** | **System** follows your computer's light or dark setting and switches with it. **Light** and **Dark** fix it. |
| **Theme** | The colors. Every theme has a light side and a dark side, and Mode picks which one you see. Each swatch shows the theme's accent on both sides. |
| **Background** | **Gradient** shows the theme's art behind slightly see-through panels; themes without art get a soft gradient made from their own colors. **Solid** is plain color and opaque panels, for every theme. |
| **Transparency** | 0–100%. Fades the app's panels, and the theme's background art if it has some, so you see what's behind the window. |

Transparency depends on the platform:

- **macOS:** what's behind the window is shown blurred, with the system's own behind-window blur. The window keeps its rounded corners.
- **Windows 11:** the same, using Acrylic.
- **Linux:** the window can't show the desktop, so transparency fades the theme's art into its plain background color.

Message bubbles always stay solid so messages stay readable. At high
transparency, readability depends on what's behind the window: a busy,
high-contrast wallpaper works best at lower values.

## Built-in themes

| Theme | Light side | Dark side | Gradient |
|---|---|---|---|
| Default | Clean white, Google-style blue | Near black, soft blue | Made from its colors |
| Graphite | Soft white, charcoal | Charcoal, silver | Made from its colors |
| Sky | Morning sky, deep blue | Deep blue, frosted glass | Painted clouds |
| Mauve | Catppuccin Latte | Catppuccin Mocha | Made from its colors |
| Aurora | Lilac dawn, orchid | Polar night, orchid glow | Mesh |
| Peach | Warm blush, rust | Embers, apricot | Mesh |
| Amber | Sunrise: cream and gold | Sunset: dusk and gold | Mesh |
| Sage | Pale sage, deep green | Forest night, mint | Mesh |

They run from plain to colorful, then around the color wheel, so each one
has its own role: two without a hue, then blue, purple, orchid, rust, gold,
and green.

Mauve uses [Catppuccin](https://catppuccin.com) Latte and Mocha (MIT), with
Latte's brighter base from
[zed-catppuccin-blur](https://github.com/jenslys/zed-catppuccin-blur) (MIT).
Mesh gradients are rendered with
[paper.design's shaders](https://shaders.paper.design) (Apache-2.0), once,
and again only when the window is resized. Without WebGL a CSS gradient
stands in.

## Making a theme

A theme is one JSON file with colors only. There is no CSS: a theme can't
change the layout, load anything from the internet, or restyle parts of the
app. That keeps every theme safe to use and the app consistent.

Today, themes ship with the app: add a file to `frontend/src/themes/` and
open a pull request. Loading your own theme files from a folder, without
building the app, is planned.

### A minimal theme

```json
{
  "$schema": "./theme.schema.json",
  "format": 1,
  "id": "harbor",
  "name": "Harbor",
  "dark": {
    "base": "#11181f",
    "mantle": "#0c1217",
    "surface0": "#1d2731",
    "surface1": "#2c3a47",
    "text": "#e4ecf2",
    "subtext": "#a7b6c2",
    "accent": "#7cc4e6",
    "onAccent": "#0c1217",
    "red": "#f4978e",
    "yellow": "#eac46f"
  }
}
```

This theme has only a dark side. A theme with only one side uses Default's
colors for the other, so it still works in both modes. Give it both sides
when you can.

### Top-level fields

| Field | Required | Description |
|---|---|---|
| `format` | yes | Always `1`. |
| `id` | yes | Lowercase letters, digits, and dashes. Saved in settings, so don't change it after release. |
| `name` | yes | Shown under the swatch. |
| `light`, `dark` | at least one | A palette (below). |
| `order` | no | Position among the themes; lower comes first. Decimals are fine (`3.5`). |
| `credit` | no | Shown when hovering the swatch. Credit the palette you based it on. |
| `$schema` | no | `./theme.schema.json`, for autocomplete and checks in your editor. |

### Palette colors

All colors are `#rrggbb`, except `surface1`, which may also be
`rgba(r, g, b, a)` for translucent dividers.

| Key | Where it's used |
|---|---|
| `base` | Conversation background, menus. |
| `mantle` | Conversation list, settings cards. |
| `surface0` | Incoming bubbles, the message box, hovered rows. |
| `surface1` | Dividers. |
| `text` | Main text. |
| `subtext` | Secondary text: times, hints, previews. |
| `accent` | Buttons, links, switches, the selected chat, and the tint of your own bubbles. |
| `onAccent` | Text and icons on the accent (buttons). |
| `red` | Errors, destructive actions. |
| `yellow` | Warning banners (mixed into `base`). |

The app derives the rest from these:

| Derived | From |
|---|---|
| Selected chat | `accent` mixed into `mantle` (20% light, 28% dark) |
| Your bubbles | `accent` mixed into `base` (18% light, 30% dark) |
| Text on your bubbles | `text` |
| Hovered rows | `surface0` |
| Warning banners | `yellow` mixed into `base` (25%) |

To set any of these exactly instead, add the optional keys `selected`,
`bubbleOut`, `onBubbleOut`, `hover`, or `warn` (all `#rrggbb`). Default uses
them to keep the app's original look.

### Background art

With the Gradient background, a side shows art under its see-through
panels: the conversation list is 70% opaque and the conversation 72% (82%
and 84% for clouds), so the art shows through softly. With Solid, art is
never shown.

Art is optional. A side without `background` gets a gradient made from its
own `base` and `mantle` with a hint of `accent`, so every theme looks right
with either setting.

A **mesh** is a smooth gradient between two to six colors:

```json
"background": {
  "mesh": {
    "colors": ["#fff1d6", "#ffcf8a", "#ffaa80", "#ffe4c2"],
    "distortion": 0.85,
    "swirl": 0.4,
    "frame": 31000
  }
}
```

- `distortion` and `swirl` (about 0–1) shape the blend.
- `frame` picks the composition. Try a few values until you like it; the art never moves.

**Clouds** paint soft white clouds over a sky of three colors, top to bottom:

```json
"background": { "clouds": { "sky": ["#7fb2ec", "#a9cdf5", "#d3e6fb"] } }
```

Give each side its own art, in its own tones: a light gradient behind dark
text, a dark one behind light text.

### Readability

Every theme must keep text readable on both sides. The UI tests check the
[WCAG AA](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html)
contrast ratio, 4.5:1, for:

- `text` on `base`, `mantle`, `surface0`, the selected chat, and your bubbles
- `subtext` on `base` and `mantle`
- `accent` on `base` and `mantle`, and `onAccent` on `accent`
- with the Gradient background: `text`, `subtext`, and `accent` on the
  see-through list, and `text` and `subtext` on the see-through
  conversation, over each of the art's colors (your own, or the made one)

Every side is tested with both backgrounds.

Practical advice:

- On a light side, keep the accent dark enough to read as text on the
  background. On a dark side, keep it light.
- Keep the art in a narrow lightness range: pale for a light side, deep for
  a dark side. One bright color in a dark gradient will fail the list check.
- Clouds are capped at 60% white, so a light sky's bottom color should stay
  pale and a dark sky's should stay deep.

### Trying a theme

1. Add the file to `frontend/src/themes/`. Your editor checks it against
   `theme.schema.json`.
2. Run `just demo` and pick it in **Settings → Appearance**. Try both modes,
   both backgrounds, and Transparency at 0 and around 50%.
3. Add its `id` to `THEME_IDS` in `frontend/e2e/ui.test.mjs` and run
   `just test`. The contrast test lists every pair that falls short, with its
   ratio.
4. Open a pull request with screenshots of both sides (`just tour /tmp/shots
   light <id>` and `just tour /tmp/shots dark <id>`).

## Settings file

Appearance is saved in `settings.json` in the app's config folder:

```json
{ "mode": "system", "theme": "amber", "background": "gradient", "transparency": 20 }
```

An unknown `theme` falls back to Default; a renamed one (Horizon is now
Amber) is updated. Settings from earlier versions,
which had separate light and dark themes, accents, or blur levels, are
converted automatically and look as close as the current themes allow.
