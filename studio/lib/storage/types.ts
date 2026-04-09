// Storage layer types for projects and workflows.
//
// The StorageAdapter interface abstracts persistence so the Zustand store
// can target different backends (IndexedDB for browser-only MVP, a REST
// API for the future hosted mode) without changing call sites.

import type { BlockNode, BlockEdge, Project, WorkflowSummary } from "../types";

export type { Project, WorkflowSummary };

/** Full workflow payload including the editor graph. */
export interface WorkflowData extends WorkflowSummary {
  graph: {
    nodes: BlockNode[];
    edges: BlockEdge[];
  };
}

/**
 * Abstract persistence surface for the studio. Implementations must be
 * async so the same interface works for both local (IndexedDB) and remote
 * (HTTP) backends. All methods are expected to be idempotent with respect
 * to repeated identical calls so the store can debounce-save freely.
 */
export interface StorageAdapter {
  // Projects
  listProjects(): Promise<Project[]>;
  createProject(name: string): Promise<Project>;
  renameProject(id: string, name: string): Promise<void>;
  deleteProject(id: string): Promise<void>;

  // Workflows
  listWorkflows(projectId: string): Promise<WorkflowSummary[]>;
  getWorkflow(id: string): Promise<WorkflowData | null>;
  createWorkflow(projectId: string, name: string): Promise<WorkflowData>;
  saveWorkflow(data: WorkflowData): Promise<void>;
  deleteWorkflow(id: string): Promise<void>;
}
