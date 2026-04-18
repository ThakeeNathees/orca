"use client";

import { memo, type CSSProperties } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { BLOCK_DEFS } from "@/lib/block-defs";
import { HANDLE_COLORS, HANDLE_COLOR_FALLBACK } from "@/lib/handle-colors";
import type { BlockNode, HandleDef } from "@/lib/types";
import { ICON_MAP } from "@/lib/icons";

/**
 * Handle box must match the visible dot (11×11). XYFlow attaches edges to the handle side
 * (e.g. top: y = handle.y), not the center; a taller invisible box shifts the line above the dot.
 * Extra hit area is done in globals.css via ::before so offsetWidth and getBoundingClientRect stay 11px.
 */
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

/** XYFlow source vs target: *-out = source, *-in = target. */
function handleFlowType(handle: HandleDef): "source" | "target" {
  return handle.id.endsWith("-out") ? "source" : "target";
}

/** Left = target (input); right = source (output). Inline transform overrides RF CSS to avoid Tailwind conflicts. */
function SideHandle({
  handle,
  side,
}: {
  handle: HandleDef;
  side: "left" | "right";
}) {
  const color = HANDLE_COLORS[handle.type] || HANDLE_COLOR_FALLBACK;
  const isLeft = side === "left";

  return (
    <Handle
      id={handle.id}
      type={isLeft ? "target" : "source"}
      position={isLeft ? Position.Left : Position.Right}
      className="!z-10"
      style={handleStyle(color, {
        transform: isLeft
          ? "translate(-50%, -50%)"
          : "translate(50%, -50%)",
      })}
    />
  );
}

/**
 * Top = outputs on model/tool nodes. Inline `left` distributes handles evenly;
 * inline `transform` centers each dot on the top border (overrides RF CSS to
 * avoid Tailwind/specificity conflicts so edge endpoints match visuals).
 */
function TopHandlesRow({ handles }: { handles: HandleDef[] }) {
  const n = handles.length;
  return (
    <>
      {handles.map((handle, i) => {
        const color = HANDLE_COLORS[handle.type] || HANDLE_COLOR_FALLBACK;
        const leftPct = ((i + 0.5) / n) * 100;
        return (
          <Handle
            key={handle.id}
            id={handle.id}
            type={handleFlowType(handle)}
            position={Position.Top}
            className="!z-10"
            style={handleStyle(color, {
              left: `${leftPct}%`,
              transform: "translate(-50%, -50%)",
            })}
          />
        );
      })}
      <div className="border-b border-border/50 pb-1.5 pt-3">
        <div className="flex justify-around gap-1 px-2">
          {handles.map((handle) => (
            <div
              key={`${handle.id}-label`}
              className="flex min-w-0 flex-1 flex-col items-center"
            >
              <span className="truncate text-center text-[10px] font-medium text-muted-foreground">
                {handle.label}
              </span>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

/**
 * Bottom = targets (model / memory / tools on agent). Same as top: inline `left` for
 * distribution, inline `transform` for centering on the bottom border.
 */
function BottomHandlesRow({ handles }: { handles: HandleDef[] }) {
  const n = handles.length;
  return (
    <>
      <div className="border-t border-border/50">
        <div className="flex justify-around gap-1 px-2 pb-2 pt-2.5">
          {handles.map((handle) => (
            <div
              key={`${handle.id}-label`}
              className="flex min-w-0 flex-1 flex-col items-center"
            >
              <span className="truncate text-center text-[10px] font-medium text-muted-foreground">
                {handle.label}
              </span>
            </div>
          ))}
        </div>
      </div>
      {handles.map((handle, i) => {
        const color = HANDLE_COLORS[handle.type] || HANDLE_COLOR_FALLBACK;
        const leftPct = ((i + 0.5) / n) * 100;
        return (
          <Handle
            key={handle.id}
            id={handle.id}
            type="target"
            position={Position.Bottom}
            className="!z-10"
            style={handleStyle(color, {
              left: `${leftPct}%`,
              transform: "translate(-50%, 50%)",
            })}
          />
        );
      })}
    </>
  );
}

function BaseNodeComponent({ data, selected }: NodeProps<BlockNode>) {
  const def = BLOCK_DEFS[data.kind];
  const Icon = ICON_MAP[def.icon];

  const leftHandles = def.handles.filter((h) => h.position === "left");
  const rightHandles = def.handles.filter((h) => h.position === "right");
  const topHandles = def.handles.filter((h) => h.position === "top");
  const bottomHandles = def.handles.filter((h) => h.position === "bottom");
  const hasTopHandles = topHandles.length > 0;
  const hasBottomHandles = bottomHandles.length > 0;

  const summaryFields = def.fields.slice(0, 3).map((f) => ({
    key: f.key,
    label: f.label,
    value: data.fields[f.key],
    placeholder: f.placeholder,
  }));

  return (
    <div
      data-selected={selected ? "true" : undefined}
      className="relative w-[240px] overflow-visible rounded-lg border border-border-standard bg-card shadow-[0_2px_8px_rgba(0,0,0,0.3)] transition-all data-[selected=true]:border-border-solid data-[selected=true]:shadow-[0_0_0_1px_var(--border-solid),0_4px_20px_rgba(0,0,0,0.4)]"
    >
      {hasTopHandles && <TopHandlesRow handles={topHandles} />}

      {/* Main block: side handles are centered on this region only (not top/bottom strips). */}
      <div className="relative">
        {leftHandles.map((h) => (
          <SideHandle key={h.id} handle={h} side="left" />
        ))}
        {rightHandles.map((h) => (
          <SideHandle key={h.id} handle={h} side="right" />
        ))}

        {/* Header */}
        <div className="flex items-center gap-2 px-3 py-2.5">
          {Icon && (
            <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          )}
          <span className="flex-1 truncate text-[13px] font-medium text-foreground">
            {data.label}
          </span>
        </div>

        {/* Description */}
        <div className="px-3 pb-2">
          <p className="text-[11px] leading-relaxed text-muted-foreground/70">
            {def.description}
          </p>
        </div>

        {/* Field summary */}
        {summaryFields.length > 0 && (
          <div className="space-y-1.5 border-t border-border/50 px-3 py-2.5">
            {summaryFields.map((f) => {
              const hasValue = f.value !== undefined && f.value !== "";
              return (
                <div key={f.key}>
                  <div className="mb-0.5 text-[10px] font-medium text-muted-foreground/60">
                    {f.label}
                  </div>
                  <div
                    className={`truncate rounded-md bg-white/5 px-2 py-1 text-[11px] ${
                      hasValue
                        ? "text-foreground/80"
                        : "text-muted-foreground/40 italic"
                    }`}
                  >
                    {hasValue ? String(f.value) : f.placeholder || "..."}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {hasBottomHandles && <BottomHandlesRow handles={bottomHandles} />}
    </div>
  );
}

export const BaseNode = memo(BaseNodeComponent);
