import type { Monaco } from "@monaco-editor/react";

/**
 * Monaco theme name used across the Studio. Registered once per Monaco
 * instance via `registerOrcaTheme` (idempotent — Monaco tolerates repeat
 * registrations by overwriting).
 *
 * Colors mirror the `--bg-*` / `--text-*` CSS tokens in `globals.css` so the
 * editor blends into the Linear-inspired surrounding UI. They're duplicated
 * here as hex literals because Monaco's theme API does not resolve CSS
 * variables — it expects hex strings at registration time.
 */
export const ORCA_MONACO_THEME = "orca-dark";

export function registerOrcaTheme(monaco: Monaco): void {
  monaco.editor.defineTheme(ORCA_MONACO_THEME, {
    base: "vs-dark",
    inherit: true,
    rules: [
      { token: "comment", foreground: "62666d", fontStyle: "italic" },
      { token: "keyword", foreground: "7170ff" },
      { token: "string", foreground: "10b981" },
      { token: "number", foreground: "d0d6e0" },
      { token: "type", foreground: "828fff" },
      { token: "variable", foreground: "f7f8f8" },
      { token: "identifier", foreground: "d0d6e0" },
    ],
    colors: {
      "editor.background": "#0a0a0a",
      "editor.foreground": "#d0d6e0",
      "editorLineNumber.foreground": "#62666d",
      "editorLineNumber.activeForeground": "#d0d6e0",
      "editor.lineHighlightBackground": "#191a1b",
      "editor.lineHighlightBorder": "#00000000",
      "editor.selectionBackground": "#5e6ad255",
      "editor.inactiveSelectionBackground": "#5e6ad233",
      "editorCursor.foreground": "#7170ff",
      "editorWhitespace.foreground": "#23252a",
      "editorIndentGuide.background": "#18191a",
      "editorIndentGuide.activeBackground": "#23252a",
      "editorGutter.background": "#0a0a0a",
      "scrollbarSlider.background": "#ffffff14",
      "scrollbarSlider.hoverBackground": "#ffffff22",
      "scrollbarSlider.activeBackground": "#ffffff33",
    },
  });
}

/** Font stack for all Monaco instances — matches the `--font-mono` CSS var
 * but spelled out because Monaco passes this string directly to CSS without
 * resolving variables. */
export const MONACO_FONT_FAMILY =
  "var(--font-jetbrains-mono), ui-monospace, SFMono-Regular, Menlo, Monaco, monospace";
