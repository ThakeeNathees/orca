"use client";

import { Play, Hammer } from "lucide-react";

export function TopBar() {
  return (
    <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-background px-4">
      <span className="text-base font-semibold tracking-tight">Orca Studio</span>

      <div className="flex items-center gap-1">
        <div
          className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-foreground cursor-default"
          title="Coming soon"
        >
          <Hammer className="h-3.5 w-3.5 shrink-0" />
          Build
        </div>
        <div
          className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-foreground cursor-default"
          title="Coming soon"
        >
          <Play className="h-3.5 w-3.5 shrink-0" />
          Run
        </div>
      </div>
    </header>
  );
}
