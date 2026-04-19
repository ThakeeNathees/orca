"use client";

import { useState, type ReactNode } from "react";
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
  MessageCircle,
  Inbox,
  Plus,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useStudioStore } from "@/lib/store";
import { ENTITY_ICON_FG } from "@/lib/entity-icon";
import { Dialog, DialogHeader, DialogBody } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ModelProvider } from "@/lib/types";
import type { SidebarSection } from "@/lib/sidebar-sections";
import { PROVIDER_LABELS, PROVIDER_MODELS } from "@/lib/model-providers";
import { ModelSelect } from "@/components/model-select";

type NavItem = { id: SidebarSection; label: string; icon: LucideIcon };

const OPERATION_ITEMS: NavItem[] = [
  { id: "cost", label: "Cost", icon: DollarSign },
  { id: "runs", label: "Runs", icon: Activity },
  { id: "settings", label: "Settings", icon: Settings },
];

/* ────────────────────────────────────────────────────────────────────── */

function SidebarItem({
  item,
  active,
  onClick,
  badge,
}: {
  item: NavItem;
  active: boolean;
  onClick: () => void;
  badge?: number;
}) {
  const Icon = item.icon;
  return (
    <button
      type="button"
      onClick={onClick}
      style={{ color: ENTITY_ICON_FG }}
      className={cn(
        "flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors cursor-pointer hover:text-foreground",
        active && "bg-sidebar-accent text-foreground"
      )}
    >
      <Icon className="size-3.5 shrink-0" />
      <span className="min-w-0 flex-1 truncate text-left">{item.label}</span>
      {badge != null && badge > 0 && (
        <span className="flex min-w-[22px] shrink-0 items-center justify-center rounded-full bg-accent-violet px-1.5 py-0.5 text-xs font-semibold leading-none text-white">
          {badge}
        </span>
      )}
    </button>
  );
}

/** Collapsible header with chevron + label + optional + button. */
function GroupHeader({
  label,
  open,
  onToggle,
  onAdd,
}: {
  label: string;
  open: boolean;
  onToggle: () => void;
  onAdd?: () => void;
}) {
  return (
    <div className="group flex w-full items-center rounded-md pr-1 text-sm text-muted-foreground transition-colors hover:text-foreground">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        className="flex min-w-0 flex-1 items-center gap-2 rounded-md px-3 py-2 cursor-pointer"
      >
        {open ? (
          <ChevronDown className="size-3.5 shrink-0 opacity-70" />
        ) : (
          <ChevronRight className="size-3.5 shrink-0 opacity-70" />
        )}
        <span className="min-w-0 flex-1 truncate text-left">{label}</span>
      </button>
      {onAdd && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onAdd();
          }}
          className="ml-1 flex size-6 shrink-0 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
          aria-label={`New ${label}`}
          title={`New ${label}`}
        >
          <Plus className="size-3.5" />
        </button>
      )}
    </div>
  );
}

/** Indented entity row inside a collapsible group. Rows render brighter
 *  than the muted group header so the actual items stand out. */
function EntityRow({
  label,
  icon: Icon,
  active,
  onClick,
}: {
  label: string;
  icon: LucideIcon;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={{ color: ENTITY_ICON_FG }}
      className={cn(
        "flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors cursor-pointer hover:text-foreground",
        active && "bg-sidebar-accent text-foreground"
      )}
    >
      <Icon className="size-4 shrink-0" aria-hidden />
      <span className="min-w-0 flex-1 truncate text-left">{label}</span>
    </button>
  );
}

/* ── Create dialogs ──────────────────────────────────────────────────── */

