"use client";

import { useCallback, useMemo, useRef, useState } from "react";
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  Controls,
  ConnectionMode,
  useReactFlow,
  type IsValidConnection,
  type OnConnectStart,
  type OnConnectEnd,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Bot, Maximize2, Plus } from "lucide-react";

import { useStudioStore } from "@/lib/store";
import { BLOCK_DEFS, HANDLE_IDS, PALETTE_GROUPS, canConnect } from "@/lib/block-defs";
import { ICON_MAP } from "@/lib/icons";
import type { BlockKind } from "@/lib/types";
import { nodeTypes } from "@/components/nodes";
import { StudioEdge } from "@/components/studio-edge";
import {
  BlockPickerMenu,
  type PickerGroup,
  type PickerItem,
} from "@/components/block-picker-menu";
import { CANVAS_DOT_COLOR, CANVAS_EDGE_STROKE } from "@/lib/canvas-colors";

const edgeTypes = { default: StudioEdge };

type PickerState = {
  /** Where on screen to anchor the menu. */
  anchor: { x: number; y: number };
  /** Flow-space position to spawn the picked node at. */
  spawnAt: { x: number; y: number };
  /** If set, the picker was opened by dropping a connection in empty
   *  space — after the node spawns we wire the source back to it. */
  pendingConnection?: { source: string; sourceHandle: string | null };
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
  const spawnNewAgent = useStudioStore((s) => s.spawnNewAgent);
  const spawnAgentRef = useStudioStore((s) => s.spawnAgentRef);
  const agents = useStudioStore((s) => s.agents);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);

  const [picker, setPicker] = useState<PickerState | null>(null);
  // Captured on handle drag-start and cleared on drag-end; used by the
  // connect-to-new-node shortcut.
  const connectFromRef = useRef<{
    source: string;
    sourceHandle: string | null;
  } | null>(null);

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
      connection.sourceHandle?.startsWith(HANDLE_IDS.routePrefix)
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

  const onPaneContextMenu = useCallback(
    (e: React.MouseEvent | MouseEvent) => {
      e.preventDefault();
      const client = { x: e.clientX, y: e.clientY };
      setPicker({ anchor: client, spawnAt: screenToFlowPosition(client) });
    },
    [screenToFlowPosition]
  );

  const onConnectStart: OnConnectStart = useCallback(
    (_evt, { nodeId, handleId, handleType }) => {
      connectFromRef.current =
        handleType === "source" && nodeId
          ? { source: nodeId, sourceHandle: handleId }
          : null;
    },
    []
  );

  // If the user drops the connection in empty space, open the block
  // picker where they dropped and remember the source handle — the next
  // picker selection both spawns the node and wires the edge.
  const onConnectEnd: OnConnectEnd = useCallback(
    (event, connectionState) => {
      const from = connectFromRef.current;
      connectFromRef.current = null;
      if (!from) return;
      if (connectionState.isValid) return;
      const mouse =
        "clientX" in event
          ? { x: event.clientX, y: event.clientY }
          : event.changedTouches?.[0]
            ? {
                x: event.changedTouches[0].clientX,
                y: event.changedTouches[0].clientY,
              }
            : null;
      if (!mouse) return;
      setPicker({
        anchor: mouse,
        spawnAt: screenToFlowPosition(mouse),
        pendingConnection: from,
      });
    },
    [screenToFlowPosition]
  );

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

  // Build the picker groups. The Agents group is special: it lists
  // "New Agent" plus every existing agent entity so users place agents
  // through the entity rather than duplicating. Every other kind maps
  // 1:1 to a plain spawn via `addNode`.
  const pickerGroups: PickerGroup[] = useMemo(() => {
    if (!picker) return [];
    const spawnAt = picker.spawnAt;
    const pending = picker.pendingConnection;

    // Single spawn→wireUp→close pathway shared by every picker row.
    // Trigger kinds have no `agent-in` handle and are intentionally
    // dropped without an edge when pending is set.
    const accept = (
      kind: BlockKind,
      spawn: () => string | Promise<string>
    ): (() => void) => {
      return () => {
        setPicker(null);
        Promise.resolve(spawn()).then((targetId) => {
          if (!pending) return;
          const def = BLOCK_DEFS[kind];
          if (!def.handles.some((h) => h.id === HANDLE_IDS.agentIn)) return;
          onConnectHandler({
            source: pending.source,
            target: targetId,
            sourceHandle: pending.sourceHandle,
            targetHandle: HANDLE_IDS.agentIn,
          });
        });
      };
    };

    const projectAgents = agents.filter((a) => a.projectId === activeProjectId);

    return PALETTE_GROUPS.map((group) => {
      if (group.label === "Agents") {
        const items: PickerItem[] = [
          {
            id: "new-agent",
            label: "New Agent",
            icon: Plus,
            keywords: ["new", "agent"],
            onSelect: accept("agent", () => spawnNewAgent(spawnAt)),
          },
          ...projectAgents.map(
            (a): PickerItem => ({
              id: `agent-ref-${a.id}`,
              label: a.name,
              icon: Bot,
              keywords: ["agent", a.name],
              onSelect: accept("agent", () => spawnAgentRef(a.id, spawnAt)),
            })
          ),
        ];
        return { label: "Agents", items };
      }

      return {
        label: group.label,
        items: group.kinds.map((kind): PickerItem => {
          const def = BLOCK_DEFS[kind];
          return {
            id: `kind-${kind}`,
            label: def.label,
            icon: ICON_MAP[def.icon],
            keywords: [def.label, def.description],
            onSelect: accept(kind, () => addNode(kind, spawnAt)),
          };
        }),
      };
    });
  }, [
    picker,
    agents,
    activeProjectId,
    addNode,
    spawnNewAgent,
    spawnAgentRef,
    onConnectHandler,
  ]);

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
        onConnectStart={onConnectStart}
        onConnectEnd={onConnectEnd}
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

      {nodes.length === 0 && (
        <div className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center">
          <button
            type="button"
            onClick={openPickerFromButton}
            className="pointer-events-auto flex flex-col items-center gap-3 rounded-xl border border-dashed border-border bg-card/70 px-8 py-6 text-muted-foreground transition-colors cursor-pointer hover:border-foreground/30 hover:bg-card hover:text-foreground"
          >
            <div className="flex size-10 items-center justify-center rounded-full border border-border bg-card">
              <Plus className="size-5" />
            </div>
            <span className="text-sm">Add your first workflow node</span>
          </button>
        </div>
      )}

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
          groups={pickerGroups}
          onClose={() => setPicker(null)}
        />
      )}
    </div>
  );
}
