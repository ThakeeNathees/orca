import { create } from "zustand";
import {
  applyNodeChanges,
  applyEdgeChanges,
  type OnNodesChange,
  type OnEdgesChange,
  type Connection,
} from "@xyflow/react";
import type {
  AgentSummary,
  BlockNode,
  BlockEdge,
  BlockKind,
  BlockNodeData,
  CronJobSummary,
  ModelProvider,
  ModelSummary,
  Project,
  SkillSummary,
  WorkflowSummary,
} from "./types";
import { BLOCK_DEFS } from "./block-defs";
import { AGENT_HANDLE_COLOR } from "./handle-colors";
import { getStorageAdapter } from "./storage";
import type { WorkflowData } from "./storage/types";
import { INBOX_MESSAGES, type InboxMessage } from "./inbox";
import { debounce } from "./debounce";
import type { SidebarSection } from "./sidebar-sections";

type StudioView = "dashboard" | "detail" | "editor";

interface StudioState {
  /* ── Hydration ── */
  /** Becomes true once the store has loaded projects/workflows from storage. */
  hydrated: boolean;
  /** Loads projects/workflows from the storage adapter. Idempotent. */
  hydrate: () => Promise<void>;

  /* ── Navigation ── */
  currentView: StudioView;
  /** Top-level sidebar section; drives the main-area placeholder for
   *  sections without detail pages (Cost/Runs/Settings/etc). */
  sidebarSection: SidebarSection;
  setSidebarSection: (section: SidebarSection) => void;
  workflows: WorkflowSummary[];
  activeWorkflowId: string | null;

  /* ── Projects ── */
  projects: Project[];
  activeProjectId: string;
  createProject: () => Promise<void>;
  renameProject: (id: string, name: string) => Promise<void>;
  deleteProject: (id: string) => Promise<void>;
  setActiveProject: (id: string) => void;

  createWorkflow: (name?: string, description?: string) => Promise<void>;
  openWorkflow: (id: string) => void;
  openWorkflowEditor: (id: string) => Promise<void>;
  renameWorkflow: (id: string, name: string) => Promise<void>;
  updateWorkflowDescription: (id: string, description: string) => Promise<void>;
  deleteWorkflow: (id: string) => Promise<void>;
  goToDashboard: () => void;

  /* ── Models ── */
  models: ModelSummary[];
  activeModelId: string | null;
  createModel: (
    name: string,
    provider: ModelProvider,
    modelName: string,
    description?: string
  ) => Promise<void>;
  openModel: (id: string) => void;
  renameModel: (id: string, name: string) => Promise<void>;
  updateModelDescription: (id: string, description: string) => Promise<void>;
  updateModelConfig: (
    id: string,
    provider: ModelProvider,
    modelName: string
  ) => Promise<void>;
  deleteModel: (id: string) => Promise<void>;

  /* ── Skills ── */
  skills: SkillSummary[];
  activeSkillId: string | null;
  createSkill: (name: string, description?: string) => Promise<void>;
  openSkill: (id: string) => void;
  renameSkill: (id: string, name: string) => Promise<void>;
  updateSkillDescription: (id: string, description: string) => Promise<void>;
  deleteSkill: (id: string) => Promise<void>;

  /* ── Agents ── */
  agents: AgentSummary[];
  activeAgentId: string | null;
  createAgent: (
    name: string,
    description?: string,
    modelId?: string,
    fallbackModelId?: string
  ) => Promise<void>;
  openAgent: (id: string) => void;
  renameAgent: (id: string, name: string) => Promise<void>;
  updateAgentDescription: (id: string, description: string) => Promise<void>;
  updateAgentPersona: (id: string, persona: string) => Promise<void>;
  updateAgentModels: (
    id: string,
    modelId?: string,
    fallbackModelId?: string
  ) => Promise<void>;
  deleteAgent: (id: string) => Promise<void>;

  /* ── Orca (chat assistant) ── */
  orcaModelId: string | null;
  orcaFallbackModelId: string | null;
  setOrcaModels: (modelId: string | null, fallbackModelId: string | null) => void;

