"use client";

import { useCallback, useRef, useState } from "react";
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  Controls,
  ConnectionMode,
  useReactFlow,
  type IsValidConnection,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Maximize2, Plus } from "lucide-react";

import { useStudioStore } from "@/lib/store";
import { BLOCK_DEFS, canConnect } from "@/lib/block-defs";
import type { BlockKind } from "@/lib/types";
import { nodeTypes } from "@/components/nodes";
import { StudioEdge } from "@/components/studio-edge";
import { BlockPickerMenu } from "@/components/block-picker-menu";
import { CANVAS_DOT_COLOR, CANVAS_EDGE_STROKE } from "@/lib/canvas-colors";

const edgeTypes = { default: StudioEdge };

type PickerState = {
  /** Where on screen to anchor the menu. */
  anchor: { x: number; y: number };
  /** Flow-space position to spawn the picked node at. */
  spawnAt: { x: number; y: number };
};

export function Canvas() {
  const reactFlowWrapper = useRef<HTMLDivElement>(null);
  const addButtonRef = useRef<HTMLButtonElement>(null);
  const { screenToFlowPosition, fitView } = useReactFlow();

  const nodes = useStudioStore((s) => s.nodes);
  const edges = useStudioStore((s) => s.edges);
  const onNodesChange = useStudioStore((s) => s.onNodesChange);
  const onEdgesChange = useStudioStore((s) => s.onEdgesChange);
  const onConnectHandler = useStudioStore((s) => s.onConnect);
  const addNode = useStudioStore((s) => s.addNode);
  const setSelectedNodeId = useStudioStore((s) => s.setSelectedNodeId);

  const [picker, setPicker] = useState<PickerState | null>(null);

  const isValidConnection: IsValidConnection = useCallback((connection) => {
    const nodes = useStudioStore.getState().nodes;
    const sourceNode = nodes.find((n) => n.id === connection.source);
    const targetNode = nodes.find((n) => n.id === connection.target);
    if (!sourceNode || !targetNode) return false;

    const sourceDef = BLOCK_DEFS[sourceNode.data.kind];
    const targetDef = BLOCK_DEFS[targetNode.data.kind];
    // Branch route handles (`route-<id>`) aren't in BLOCK_DEFS since they're
    // rendered dynamically per-row; treat them as agent-typed sources.
    const sourceType =
      sourceNode.data.kind === "branch" &&
      connection.sourceHandle?.startsWith("route-")
        ? "agent"
        : sourceDef.handles.find((h) => h.id === connection.sourceHandle)?.type;
    const targetHandle = targetDef.handles.find(
      (h) => h.id === connection.targetHandle
    );
    if (!sourceType || !targetHandle) return false;

    return canConnect(sourceType, targetHandle.type);
  }, []);

  const onNodeClick = useCallback(
    (_: React.MouseEvent, node: { id: string }) => {
      setSelectedNodeId(node.id);
    },
    [setSelectedNodeId]
  );

  const onPaneClick = useCallback(() => {
    setSelectedNodeId(null);
  }, [setSelectedNodeId]);

  /** Edge selection is managed by React Flow; clear inspector node so the panel does not show a stale node. */
  const onEdgeClick = useCallback(() => {
    setSelectedNodeId(null);
  }, [setSelectedNodeId]);

  const onNodesDelete = useCallback(
    (deleted: { id: string }[]) => {
      const deletedIds = new Set(deleted.map((n) => n.id));
      const current = useStudioStore.getState().selectedNodeId;
      if (current && deletedIds.has(current)) {
        setSelectedNodeId(null);
      }
    },
    [setSelectedNodeId]
  );

  // Right-click on the canvas opens the block picker at the cursor, and the
  // picked node spawns at the same flow-space point the user clicked.
  const onPaneContextMenu = useCallback(
    (e: React.MouseEvent | MouseEvent) => {
      e.preventDefault();
      const client = { x: e.clientX, y: e.clientY };
      setPicker({
        anchor: client,
        spawnAt: screenToFlowPosition(client),
      });
    },
    [screenToFlowPosition]
  );

  // Floating + button anchors the menu below the button and spawns the
  // picked node at the visible viewport centre so it lands where the user
  // is actually looking.
  const openPickerFromButton = useCallback(() => {
    const wrapper = reactFlowWrapper.current;
    const button = addButtonRef.current;
    if (!wrapper) return;
    const rect = wrapper.getBoundingClientRect();
    const buttonRect = button?.getBoundingClientRect();
    const anchor = buttonRect
      ? { x: buttonRect.left, y: buttonRect.bottom + 6 }
      : { x: rect.left + 16, y: rect.top + 56 };
    const spawnAt = screenToFlowPosition({
      x: rect.left + rect.width / 2,
      y: rect.top + rect.height / 2,
    });
    setPicker({ anchor, spawnAt });
  }, [screenToFlowPosition]);

  const onPick = useCallback(
    (kind: BlockKind) => {
      if (!picker) return;
      addNode(kind, picker.spawnAt);
      setPicker(null);
    },
    [addNode, picker]
  );

  return (
    <div ref={reactFlowWrapper} className="relative flex-1">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnectHandler}
        onNodeClick={onNodeClick}
        onEdgeClick={onEdgeClick}
        onPaneClick={onPaneClick}
        onPaneContextMenu={onPaneContextMenu}
        onNodesDelete={onNodesDelete}
        elevateEdgesOnSelect
        isValidConnection={isValidConnection}
        connectionMode={ConnectionMode.Loose}
        connectionRadius={24}
        fitView
        fitViewOptions={{ padding: 0.2, maxZoom: 1, minZoom: 0.5 }}
        snapToGrid
        snapGrid={[10, 10]}
        deleteKeyCode={["Backspace", "Delete"]}
        proOptions={{ hideAttribution: true }}
        defaultEdgeOptions={{
          type: "default",
          selectable: true,
          deletable: true,
          focusable: true,
          interactionWidth: 24,
          style: { strokeWidth: 1.5, stroke: CANVAS_EDGE_STROKE },
        }}
        className="bg-background"
      >
        <Background
          gap={20}
          size={2}
          variant={BackgroundVariant.Dots}
          color={CANVAS_DOT_COLOR}
        />
        <Controls
          showInteractive={false}
          className="!rounded-lg !border-border !bg-card !shadow-lg [&>button]:!border-border [&>button]:!bg-card [&>button]:!fill-foreground hover:[&>button]:!bg-accent"
        />
      </ReactFlow>

      <div className="absolute left-3 top-3 z-10 flex flex-col gap-1.5">
        <button
          ref={addButtonRef}
          type="button"
          onClick={openPickerFromButton}
          aria-label="Add block"
          title="Add block"
          className="flex size-9 items-center justify-center rounded-md border border-border bg-card text-foreground shadow-lg transition-colors cursor-pointer hover:bg-accent/40"
        >
          <Plus className="size-4" />
        </button>
        <button
          type="button"
          onClick={() =>
            fitView({ padding: 0.2, maxZoom: 1, minZoom: 0.5, duration: 250 })
          }
          aria-label="Fit view"
          title="Fit view"
          className="flex size-9 items-center justify-center rounded-md border border-border bg-card text-foreground shadow-lg transition-colors cursor-pointer hover:bg-accent/40"
        >
          <Maximize2 className="size-4" />
        </button>
      </div>

      {picker && (
        <BlockPickerMenu
          anchor={picker.anchor}
          onPick={onPick}
          onClose={() => setPicker(null)}
        />
      )}
    </div>
  );
}
