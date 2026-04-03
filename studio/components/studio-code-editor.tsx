"use client";

import Editor from "@monaco-editor/react";

type StudioCodeEditorProps = {
  value: string;
  onChange: (value: string) => void;
};

/**
 * Orca source buffer using Monaco (client-only; parent should load via next/dynamic ssr:false).
 * Monaco’s built-in `hcl` mode matches Terraform / HashiCorp-style blocks (close to `.oc`).
 */
export function StudioCodeEditor({ value, onChange }: StudioCodeEditorProps) {
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col border-x border-border bg-card">
      <Editor
        height="100%"
        language="hcl"
        theme="vs-dark"
        value={value}
        onChange={(v) => onChange(v ?? "")}
        options={{
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