  /* ── Inbox ── */
  inboxMessages: InboxMessage[];
  markInboxRead: (id: string) => void;
  toggleInboxUnread: (id: string) => void;
  toggleInboxImportant: (id: string) => void;

  /* ── Cron Jobs ── */
  cronJobs: CronJobSummary[];
  activeCronJobId: string | null;
  createCronJob: (name: string, description?: string) => Promise<void>;
  openCronJob: (id: string) => void;
  renameCronJob: (id: string, name: string) => Promise<void>;
  updateCronJobDescription: (
    id: string,
    description: string
  ) => Promise<void>;
  deleteCronJob: (id: string) => Promise<void>;

  /* ── Editor (per-workflow) ── */
  nodes: BlockNode[];
  edges: BlockEdge[];
  selectedNodeId: string | null;

  onNodesChange: OnNodesChange<BlockNode>;
  onEdgesChange: OnEdgesChange<BlockEdge>;
  onConnect: (connection: Connection) => void;

  addNode: (kind: BlockKind, position: { x: number; y: number }) => string;
  /** Creates a new agent entity + places a node wired to its id. Returns the new node id. */
  spawnNewAgent: (position: { x: number; y: number }) => Promise<string>;
  /** Places a node wired to an existing agent entity id. Returns the new node id. */
  spawnAgentRef: (agentId: string, position: { x: number; y: number }) => string;
  /** Points an existing agent node at a different agent entity (used to
   *  recover from a deleted-entity broken state). */
  relinkAgentNode: (nodeId: string, agentId: string) => void;
  removeNode: (id: string) => void;
  /** Merges into `data.fields` only. Top-level `data` keys (label,
   *  agentId, routes, kind) have dedicated setters so the persisted
   *  shape stays explicit. */
  updateNodeData: (
    id: string,
    fields: Partial<BlockNodeData["fields"]>
  ) => void;
  updateNodeLabel: (id: string, label: string) => void;
  updateNodeRoutes: (
    id: string,
    routes: NonNullable<BlockNodeData["routes"]>
  ) => void;
  setSelectedNodeId: (id: string | null) => void;
}

/** Returns a `set()` partial that clears every `active*Id` except the one
 *  named. Centralised so adding a new detail-capable entity doesn't risk
 *  drift across the dozen open/create call sites. */
function clearActiveIdsExcept(
  except:
    | "activeWorkflowId"
    | "activeModelId"
    | "activeSkillId"
    | "activeAgentId"
    | "activeCronJobId"
    | null
) {
  const all = {
    activeWorkflowId: null,
    activeModelId: null,
    activeSkillId: null,
    activeAgentId: null,
    activeCronJobId: null,
  } satisfies Record<string, null>;
  if (except) delete (all as Record<string, unknown>)[except];
  return all;
}

// Local id generators for nodes/edges — these ids live inside a workflow's
// persisted graph payload, not in a separate store, so adapter-assigned ids
// are not required. Timestamp suffix avoids collisions across sessions.
let nodeIdCounter = 0;
function nextNodeId(): string {
  return `node-${++nodeIdCounter}-${Date.now().toString(36)}`;
}

let edgeIdCounter = 0;
function nextEdgeId(): string {
  return `edge-${++edgeIdCounter}-${Date.now().toString(36)}`;
}

/** Builds the canonical agent-node shape. Shared by the two spawn paths
 *  (fresh entity + existing reference) so the node data stays in lockstep. */
function buildAgentNode(
  agentId: string,
  label: string,
  position: { x: number; y: number }
): BlockNode {
  return {
    id: nextNodeId(),
    type: "agent",
    position,
    data: { kind: "agent", label, fields: {}, agentId },
  };
}

/* ── Seed graph ──────────────────────────────────────────────────────── */
// New workflows start empty. The editor surfaces a centred "add your
// first node" affordance that opens the block picker when the graph is
// empty, so there's nothing to seed.

export function buildSeedGraph(): { nodes: BlockNode[]; edges: BlockEdge[] } {
  return { nodes: [], edges: [] };
}

