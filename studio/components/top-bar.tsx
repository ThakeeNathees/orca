"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, Hammer, Play, Plus, Workflow } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useStudioStore } from "@/lib/store";

const DEFAULT_WORKFLOW_NAME = "My Workflow";

export function TopBar() {
  const currentView = useStudioStore((s) => s.currentView);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);
  const createWorkflow = useStudioStore((s) => s.createWorkflow);
  const activeWorkflowId = useStudioStore((s) => s.activeWorkflowId);
  const workflows = useStudioStore((s) => s.workflows);
  const isEditor = currentView === "editor";

  const activeWorkflow = workflows.find((w) => w.id === activeWorkflowId);
  const initialName = activeWorkflow?.name ?? DEFAULT_WORKFLOW_NAME;

  const [workflowName, setWorkflowName] = useState(initialName);
  const [draft, setDraft] = useState(initialName);
  const [editing, setEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  // Sync when switching workflows
  useEffect(() => {
    setWorkflowName(initialName);
    setDraft(initialName);
  }, [initialName]);

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
      <div className="flex min-w-0 flex-1 items-center gap-2">
        {isEditor && (
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={goToDashboard}
            className="shrink-0 cursor-pointer text-muted-foreground hover:text-foreground"
            aria-label="Back to workflows"
          >
            <ArrowLeft className="size-4" />
          </Button>
        )}
        <span className="text-base font-semibold tracking-tight">
          {isEditor ? "Orca Studio" : "Workflows"}
        </span>
      </div>

      {isEditor && (
        <div className="flex shrink-0 items-center gap-2">
          <div
            className="flex size-8 shrink-0 items-center justify-center rounded-md bg-green-300 shadow-sm dark:bg-green-300"
            aria-hidden
          >
            <Workflow
              className="size-[15px] text-zinc-900 dark:text-slate-900"
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
      )}

      <div className="flex min-w-0 flex-1 items-center justify-end gap-1">
        {isEditor ? (
          <>
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
            <a
              href="https://github.com/ThakeeNathees/orca"
              target="_blank"
              rel="noopener noreferrer"
              className="flex size-8 shrink-0 items-center justify-center rounded-md text-sidebar-foreground transition-colors hover:bg-sidebar-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
              aria-label="Orca repository on GitHub"
              title="View source on GitHub"
            >
              <svg
                className="size-[18px] shrink-0"
                viewBox="0 0 98 96"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden
              >
                <path
                  fill="currentColor"
                  fillRule="evenodd"
                  clipRule="evenodd"
                  d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"
                />
              </svg>
            </a>
          </>
        ) : (
          <Button
            onClick={createWorkflow}
            size="sm"
            className="gap-1.5 cursor-pointer"
          >
            <Plus className="size-4" />
            New Workflow
          </Button>
        )}
      </div>
    </header>
  );
}