function NameDescDialog({
  open,
  title,
  onClose,
  onSubmit,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  onSubmit: (name: string, desc: string) => void;
}) {
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");

  const handleClose = () => {
    setName("");
    setDesc("");
    onClose();
  };
  const submit = () => {
    if (!name.trim()) return;
    onSubmit(name, desc);
    handleClose();
  };

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : handleClose())}>
      <DialogHeader onClose={handleClose}>{title}</DialogHeader>
      <DialogBody className="px-4 py-4">
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="entity-name">Name</Label>
            <Input
              id="entity-name"
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && name.trim()) submit();
              }}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="entity-desc">Description</Label>
            <Textarea
              id="entity-desc"
              value={desc}
              onChange={(e) => setDesc(e.target.value)}
              rows={3}
            />
          </div>
        </div>
        <div className="mt-6 flex justify-end">
          <Button
            size="sm"
            onClick={submit}
            disabled={!name.trim()}
            className="cursor-pointer bg-[#E2E2E2] text-zinc-900 hover:bg-[#E2E2E2]/90"
          >
            Create
          </Button>
        </div>
      </DialogBody>
    </Dialog>
  );
}


function NewAgentDialog({
  open,
  onClose,
  onSubmit,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (
    name: string,
    desc: string,
    modelId: string,
    fallbackModelId: string
  ) => void;
}) {
  const models = useStudioStore((s) => s.models);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const projectModels = models.filter((m) => m.projectId === activeProjectId);

  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");
  const [modelId, setModelId] = useState("");
  const [fallbackModelId, setFallbackModelId] = useState("");

  const handleClose = () => {
    setName("");
    setDesc("");
    setModelId("");
    setFallbackModelId("");
    onClose();
  };
  const submit = () => {
    if (!name.trim()) return;
    onSubmit(name, desc, modelId, fallbackModelId);
    handleClose();
  };

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : handleClose())}>
      <DialogHeader onClose={handleClose}>New Agent</DialogHeader>
      <DialogBody className="px-4 py-4">
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="agent-name">Name</Label>
            <Input
              id="agent-name"
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && name.trim()) submit();
              }}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="agent-desc">Description</Label>
            <Textarea
              id="agent-desc"
              value={desc}
              onChange={(e) => setDesc(e.target.value)}
              rows={2}
            />
          </div>
          <ModelSelect
            label="Model"
            value={modelId}
            models={projectModels}
            placeholder="Select a model"
            onChange={setModelId}
          />
          <ModelSelect
            label="Fallback Model"
            value={fallbackModelId}
            models={projectModels}
            placeholder="Select a fallback model"
            onChange={setFallbackModelId}
          />
        </div>
        <div className="mt-6 flex justify-end">
          <Button
            size="sm"
            onClick={submit}
            disabled={!name.trim()}
            className="cursor-pointer bg-[#E2E2E2] text-zinc-900 hover:bg-[#E2E2E2]/90"
          >
            Create Agent
          </Button>
        </div>
      </DialogBody>
    </Dialog>
  );
}

