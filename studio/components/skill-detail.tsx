"use client";

import { useState } from "react";
import { Wrench } from "lucide-react";
import { useStudioStore } from "@/lib/store";
import {
  ComingSoonTab,
  DeleteButton,
  DeleteEntityDialog,
  DetailHeader,
  DetailShell,
  TabBar,
} from "@/components/detail-shell";

type Tab = "dashboard" | "configuration";

const TABS: { id: Tab; label: string }[] = [
  { id: "dashboard", label: "Dashboard" },
  { id: "configuration", label: "Configuration" },
];

export function SkillDetail() {
  const activeSkillId = useStudioStore((s) => s.activeSkillId);
  const skills = useStudioStore((s) => s.skills);
  const renameSkill = useStudioStore((s) => s.renameSkill);
  const updateSkillDescription = useStudioStore(
    (s) => s.updateSkillDescription
  );
  const deleteSkill = useStudioStore((s) => s.deleteSkill);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);

  const skill = skills.find((s) => s.id === activeSkillId);

  const [tab, setTab] = useState<Tab>("dashboard");
  const [deleteOpen, setDeleteOpen] = useState(false);

  if (!skill) return null;

  const confirmDelete = () => {
    void deleteSkill(skill.id);
    setDeleteOpen(false);
    goToDashboard();
  };

  return (
    <DetailShell>
      <DetailHeader
        icon={Wrench}
        name={skill.name}
        description={skill.description ?? ""}
        onRename={(n) => void renameSkill(skill.id, n)}
        onDescriptionCommit={(d) => void updateSkillDescription(skill.id, d)}
        actions={<DeleteButton onClick={() => setDeleteOpen(true)} />}
      />

      <TabBar tabs={TABS} value={tab} onChange={setTab} />

      <div className="mt-6">
        <ComingSoonTab />
      </div>

      <DeleteEntityDialog
        open={deleteOpen}
        entityLabel="Skill"
        name={skill.name}
        onConfirm={confirmDelete}
        onClose={() => setDeleteOpen(false)}
      />
    </DetailShell>
  );
}
