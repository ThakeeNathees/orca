"use client";

import dynamic from "next/dynamic";
import { useState } from "react";
import { ReactFlowProvider } from "@xyflow/react";
import { TopBar } from "@/components/top-bar";
import { Palette } from "@/components/palette";
import { Canvas } from "@/components/canvas";
import { Inspector } from "@/components/inspector";
import { NavSidebar } from "@/components/nav-sidebar";
import { ProjectSidebar } from "@/components/project-sidebar";
import { Dashboard } from "@/components/dashboard";
import {
  ViewModeToggle,
  type StudioViewMode,
} from "@/components/view-mode-toggle";
import { SAMPLE_ORCA_SOURCE } from "@/lib/sample-oc";
import { useStudioStore } from "@/lib/store";

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
  const [sourceCode, setSourceCode] = useState(SAMPLE_ORCA_SOURCE);

  return (
    <div className="flex h-full flex-col">
      <div className="relative flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="pointer-events-none absolute left-1/2 top-[16px] z-50 -translate-x-1/2">
          <div className="pointer-events-auto">
            <ViewModeToggle mode={viewMode} onModeChange={setViewMode} />
          </div>
        </div>
        <div className="flex min-h-0 flex-1 overflow-hidden">
          <Palette />
          <div
            className="relative flex min-h-0 min-w-0 flex-1 flex-col"
            id="studio-panel-center"
            role="tabpanel"
            aria-labelledby={
              viewMode === "ui" ? "studio-tab-ui" : "studio-tab-code"
            }
          >
            {viewMode === "ui" ? (
              <Canvas />
            ) : (
              <StudioCodeEditor value={sourceCode} onChange={setSourceCode} />
            )}
          </div>
          {viewMode === "ui" ? <Inspector /> : null}
        </div>
      </div>
    </div>
  );
}

export default function Home() {
  const currentView = useStudioStore((s) => s.currentView);

  return (
    <ReactFlowProvider>
      <div className="flex h-full">
        <NavSidebar />
        {currentView === "dashboard" && <ProjectSidebar />}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <TopBar />
          {currentView === "dashboard" ? <Dashboard /> : <WorkflowEditor />}
        </div>
      </div>
    </ReactFlowProvider>
  );
}
