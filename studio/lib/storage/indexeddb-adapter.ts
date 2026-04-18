// IndexedDB-backed StorageAdapter.
//
// Uses raw IndexedDB (no `idb` dependency) to keep the bundle lean. The
// DB schema is intentionally minimal: two object stores keyed by `id`,
// with a `by_project` index on `workflows` so listWorkflows() does not
// have to scan the entire store.
//
// All operations open the DB lazily via `ensureDb()` so the adapter can
// be constructed in contexts where `indexedDB` is not yet available
// (e.g. imported at module top-level before hydration).

import type {
  Project,
  StorageAdapter,
  WorkflowData,
  WorkflowSummary,
} from "./types";

const DB_NAME = "orca-studio";
const DB_VERSION = 5;
const STORE_PROJECTS = "projects";
const STORE_WORKFLOWS = "workflows";
const INDEX_BY_PROJECT = "by_project";

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

/**
 * Generates a short unique id. `crypto.randomUUID` is available in all
 * modern browsers that support IndexedDB v3, so no polyfill needed.
 */
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
        if (!db.objectStoreNames.contains(STORE_PROJECTS)) {
          db.createObjectStore(STORE_PROJECTS, { keyPath: "id" });
        }
        if (!db.objectStoreNames.contains(STORE_WORKFLOWS)) {
          const wfStore = db.createObjectStore(STORE_WORKFLOWS, {
            keyPath: "id",
          });
          wfStore.createIndex(INDEX_BY_PROJECT, "projectId", { unique: false });
        }
      };
      open.onsuccess = () => resolve(open.result);
      open.onerror = () => reject(open.error);
    });
    return this.dbPromise;
  }

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
    // Single transaction spanning both stores keeps the cascade atomic:
    // if the workflow cleanup fails the project delete is rolled back.
    const tx = db.transaction([STORE_PROJECTS, STORE_WORKFLOWS], "readwrite");
    tx.objectStore(STORE_PROJECTS).delete(id);

    const wfStore = tx.objectStore(STORE_WORKFLOWS);
    const index = wfStore.index(INDEX_BY_PROJECT);
    const cursorReq = index.openCursor(IDBKeyRange.only(id));
    cursorReq.onsuccess = () => {
      const cursor = cursorReq.result;
      if (!cursor) return;
      cursor.delete();
      cursor.continue();
    };
    await txDone(tx);
  }

  async listWorkflows(projectId: string): Promise<WorkflowSummary[]> {
    const db = await this.ensureDb();
    const tx = db.transaction(STORE_WORKFLOWS, "readonly");
    const index = tx.objectStore(STORE_WORKFLOWS).index(INDEX_BY_PROJECT);
    const rows = (await req(
      index.getAll(IDBKeyRange.only(projectId))
    )) as WorkflowData[];
    // Strip graph payload — callers that need the full workflow use getWorkflow.
    return rows.map((row) => ({
      id: row.id,
      name: row.name,
      projectId: row.projectId,
      color: row.color,
      updatedAt: row.updatedAt,
    }));
  }

  async getWorkflow(id: string): Promise<WorkflowData | null> {
    const db = await this.ensureDb();
    const tx = db.transaction(STORE_WORKFLOWS, "readonly");
    const result = (await req(tx.objectStore(STORE_WORKFLOWS).get(id))) as
      | WorkflowData
      | undefined;
    return result ?? null;
  }

  async createWorkflow(
    projectId: string,
    name: string
  ): Promise<WorkflowData> {
    const db = await this.ensureDb();
    const wf: WorkflowData = {
      id: uid("wf"),
      name,
      projectId,
      color: "#a78bfa",
      updatedAt: new Date(),
      graph: { nodes: [], edges: [] },
    };
    const tx = db.transaction(STORE_WORKFLOWS, "readwrite");
    tx.objectStore(STORE_WORKFLOWS).put(wf);
    await txDone(tx);
    return wf;
  }

  async saveWorkflow(data: WorkflowData): Promise<void> {
    const db = await this.ensureDb();
    const tx = db.transaction(STORE_WORKFLOWS, "readwrite");
    tx.objectStore(STORE_WORKFLOWS).put({ ...data, updatedAt: new Date() });
    await txDone(tx);
  }

  async deleteWorkflow(id: string): Promise<void> {
    const db = await this.ensureDb();
    const tx = db.transaction(STORE_WORKFLOWS, "readwrite");
    tx.objectStore(STORE_WORKFLOWS).delete(id);
    await txDone(tx);
  }
}
