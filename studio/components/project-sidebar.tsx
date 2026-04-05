"use client";

import { useState, useRef, useEffect } from "react";
import { Plus, MoreHorizontal } from "lucide-react";
import { cn } from "@/lib/utils";
import { useStudioStore } from "@/lib/store";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";

function ProjectItem({
  project,
  isActive,
  onSelect,
  onRename,
  onDelete,
}: {
  project: { id: string; name: string };
  isActive: boolean;
  onSelect: () => void;
  onRename: (name: string) => void;
  onDelete: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState(project.name);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [editing]);

  const commitRename = () => {
    const trimmed = editValue.trim();
    if (trimmed && trimmed !== project.name) {
      onRename(trimmed);
    } else {
      setEditValue(project.name);
    }
    setEditing(false);
  };

  // eslint-disable-next-line no-restricted-syntax -- compound row with nested buttons
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => {
        if (!editing) onSelect();
      }}
      onDoubleClick={() => {
        setEditing(true);
        setEditValue(project.name);
      }}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") onSelect();
      }}
      className={cn(
        "group flex w-full items-center justify-between rounded-md px-3 py-2 text-sm transition-colors cursor-pointer",
        isActive
          ? "bg-sidebar-accent text-foreground"
          : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
      )}
    >
      {editing ? (
        <input
          ref={inputRef}
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          onBlur={commitRename}
          onKeyDown={(e) => {
            if (e.key === "Enter") commitRename();
            if (e.key === "Escape") {
              setEditValue(project.name);
              setEditing(false);
            }
            e.stopPropagation();
          }}
          onClick={(e) => e.stopPropagation()}
          className="min-w-0 flex-1 rounded border border-border bg-background px-1.5 py-0.5 text-sm text-foreground outline-none focus:ring-1 focus:ring-ring"
        />
      ) : (
        <span className="min-w-0 flex-1 truncate">{project.name}</span>
      )}

      {/* Three-dot menu */}
      {!editing && (
        <DropdownMenu>
          <DropdownMenuTrigger
            className="ml-1 flex size-6 items-center justify-center rounded text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 hover:bg-accent hover:text-foreground cursor-pointer"
            aria-label={`Options for ${project.name}`}
          >
            <MoreHorizontal className="size-3.5" />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-32">
            <DropdownMenuItem
              onClick={() => {
                setEditing(true);
                setEditValue(project.name);
              }}
            >
              Rename
            </DropdownMenuItem>
            <DropdownMenuItem variant="destructive" onClick={onDelete}>
              Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
}

export function ProjectSidebar() {
  const projects = useStudioStore((s) => s.projects);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const createProject = useStudioStore((s) => s.createProject);
  const renameProject = useStudioStore((s) => s.renameProject);
  const deleteProject = useStudioStore((s) => s.deleteProject);
  const setActiveProject = useStudioStore((s) => s.setActiveProject);

  return (
    <aside className="flex w-[220px] shrink-0 flex-col border-r border-border bg-sidebar">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3">
        <span className="text-sm font-semibold text-foreground">Projects</span>
        <button
          type="button"
          onClick={createProject}
          className="flex size-6 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
          aria-label="New project"
        >
          <Plus className="size-4" />
        </button>
      </div>

      {/* Project list */}
      <div className="flex-1 space-y-0.5 overflow-y-auto px-2">
        {projects.map((project) => (
          <ProjectItem
            key={project.id}
            project={project}
            isActive={project.id === activeProjectId}
            onSelect={() => setActiveProject(project.id)}
            onRename={(name) => renameProject(project.id, name)}
            onDelete={() => deleteProject(project.id)}
          />
        ))}
      </div>
    </aside>
  );
}
