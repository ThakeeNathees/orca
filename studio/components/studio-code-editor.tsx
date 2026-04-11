"use client";

import Editor from "@monaco-editor/react";
import { Info } from "lucide-react";

type StudioCodeEditorProps = {
  /** Generated `.orca` source derived from the current graph. */
  value: string;
};

/**
 * Read-only `.orca` source view.
 *
 * The buffer is derived from the visual graph via `generateOrcaSource` in
 * the parent — there is no write path yet, so Monaco is locked in
 * read-only mode and a subtle banner reminds the user. Bidirectional code
 * editing is a future phase that requires a CST-preserving parser.
 *
 * Monaco's built-in `hcl` mode is close enough to `.orca` (HashiCorp-style
 * blocks, `key = value`) to give useful syntax highlighting for free.
 */
export function StudioCodeEditor({ value }: StudioCodeEditorProps) {
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col border-x border-border bg-card">
      <div
        className="flex shrink-0 items-center gap-2 border-b border-border bg-muted/40 px-3 py-1.5 text-[11px] text-muted-foreground"
        role="note"
      >
        <Info className="size-3.5 shrink-0" aria-hidden />
        <span>Generated from visual editor. Code editing coming soon.</span>
      </div>
      <Editor
        height="100%"
        language="hcl"
        theme="vs-dark"
        value={value}
        options={{
          readOnly: true,
          // Hide the read-only tooltip that Monaco pops up on edit attempts —
          // the banner above already communicates the constraint.
          domReadOnly: true,
          minimap: { enabled: false },
          fontSize: 13,
          fontFamily:
            "var(--font-geist-mono), ui-monospace, SFMono-Regular, Menlo, Monaco, monospace",
          scrollBeyondLastLine: false,
          wordWrap: "on",
          padding: { top: 12 },
          tabSize: 4,
        }}
        loading={
          <div className="flex h-full min-h-[12rem] items-center justify-center text-sm text-muted-foreground">
            Loading editor…
          </div>
        }
      />
    </div>
  );
}