/* ── Debounced graph saver ───────────────────────────────────────────── */
// Collapses rapid graph mutations (drags, field edits, connect/disconnect)
// into a single adapter write ~500ms after the user stops interacting. It
// reads store state at flush time — not at schedule time — so callers do
// not need to pass the current graph snapshot.

const SAVE_DEBOUNCE_MS = 500;

/* ── Debounced entity persistence ────────────────────────────────────── */
// Agents (and future entity types) accept per-keystroke edits from the
// inspector. We keep optimistic local state but coalesce adapter writes
// so typing "hello" doesn't issue five disk writes.
const ENTITY_SAVE_DEBOUNCE_MS = 500;
const debouncedAgentSavers = new Map<string, () => void>();
function scheduleAgentSave(id: string) {
  let saver = debouncedAgentSavers.get(id);
  if (!saver) {
    saver = debounce(() => {
      const agent = useStudioStore.getState().agents.find((a) => a.id === id);
      if (!agent) return;
      void getStorageAdapter().saveAgent(agent);
    }, ENTITY_SAVE_DEBOUNCE_MS);
    debouncedAgentSavers.set(id, saver);
  }
  saver();
}

const debouncedSaveGraph = debounce(() => {
  const s = useStudioStore.getState();
  if (!s.hydrated || !s.activeWorkflowId) return;
  const summary = s.workflows.find((w) => w.id === s.activeWorkflowId);
  if (!summary) return;

  const now = new Date();
  // Bump local updatedAt so the dashboard reflects recency without reload.
  useStudioStore.setState({
    workflows: s.workflows.map((w) =>
      w.id === s.activeWorkflowId ? { ...w, updatedAt: now } : w
    ),
  });

  const data: WorkflowData = {
    ...summary,
    updatedAt: now,
    graph: { nodes: s.nodes, edges: s.edges },
  };
  void getStorageAdapter().saveWorkflow(data);
}, SAVE_DEBOUNCE_MS);

