"use client";

import { memo } from "react";
import { BezierEdge, type EdgeProps } from "@xyflow/react";
import { HANDLE_COLORS } from "@/lib/handle-colors";
import { CANVAS_EDGE_STROKE } from "@/lib/canvas-colors";

const DEFAULT_STROKE = CANVAS_EDGE_STROKE;

export type StudioEdgeData = { accentColor?: string };

/**
 * Renders the default bezier edge; when selected, stroke matches the source handle dot color
 * (`data.accentColor` from the store).
 */
export const StudioEdge = memo(function StudioEdge(props: EdgeProps) {
  const accent =
    (props.data as StudioEdgeData | undefined)?.accentColor ?? HANDLE_COLORS.any;
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
