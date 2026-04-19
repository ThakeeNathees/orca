"use client";

import { memo } from "react";
import { BezierEdge, type EdgeProps } from "@xyflow/react";
import { AGENT_HANDLE_COLOR } from "@/lib/handle-colors";
import { CANVAS_EDGE_STROKE } from "@/lib/canvas-colors";

const DEFAULT_STROKE = CANVAS_EDGE_STROKE;

export type StudioEdgeData = { accentColor?: string };

/**
 * Renders the default bezier edge; when selected, stroke flips to the
 * shared flow purple.
 */
export const StudioEdge = memo(function StudioEdge(props: EdgeProps) {
  const accent =
    (props.data as StudioEdgeData | undefined)?.accentColor ?? AGENT_HANDLE_COLOR;
  const stroke = props.selected ? accent : DEFAULT_STROKE;
  return (
    <BezierEdge
      {...props}
      style={{
        ...props.style,
        stroke,
        strokeWidth: props.selected ? 2 : 1.5,
      }}
    />
  );
});
