"use client";

import { useCallback, useRef } from "react";
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

import { useStudioStore } from "@/lib/store";
import { BLOCK_DEFS, canConnect } from "@/lib/block-defs";
import type { BlockKind } from "@/lib/types";
import { nodeTypes } from "@/components/nodes";
import { StudioEdge } from "@/components/studio-edge";

const edgeTypes = { default: StudioEdge };

export function Canvas() {
  const reactFlowWrapper = useRef<HTMLDivElement>(null);
  const { screenToFlowPosition } = useReactFlow();

  const nodes = useStudioStore((s) => s.nodes);
  const edges = useStudioStore((s) => s.edges);
  const onNodesChange = useStudioStore((s) => s.onNodesChange);
  const onEdgesChange = useStudioStore((s) => s.onEdgesChange);
  const onConnectHandler = useStudioStore((s) => s.onConnect);
  const addNode = useStudioStore((s) => s.addNode);
  const setSelectedNodeId = useStudioStore((s) => s.setSelectedNodeId);

  const onDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
  }, []);

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const kind = e.dataTransfer.getData(
        "application/orca-block-kind"
      ) as BlockKind;
      if (!kind || !BLOCK_DEFS[kind]) return;

      const position = screenToFlowPosition({
        x: e.clientX,
        y: e.clientY,
      });

      addNode(kind, position);
    },
    [screenToFlowPosition, addNode]
  );

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

  return (
    <div ref={reactFlowWrapper} className="flex-1">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnectHandler}
        onDragOver={onDragOver}
        onDrop={onDrop}
        onNodeClick={onNodeClick}
        onEdgeClick={onEdgeClick}
        onPaneClick={onPaneClick}
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
          style: { strokeWidth: 1.5, stroke: "oklch(0.50 0 0)" },
        }}
        className="bg-background"
      >
        <Background
          gap={20}
          size={2}
          variant={BackgroundVariant.Dots}
          color="oklch(1 0 0 / 25%)"
        />
        <Controls
          showInteractive={false}
          className="!rounded-lg !border-border !bg-card !shadow-lg [&>button]:!border-border [&>button]:!bg-card [&>button]:!fill-foreground hover:[&>button]:!bg-accent"
        />
      </ReactFlow>
    </div>
  );
}
