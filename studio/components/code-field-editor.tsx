"use client";

import { useState, useCallback } from "react";
import Editor from "@monaco-editor/react";
import { Maximize2 } from "lucide-react";
import { Dialog, DialogHeader, DialogBody } from "@/components/ui/dialog";
import {
  MONACO_FONT_FAMILY,
  ORCA_MONACO_THEME,
  registerOrcaTheme,
} from "@/lib/monaco-theme";

interface CodeFieldEditorProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  label?: string;
}

const EDITOR_OPTIONS = {
  minimap: { enabled: false },
  fontSize: 12,
  fontFamily: MONACO_FONT_FAMILY,
  scrollBeyondLastLine: false,
  wordWrap: "on" as const,
  tabSize: 4,
  lineNumbers: "off" as const,
  glyphMargin: false,
  folding: false,
  lineDecorationsWidth: 8,
  lineNumbersMinChars: 0,
  renderLineHighlight: "none" as const,
  overviewRulerLanes: 0,
  hideCursorInOverviewRuler: true,
  overviewRulerBorder: false,
  scrollbar: { vertical: "hidden" as const, horizontal: "auto" as const },
};

const MODAL_EDITOR_OPTIONS = {
  minimap: { enabled: false },
  fontSize: 13,
  fontFamily: MONACO_FONT_FAMILY,
  scrollBeyondLastLine: false,
  wordWrap: "on" as const,
  padding: { top: 12 },
  tabSize: 4,
};

/**
 * Inline Python code editor with an expand-to-modal button.
 * Shows a compact Monaco editor in the inspector; the expand button opens
 * a full-size modal editor for comfortable editing.
 */
export function CodeFieldEditor({
  value,
  onChange,
  placeholder,
  label,
}: CodeFieldEditorProps) {
  const [modalOpen, setModalOpen] = useState(false);
  const displayValue = value || placeholder || "";

  const handleChange = useCallback(
    (v: string | undefined) => onChange(v ?? ""),
    [onChange]
  );

  return (
    <>
      {/* Inline compact editor */}
      <div className="group relative overflow-hidden rounded-md border border-transparent bg-muted/40 focus-within:border-border">
        <Editor
          height="120px"
          language="python"
          theme={ORCA_MONACO_THEME}
          beforeMount={registerOrcaTheme}
          value={displayValue}
          onChange={handleChange}
          options={EDITOR_OPTIONS}
          loading={
            <div className="flex h-[120px] items-center justify-center text-xs text-muted-foreground">
              Loading...
            </div>
          }
        />
        <button
          onClick={() => setModalOpen(true)}
          className="absolute right-1.5 top-1.5 rounded p-1 text-muted-foreground/40 opacity-0 transition-all hover:bg-accent hover:text-foreground group-hover:opacity-100"
          title="Expand editor"
        >
          <Maximize2 className="h-3.5 w-3.5" />
        </button>
      </div>

      {/* Expanded modal editor */}
      <Dialog open={modalOpen} onOpenChange={setModalOpen}>
        <DialogHeader onClose={() => setModalOpen(false)}>
          {label ?? "Code Editor"}
        </DialogHeader>
        <DialogBody>
          <Editor
            height="60vh"
            language="python"
            theme={ORCA_MONACO_THEME}
            beforeMount={registerOrcaTheme}
            value={displayValue}
            onChange={handleChange}
            options={MODAL_EDITOR_OPTIONS}
            loading={
              <div className="flex h-[60vh] items-center justify-center text-sm text-muted-foreground">
                Loading editor...
              </div>
            }
          />
        </DialogBody>
      </Dialog>
    </>
  );
}
