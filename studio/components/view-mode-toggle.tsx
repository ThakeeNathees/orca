"use client";

import { FileCode2, LayoutPanelLeft } from "lucide-react";
import { cn } from "@/lib/utils";

/** Whether the center pane shows the React Flow canvas or the Monaco buffer. */
export type StudioViewMode = "ui" | "code";

type ViewModeToggleProps = {
  mode: StudioViewMode;
  onModeChange: (mode: StudioViewMode) => void;
};

const segmentClass =
  "inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40";

/**
 * Segmented control to switch between visual canvas and source editing.
 * Uses tab semantics for keyboard and screen-reader users.
 */
export function ViewModeToggle({ mode, onModeChange }: ViewModeToggleProps) {
  return (
    <div
      role="tablist"
      aria-label="Editor view"
      className="flex items-center gap-0.5 rounded-full border border-border bg-sidebar/95 px-1 py-0.5 shadow-md backdrop-blur-sm dark:bg-[#18181B]/95"
    >
      <button
        type="button"
        role="tab"
        aria-selected={mode === "ui"}
        id="studio-tab-ui"
        aria-controls="studio-panel-center"
        tabIndex={mode === "ui" ? 0 : -1}
        onClick={() => onModeChange("ui")}
        className={cn(
          segmentClass,
          mode === "ui"
            ? "bg-neutral-200 text-sidebar-foreground shadow-sm dark:bg-zinc-600 dark:text-zinc-50"
            : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground"
        )}
      >
        <LayoutPanelLeft className="size-3.5 shrink-0" aria-hidden />
        UI
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={mode === "code"}
        id="studio-tab-code"
        aria-controls="studio-panel-center"
        tabIndex={mode === "code" ? 0 : -1}
        onClick={() => onModeChange("code")}
        className={cn(
          segmentClass,
          mode === "code"
            ? "bg-neutral-200 text-sidebar-foreground shadow-sm dark:bg-zinc-600 dark:text-zinc-50"
            : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground"
        )}
      >
        <FileCode2 className="size-3.5 shrink-0" aria-hidden />
        Code
      </button>
    </div>
  );
}
