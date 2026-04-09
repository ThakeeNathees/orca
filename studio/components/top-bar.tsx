"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, Hammer, Play, Plus, Workflow } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useStudioStore } from "@/lib/store";
import { generateOrcaSource, sanitizeIdent } from "@/lib/orca-gen";

const DEFAULT_WORKFLOW_NAME = "My Workflow";

export function TopBar() {
  const currentView = useStudioStore((s) => s.currentView);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);
  const createWorkflow = useStudioStore((s) => s.createWorkflow);
  const renameWorkflow = useStudioStore((s) => s.renameWorkflow);
  const activeWorkflowId = useStudioStore((s) => s.activeWorkflowId);
  const workflows = useStudioStore((s) => s.workflows);
  const nodes = useStudioStore((s) => s.nodes);
  const edges = useStudioStore((s) => s.edges);
  const isEditor = currentView === "editor";

  const activeWorkflow = workflows.find((w) => w.id === activeWorkflowId);
  const activeName = activeWorkflow?.name ?? DEFAULT_WORKFLOW_NAME;
  // Pastel accent — matches the swatch on the dashboard card so the icon
  // stays visually tied to its workflow across pages.
  const accentColor = activeWorkflow?.color ?? "#86efac";

  const [draft, setDraft] = useState(activeName);
  const [editing, setEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  // Keep the local draft in sync when the active workflow (or its name)
  // changes from elsewhere — e.g. switching workflows on the dashboard.
  useEffect(() => {
    setDraft(activeName);
  }, [activeName]);

  const commit = useCallback(() => {
    const t = draft.trim();
    const next = t.length > 0 ? t : DEFAULT_WORKFLOW_NAME;
    setDraft(next);
    setEditing(false);
    // Only persist if something actually changed and we have a workflow.
    if (activeWorkflowId && next !== activeName) {
      void renameWorkflow(activeWorkflowId, next);
    }
  }, [draft, activeWorkflowId, activeName, renameWorkflow]);

  const cancel = useCallback(() => {
    setDraft(activeName);
    setEditing(false);
  }, [activeName]);

  useEffect(() => {
    if (!editing) return;
    const el = inputRef.current;
    if (!el) return;
    el.focus();
    el.select();
  }, [editing]);

  // A build is only meaningful when there is something on the canvas.
  // We also guard against the pre-hydration empty state.
  const canBuild = isEditor && nodes.length > 0;

  const handleBuild = useCallback(() => {
    if (!canBuild) return;
    const source = generateOrcaSource(nodes, edges);

    // Derive a safe filename from the workflow name. sanitizeIdent already
    // enforces snake_case / no weird characters — exactly what we want for
    // a cross-platform download name.
    const base = sanitizeIdent(activeName) || "workflow";
    const filename = `${base}.oc`;

    // Browser download dance: Blob → object URL → hidden anchor click →
    // revoke. No backend, no IndexedDB round-trip — the source is
    // regenerated on the spot from current store state.
    const blob = new Blob([source], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    // Keep the element off-layout but still click-dispatchable.
    link.style.display = "none";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    // Deferred revoke — some browsers need the URL alive briefly past the
    // click for the download to register.
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }, [canBuild, nodes, edges, activeName]);

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
            className="flex size-8 shrink-0 items-center justify-center rounded-md shadow-sm"
            style={{ backgroundColor: accentColor }}
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
                setDraft(activeName);
                setEditing(true);
              }}
              className="max-w-[min(24rem,calc(100vw-12rem))] truncate rounded-md px-1.5 py-0.5 text-left text-base font-semibold tracking-tight text-sidebar-foreground transition-colors hover:bg-sidebar-accent focus-visible:border-ring focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
              title="Click to rename"
            >
              {activeName}
            </button>
          )}
        </div>
      )}

      <div className="flex min-w-0 flex-1 items-center justify-end gap-1">
        {isEditor ? (
          <>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleBuild}
              disabled={!canBuild}
              className="gap-1.5 cursor-pointer text-sidebar-foreground hover:bg-sidebar-accent disabled:cursor-not-allowed disabled:opacity-50"
              title={
                canBuild
                  ? "Download .oc source"
                  : "Add nodes to the canvas to build"
              }
            >
              <Hammer className="h-3.5 w-3.5 shrink-0" />
              Build
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled
              className="gap-1.5 cursor-not-allowed text-sidebar-foreground disabled:opacity-50"
              title="Download and run with `orca run` on your machine."
            >
              <Play className="h-3.5 w-3.5 shrink-0" />
              Run
            </Button>
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
