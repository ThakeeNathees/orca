/**
 * Raw color constants for the React Flow canvas.
 *
 * React Flow renders edges and the background pattern as SVG and its
 * color props are passed through to `stroke`/`fill` attributes. SVG
 * attributes do not resolve CSS custom properties (e.g. `var(--foo)`)
 * reliably across browsers, so these mirror the Linear palette from
 * `globals.css` as literal hex strings. Keep them in sync with the
 * `--text-quaternary` / `--border-subtle` / `--accent` tokens.
 */

/** Default edge stroke — Linear quaternary text gray (`#62666d`). */
export const CANVAS_EDGE_STROKE = "#62666d";

/** Background dot pattern — faint white that survives on `#08090a`. */
export const CANVAS_DOT_COLOR = "rgba(255, 255, 255, 0.08)";
