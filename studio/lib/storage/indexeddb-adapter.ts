// IndexedDB-backed StorageAdapter.
//
// Uses raw IndexedDB (no `idb` dependency) to keep the bundle lean. Each
// project-scoped entity type (workflows, models, skills, agents) has its
// own object store keyed by `id`, with a `by_project` index so
// list-by-project does not have to scan the entire store.

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

const DB_NAME = "orca-studio";
const DB_VERSION = 8;
const STORE_PROJECTS = "projects";
const STORE_WORKFLOWS = "workflows";
const STORE_MODELS = "models";
const STORE_SKILLS = "skills";
const STORE_AGENTS = "agents";
const STORE_CRON_JOBS = "cron_jobs";
const INDEX_BY_PROJECT = "by_project";

/** Project-scoped entity stores that share the same create/list/get/save/delete shape. */
const ENTITY_STORES = [
  STORE_WORKFLOWS,
  STORE_MODELS,
  STORE_SKILLS,
  STORE_AGENTS,
  STORE_CRON_JOBS,
] as const;

/** Promisifies an IDBRequest into a typed Promise. */
function req<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

/** Promisifies an IDBTransaction's completion. */
function txDone(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);
  });
}

function uid(prefix: string): string {
  return `${prefix}-${crypto.randomUUID()}`;
}

export class IndexedDBStorageAdapter implements StorageAdapter {
  private dbPromise: Promise<IDBDatabase> | null = null;

  /** Opens (and migrates, if necessary) the backing database exactly once. */
  private ensureDb(): Promise<IDBDatabase> {
    if (this.dbPromise) return this.dbPromise;
    this.dbPromise = new Promise((resolve, reject) => {
      const open = indexedDB.open(DB_NAME, DB_VERSION);
      open.onupgradeneeded = () => {
        const db = open.result;
        const tx = open.transaction;
        if (!db.objectStoreNames.contains(STORE_PROJECTS)) {
          db.createObjectStore(STORE_PROJECTS, { keyPath: "id" });
        }
        for (const name of ENTITY_STORES) {
          const store = db.objectStoreNames.contains(name)
            ? tx!.objectStore(name)
            : db.createObjectStore(name, { keyPath: "id" });
          // Ensure the by_project index exists even on stores that were
          // created by an older schema version without it.
          if (!store.indexNames.contains(INDEX_BY_PROJECT)) {
            store.createIndex(INDEX_BY_PROJECT, "projectId", { unique: false });
          }
        }
      };
      open.onsuccess = () => resolve(open.result);
      open.onerror = () => reject(open.error);
    });
    return this.dbPromise;
  }

  /** Generic list-by-project helper for entity stores. */
  private async listByProject<T>(
    store: string,
    projectId: string
  ): Promise<T[]> {
    const db = await this.ensureDb();
    const tx = db.transaction(store, "readonly");
    const index = tx.objectStore(store).index(INDEX_BY_PROJECT);
    return (await req(index.getAll(IDBKeyRange.only(projectId)))) as T[];
  }

  /** Generic get-by-id helper for entity stores. */
  private async getById<T>(store: string, id: string): Promise<T | null> {
    const db = await this.ensureDb();
    const tx = db.transaction(store, "readonly");
    const result = (await req(tx.objectStore(store).get(id))) as T | undefined;
    return result ?? null;
  }

  /** Generic put helper for entity stores — stamps updatedAt. */
  private async put(store: string, data: { id: string }): Promise<void> {
    const db = await this.ensureDb();
    const tx = db.transaction(store, "readwrite");
    tx.objectStore(store).put({ ...data, updatedAt: new Date() });
    await txDone(tx);
  }

  /** Generic delete-by-id helper for entity stores. */
  private async deleteById(store: string, id: string): Promise<void> {
    const db = await this.ensureDb();
    const tx = db.transaction(store, "readwrite");
    tx.objectStore(store).delete(id);
    await txDone(tx);
  }

  /* ── Projects ────────────────────────────────────────────────────── */

  async listProjects(): Promise<Project[]> {
    const db = await this.ensureDb();
    const tx = db.transaction(STORE_PROJECTS, "readonly");
    return req(tx.objectStore(STORE_PROJECTS).getAll() as IDBRequest<Project[]>);
  }

  async createProject(name: string): Promise<Project> {
    const db = await this.ensureDb();
    const project: Project = { id: uid("proj"), name };
    const tx = db.transaction(STORE_PROJECTS, "readwrite");
    tx.objectStore(STORE_PROJECTS).put(project);
    await txDone(tx);
    return project;
  }

  async renameProject(id: string, name: string): Promise<void> {
    const db = await this.ensureDb();
    const tx = db.transaction(STORE_PROJECTS, "readwrite");
    const store = tx.objectStore(STORE_PROJECTS);
    const existing = (await req(store.get(id))) as Project | undefined;
    if (existing) store.put({ ...existing, name });
    await txDone(tx);
  }

