import { BLOCK_DEFS } from "./block-defs";
import type { BlockNode } from "./types";

/** Side-handle purple for agent I/O; trigger outputs use the same color so they graphically match agent inputs. */
const AGENT_HANDLE_COLOR = "#a78bfa";

/** Port dot colors (must stay in sync with node handles in `base-node.tsx`). */
export const HANDLE_COLORS: Record<string, string> = {
  model: "#e879a0",
  agent: AGENT_HANDLE_COLOR,
  trigger: AGENT_HANDLE_COLOR,
  tool: "#67e8b4",
  knowledge: "#fbbf24",
  memory: "#fbbf24",
  schema: "#94a3b8",
  any: "#a1a1aa",
};

const FALLBACK = HANDLE_COLORS.any;

/**
 * Edge accent follows the **source** port (the dot the edge leaves from), e.g. model-out → model pink,
 * tool-out → tool green.
 */
export function getEdgeAccentColor(
  nodes: BlockNode[],
  sourceNodeId: string,
  sourceHandleId: string | null | undefined
): string {
  const node = nodes.find((n) => n.id === sourceNodeId);
  if (!node || !sourceHandleId) return FALLBACK;
  const def = BLOCK_DEFS[node.data.kind];
  const handle = def.handles.find((h) => h.id === sourceHandleId);
  if (!handle) return FALLBACK;
  return HANDLE_COLORS[handle.type] ?? FALLBACK;
}
