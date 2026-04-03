"use client";

import { ReactFlowProvider } from "@xyflow/react";
import { TopBar } from "@/components/top-bar";
import { Palette } from "@/components/palette";
import { Canvas } from "@/components/canvas";
import { Inspector } from "@/components/inspector";

export default function Home() {
  return (
    <ReactFlowProvider>
      <div className="flex h-full flex-col">
        <TopBar />
        <div className="flex flex-1 overflow-hidden">
          <Palette />
          <Canvas />
          <Inspector />
        </div>
      </div>
    </ReactFlowProvider>
  );
}