  async deleteProject(id: string): Promise<void> {
    const db = await this.ensureDb();
    // Single transaction spans all entity stores so the cascade is atomic.
    const tx = db.transaction(
      [STORE_PROJECTS, ...ENTITY_STORES],
      "readwrite"
    );
    tx.objectStore(STORE_PROJECTS).delete(id);
    for (const name of ENTITY_STORES) {
      const store = tx.objectStore(name);
      const cursorReq = store
        .index(INDEX_BY_PROJECT)
        .openCursor(IDBKeyRange.only(id));
      cursorReq.onsuccess = () => {
        const cursor = cursorReq.result;
        if (!cursor) return;
        cursor.delete();
        cursor.continue();
      };
    }
    await txDone(tx);
  }

  /* ── Workflows ───────────────────────────────────────────────────── */

  async listWorkflows(projectId: string): Promise<WorkflowSummary[]> {
    const rows = await this.listByProject<WorkflowData>(
      STORE_WORKFLOWS,
      projectId
    );
    // Strip graph payload — callers that need the full workflow use getWorkflow.
    return rows.map(({ graph: _graph, ...summary }) => summary);
  }

  async getWorkflow(id: string): Promise<WorkflowData | null> {
    return this.getById<WorkflowData>(STORE_WORKFLOWS, id);
  }

  async createWorkflow(
    projectId: string,
    name: string
  ): Promise<WorkflowData> {
    const wf: WorkflowData = {
      id: uid("wf"),
      name,
      projectId,
      updatedAt: new Date(),
      graph: { nodes: [], edges: [] },
    };
    await this.put(STORE_WORKFLOWS, wf);
    return wf;
  }

  saveWorkflow(data: WorkflowData): Promise<void> {
    return this.put(STORE_WORKFLOWS, data);
  }

  deleteWorkflow(id: string): Promise<void> {
    return this.deleteById(STORE_WORKFLOWS, id);
  }

  /* ── Models ──────────────────────────────────────────────────────── */

  listModels(projectId: string): Promise<ModelSummary[]> {
    return this.listByProject<ModelSummary>(STORE_MODELS, projectId);
  }

  getModel(id: string): Promise<ModelData | null> {
    return this.getById<ModelData>(STORE_MODELS, id);
  }

  async createModel(
    projectId: string,
    name: string,
    provider: ModelProvider,
    modelName: string
  ): Promise<ModelData> {
    const model: ModelData = {
      id: uid("model"),
      name,
      projectId,
      provider,
      modelName,
      updatedAt: new Date(),
    };
    await this.put(STORE_MODELS, model);
    return model;
  }

  saveModel(data: ModelData): Promise<void> {
    return this.put(STORE_MODELS, data);
  }

  deleteModel(id: string): Promise<void> {
    return this.deleteById(STORE_MODELS, id);
  }

  /* ── Skills ──────────────────────────────────────────────────────── */

  listSkills(projectId: string): Promise<SkillSummary[]> {
    return this.listByProject<SkillSummary>(STORE_SKILLS, projectId);
  }

  getSkill(id: string): Promise<SkillData | null> {
    return this.getById<SkillData>(STORE_SKILLS, id);
  }

  async createSkill(projectId: string, name: string): Promise<SkillData> {
    const skill: SkillData = {
      id: uid("skill"),
      name,
      projectId,
      updatedAt: new Date(),
    };
    await this.put(STORE_SKILLS, skill);
    return skill;
  }

  saveSkill(data: SkillData): Promise<void> {
    return this.put(STORE_SKILLS, data);
  }

  deleteSkill(id: string): Promise<void> {
    return this.deleteById(STORE_SKILLS, id);
  }

  /* ── Agents ──────────────────────────────────────────────────────── */

  listAgents(projectId: string): Promise<AgentSummary[]> {
    return this.listByProject<AgentSummary>(STORE_AGENTS, projectId);
  }

  getAgent(id: string): Promise<AgentData | null> {
    return this.getById<AgentData>(STORE_AGENTS, id);
  }

  async createAgent(projectId: string, name: string): Promise<AgentData> {
    const agent: AgentData = {
      id: uid("agent"),
      name,
      projectId,
      updatedAt: new Date(),
    };
    await this.put(STORE_AGENTS, agent);
    return agent;
  }

  saveAgent(data: AgentData): Promise<void> {
    return this.put(STORE_AGENTS, data);
  }

  deleteAgent(id: string): Promise<void> {
    return this.deleteById(STORE_AGENTS, id);
  }

  /* ── Cron Jobs ───────────────────────────────────────────────────── */

  listCronJobs(projectId: string): Promise<CronJobSummary[]> {
    return this.listByProject<CronJobSummary>(STORE_CRON_JOBS, projectId);
  }

  getCronJob(id: string): Promise<CronJobData | null> {
    return this.getById<CronJobData>(STORE_CRON_JOBS, id);
  }

  async createCronJob(
    projectId: string,
    name: string
  ): Promise<CronJobData> {
    const job: CronJobData = {
      id: uid("cron"),
      name,
      projectId,
      updatedAt: new Date(),
    };
    await this.put(STORE_CRON_JOBS, job);
    return job;
  }

  saveCronJob(data: CronJobData): Promise<void> {
    return this.put(STORE_CRON_JOBS, data);
  }

  deleteCronJob(id: string): Promise<void> {
    return this.deleteById(STORE_CRON_JOBS, id);
  }
}
