"use client";

import { useCallback, useState } from "react";
import { Workflow, MoreHorizontal, FolderOpen, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Dialog, DialogHeader, DialogBody } from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { useStudioStore } from "@/lib/store";
import type { WorkflowSummary } from "@/lib/types";

/** Format a Date as a relative time string (e.g. "Edited 2 minutes ago"). */
function timeAgo(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return "Edited just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60)
    return `Edited ${minutes} minute${minutes !== 1 ? "s" : ""} ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `Edited ${hours} hour${hours !== 1 ? "s" : ""} ago`;
  const days = Math.floor(hours / 24);
  return `Edited ${days} day${days !== 1 ? "s" : ""} ago`;
}

/** Dropdown menu anchored to the three-dot button. */
function WorkflowMenu({
  workflow,
  onOpen,
  onDelete,
}: {
  workflow: WorkflowSummary;
  onOpen: () => void;
  onDelete: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="flex size-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground cursor-pointer"
        aria-label={`Options for ${workflow.name}`}
      >
        <MoreHorizontal className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={onOpen}>
          <FolderOpen className="size-4" />
          Open
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onClick={onDelete}>
          <Trash2 className="size-4" />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function Dashboard() {
  const allWorkflows = useStudioStore((s) => s.workflows);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const openWorkflow = useStudioStore((s) => s.openWorkflow);
  const deleteWorkflow = useStudioStore((s) => s.deleteWorkflow);

  const workflows = allWorkflows.filter((w) => w.projectId === activeProjectId);

  const [deleteTarget, setDeleteTarget] = useState<WorkflowSummary | null>(
    null
  );

  const confirmDelete = useCallback(() => {
    if (deleteTarget) {
      deleteWorkflow(deleteTarget.id);
      setDeleteTarget(null);
    }
  }, [deleteTarget, deleteWorkflow]);

  return (
    <div className="flex h-full flex-1 flex-col bg-sidebar">
      {/* Workflow list */}
      <ScrollArea className="flex-1">
        <div className="px-8 py-4">
          {workflows.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-muted-foreground">
              <Workflow className="size-10 mb-3 opacity-40" />
              <p className="text-sm">No workflows yet</p>
              <p className="text-xs mt-1">
                Click &quot;+ New Workflow&quot; to get started
              </p>
            </div>
          ) : (
            <div className="space-y-1">
              {workflows.map((wf) => (
                <div
                  key={wf.id}
                  role="button"
                  tabIndex={0}
                  onClick={() => openWorkflow(wf.id)}
                  onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") openWorkflow(wf.id); }}
                  className="group flex w-full items-center gap-3 rounded-lg px-4 py-3 text-left transition-colors hover:bg-sidebar-accent cursor-pointer"
                >
                  {/* Icon */}
                  <div
                    className="flex size-10 shrink-0 items-center justify-center rounded-lg shadow-sm"
                    style={{ backgroundColor: wf.color }}
                  >
                    <Workflow className="size-[18px] text-[#18181b]" />
                  </div>

                  {/* Name + timestamp */}
                  <div className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-foreground">
                      {wf.name}
                    </span>
                    <span className="block text-xs text-muted-foreground">
                      {timeAgo(wf.updatedAt)}
                    </span>
                  </div>

                  {/* Three-dot menu */}
                  <WorkflowMenu
                    workflow={wf}
                    onOpen={() => openWorkflow(wf.id)}
                    onDelete={() => setDeleteTarget(wf)}
                  />
                </div>
              ))}
            </div>
          )}
        </div>
      </ScrollArea>

      {/* Delete confirmation dialog */}
      <Dialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <DialogHeader onClose={() => setDeleteTarget(null)}>
          Delete Workflow
        </DialogHeader>
        <DialogBody className="px-4 py-4">
          <p className="text-sm text-muted-foreground">
            Are you sure you want to delete{" "}
            <span className="font-medium text-foreground">
              {deleteTarget?.name}
            </span>
            ? This action cannot be undone.
          </p>
          <div className="mt-4 flex justify-end gap-2">
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDeleteTarget(null)}
              className="cursor-pointer"
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={confirmDelete}
              className="bg-[#ef4444] text-white hover:bg-[#dc2626] cursor-pointer"
            >
              Delete
            </Button>
          </div>
        </DialogBody>
      </Dialog>
    </div>
  );
}
