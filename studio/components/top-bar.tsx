"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Play, Hammer, Workflow } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

const DEFAULT_WORKFLOW_NAME = "My Workflow";

export function TopBar() {
  const [workflowName, setWorkflowName] = useState(DEFAULT_WORKFLOW_NAME);
  const [draft, setDraft] = useState(DEFAULT_WORKFLOW_NAME);
  const [editing, setEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const commit = useCallback(() => {
    const t = draft.trim();
    const next = t.length > 0 ? t : DEFAULT_WORKFLOW_NAME;
    setWorkflowName(next);
    setDraft(next);
    setEditing(false);
  }, [draft]);

  const cancel = useCallback(() => {
    setDraft(workflowName);
    setEditing(false);
  }, [workflowName]);

  useEffect(() => {
    if (!editing) return;
    const el = inputRef.current;
    if (!el) return;
    el.focus();
    el.select();
  }, [editing]);

  return (
    <header className="flex h-12 shrink-0 items-center border-b border-border bg-sidebar px-4 text-sidebar-foreground">
      <div className="flex min-w-0 flex-1 items-center">
        <span className="text-base font-semibold tracking-tight">
          Orca Studio
        </span>
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <div
          className="flex size-8 shrink-0 items-center justify-center rounded-md bg-[#86efac] shadow-sm dark:bg-[#6ee7b7]"
          aria-hidden
        >
          <Workflow
            className="size-[15px] text-[#18181b] dark:text-[#0f172a]"
            strokeWidth={2}
          />
        </div>
        {editing ? (
          <Input
            ref={inputRef}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={commit}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                commit();
              }
              if (e.key === "Escape") {
                e.preventDefault();
                cancel();
              }
            }}
            className={cn(
              "h-8 min-w-[10rem] max-w-[min(24rem,calc(100vw-12rem))] border-border bg-sidebar-accent px-2.5 text-base font-semibold text-sidebar-foreground",
              "focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40"
            )}
            aria-label="Workflow name"
            maxLength={120}
          />
        ) : (
          <button
            type="button"
            onClick={() => {
              setDraft(workflowName);
              setEditing(true);
            }}
            className="max-w-[min(24rem,calc(100vw-12rem))] truncate rounded-md px-1.5 py-0.5 text-left text-base font-semibold tracking-tight text-sidebar-foreground transition-colors hover:bg-sidebar-accent focus-visible:border-ring focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
            title="Click to rename"
          >
            {workflowName}
          </button>
        )}
      </div>

      <div className="flex min-w-0 flex-1 items-center justify-end gap-1">
        <div
          className="flex cursor-default items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-sidebar-foreground"
          title="Coming soon"
        >
          <Hammer className="h-3.5 w-3.5 shrink-0" />
          Build
        </div>
        <div
          className="flex cursor-default items-center gap-1.5 rounded-md px-3 py-1.5 text-sm text-sidebar-foreground"
          title="Coming soon"
        >
          <Play className="h-3.5 w-3.5 shrink-0" />
          Run
        </div>
      </div>
    </header>
  );
}
