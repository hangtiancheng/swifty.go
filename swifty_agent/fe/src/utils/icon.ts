import { createElement } from "react";
import { renderToString } from "react-dom/server";
import type { LucideIcon } from "lucide-react";

/**
 * Render a lucide-react icon component to an SVG string.
 * Used inside Lit templates via `unsafeHTML` so Tailwind/daisyUI
 * utility classes on the wrapping <span> can size the svg.
 *
 * This replaces the lucide-static `?raw` imports so the project no
 * longer depends on lucide-static.
 */
export function iconToSvg(Icon: LucideIcon, size = 20): string {
  return renderToString(createElement(Icon, { size }));
}
