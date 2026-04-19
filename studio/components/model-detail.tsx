"use client";

import { useState } from "react";
import { Brain } from "lucide-react";
import { useStudioStore } from "@/lib/store";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
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
import type { ModelProvider } from "@/lib/types";
import { PROVIDER_LABELS, PROVIDER_MODELS } from "@/lib/model-providers";

type Tab = "dashboard" | "configuration" | "usage" | "budget";

const TABS: { id: Tab; label: string }[] = [
  { id: "dashboard", label: "Dashboard" },
  { id: "configuration", label: "Configuration" },
  { id: "usage", label: "Usage" },
  { id: "budget", label: "Budget" },
];


function DashboardTab() {
  return (
    <div className="space-y-6">
      <DashboardSection title="Summary">
        <div className="flex gap-3">
          <StatCard title="Invocations" value="No usage yet" />
          <StatCard title="Tokens" value="No usage yet" />
        </div>
      </DashboardSection>
      <DashboardSection title="Costs">
        <CostRow />
      </DashboardSection>
    </div>
  );
}

function ConfigurationTab({
  provider,
  modelName,
  onChange,
}: {
  provider: ModelProvider;
  modelName: string;
  onChange: (provider: ModelProvider, modelName: string) => void;
}) {
  return (
    <div className="max-w-md space-y-4">
      <div className="space-y-1.5">
        <Label>Provider</Label>
        <Select
          value={provider}
          onValueChange={(v) => {
            if (!v) return;
            const p = v as ModelProvider;
            onChange(p, PROVIDER_MODELS[p][0]);
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
          onValueChange={(v) => v && onChange(provider, v)}
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
  );
}

export function ModelDetail() {
  const activeModelId = useStudioStore((s) => s.activeModelId);
  const models = useStudioStore((s) => s.models);
  const renameModel = useStudioStore((s) => s.renameModel);
  const updateModelDescription = useStudioStore(
    (s) => s.updateModelDescription
  );
  const updateModelConfig = useStudioStore((s) => s.updateModelConfig);
  const deleteModel = useStudioStore((s) => s.deleteModel);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);

  const model = models.find((m) => m.id === activeModelId);

  const [tab, setTab] = useState<Tab>("dashboard");
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (!model) return null;

  const confirmDelete = () => {
    void deleteModel(model.id);
    setDeleteOpen(false);
    goToDashboard();
  };

  return (
    <DetailShell>
      <DetailHeader
        icon={Brain}
        name={model.name}
        description={model.description ?? ""}
        onRename={(n) => void renameModel(model.id, n)}
        onDescriptionCommit={(d) => void updateModelDescription(model.id, d)}
        actions={<DeleteButton onClick={() => setDeleteOpen(true)} />}
      />

      <TabBar tabs={TABS} value={tab} onChange={setTab} />

      <div className="mt-6">
        {tab === "dashboard" && <DashboardTab />}
        {tab === "configuration" && (
          <ConfigurationTab
            provider={model.provider}
            modelName={model.modelName}
            onChange={(p, m) => void updateModelConfig(model.id, p, m)}
          />
        )}
        {(tab === "usage" || tab === "budget") && <ComingSoonTab />}
      </div>

      <DeleteEntityDialog
        open={deleteOpen}
        entityLabel="Model"
        name={model.name}
        onConfirm={confirmDelete}
        onClose={() => setDeleteOpen(false)}
      />
    </DetailShell>
  );
}
