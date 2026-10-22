// Still shader backdrops for the Sage, Peach, and Aurora themes: a smooth
// mesh gradient, no grain (paper.design's shaders, Apache-2.0). Rendered
// once, and again only when the window resizes — no animation, so no
// ongoing GPU work.

import {
  defaultObjectSizing,
  getShaderColorFromString,
  meshGradientFragmentShader,
  ShaderFitOptions,
  ShaderMount,
  type ShaderSizingParams,
} from "@paper-design/shaders";

export interface Backdrop {
  /** The mesh gradient's colors. */
  colors: string[];
  distortion: number;
  swirl: number;
  /** Where in the (still) pattern to look: picks the composition. */
  frame: number;
  /** A CSS stand-in: the gallery preview, and the backdrop if WebGL fails. */
  css: string;
}

function sizing(base: Required<ShaderSizingParams>) {
  return {
    u_fit: ShaderFitOptions[base.fit],
    u_scale: base.scale,
    u_rotation: base.rotation,
    u_originX: base.originX,
    u_originY: base.originY,
    u_offsetX: base.offsetX,
    u_offsetY: base.offsetY,
    u_worldWidth: base.worldWidth,
    u_worldHeight: base.worldHeight,
  };
}

/** Paints the backdrop into `host`; returns a function that removes it. */
export function mountBackdrop(host: HTMLElement, b: Backdrop): () => void {
  const mounts: ShaderMount[] = [];
  const layer = () => {
    const div = document.createElement("div");
    div.className = "layer";
    host.append(div);
    return div;
  };
  try {
    mounts.push(
      new ShaderMount(
        layer(),
        meshGradientFragmentShader,
        {
          u_colors: b.colors.map(getShaderColorFromString),
          u_colorsCount: b.colors.length,
          u_distortion: b.distortion,
          u_swirl: b.swirl,
          u_grainMixer: 0,
          u_grainOverlay: 0,
          ...sizing(defaultObjectSizing),
        },
        undefined,
        0,
        b.frame,
      ),
    );
  } catch (e) {
    // No WebGL: the CSS stand-in under the layers stays.
    console.warn("backdrop shader unavailable:", e);
  }
  return () => {
    for (const m of mounts) m.dispose();
    host.querySelectorAll(".layer").forEach((el) => el.remove());
  };
}
