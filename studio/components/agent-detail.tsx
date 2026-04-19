"use client";

import { useEffect, useState } from "react";
import { Bot } from "lucide-react";
import { useStudioStore } from "@/lib/store";
import { Textarea } from "@/components/ui/textarea";
import {
  ComingSoonTab,
  DeleteButton,
  DeleteEntityDialog,
  DetailHeader,
  DetailShell,
  TabBar,
} from "@/components/detail-shell";
import {
  CostRow,
  DashboardSection,
  StatCard,
} from "@/components/detail-widgets";
import { ModelSelect } from "@/components/model-select";
import type { ModelSummary } from "@/lib/types";

type Tab =
  | "dashboard"
  | "persona"
  | "skills"
  | "configuration"
  | "runs"
  | "budget";

const TABS: { id: Tab; label: string }[] = [
  { id: "dashboard", label: "Dashboard" },
  { id: "persona", label: "Persona" },
  { id: "skills", label: "Skills" },
  { id: "configuration", label: "Configuration" },
  { id: "runs", label: "Runs" },
  { id: "budget", label: "Budget" },
];

function DashboardTab({
  modelId,
  fallbackModelId,
  models,
  onModelsChange,
}: {
  modelId: string;
  fallbackModelId: string;
  models: ModelSummary[];
  onModelsChange: (modelId: string, fallbackModelId: string) => void;
}) {
  return (
    <div className="space-y-6">
      <div className="flex gap-4">
        <ModelSelect
          label="Model"
          value={modelId}
          models={models}
          placeholder="Select a model"
          onChange={(id) => onModelsChange(id, fallbackModelId)}
        />
        <ModelSelect
          label="Fallback Model"
          value={fallbackModelId}
          models={models}
          placeholder="Select a fallback model"
          onChange={(id) => onModelsChange(modelId, id)}
        />
      </div>
      <DashboardSection title="Summary">
        <div className="flex gap-3">
          <StatCard title="Run Activity" value="No runs yet" />
          <StatCard title="Success Rate" value="No runs yet" />
        </div>
      </DashboardSection>
      <DashboardSection title="Costs">
        <CostRow />
      </DashboardSection>
    </div>
  );
}

/** Persona editor — `Persona.md` card with a single textarea. */
function PersonaTab({
  persona,
  onCommit,
}: {
  persona: string;
  onCommit: (next: string) => void;
}) {
  const [draft, setDraft] = useState(persona);
  useEffect(() => {
    setDraft(persona);
  }, [persona]);

  return (
    <div className="rounded-lg border border-border bg-card">
      <div className="border-b border-border px-4 py-2 text-sm font-medium text-foreground">
        Persona.md
      </div>
      <Textarea
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={() => {
          if (draft !== persona) onCommit(draft);
        }}
        placeholder="You're a helpful assistant..."
        rows={10}
        className="min-h-[200px] rounded-none border-0 bg-transparent font-mono focus-visible:ring-0"
      />
    </div>
  );
}

export function AgentDetail() {
  const activeAgentId = useStudioStore((s) => s.activeAgentId);
  const agents = useStudioStore((s) => s.agents);
  const renameAgent = useStudioStore((s) => s.renameAgent);
  const updateAgentDescription = useStudioStore(
    (s) => s.updateAgentDescription
  );
  const updateAgentPersona = useStudioStore((s) => s.updateAgentPersona);
  const updateAgentModels = useStudioStore((s) => s.updateAgentModels);
  const allModels = useStudioStore((s) => s.models);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const deleteAgent = useStudioStore((s) => s.deleteAgent);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);

  const agent = agents.find((a) => a.id === activeAgentId);
  const projectModels = allModels.filter(
    (m) => m.projectId === activeProjectId
  );

  const [tab, setTab] = useState<Tab>("dashboard");
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (!agent) return null;

  const confirmDelete = () => {
    void deleteAgent(agent.id);
    setDeleteOpen(false);
    goToDashboard();
  };

  return (
    <DetailShell>
      <DetailHeader
        icon={Bot}
        name={agent.name}
        description={agent.description ?? ""}
        onRename={(n) => void renameAgent(agent.id, n)}
        onDescriptionCommit={(d) => void updateAgentDescription(agent.id, d)}
        actions={<DeleteButton onClick={() => setDeleteOpen(true)} />}
      />

      <TabBar tabs={TABS} value={tab} onChange={setTab} />

      <div className="mt-6">
        {tab === "dashboard" && (
          <DashboardTab
            modelId={agent.modelId ?? ""}
            fallbackModelId={agent.fallbackModelId ?? ""}
            models={projectModels}
            onModelsChange={(m, f) =>
              void updateAgentModels(agent.id, m, f)
            }
          />
        )}
        {tab === "persona" && (
          <PersonaTab
            persona={agent.persona ?? ""}
            onCommit={(p) => void updateAgentPersona(agent.id, p)}
          />
        )}
        {tab !== "dashboard" && tab !== "persona" && <ComingSoonTab />}
      </div>

      <DeleteEntityDialog
        open={deleteOpen}
        entityLabel="Agent"
        name={agent.name}
        onConfirm={confirmDelete}
        onClose={() => setDeleteOpen(false)}
      />
    </DetailShell>
  );
}
