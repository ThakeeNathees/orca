// In-memory StorageAdapter implementation.
//
// Serves two purposes:
//   1. Unit-test fixture for the contract suite without pulling in a fake
//      IndexedDB dependency.
//   2. SSR / non-browser fallback so server-rendered pages never crash on
//      window.indexedDB access.

import type {
  Project,
  StorageAdapter,
  WorkflowData,
  WorkflowSummary,
} from "./types";

/** Monotonic id generator — scoped per adapter instance to avoid test bleed. */
function makeIdGen(prefix: string) {
  let n = 0;
  return () => `${prefix}-${++n}-${Date.now().toString(36)}`;
}

/** Returns the workflow with the heavy `graph` field removed. */
function stripGraph(wf: WorkflowData): WorkflowSummary {
  return {
    id: wf.id,
    name: wf.name,
    projectId: wf.projectId,
    color: wf.color,
    updatedAt: wf.updatedAt,
  };
}

export class MemoryStorageAdapter implements StorageAdapter {
  private projects = new Map<string, Project>();
  private workflows = new Map<string, WorkflowData>();
  private nextProjectId = makeIdGen("proj");
  private nextWorkflowId = makeIdGen("wf");

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
    // Cascade: drop workflows belonging to this project.
    for (const [wfId, wf] of this.workflows) {
      if (wf.projectId === id) this.workflows.delete(wfId);
    }
  }

  async listWorkflows(projectId: string): Promise<WorkflowSummary[]> {
    const result: WorkflowSummary[] = [];
    for (const wf of this.workflows.values()) {
      if (wf.projectId !== projectId) continue;
      // Strip the graph — summaries are cheap to transfer.
      result.push(stripGraph(wf));
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
      color: "#a78bfa",
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
}
