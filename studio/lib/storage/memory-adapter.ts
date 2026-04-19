// In-memory StorageAdapter implementation.
//
// Serves two purposes:
//   1. Unit-test fixture for the contract suite without pulling in a fake
//      IndexedDB dependency.
//   2. SSR / non-browser fallback so server-rendered pages never crash on
//      window.indexedDB access.

import type {
  AgentData,
  AgentSummary,
  CronJobData,
  CronJobSummary,
  ModelData,
  ModelSummary,
  Project,
  SkillData,
  SkillSummary,
  StorageAdapter,
  WorkflowData,
  WorkflowSummary,
} from "./types";
import type { ModelProvider } from "../types";

/** Monotonic id generator — scoped per adapter instance to avoid test bleed. */
function makeIdGen(prefix: string) {
  let n = 0;
  return () => `${prefix}-${++n}-${Date.now().toString(36)}`;
}

export class MemoryStorageAdapter implements StorageAdapter {
  private projects = new Map<string, Project>();
  private workflows = new Map<string, WorkflowData>();
  private models = new Map<string, ModelData>();
  private skills = new Map<string, SkillData>();
  private agents = new Map<string, AgentData>();
  private cronJobs = new Map<string, CronJobData>();
  private nextProjectId = makeIdGen("proj");
  private nextWorkflowId = makeIdGen("wf");
  private nextModelId = makeIdGen("model");
  private nextSkillId = makeIdGen("skill");
  private nextAgentId = makeIdGen("agent");
  private nextCronJobId = makeIdGen("cron");

  /* ── Projects ────────────────────────────────────────────────────── */

  async listProjects(): Promise<Project[]> {
    return Array.from(this.projects.values());
  }

  async createProject(name: string): Promise<Project> {
    const p: Project = { id: this.nextProjectId(), name };
    this.projects.set(p.id, p);
    return p;
  }

  async renameProject(id: string, name: string): Promise<void> {
    const existing = this.projects.get(id);
    if (!existing) return;
    this.projects.set(id, { ...existing, name });
  }

  async deleteProject(id: string): Promise<void> {
    this.projects.delete(id);
    // Cascade: drop entities belonging to this project.
    for (const [k, v] of this.workflows)
      if (v.projectId === id) this.workflows.delete(k);
    for (const [k, v] of this.models)
      if (v.projectId === id) this.models.delete(k);
    for (const [k, v] of this.skills)
      if (v.projectId === id) this.skills.delete(k);
    for (const [k, v] of this.agents)
      if (v.projectId === id) this.agents.delete(k);
    for (const [k, v] of this.cronJobs)
      if (v.projectId === id) this.cronJobs.delete(k);
  }

  /* ── Workflows ───────────────────────────────────────────────────── */

  async listWorkflows(projectId: string): Promise<WorkflowSummary[]> {
    const result: WorkflowSummary[] = [];
    for (const wf of this.workflows.values()) {
      if (wf.projectId !== projectId) continue;
      const { graph: _graph, ...summary } = wf;
      result.push(summary);
    }
    return result;
  }

  async getWorkflow(id: string): Promise<WorkflowData | null> {
    return this.workflows.get(id) ?? null;
  }

  async createWorkflow(
    projectId: string,
    name: string
  ): Promise<WorkflowData> {
    const wf: WorkflowData = {
      id: this.nextWorkflowId(),
      name,
      projectId,
      updatedAt: new Date(),
      graph: { nodes: [], edges: [] },
    };
    this.workflows.set(wf.id, wf);
    return wf;
  }

  async saveWorkflow(data: WorkflowData): Promise<void> {
    this.workflows.set(data.id, { ...data, updatedAt: new Date() });
  }

  async deleteWorkflow(id: string): Promise<void> {
    this.workflows.delete(id);
  }

  /* ── Models ──────────────────────────────────────────────────────── */

  async listModels(projectId: string): Promise<ModelSummary[]> {
    return Array.from(this.models.values()).filter(
      (m) => m.projectId === projectId
    );
  }

  async getModel(id: string): Promise<ModelData | null> {
    return this.models.get(id) ?? null;
  }

  async createModel(
    projectId: string,
    name: string,
    provider: ModelProvider,
    modelName: string
  ): Promise<ModelData> {
    const data: ModelData = {
      id: this.nextModelId(),
      name,
      projectId,
      provider,
      modelName,
      updatedAt: new Date(),
    };
    this.models.set(data.id, data);
    return data;
  }

  async saveModel(data: ModelData): Promise<void> {
    this.models.set(data.id, { ...data, updatedAt: new Date() });
  }

  async deleteModel(id: string): Promise<void> {
    this.models.delete(id);
  }

  /* ── Skills ──────────────────────────────────────────────────────── */

  async listSkills(projectId: string): Promise<SkillSummary[]> {
    return Array.from(this.skills.values()).filter(
      (s) => s.projectId === projectId
    );
  }

  async getSkill(id: string): Promise<SkillData | null> {
    return this.skills.get(id) ?? null;
  }

  async createSkill(projectId: string, name: string): Promise<SkillData> {
    const data: SkillData = {
      id: this.nextSkillId(),
      name,
      projectId,
      updatedAt: new Date(),
    };
    this.skills.set(data.id, data);
    return data;
  }

  async saveSkill(data: SkillData): Promise<void> {
    this.skills.set(data.id, { ...data, updatedAt: new Date() });
  }

  async deleteSkill(id: string): Promise<void> {
    this.skills.delete(id);
  }

  /* ── Agents ──────────────────────────────────────────────────────── */

  async listAgents(projectId: string): Promise<AgentSummary[]> {
    return Array.from(this.agents.values()).filter(
      (a) => a.projectId === projectId
    );
  }

  async getAgent(id: string): Promise<AgentData | null> {
    return this.agents.get(id) ?? null;
  }

  async createAgent(projectId: string, name: string): Promise<AgentData> {
    const data: AgentData = {
      id: this.nextAgentId(),
      name,
      projectId,
      updatedAt: new Date(),
    };
    this.agents.set(data.id, data);
    return data;
  }

  async saveAgent(data: AgentData): Promise<void> {
    this.agents.set(data.id, { ...data, updatedAt: new Date() });
  }

  async deleteAgent(id: string): Promise<void> {
    this.agents.delete(id);
  }

  /* ── Cron Jobs ───────────────────────────────────────────────────── */

  async listCronJobs(projectId: string): Promise<CronJobSummary[]> {
    return Array.from(this.cronJobs.values()).filter(
      (c) => c.projectId === projectId
    );
  }

  async getCronJob(id: string): Promise<CronJobData | null> {
    return this.cronJobs.get(id) ?? null;
  }

  async createCronJob(projectId: string, name: string): Promise<CronJobData> {
    const data: CronJobData = {
      id: this.nextCronJobId(),
      name,
      projectId,
      updatedAt: new Date(),
    };
    this.cronJobs.set(data.id, data);
    return data;
  }

  async saveCronJob(data: CronJobData): Promise<void> {
    this.cronJobs.set(data.id, { ...data, updatedAt: new Date() });
  }

  async deleteCronJob(id: string): Promise<void> {
    this.cronJobs.delete(id);
  }
}
