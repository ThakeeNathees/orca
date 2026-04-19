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
  | "chat"
  | "custom_tool"
  | "branch";

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
  type: "text" | "textarea" | "select" | "slider" | "password" | "number" | "code";
  placeholder?: string;
  options?: { value: string; label: string }[];
  min?: number;
  max?: number;
  step?: number;
  defaultValue?: string | number;
}

/**
 * A branch route row: stable `id` is used as the React key AND as the suffix
 * of the source handle id (`route-<id>`), so edges stay attached even if the
 * user renames the route key.
 */
export interface BranchRoute {
  id: string;
  key: string;
}

export interface BlockNodeData {
  kind: BlockKind;
  label: string;
  fields: Record<string, string | number>;
  /** Only set on `branch` nodes — per-route handles rendered by BranchNode. */
  routes?: BranchRoute[];
  [key: string]: unknown;
}

export type BlockNode = Node<BlockNodeData>;

/** `accentColor` matches the source port dot (see `getEdgeAccentColor` in `handle-colors.ts`). */
export type BlockEdge = Edge<{ accentColor?: string }>;

/** Summary entry for the workflow dashboard list. */
export interface WorkflowSummary {
  id: string;
  name: string;
  description?: string;
  updatedAt: Date;
  projectId: string;
}

/** LLM model provider identifier. */
export type ModelProvider = "openai" | "anthropic" | "gemini" | "ollama";

export interface ModelSummary {
  id: string;
  name: string;
  description?: string;
  provider: ModelProvider;
  modelName: string;
  updatedAt: Date;
  projectId: string;
}

export interface SkillSummary {
  id: string;
  name: string;
  description?: string;
  updatedAt: Date;
  projectId: string;
}

export interface AgentSummary {
  id: string;
  name: string;
  description?: string;
  persona?: string;
  /** Primary model used by this agent, referenced by `ModelSummary.id`. */
  modelId?: string;
  /** Fallback model if the primary fails or is unavailable. */
  fallbackModelId?: string;
  updatedAt: Date;
  projectId: string;
}

export interface CronJobSummary {
  id: string;
  name: string;
  description?: string;
  updatedAt: Date;
  projectId: string;
}

/** A project groups related workflows, models, skills, and agents. */
export interface Project {
  id: string;
  name: string;
}
