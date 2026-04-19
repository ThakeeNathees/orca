"use client";

import dynamic from "next/dynamic";
import { useEffect, useMemo, useState } from "react";
import { ReactFlowProvider } from "@xyflow/react";
import { TopBar } from "@/components/top-bar";
import { Palette } from "@/components/palette";
import { Canvas } from "@/components/canvas";
import { Inspector } from "@/components/inspector";
import { NavSidebar } from "@/components/nav-sidebar";
import { ProjectSidebar } from "@/components/project-sidebar";
import { WorkflowDetail } from "@/components/workflow-detail";
import { ModelDetail } from "@/components/model-detail";
import { SkillDetail } from "@/components/skill-detail";
import { AgentDetail } from "@/components/agent-detail";
import { CronJobDetail } from "@/components/cron-job-detail";
import { OrcaPage } from "@/components/orca-page";
import { InboxPage } from "@/components/inbox-page";
import { CommandPalette } from "@/components/command-palette";
import {
  ViewModeToggle,
  type StudioViewMode,
} from "@/components/view-mode-toggle";
import { ErrorBoundary } from "@/components/error-boundary";
import { useStudioStore } from "@/lib/store";
import { generateOrcaSource } from "@/lib/orca-gen";
import {
  SECTION_LABELS,
  SECTION_PARENT,
} from "@/lib/sidebar-sections";

const StudioCodeEditor = dynamic(
  () =>
    import("@/components/studio-code-editor").then((m) => m.StudioCodeEditor),
  {
    ssr: false,
    loading: () => (
      <div className="flex min-h-0 min-w-0 flex-1 flex-col items-center justify-center border-x border-border bg-card text-sm text-muted-foreground">
        Loading editor…
      </div>
    ),
  }
);

function WorkflowEditor() {
  const [viewMode, setViewMode] = useState<StudioViewMode>("ui");
  const nodes = useStudioStore((s) => s.nodes);
  const edges = useStudioStore((s) => s.edges);

  const sourceCode = useMemo(
    () => generateOrcaSource(nodes, edges),
    [nodes, edges]
  );

  return (
    <div className="flex h-full flex-col">
      <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="pointer-events-none absolute left-1/2 top-[16px] z-50 -translate-x-1/2">
          <div className="pointer-events-auto">
            <ViewModeToggle mode={viewMode} onModeChange={setViewMode} />
          </div>
        </div>
        <div className="flex min-h-0 flex-1 overflow-hidden">
          <ErrorBoundary>
            <Palette />
          </ErrorBoundary>
          <div
            className="relative flex min-h-0 min-w-0 flex-1 flex-col"
            id="studio-panel-center"
            role="tabpanel"
            aria-labelledby={
              viewMode === "ui" ? "studio-tab-ui" : "studio-tab-code"
            }
          >
            {viewMode === "ui" ? (
              <ErrorBoundary>
                <Canvas />
              </ErrorBoundary>
            ) : (
              <ErrorBoundary>
                <StudioCodeEditor value={sourceCode} />
              </ErrorBoundary>
            )}
          </div>
          {viewMode === "ui" ? (
            <ErrorBoundary>
              <Inspector />
            </ErrorBoundary>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function ComingSoon({ label }: { label: string }) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center bg-sidebar text-muted-foreground">
      <p className="text-base font-medium text-foreground">{label}</p>
      <p className="mt-1 text-sm">TODO</p>
    </div>
  );
}

/** Picks the right detail component based on which entity is active. */
function DetailView() {
  const activeWorkflowId = useStudioStore((s) => s.activeWorkflowId);
  const activeModelId = useStudioStore((s) => s.activeModelId);
  const activeSkillId = useStudioStore((s) => s.activeSkillId);
  const activeAgentId = useStudioStore((s) => s.activeAgentId);
  const activeCronJobId = useStudioStore((s) => s.activeCronJobId);

  if (activeWorkflowId) return <WorkflowDetail />;
  if (activeModelId) return <ModelDetail />;
  if (activeSkillId) return <SkillDetail />;
  if (activeAgentId) return <AgentDetail />;
  if (activeCronJobId) return <CronJobDetail />;
  return <ComingSoon label="Detail" />;
}

export default function Home() {
  const currentView = useStudioStore((s) => s.currentView);
  const sidebarSection = useStudioStore((s) => s.sidebarSection);
  const hydrated = useStudioStore((s) => s.hydrated);
  const hydrate = useStudioStore((s) => s.hydrate);

  useEffect(() => {
    void hydrate();
  }, [hydrate]);

  if (!hydrated) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        Loading studio…
      </div>
    );
  }

  return (
    <ReactFlowProvider>
      <CommandPalette />
      <div className="flex h-full">
        <NavSidebar />
        {currentView !== "editor" && (
          <ErrorBoundary>
            <ProjectSidebar />
          </ErrorBoundary>
        )}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <TopBar />
          {currentView === "dashboard" ? (
            sidebarSection === "orca" ? (
              <ErrorBoundary>
                <OrcaPage />
              </ErrorBoundary>
            ) : sidebarSection === "inbox" ? (
              <ErrorBoundary>
                <InboxPage />
              </ErrorBoundary>
            ) : // Top-level entity groups (Models/Skills/Agents/Workflows/
            // Cron Jobs) have no landing page — the main area stays empty
            // until the user drills into an item. Leaf sections keep the
            // "coming soon" placeholder.
            SECTION_PARENT[sidebarSection] === null ? (
              <div className="flex-1 bg-sidebar" />
            ) : (
              <ComingSoon label={SECTION_LABELS[sidebarSection]} />
            )
          ) : currentView === "detail" ? (
            <ErrorBoundary>
              <DetailView />
            </ErrorBoundary>
          ) : (
            <WorkflowEditor />
          )}
        </div>
      </div>
    </ReactFlowProvider>
  );
}