export const useStudioStore = create<StudioState>((set, get) => ({
  /* ── Hydration ── */
  hydrated: false,

  hydrate: async () => {
    if (get().hydrated) return;
    const adapter = getStorageAdapter();
    let projects = await adapter.listProjects();

    // First visit: seed one project with one example workflow.
    if (projects.length === 0) {
      const proj = await adapter.createProject("My Project");
      const wf = await adapter.createWorkflow(proj.id, "Getting Started");
      await adapter.saveWorkflow({ ...wf, graph: buildSeedGraph() });
      projects = [proj];
    }

    // Fan out list-by-project across every entity type and every project
    // in parallel — on IndexedDB each call is an independent transaction.
    const perProject = await Promise.all(
      projects.map(async (p) => {
        const [wf, m, s, a, c] = await Promise.all([
          adapter.listWorkflows(p.id),
          adapter.listModels(p.id),
          adapter.listSkills(p.id),
          adapter.listAgents(p.id),
          adapter.listCronJobs(p.id),
        ]);
        return { wf, m, s, a, c };
      })
    );
    // `updatedAt` is normalised to a Date so UI date math works regardless
    // of whether the adapter persisted strings or Dates.
    const normalise = <T extends { updatedAt: Date }>(x: T): T => ({
      ...x,
      updatedAt: new Date(x.updatedAt),
    });
    const byRecency = (a: { updatedAt: Date }, b: { updatedAt: Date }) =>
      b.updatedAt.getTime() - a.updatedAt.getTime();
    const workflows = perProject.flatMap((p) => p.wf).map(normalise).sort(byRecency);
    const models = perProject.flatMap((p) => p.m).map(normalise).sort(byRecency);
    const skills = perProject.flatMap((p) => p.s).map(normalise).sort(byRecency);
    const agents = perProject.flatMap((p) => p.a).map(normalise).sort(byRecency);
    const cronJobs = perProject.flatMap((p) => p.c).map(normalise).sort(byRecency);

    set({
      projects,
      workflows,
      models,
      skills,
      agents,
      cronJobs,
      activeProjectId: projects[0].id,
      hydrated: true,
    });
  },

  /* ── Navigation ── */
  currentView: "dashboard",
  sidebarSection: "orca",
  setSidebarSection: (section) => {
    set({
      sidebarSection: section,
      currentView: "dashboard",
      ...clearActiveIdsExcept(null),
    });
  },
  workflows: [],
  activeWorkflowId: null,

  /* ── Projects ── */
  projects: [],
  activeProjectId: "",

  createProject: async () => {
    const adapter = getStorageAdapter();
    const project = await adapter.createProject("New Project");
    const wf = await adapter.createWorkflow(project.id, "Getting Started");
    await adapter.saveWorkflow({ ...wf, graph: buildSeedGraph() });

    const summary: WorkflowSummary = {
      id: wf.id,
      name: wf.name,
      projectId: wf.projectId,
      updatedAt: new Date(),
    };

    set({
      projects: [...get().projects, project],
      workflows: [summary, ...get().workflows],
      activeProjectId: project.id,
    });
  },

  renameProject: async (id, name) => {
    // Optimistic update — UI reflects the new name immediately.
    set({
      projects: get().projects.map((p) =>
        p.id === id ? { ...p, name } : p
      ),
    });
    await getStorageAdapter().renameProject(id, name);
  },

  deleteProject: async (id) => {
    const remaining = get().projects.filter((p) => p.id !== id);
    if (remaining.length === 0) return; // don't delete the last project
    set({
      projects: remaining,
      workflows: get().workflows.filter((w) => w.projectId !== id),
      models: get().models.filter((m) => m.projectId !== id),
      skills: get().skills.filter((s) => s.projectId !== id),
      agents: get().agents.filter((a) => a.projectId !== id),
      cronJobs: get().cronJobs.filter((c) => c.projectId !== id),
      activeProjectId:
        get().activeProjectId === id ? remaining[0].id : get().activeProjectId,
    });
    await getStorageAdapter().deleteProject(id);
  },

  setActiveProject: (id) => {
    set({ activeProjectId: id });
  },

  createWorkflow: async (name?: string, description?: string) => {
    const adapter = getStorageAdapter();
    const projectId = get().activeProjectId;
    const wfName = name?.trim() || "Untitled Workflow";
    const wfDesc = description?.trim() || undefined;
    const wf = await adapter.createWorkflow(projectId, wfName);
    const seed = buildSeedGraph();
    await adapter.saveWorkflow({ ...wf, description: wfDesc, graph: seed });

    const summary: WorkflowSummary = {
      id: wf.id,
      name: wf.name,
      description: wfDesc,
      projectId: wf.projectId,
      updatedAt: new Date(wf.updatedAt),
    };

    set({
      workflows: [summary, ...get().workflows],
      activeWorkflowId: wf.id,
      sidebarSection: "workflows",
      currentView: "editor",
      nodes: seed.nodes,
      edges: seed.edges,
      selectedNodeId: null,
    });
  },

  renameWorkflow: async (id, name) => {
    // Optimistic local update — the top bar's input commits immediately.
    set({
      workflows: get().workflows.map((w) =>
        w.id === id ? { ...w, name } : w
      ),
    });
    // Persist via saveWorkflow so the change survives reload. We need the
    // full WorkflowData payload, so load the current graph from storage
    // first and rewrite the name on top of it.
    const adapter = getStorageAdapter();
    const existing = await adapter.getWorkflow(id);
    if (!existing) return;
    await adapter.saveWorkflow({ ...existing, name });
  },

  deleteWorkflow: async (id) => {
    set({ workflows: get().workflows.filter((w) => w.id !== id) });
    await getStorageAdapter().deleteWorkflow(id);
  },

  openWorkflow: (id) => {
    // Graph is not loaded here — the editor loads it on demand via
    // openWorkflowEditor so the detail page doesn't pay the cost.
    debouncedSaveGraph.flush();
    set({
      ...clearActiveIdsExcept("activeWorkflowId"),
      activeWorkflowId: id,
      sidebarSection: "workflows",
      currentView: "detail",
      selectedNodeId: null,
    });
  },

  /* ── Cron Jobs ──────────────────────────────────────────────────── */
  cronJobs: [],
  activeCronJobId: null,

  createCronJob: async (name, description) => {
    const adapter = getStorageAdapter();
    const projectId = get().activeProjectId;
    const trimmed = name.trim() || "Untitled Cron Job";
    const desc = description?.trim() || undefined;
    const created = await adapter.createCronJob(projectId, trimmed);
    if (desc) await adapter.saveCronJob({ ...created, description: desc });
    const summary: CronJobSummary = {
      ...created,
      description: desc,
      updatedAt: new Date(created.updatedAt),
    };
    set({
      cronJobs: [summary, ...get().cronJobs],
      ...clearActiveIdsExcept("activeCronJobId"),
      activeCronJobId: summary.id,
      sidebarSection: "crons",
      currentView: "detail",
    });
  },

  openCronJob: (id) => {
    set({
      ...clearActiveIdsExcept("activeCronJobId"),
      activeCronJobId: id,
      sidebarSection: "crons",
      currentView: "detail",
    });
  },

  renameCronJob: async (id, name) => {
    set({
      cronJobs: get().cronJobs.map((c) =>
        c.id === id ? { ...c, name } : c
      ),
    });
    const adapter = getStorageAdapter();
    const existing = await adapter.getCronJob(id);
    if (!existing) return;
    await adapter.saveCronJob({ ...existing, name });
  },

  updateCronJobDescription: async (id, description) => {
    const desc = description.trim() || undefined;
    set({
      cronJobs: get().cronJobs.map((c) =>
        c.id === id ? { ...c, description: desc } : c
      ),
    });
    const adapter = getStorageAdapter();
    const existing = await adapter.getCronJob(id);
    if (!existing) return;
    await adapter.saveCronJob({ ...existing, description: desc });
  },

  deleteCronJob: async (id) => {
    set({
      cronJobs: get().cronJobs.filter((c) => c.id !== id),
      activeCronJobId:
        get().activeCronJobId === id ? null : get().activeCronJobId,
    });
    await getStorageAdapter().deleteCronJob(id);
  },

  openWorkflowEditor: async (id) => {
    // Flush any pending save for the workflow we are leaving first, so the
    // outgoing graph is persisted under its own id — not misattributed to
    // the incoming workflow after we swap activeWorkflowId below.
    debouncedSaveGraph.flush();

    const data = await getStorageAdapter().getWorkflow(id);
    if (!data) return;
    set({
      activeWorkflowId: id,
      currentView: "editor",
      nodes: data.graph.nodes,
      edges: data.graph.edges,
      selectedNodeId: null,
    });
  },

  updateWorkflowDescription: async (id, description) => {
    const desc = description.trim() || undefined;
    set({
      workflows: get().workflows.map((w) =>
        w.id === id ? { ...w, description: desc } : w
      ),
    });
    const adapter = getStorageAdapter();
    const existing = await adapter.getWorkflow(id);
    if (!existing) return;
    await adapter.saveWorkflow({ ...existing, description: desc });
  },

  goToDashboard: () => {
    // Flush pending graph edits before clearing activeWorkflowId.
    debouncedSaveGraph.flush();
    set({
      currentView: "dashboard",
      // Orca is the studio's home — landing there avoids showing an empty
      // entity-group pane (e.g. after deleting the last model).
      sidebarSection: "orca",
      ...clearActiveIdsExcept(null),
      selectedNodeId: null,
    });
  },

  /* ── Orca ───────────────────────────────────────────────────────── */
  // Chat-assistant model config is session-only for now; persistence
  // will be added alongside the backend.
  orcaModelId: null,
  orcaFallbackModelId: null,
  setOrcaModels: (modelId, fallbackModelId) => {
    set({
      orcaModelId: modelId || null,
      orcaFallbackModelId: fallbackModelId || null,
    });
  },

  /* ── Inbox ────────────────────────────────────────────────────────── */
  // Seeded from the hardcoded fixture for now. Persistence will arrive
  // when the notifications backend lands.
  inboxMessages: INBOX_MESSAGES,
  markInboxRead: (id) => {
    set({
      inboxMessages: get().inboxMessages.map((m) =>
        m.id === id && m.unread ? { ...m, unread: false } : m
      ),
    });
  },
  toggleInboxUnread: (id) => {
    set({
      inboxMessages: get().inboxMessages.map((m) =>
        m.id === id ? { ...m, unread: !m.unread } : m
      ),
    });
  },
  toggleInboxImportant: (id) => {
    set({
      inboxMessages: get().inboxMessages.map((m) =>
        m.id === id ? { ...m, important: !m.important } : m
      ),
    });
  },

  /* ── Models ──────────────────────────────────────────────────────── */
  models: [],
  activeModelId: null,

  createModel: async (name, provider, modelName, description) => {
    const adapter = getStorageAdapter();
    const projectId = get().activeProjectId;
    const trimmed = name.trim() || "Untitled Model";
    const desc = description?.trim() || undefined;
    const created = await adapter.createModel(
      projectId,
      trimmed,
      provider,
      modelName
    );
    if (desc) await adapter.saveModel({ ...created, description: desc });
    const summary: ModelSummary = {
      ...created,
      description: desc,
      updatedAt: new Date(created.updatedAt),
    };
    set({
      models: [summary, ...get().models],
      ...clearActiveIdsExcept("activeModelId"),
      activeModelId: summary.id,
      sidebarSection: "models",
      currentView: "detail",
    });
  },

  openModel: (id) => {
    set({
      ...clearActiveIdsExcept("activeModelId"),
      activeModelId: id,
      sidebarSection: "models",
      currentView: "detail",
    });
  },

  renameModel: async (id, name) => {
    set({
      models: get().models.map((m) => (m.id === id ? { ...m, name } : m)),
    });
    const adapter = getStorageAdapter();
    const existing = await adapter.getModel(id);
    if (!existing) return;
    await adapter.saveModel({ ...existing, name });
  },

  updateModelDescription: async (id, description) => {
    const desc = description.trim() || undefined;
    set({
      models: get().models.map((m) =>
        m.id === id ? { ...m, description: desc } : m
      ),
    });
    const adapter = getStorageAdapter();
    const existing = await adapter.getModel(id);
    if (!existing) return;
    await adapter.saveModel({ ...existing, description: desc });
  },

  updateModelConfig: async (id, provider, modelName) => {
    set({
      models: get().models.map((m) =>
        m.id === id ? { ...m, provider, modelName } : m
      ),
    });
    const adapter = getStorageAdapter();
    const existing = await adapter.getModel(id);
    if (!existing) return;
    await adapter.saveModel({ ...existing, provider, modelName });
  },

  deleteModel: async (id) => {
    set({
      models: get().models.filter((m) => m.id !== id),
      activeModelId: get().activeModelId === id ? null : get().activeModelId,
    });
    await getStorageAdapter().deleteModel(id);
  },

  /* ── Skills ──────────────────────────────────────────────────────── */
  skills: [],
  activeSkillId: null,

  createSkill: async (name, description) => {
    const adapter = getStorageAdapter();
    const projectId = get().activeProjectId;
    const trimmed = name.trim() || "Untitled Skill";
    const desc = description?.trim() || undefined;
    const created = await adapter.createSkill(projectId, trimmed);
    if (desc) await adapter.saveSkill({ ...created, description: desc });
    const summary: SkillSummary = {
      ...created,
      description: desc,
      updatedAt: new Date(created.updatedAt),
    };
    set({
      skills: [summary, ...get().skills],
      ...clearActiveIdsExcept("activeSkillId"),
      activeSkillId: summary.id,
      sidebarSection: "skills",
      currentView: "detail",
    });
  },

  openSkill: (id) => {
    set({
      ...clearActiveIdsExcept("activeSkillId"),
      activeSkillId: id,
      sidebarSection: "skills",
      currentView: "detail",
    });
  },

  renameSkill: async (id, name) => {
    set({
      skills: get().skills.map((s) => (s.id === id ? { ...s, name } : s)),
    });
    const adapter = getStorageAdapter();
    const existing = await adapter.getSkill(id);
    if (!existing) return;
    await adapter.saveSkill({ ...existing, name });
  },

  updateSkillDescription: async (id, description) => {
    const desc = description.trim() || undefined;
    set({
      skills: get().skills.map((s) =>
        s.id === id ? { ...s, description: desc } : s
      ),
    });
    const adapter = getStorageAdapter();
    const existing = await adapter.getSkill(id);
    if (!existing) return;
    await adapter.saveSkill({ ...existing, description: desc });
  },

  deleteSkill: async (id) => {
    set({
      skills: get().skills.filter((s) => s.id !== id),
      activeSkillId: get().activeSkillId === id ? null : get().activeSkillId,
    });
    await getStorageAdapter().deleteSkill(id);
  },

  /* ── Agents ──────────────────────────────────────────────────────── */
  agents: [],
  activeAgentId: null,

  createAgent: async (name, description, modelId, fallbackModelId) => {
    const adapter = getStorageAdapter();
    const projectId = get().activeProjectId;
    const trimmed = name.trim() || "Untitled Agent";
    const desc = description?.trim() || undefined;
    const created = await adapter.createAgent(projectId, trimmed);
    const payload: AgentSummary = {
      ...created,
      description: desc,
      modelId: modelId || undefined,
      fallbackModelId: fallbackModelId || undefined,
    };
    if (desc || modelId || fallbackModelId) {
      await adapter.saveAgent(payload);
    }
    const summary: AgentSummary = {
      ...payload,
      updatedAt: new Date(created.updatedAt),
    };
    set({
      agents: [summary, ...get().agents],
      ...clearActiveIdsExcept("activeAgentId"),
      activeAgentId: summary.id,
      sidebarSection: "agents",
      currentView: "detail",
    });
  },

  openAgent: (id) => {
    set({
      ...clearActiveIdsExcept("activeAgentId"),
      activeAgentId: id,
      sidebarSection: "agents",
      currentView: "detail",
    });
  },

  renameAgent: async (id, name) => {
    set({
      agents: get().agents.map((a) => (a.id === id ? { ...a, name } : a)),
    });
    scheduleAgentSave(id);
  },

  updateAgentDescription: async (id, description) => {
    const desc = description.trim() || undefined;
    set({
      agents: get().agents.map((a) =>
        a.id === id ? { ...a, description: desc } : a
      ),
    });
    scheduleAgentSave(id);
  },

  updateAgentModels: async (id, modelId, fallbackModelId) => {
    set({
      agents: get().agents.map((a) =>
        a.id === id
          ? {
              ...a,
              modelId: modelId || undefined,
              fallbackModelId: fallbackModelId || undefined,
            }
          : a
      ),
    });
    scheduleAgentSave(id);
  },

  updateAgentPersona: async (id, persona) => {
    const p = persona || undefined;
    set({
      agents: get().agents.map((a) =>
        a.id === id ? { ...a, persona: p } : a
      ),
    });
    scheduleAgentSave(id);
  },

  deleteAgent: async (id) => {
    set({
      agents: get().agents.filter((a) => a.id !== id),
      activeAgentId: get().activeAgentId === id ? null : get().activeAgentId,
    });
    await getStorageAdapter().deleteAgent(id);
  },

  /* ── Editor ── */
  nodes: [],
  edges: [],
  selectedNodeId: null,

  onNodesChange: (changes) => {
    set({ nodes: applyNodeChanges(changes, get().nodes) });
    debouncedSaveGraph();
  },

  onEdgesChange: (changes) => {
    set({ edges: applyEdgeChanges(changes, get().edges) });
    debouncedSaveGraph();
  },

  onConnect: (connection) => {
    const newEdge: BlockEdge = {
      id: nextEdgeId(),
      source: connection.source,
      target: connection.target,
      sourceHandle: connection.sourceHandle,
      targetHandle: connection.targetHandle,
      animated: false,
      data: { accentColor: AGENT_HANDLE_COLOR },
    };
    set({ edges: [...get().edges, newEdge] });
    debouncedSaveGraph();
  },

  addNode: (kind, position) => {
    const def = BLOCK_DEFS[kind];
    const defaultFields: Record<string, string | number> = {};
    for (const field of def.fields) {
      if (field.defaultValue !== undefined) {
        defaultFields[field.key] = field.defaultValue;
      }
    }

    const id = nextNodeId();
    // Branch nodes carry dynamic route rows (rendered by BranchNode) in
    // addition to the standard fields. Seed with a single "default" route
    // so the node is useful immediately after drop.
    const routes: BlockNodeData["routes"] | undefined =
      kind === "branch"
        ? [{ id: `route-${Date.now().toString(36)}`, key: "default" }]
        : undefined;
    const newNode: BlockNode = {
      id,
      type: kind,
      position,
      data: {
        kind,
        label: def.label,
        fields: defaultFields,
        ...(routes ? { routes } : {}),
      },
    };
    set({ nodes: [...get().nodes, newNode] });
    debouncedSaveGraph();
    return id;
  },

  spawnNewAgent: async (position) => {
    const adapter = getStorageAdapter();
    const projectId = get().activeProjectId;
    const created = await adapter.createAgent(projectId, "Untitled Agent");
    const summary: AgentSummary = {
      ...created,
      updatedAt: new Date(created.updatedAt),
    };
    const node = buildAgentNode(summary.id, summary.name, position);
    set({
      agents: [summary, ...get().agents],
      nodes: [...get().nodes, node],
    });
    debouncedSaveGraph();
    return node.id;
  },

  spawnAgentRef: (agentId, position) => {
    const agent = get().agents.find((a) => a.id === agentId);
    const node = buildAgentNode(agentId, agent?.name ?? "Agent", position);
    set({ nodes: [...get().nodes, node] });
    debouncedSaveGraph();
    return node.id;
  },

  relinkAgentNode: (nodeId, agentId) => {
    set({
      nodes: get().nodes.map((n) =>
        n.id === nodeId
          ? { ...n, data: { ...n.data, agentId } }
          : n
      ),
    });
    debouncedSaveGraph();
  },

  removeNode: (id) => {
    set({
      nodes: get().nodes.filter((n) => n.id !== id),
      edges: get().edges.filter((e) => e.source !== id && e.target !== id),
      selectedNodeId: get().selectedNodeId === id ? null : get().selectedNodeId,
    });
    debouncedSaveGraph();
  },

  updateNodeData: (id, fields) => {
    set({
      nodes: get().nodes.map((node) => {
        if (node.id !== id) return node;
        const merged: Record<string, string | number> = {
          ...node.data.fields,
        };
        for (const [k, v] of Object.entries(fields)) {
          if (v !== undefined) merged[k] = v;
        }
        return { ...node, data: { ...node.data, fields: merged } };
      }),
    });
    debouncedSaveGraph();
  },

  updateNodeLabel: (id, label) => {
    set({
      nodes: get().nodes.map((node) =>
        node.id === id ? { ...node, data: { ...node.data, label } } : node
      ),
    });
    debouncedSaveGraph();
  },

  updateNodeRoutes: (id, routes) => {
    // Drop edges whose source handle referenced a removed route row so
    // the graph doesn't keep dangling edges pointing at non-existent
    // handles. We match handle ids `route-<routeId>` against the new set.
    const validHandleIds = new Set(routes.map((r) => `route-${r.id}`));
    set({
      nodes: get().nodes.map((node) =>
        node.id === id ? { ...node, data: { ...node.data, routes } } : node
      ),
      edges: get().edges.filter(
        (e) =>
          e.source !== id ||
          !e.sourceHandle?.startsWith("route-") ||
          validHandleIds.has(e.sourceHandle)
      ),
    });
    debouncedSaveGraph();
  },

  setSelectedNodeId: (id) => {
    set({ selectedNodeId: id });
  },
}));
