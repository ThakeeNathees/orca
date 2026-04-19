// Storage layer types for projects, workflows, models, skills, and agents.
//
// The StorageAdapter interface abstracts persistence so the Zustand store
// can target different backends (IndexedDB for browser-only MVP, a REST
// API for the future hosted mode) without changing call sites.

import type {
  AgentSummary,
  BlockNode,
  BlockEdge,
  CronJobSummary,
  ModelSummary,
  Project,
  SkillSummary,
  WorkflowSummary,
} from "../types";

export type {
  AgentSummary,
  CronJobSummary,
  ModelSummary,
  Project,
  SkillSummary,
  WorkflowSummary,
};

/** Full workflow payload including the editor graph. */
export interface WorkflowData extends WorkflowSummary {
  graph: {
    nodes: BlockNode[];
    edges: BlockEdge[];
  };
}

/** Full model payload (currently identical to the summary). */
export type ModelData = ModelSummary;

/** Full skill payload (currently identical to the summary). */
export type SkillData = SkillSummary;

/** Full agent payload (currently identical to the summary). */
export type AgentData = AgentSummary;

/** Full cron job payload (currently identical to the summary). */
export type CronJobData = CronJobSummary;

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

  // Models
  listModels(projectId: string): Promise<ModelSummary[]>;
  getModel(id: string): Promise<ModelData | null>;
  createModel(
    projectId: string,
    name: string,
    provider: ModelSummary["provider"],
    modelName: string
  ): Promise<ModelData>;
  saveModel(data: ModelData): Promise<void>;
  deleteModel(id: string): Promise<void>;

  // Skills
  listSkills(projectId: string): Promise<SkillSummary[]>;
  getSkill(id: string): Promise<SkillData | null>;
  createSkill(projectId: string, name: string): Promise<SkillData>;
  saveSkill(data: SkillData): Promise<void>;
  deleteSkill(id: string): Promise<void>;

  // Agents
  listAgents(projectId: string): Promise<AgentSummary[]>;
  getAgent(id: string): Promise<AgentData | null>;
  createAgent(projectId: string, name: string): Promise<AgentData>;
  saveAgent(data: AgentData): Promise<void>;
  deleteAgent(id: string): Promise<void>;

  // Cron Jobs
  listCronJobs(projectId: string): Promise<CronJobSummary[]>;
  getCronJob(id: string): Promise<CronJobData | null>;
  createCronJob(projectId: string, name: string): Promise<CronJobData>;
  saveCronJob(data: CronJobData): Promise<void>;
  deleteCronJob(id: string): Promise<void>;
}
