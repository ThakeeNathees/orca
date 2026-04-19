"use client";

import { memo, useCallback, useMemo, type CSSProperties } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Plus, Trash2 } from "lucide-react";
import { BLOCK_DEFS } from "@/lib/block-defs";
import { HANDLE_COLORS, HANDLE_COLOR_FALLBACK } from "@/lib/handle-colors";
import type { BlockNode, BranchRoute, HandleDef } from "@/lib/types";
import { ICON_MAP } from "@/lib/icons";
import { useStudioStore } from "@/lib/store";

// Keep in sync with base-node.tsx — the visible handle dot must match the
// DOM size so edges render against the correct side of the node.
const HANDLE_SIZE = 11;

function handleStyle(color: string, extra?: CSSProperties): CSSProperties {
  return {
    backgroundColor: color,
    width: HANDLE_SIZE,
    height: HANDLE_SIZE,
    border: `1.5px solid color-mix(in oklch, ${color}, black 25%)`,
    ...extra,
    ["--handle-color" as string]: color,
  };
}

/** Mirrors base-node's left-side input handle style. */
function LeftHandle({ handle }: { handle: HandleDef }) {
  const color = HANDLE_COLORS[handle.type] || HANDLE_COLOR_FALLBACK;
  return (
    <Handle
      id={handle.id}
      type="target"
      position={Position.Left}
      className="!z-10"
      style={handleStyle(color, { transform: "translate(-50%, -50%)" })}
    />
  );
}

/**
 * A single route row: delete button, editable key input, and a right-side
 * source handle whose id is `route-<routeId>` so edges survive key renames.
 */
function RouteRow({
  nodeId,
  route,
  onChangeKey,
  onDelete,
}: {
  nodeId: string;
  route: BranchRoute;
  onChangeKey: (key: string) => void;
  onDelete: () => void;
}) {
  const color = HANDLE_COLORS.agent || HANDLE_COLOR_FALLBACK;
  return (
    <div className="relative flex items-center gap-1.5 px-3 py-1.5">
      <button
        type="button"
        onClick={onDelete}
        className="flex h-5 w-5 shrink-0 items-center justify-center rounded text-muted-foreground/50 hover:bg-destructive/20 hover:text-destructive"
        aria-label={`Delete route ${route.key}`}
      >
        <Trash2 className="h-3 w-3" />
      </button>
      <input
        type="text"
        value={route.key}
        onChange={(e) => onChangeKey(e.target.value)}
        placeholder="route_key"
        className="nodrag min-w-0 flex-1 rounded-md border border-transparent bg-white/5 px-2 py-1 text-[11px] text-foreground/90 outline-none transition focus:border-border focus:bg-white/10"
      />
      <Handle
        id={`route-${route.id}`}
        type="source"
        position={Position.Right}
        // Key the node id into the DOM id so multiple branch nodes don't
        // clash in the same document (purely cosmetic — RF keys by node+handle).
        data-node-id={nodeId}
        className="!z-10"
        style={handleStyle(color, {
          position: "absolute",
          right: 0,
          top: "50%",
          transform: "translate(50%, -50%)",
        })}
      />
    </div>
  );
}

function BranchNodeComponent({ id, data, selected }: NodeProps<BlockNode>) {
  const def = BLOCK_DEFS[data.kind];
  const Icon = ICON_MAP[def.icon];
  const updateNodeRoutes = useStudioStore((s) => s.updateNodeRoutes);

  // `data.routes` is optional on BlockNodeData; fall back to a stable empty
  // array so the useCallback deps below don't churn every render.
  const routes = useMemo(() => data.routes ?? [], [data.routes]);
  const transform = data.fields.transform;
  const hasTransform = transform !== undefined && transform !== "";
  const transformField = def.fields.find((f) => f.key === "transform");
  const leftHandle = def.handles.find((h) => h.position === "left");

  const addRoute = useCallback(() => {
    const nextId = `route-${Date.now().toString(36)}-${Math.random()
      .toString(36)
      .slice(2, 6)}`;
    updateNodeRoutes(id, [...routes, { id: nextId, key: "" }]);
  }, [id, routes, updateNodeRoutes]);

  const renameRoute = useCallback(
    (routeId: string, key: string) => {
      updateNodeRoutes(
        id,
        routes.map((r) => (r.id === routeId ? { ...r, key } : r))
      );
    },
    [id, routes, updateNodeRoutes]
  );

  const deleteRoute = useCallback(
    (routeId: string) => {
      updateNodeRoutes(
        id,
        routes.filter((r) => r.id !== routeId)
      );
    },
    [id, routes, updateNodeRoutes]
  );

  return (
    <div
      data-selected={selected ? "true" : undefined}
      className="relative w-[240px] overflow-visible rounded-lg border border-border-standard bg-card shadow-[0_2px_8px_rgba(0,0,0,0.3)] transition-all data-[selected=true]:border-border-solid data-[selected=true]:shadow-[0_0_0_1px_var(--border-solid),0_4px_20px_rgba(0,0,0,0.4)]"
    >
      <div className="relative">
        {leftHandle && <LeftHandle handle={leftHandle} />}

        {/* Header */}
        <div className="flex items-center gap-2 px-3 py-2.5">
          {Icon && (
            <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          )}
          <span className="flex-1 truncate text-[13px] font-medium text-foreground">
            {data.label}
          </span>
        </div>

        <div className="px-3 pb-2">
          <p className="text-[11px] leading-relaxed text-muted-foreground/70">
            {def.description}
          </p>
        </div>

        {/* Transform summary — read-only mirror of fields.transform */}
        {transformField && (
          <div className="space-y-1.5 border-t border-border/50 px-3 py-2.5">
            <div className="mb-0.5 text-[10px] font-medium text-muted-foreground/60">
              {transformField.label}
            </div>
            <div
              className={`truncate rounded-md bg-white/5 px-2 py-1 text-[11px] ${
                hasTransform
                  ? "text-foreground/80"
                  : "text-muted-foreground/40 italic"
              }`}
            >
              {hasTransform ? String(transform) : transformField.placeholder || "..."}
            </div>
          </div>
        )}

        {/* Routes — dynamic rows, each owning a right-side source handle */}
        <div className="border-t border-border/50 py-1">
          <div className="px-3 pb-1 pt-1.5 text-[10px] font-medium text-muted-foreground/60">
            Routes
          </div>
          {routes.map((route) => (
            <RouteRow
              key={route.id}
              nodeId={id}
              route={route}
              onChangeKey={(key) => renameRoute(route.id, key)}
              onDelete={() => deleteRoute(route.id)}
            />
          ))}
          <div className="px-3 pb-2 pt-1">
            <button
              type="button"
              onClick={addRoute}
              className="nodrag flex w-full items-center justify-center gap-1 rounded-md border border-dashed border-border/60 px-2 py-1 text-[10px] font-medium text-muted-foreground/70 transition hover:border-border hover:bg-white/5 hover:text-foreground"
            >
              <Plus className="h-3 w-3" />
              Add route
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

export const BranchNode = memo(BranchNodeComponent);
