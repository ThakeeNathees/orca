"use client";

import { useState } from "react";
import { Workflow } from "lucide-react";
import { useStudioStore } from "@/lib/store";
import { Button } from "@/components/ui/button";
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

type Tab = "dashboard" | "configuration" | "runs" | "budgets";

const TABS: { id: Tab; label: string }[] = [
  { id: "dashboard", label: "Dashboard" },
  { id: "configuration", label: "Configuration" },
  { id: "runs", label: "Runs" },
  { id: "budgets", label: "Budgets" },
];

function DashboardTab() {
  return (
    <div className="space-y-6">
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

export function WorkflowDetail() {
  const activeWorkflowId = useStudioStore((s) => s.activeWorkflowId);
  const workflows = useStudioStore((s) => s.workflows);
  const renameWorkflow = useStudioStore((s) => s.renameWorkflow);
  const updateWorkflowDescription = useStudioStore(
    (s) => s.updateWorkflowDescription
  );
  const deleteWorkflow = useStudioStore((s) => s.deleteWorkflow);
  const openWorkflowEditor = useStudioStore((s) => s.openWorkflowEditor);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);

  const workflow = workflows.find((w) => w.id === activeWorkflowId);

  const [tab, setTab] = useState<Tab>("dashboard");
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (!workflow) return null;

  const confirmDelete = () => {
    void deleteWorkflow(workflow.id);
    setDeleteOpen(false);
    goToDashboard();
  };

  return (
    <DetailShell>
      <DetailHeader
        icon={Workflow}
        name={workflow.name}
        description={workflow.description ?? ""}
        onRename={(n) => void renameWorkflow(workflow.id, n)}
        onDescriptionCommit={(d) =>
          void updateWorkflowDescription(workflow.id, d)
        }
        actions={
          <>
            <Button
              onClick={() => void openWorkflowEditor(workflow.id)}
              size="sm"
              className="cursor-pointer bg-[#E2E2E2] text-zinc-900 hover:bg-[#E2E2E2]/90"
            >
              Edit Workflow
            </Button>
            <DeleteButton onClick={() => setDeleteOpen(true)} />
          </>
        }
      />

      <TabBar tabs={TABS} value={tab} onChange={setTab} />

      <div className="mt-6">
        {tab === "dashboard" ? <DashboardTab /> : <ComingSoonTab />}
      </div>

      <DeleteEntityDialog
        open={deleteOpen}
        entityLabel="Workflow"
        name={workflow.name}
        onConfirm={confirmDelete}
        onClose={() => setDeleteOpen(false)}
      />
    </DetailShell>
  );
}
