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
import { Dashboard } from "@/components/dashboard";
import {
  ViewModeToggle,
  type StudioViewMode,
} from "@/components/view-mode-toggle";
import { ErrorBoundary } from "@/components/error-boundary";
import { useStudioStore } from "@/lib/store";
import { generateOrcaSource } from "@/lib/orca-gen";

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

  // Derive `.orca` source from the current graph on every change. The
  // generator is pure and cheap for typical graph sizes, so memoising on
  // the node/edge refs is enough — Zustand hands back stable references
  // when nothing changed, so this recomputes only on real edits.
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

export default function Home() {
  const currentView = useStudioStore((s) => s.currentView);
  const hydrated = useStudioStore((s) => s.hydrated);
  const hydrate = useStudioStore((s) => s.hydrate);

  // Kick off storage hydration once on mount. Runs on the client only
  // (this is a client component), so IndexedDB is guaranteed available.
  useEffect(() => {
    void hydrate();
  }, [hydrate]);

  // Avoid rendering dashboard/editor with empty placeholder state before
  // the adapter has loaded. This also sidesteps SSR hydration mismatches
  // since the server renders an empty shell and the client fills it in.
  if (!hydrated) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        Loading studio…
      </div>
    );
  }

  return (
    <ReactFlowProvider>
      <div className="flex h-full">
        <NavSidebar />
        {currentView === "dashboard" && (
          <ErrorBoundary>
            <ProjectSidebar />
          </ErrorBoundary>
        )}
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
          <TopBar />
          {currentView === "dashboard" ? (
            <ErrorBoundary>
              <Dashboard />
            </ErrorBoundary>
          ) : (
            <WorkflowEditor />
          )}
        </div>
      </div>
    </ReactFlowProvider>
  );
}
