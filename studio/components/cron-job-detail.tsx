"use client";

import { useState } from "react";
import { Clock } from "lucide-react";
import { useStudioStore } from "@/lib/store";
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

type Tab = "dashboard" | "schedule" | "runs" | "budget";

const TABS: { id: Tab; label: string }[] = [
  { id: "dashboard", label: "Dashboard" },
  { id: "schedule", label: "Schedule" },
  { id: "runs", label: "Runs" },
  { id: "budget", label: "Budget" },
];

function DashboardTab() {
  return (
    <div className="space-y-6">
      <DashboardSection title="Summary">
        <div className="flex gap-3">
          <StatCard title="Last Run" value="No runs yet" />
          <StatCard title="Next Run" value="Not scheduled" />
        </div>
      </DashboardSection>
      <DashboardSection title="Costs">
        <CostRow />
      </DashboardSection>
    </div>
  );
}

export function CronJobDetail() {
  const activeCronJobId = useStudioStore((s) => s.activeCronJobId);
  const cronJobs = useStudioStore((s) => s.cronJobs);
  const renameCronJob = useStudioStore((s) => s.renameCronJob);
  const updateCronJobDescription = useStudioStore(
    (s) => s.updateCronJobDescription
  );
  const deleteCronJob = useStudioStore((s) => s.deleteCronJob);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);

  const cron = cronJobs.find((c) => c.id === activeCronJobId);

  const [tab, setTab] = useState<Tab>("dashboard");
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (!cron) return null;

  const confirmDelete = () => {
    void deleteCronJob(cron.id);
    setDeleteOpen(false);
    goToDashboard();
  };

  return (
    <DetailShell>
      <DetailHeader
        icon={Clock}
        name={cron.name}
        description={cron.description ?? ""}
        onRename={(n) => void renameCronJob(cron.id, n)}
        onDescriptionCommit={(d) =>
          void updateCronJobDescription(cron.id, d)
        }
        actions={<DeleteButton onClick={() => setDeleteOpen(true)} />}
      />

      <TabBar tabs={TABS} value={tab} onChange={setTab} />

      <div className="mt-6">
        {tab === "dashboard" ? <DashboardTab /> : <ComingSoonTab />}
      </div>

      <DeleteEntityDialog
        open={deleteOpen}
        entityLabel="Cron Job"
        name={cron.name}
        onConfirm={confirmDelete}
        onClose={() => setDeleteOpen(false)}
      />
    </DetailShell>
  );
}