function NewModelDialog({
  open,
  onClose,
  onSubmit,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (
    name: string,
    desc: string,
    provider: ModelProvider,
    modelName: string
  ) => void;
}) {
  const [name, setName] = useState("");
  const [desc, setDesc] = useState("");
  const [provider, setProvider] = useState<ModelProvider>("openai");
  const [modelName, setModelName] = useState(PROVIDER_MODELS.openai[0]);

  const handleClose = () => {
    setName("");
    setDesc("");
    setProvider("openai");
    setModelName(PROVIDER_MODELS.openai[0]);
    onClose();
  };

  const submit = () => {
    if (!name.trim()) return;
    onSubmit(name, desc, provider, modelName);
    handleClose();
  };

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : handleClose())}>
      <DialogHeader onClose={handleClose}>New Model</DialogHeader>
      <DialogBody className="px-4 py-4">
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="model-name">Name</Label>
            <Input
              id="model-name"
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="model-desc">Description</Label>
            <Textarea
              id="model-desc"
              value={desc}
              onChange={(e) => setDesc(e.target.value)}
              rows={2}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Provider</Label>
            <Select
              value={provider}
              onValueChange={(v) => {
                const p = v as ModelProvider;
                setProvider(p);
                setModelName(PROVIDER_MODELS[p][0]);
              }}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(PROVIDER_LABELS) as ModelProvider[]).map((p) => (
                  <SelectItem key={p} value={p}>
                    {PROVIDER_LABELS[p]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label>Model Name</Label>
            <Select
              value={modelName}
              onValueChange={(v) => v && setModelName(v)}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROVIDER_MODELS[provider].map((m) => (
                  <SelectItem key={m} value={m}>
                    {m}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <div className="mt-6 flex justify-end">
          <Button
            size="sm"
            onClick={submit}
            disabled={!name.trim()}
            className="cursor-pointer bg-[#E2E2E2] text-zinc-900 hover:bg-[#E2E2E2]/90"
          >
            Create Model
          </Button>
        </div>
      </DialogBody>
    </Dialog>
  );
}

/* ── Collapsible group wrapper ───────────────────────────────────────── */

function CollapsibleGroup({
  label,
  defaultOpen = true,
  onAdd,
  children,
}: {
  label: string;
  defaultOpen?: boolean;
  onAdd?: () => void;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div>
      <GroupHeader
        label={label}
        open={open}
        onToggle={() => setOpen((v) => !v)}
        onAdd={onAdd}
      />
      {open && <div className="mt-0.5 space-y-0.5 pl-4">{children}</div>}
    </div>
  );
}

/* ── Main sidebar ────────────────────────────────────────────────────── */

export function ProjectSidebar() {
  const [operationsOpen, setOperationsOpen] = useState(true);
  const [modelDialog, setModelDialog] = useState(false);
  const [skillDialog, setSkillDialog] = useState(false);
  const [agentDialog, setAgentDialog] = useState(false);
  const [workflowDialog, setWorkflowDialog] = useState(false);
  const [cronDialog, setCronDialog] = useState(false);

  const activeSection = useStudioStore((s) => s.sidebarSection);
  const onSectionChange = useStudioStore((s) => s.setSidebarSection);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const workflows = useStudioStore((s) => s.workflows);
  const models = useStudioStore((s) => s.models);
  const skills = useStudioStore((s) => s.skills);
  const agents = useStudioStore((s) => s.agents);
  const cronJobs = useStudioStore((s) => s.cronJobs);
  const activeWorkflowId = useStudioStore((s) => s.activeWorkflowId);
  const activeModelId = useStudioStore((s) => s.activeModelId);
  const activeSkillId = useStudioStore((s) => s.activeSkillId);
  const activeAgentId = useStudioStore((s) => s.activeAgentId);
  const activeCronJobId = useStudioStore((s) => s.activeCronJobId);

  const createWorkflow = useStudioStore((s) => s.createWorkflow);
  const openWorkflow = useStudioStore((s) => s.openWorkflow);
  const createModel = useStudioStore((s) => s.createModel);
  const openModel = useStudioStore((s) => s.openModel);
  const createSkill = useStudioStore((s) => s.createSkill);
  const openSkill = useStudioStore((s) => s.openSkill);
  const createAgent = useStudioStore((s) => s.createAgent);
  const openAgent = useStudioStore((s) => s.openAgent);
  const createCronJob = useStudioStore((s) => s.createCronJob);
  const inboxUnread = useStudioStore(
    (s) => s.inboxMessages.filter((m) => m.unread).length
  );
  const openCronJob = useStudioStore((s) => s.openCronJob);

  const projectWorkflows = workflows.filter(
    (w) => w.projectId === activeProjectId
  );
  const projectModels = models.filter((m) => m.projectId === activeProjectId);
  const projectSkills = skills.filter((s) => s.projectId === activeProjectId);
  const projectAgents = agents.filter((a) => a.projectId === activeProjectId);
  const projectCronJobs = cronJobs.filter(
    (c) => c.projectId === activeProjectId
  );

  return (
    <aside className="flex w-[220px] shrink-0 flex-col border-r border-border bg-sidebar">
      <div className="flex-1 overflow-y-auto px-2 py-3 space-y-0.5">
        <SidebarItem
          item={{ id: "orca", label: "Orca", icon: MessageCircle }}
          active={activeSection === "orca"}
          onClick={() => onSectionChange("orca")}
        />
        <SidebarItem
          item={{ id: "inbox", label: "Inbox", icon: Inbox }}
          active={activeSection === "inbox"}
          onClick={() => onSectionChange("inbox")}
          badge={inboxUnread}
        />

        <CollapsibleGroup label="Models" onAdd={() => setModelDialog(true)}>
          {projectModels.map((m) => (
            <EntityRow
              key={m.id}
              label={m.name}
              icon={Brain}
              active={m.id === activeModelId}
              onClick={() => openModel(m.id)}
            />
          ))}
        </CollapsibleGroup>

        <CollapsibleGroup label="Skills" onAdd={() => setSkillDialog(true)}>
          {projectSkills.map((s) => (
            <EntityRow
              key={s.id}
              label={s.name}
              icon={Wrench}
              active={s.id === activeSkillId}
              onClick={() => openSkill(s.id)}
            />
          ))}
        </CollapsibleGroup>

        <CollapsibleGroup label="Agents" onAdd={() => setAgentDialog(true)}>
          {projectAgents.map((a) => (
            <EntityRow
              key={a.id}
              label={a.name}
              icon={Bot}
              active={a.id === activeAgentId}
              onClick={() => openAgent(a.id)}
            />
          ))}
        </CollapsibleGroup>

        <CollapsibleGroup
          label="Workflows"
          onAdd={() => setWorkflowDialog(true)}
        >
          {projectWorkflows.map((wf) => (
            <EntityRow
              key={wf.id}
              label={wf.name}
              icon={Workflow}
              active={wf.id === activeWorkflowId}
              onClick={() => openWorkflow(wf.id)}
            />
          ))}
        </CollapsibleGroup>

        <CollapsibleGroup label="Cron Jobs" onAdd={() => setCronDialog(true)}>
          {projectCronJobs.map((c) => (
            <EntityRow
              key={c.id}
              label={c.name}
              icon={Clock}
              active={c.id === activeCronJobId}
              onClick={() => openCronJob(c.id)}
            />
          ))}
        </CollapsibleGroup>

        <div className="pt-2">
          <button
            type="button"
            onClick={() => setOperationsOpen((v) => !v)}
            aria-expanded={operationsOpen}
            className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
          >
            {operationsOpen ? (
              <ChevronDown className="size-3.5 shrink-0 opacity-70" />
            ) : (
              <ChevronRight className="size-3.5 shrink-0 opacity-70" />
            )}
            <span>Operations</span>
          </button>
          {operationsOpen && (
            <div className="mt-1 space-y-0.5 pl-4">
              {OPERATION_ITEMS.map((item) => (
                <EntityRow
                  key={item.id}
                  label={item.label}
                  icon={item.icon}
                  active={item.id === activeSection}
                  onClick={() => onSectionChange(item.id)}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      <NewModelDialog
        open={modelDialog}
        onClose={() => setModelDialog(false)}
        onSubmit={(n, d, p, m) => void createModel(n, p, m, d)}
      />
      <NameDescDialog
        open={skillDialog}
        title="New Skill"
        onClose={() => setSkillDialog(false)}
        onSubmit={(n, d) => void createSkill(n, d)}
      />
      <NewAgentDialog
        open={agentDialog}
        onClose={() => setAgentDialog(false)}
        onSubmit={(n, d, m, f) => void createAgent(n, d, m, f)}
      />
      <NameDescDialog
        open={workflowDialog}
        title="New Workflow"
        onClose={() => setWorkflowDialog(false)}
        onSubmit={(n, d) => void createWorkflow(n, d)}
      />
      <NameDescDialog
        open={cronDialog}
        title="New Cron Job"
        onClose={() => setCronDialog(false)}
        onSubmit={(n, d) => void createCronJob(n, d)}
      />
    </aside>
  );
}
