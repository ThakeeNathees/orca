"use client";

import dynamic from "next/dynamic";
import { useState } from "react";
import { ReactFlowProvider } from "@xyflow/react";
import { TopBar } from "@/components/top-bar";
import { Palette } from "@/components/palette";
import { Canvas } from "@/components/canvas";
import { Inspector } from "@/components/inspector";
import {
  ViewModeToggle,
  type StudioViewMode,
} from "@/components/view-mode-toggle";
import { SAMPLE_ORCA_SOURCE } from "@/lib/sample-oc";

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

export default function Home() {
  const [viewMode, setViewMode] = useState<StudioViewMode>("ui");
  const [sourceCode, setSourceCode] = useState(SAMPLE_ORCA_SOURCE);

  return (
    <ReactFlowProvider>
      <div className="flex h-full flex-col">
        <TopBar />
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
    </ReactFlowProvider>
  );
}
