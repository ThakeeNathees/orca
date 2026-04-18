"use client";

import { useState } from "react";
import {
  Brain,
  Wrench,
  Bot,
  Workflow,
  Clock,
  ChevronDown,
  ChevronRight,
  DollarSign,
  Activity,
  Settings,
  Plus,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useStudioStore } from "@/lib/store";
import { Dialog, DialogHeader, DialogBody } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";

export type SidebarSection =
  | "models"
  | "skills"
  | "agents"
  | "workflows"
  | "crons"
  | "cost"
  | "runs"
  | "settings";

type NavItem = { id: SidebarSection; label: string; icon: LucideIcon };

const PRIMARY_ITEMS_BEFORE_WORKFLOWS: NavItem[] = [
  { id: "models", label: "Models", icon: Brain },
  { id: "skills", label: "Skills", icon: Wrench },
  { id: "agents", label: "Agents", icon: Bot },
];

const PRIMARY_ITEMS_AFTER_WORKFLOWS: NavItem[] = [
  { id: "crons", label: "Cron Jobs", icon: Clock },
];

const OPERATION_ITEMS: NavItem[] = [
  { id: "cost", label: "Cost", icon: DollarSign },
  { id: "runs", label: "Runs", icon: Activity },
  { id: "settings", label: "Settings", icon: Settings },
];

export const SECTION_LABELS: Record<SidebarSection, string> = Object.fromEntries(
  [
    ...PRIMARY_ITEMS_BEFORE_WORKFLOWS,
    { id: "workflows" as const, label: "Workflows" },
    ...PRIMARY_ITEMS_AFTER_WORKFLOWS,
    ...OPERATION_ITEMS,
  ].map((i) => [i.id, i.label])
) as Record<SidebarSection, string>;

function SidebarItem({
  item,
  active,
  onClick,
}: {
  item: NavItem;
  active: boolean;
  onClick: () => void;
}) {
  const Icon = item.icon;
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors cursor-pointer",
        active
          ? "bg-sidebar-accent text-foreground"
          : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
      )}
    >
      <Icon className="size-4 shrink-0" />
      <span className="min-w-0 flex-1 truncate text-left">{item.label}</span>
    </button>
  );
}

export function ProjectSidebar({
  activeSection,
  onSectionChange,
}: {
  activeSection: SidebarSection;
  onSectionChange: (section: SidebarSection) => void;
}) {
  const [operationOpen, setOperationOpen] = useState(true);
  const [workflowsOpen, setWorkflowsOpen] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");

  const workflows = useStudioStore((s) => s.workflows);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const createWorkflow = useStudioStore((s) => s.createWorkflow);
  const openWorkflow = useStudioStore((s) => s.openWorkflow);

  const projectWorkflows = workflows.filter(
    (w) => w.projectId === activeProjectId
  );

  return (
    <aside className="flex w-[220px] shrink-0 flex-col border-r border-border bg-sidebar">
      <div className="flex-1 overflow-y-auto px-2 py-3 space-y-0.5">
        {PRIMARY_ITEMS_BEFORE_WORKFLOWS.map((item) => (
          <SidebarItem
            key={item.id}
            item={item}
            active={item.id === activeSection}
            onClick={() => onSectionChange(item.id)}
          />
        ))}

        <div>
          <div
            className={cn(
              "group flex w-full items-center rounded-md pr-1 text-sm transition-colors",
              activeSection === "workflows"
                ? "bg-sidebar-accent text-foreground"
                : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
            )}
          >
            <button
              type="button"
              onClick={() => {
                onSectionChange("workflows");
                setWorkflowsOpen((v) => !v);
              }}
              aria-expanded={workflowsOpen}
              className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-3 py-2 cursor-pointer"
            >
              {workflowsOpen ? (
                <ChevronDown className="size-3.5 shrink-0 opacity-70" />
              ) : (
                <ChevronRight className="size-3.5 shrink-0 opacity-70" />
              )}
              <span className="min-w-0 flex-1 truncate text-left">
                Workflows
              </span>
            </button>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                setNewName("");
                setNewDesc("");
                setCreateOpen(true);
              }}
              className="ml-1 flex size-6 shrink-0 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
              aria-label="New workflow"
              title="New workflow"
            >
              <Plus className="size-3.5" />
            </button>
          </div>
          {workflowsOpen && projectWorkflows.length > 0 && (
            <div className="mt-0.5 space-y-0.5 pl-4">
              {projectWorkflows.map((wf) => (
                <button
                  key={wf.id}
                  type="button"
                  onClick={() => void openWorkflow(wf.id)}
                  className="flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                >
                  <Workflow className="size-4 shrink-0" aria-hidden />
                  <span className="min-w-0 flex-1 truncate text-left">
                    {wf.name}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>

        {PRIMARY_ITEMS_AFTER_WORKFLOWS.map((item) => (
          <SidebarItem
            key={item.id}
            item={item}
            active={item.id === activeSection}
            onClick={() => onSectionChange(item.id)}
          />
        ))}

        <div className="pt-2">
          <button
            type="button"
            onClick={() => setOperationOpen((v) => !v)}
            className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-caption uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            aria-expanded={operationOpen}
          >
            {operationOpen ? (
              <ChevronDown className="size-3.5 shrink-0 opacity-70" />
            ) : (
              <ChevronRight className="size-3.5 shrink-0 opacity-70" />
            )}
            <span>Operation</span>
          </button>
          {operationOpen && (
            <div className="mt-1 space-y-0.5">
              {OPERATION_ITEMS.map((item) => (
                <SidebarItem
                  key={item.id}
                  item={item}
                  active={item.id === activeSection}
                  onClick={() => onSectionChange(item.id)}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogHeader onClose={() => setCreateOpen(false)}>
          New Workflow
        </DialogHeader>
        <DialogBody className="px-4 py-4">
          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="new-workflow-name">Name</Label>
              <Input
                id="new-workflow-name"
                autoFocus
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="My workflow"
                onKeyDown={(e) => {
                  if (e.key === "Enter" && newName.trim()) {
                    void createWorkflow(newName);
                    setCreateOpen(false);
                    setWorkflowsOpen(true);
                  }
                }}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="new-workflow-desc">Description</Label>
              <Textarea
                id="new-workflow-desc"
                value={newDesc}
                onChange={(e) => setNewDesc(e.target.value)}
                placeholder="What does this workflow do?"
                rows={3}
              />
            </div>
          </div>
          <div className="mt-6 flex justify-end">
            <Button
              size="sm"
              onClick={() => {
                void createWorkflow(newName);
                setCreateOpen(false);
                setWorkflowsOpen(true);
              }}
              disabled={!newName.trim()}
              className="cursor-pointer bg-[#E2E2E2] text-zinc-900 hover:bg-[#E2E2E2]/90"
            >
              Create Workflow
            </Button>
          </div>
        </DialogBody>
      </Dialog>
    </aside>
  );
}
