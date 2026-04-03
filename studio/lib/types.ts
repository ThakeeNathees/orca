import type { Node, Edge } from "@xyflow/react";

export type BlockKind =
  | "model"
  | "agent"
  | "web_search"
  | "code_exec"
  | "api_request"
  | "sql_query"
  | "knowledge"
  | "memory"
  | "workflow"
  | "input"
  | "schema"
  | "cron"
  | "webhook"
  | "chat";

export type HandleType =
  | "model"
  | "agent"
  | "tool"
  | "knowledge"
  | "memory"
  | "schema"
  | "trigger"
  | "any";

export interface HandleDef {
  id: string;
  label: string;
  type: HandleType;
  /**
   * Edge of the node: left/right = graph in/out; bottom = agent model/memory/tools inputs;
   * top = model/tool/memory node outputs (n8n-style).
   */
  position: "left" | "right" | "top" | "bottom";
  multiple?: boolean;
}

export interface FieldDef {
  key: string;
  label: string;
  type: "text" | "textarea" | "select" | "slider" | "password" | "number";
  placeholder?: string;
  options?: { value: string; label: string }[];
  min?: number;
  max?: number;
  step?: number;
  defaultValue?: string | number;
}

export interface BlockNodeData {
  kind: BlockKind;
  label: string;
  fields: Record<string, string | number>;
  [key: string]: unknown;
}

export type BlockNode = Node<BlockNodeData>;

/** `accentColor` matches the source port dot (see `getEdgeAccentColor` in `handle-colors.ts`). */
export type BlockEdge = Edge<{ accentColor?: string }>;
