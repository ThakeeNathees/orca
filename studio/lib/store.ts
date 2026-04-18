import { create } from "zustand";
import {
  applyNodeChanges,
  applyEdgeChanges,
  type OnNodesChange,
  type OnEdgesChange,
  type Connection,
} from "@xyflow/react";
import type {
  BlockNode,
  BlockEdge,
  BlockKind,
  BlockNodeData,
  WorkflowSummary,
  Project,
} from "./types";
import { BLOCK_DEFS } from "./block-defs";
import { getEdgeAccentColor } from "./handle-colors";
import { getStorageAdapter } from "./storage";
import type { WorkflowData } from "./storage/types";
import { debounce } from "./debounce";

type StudioView = "dashboard" | "editor";

interface StudioState {
  /* ── Hydration ── */
  /** Becomes true once the store has loaded projects/workflows from storage. */
  hydrated: boolean;
  /** Loads projects/workflows from the storage adapter. Idempotent. */
  hydrate: () => Promise<void>;

  /* ── Navigation ── */
  currentView: StudioView;
  workflows: WorkflowSummary[];
  activeWorkflowId: string | null;

  /* ── Projects ── */
  projects: Project[];
  activeProjectId: string;
  createProject: () => Promise<void>;
  renameProject: (id: string, name: string) => Promise<void>;
  deleteProject: (id: string) => Promise<void>;
  setActiveProject: (id: string) => void;

  createWorkflow: (name?: string) => Promise<void>;
  openWorkflow: (id: string) => Promise<void>;
  renameWorkflow: (id: string, name: string) => Promise<void>;
  deleteWorkflow: (id: string) => Promise<void>;
  goToDashboard: () => void;

  /* ── Editor (per-workflow) ── */
  nodes: BlockNode[];
  edges: BlockEdge[];
  selectedNodeId: string | null;

  onNodesChange: OnNodesChange<BlockNode>;
  onEdgesChange: OnEdgesChange<BlockEdge>;
  onConnect: (connection: Connection) => void;

  addNode: (kind: BlockKind, position: { x: number; y: number }) => void;
  removeNode: (id: string) => void;
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

const PASTEL_COLORS = [
  "#f9a8d4", // pink
  "#a78bfa", // purple
  "#93c5fd", // blue
  "#6ee7b7", // green
  "#fcd34d", // yellow
  "#fdba74", // orange
  "#f87171", // red
  "#67e8f9", // cyan
  "#c4b5fd", // violet
  "#86efac", // mint
];

function randomPastelColor(): string {
  return PASTEL_COLORS[Math.floor(Math.random() * PASTEL_COLORS.length)];
}

/* ── Seed graph ──────────────────────────────────────────────────────── */
// Example pipeline used ONLY to seed fresh workflows. Shows a typical
// research→write→persist flow:
//
//   webhook → researcher → writer → sql_query
//                ↑            ↑
//                └─── model ──┘   (shared between the two agents)
//           memory ─┘   web_search ─┘
//
// The writer → sql_query edge uses the agent-type handles on the tool
// (left/right, purple) — NOT the tool-out slot — so it reads as "writer
// pushes its output into the database" rather than "sql is a tool the
// writer can invoke".

const TOP_Y = 48;
const BOTTOM_Y = 340;

// Top row: webhook → researcher → writer → sql_query. Node width is
// 240px so a 300px stride leaves a 60px gap between cards.
const TOP_X = [40, 340, 640, 940];
// Bottom row: memory, shared model, web_search — staggered under the agents.
const BOTTOM_X = [220, 490, 760];

export function buildSeedGraph(): { nodes: BlockNode[]; edges: BlockEdge[] } {
  const nodes: BlockNode[] = [
    {
      id: "seed-webhook",
      type: "webhook",
      position: { x: TOP_X[0], y: TOP_Y },
      data: {
        kind: "webhook",
        label: "Webhook",
        fields: { path: "/api/research", method: "POST" },
      },
    },
    {
      id: "seed-researcher",
      type: "agent",
      position: { x: TOP_X[1], y: TOP_Y },
      data: {
        kind: "agent",
        label: "researcher",
        fields: { persona: "You find and summarize information." },
      },
    },
    {
      id: "seed-writer",
      type: "agent",
      position: { x: TOP_X[2], y: TOP_Y },
      data: {
        kind: "agent",
        label: "writer",
        fields: {
          persona:
            "You turn research notes into a polished article, then store it.",
        },
      },
    },
    {
      id: "seed-sql",
      type: "sql_query",
      position: { x: TOP_X[3], y: TOP_Y },
      data: {
        kind: "sql_query",
        label: "Articles DB",
        fields: {
          connection: "postgresql://user:pass@host/db",
          dialect: "postgresql",
          max_rows: 100,
        },
      },
    },
    {
      id: "seed-memory",
      type: "memory",
      position: { x: BOTTOM_X[0], y: BOTTOM_Y },
      data: {
        kind: "memory",
        label: "Session memory",
        fields: {
          name: "session_store",
          desc: "Conversation and tool-call history",
        },
      },
    },
    {
      id: "seed-model",
      type: "model",
      position: { x: BOTTOM_X[1], y: BOTTOM_Y },
      data: {
        kind: "model",
        label: "gpt4",
        fields: { provider: "openai", model_name: "gpt-4o", temperature: 0.7 },
      },
    },
    {
      id: "seed-web-search",
      type: "web_search",
      position: { x: BOTTOM_X[2], y: BOTTOM_Y },
      data: {
        kind: "web_search",
        label: "Web Search",
        fields: { provider: "tavily", max_results: 5 },
      },
    },
  ];

  const mkEdge = (
    id: string,
    source: string,
    target: string,
    sourceHandle: string,
    targetHandle: string
  ): BlockEdge => ({
    id,
    source,
    target,
    sourceHandle,
    targetHandle,
    data: { accentColor: getEdgeAccentColor(nodes, source, sourceHandle) },
  });

  const edges: BlockEdge[] = [
    // Top-row flow: webhook → researcher → writer → sql (all agent-type handles).
    mkEdge(
      "seed-edge-trigger",
      "seed-webhook",
      "seed-researcher",
      "trigger-out",
      "agent-in"
    ),
    mkEdge(
      "seed-edge-flow-1",
      "seed-researcher",
      "seed-writer",
      "agent-out",
      "agent-in"
    ),
    // writer → sql via the purple left/right handles (not the tool slot),
    // reading as "writer persists its output into the database".
    mkEdge(
      "seed-edge-flow-2",
      "seed-writer",
      "seed-sql",
      "agent-out",
      "agent-in"
    ),

    // Shared model fans out to both agents.
    mkEdge(
      "seed-edge-model-r",
      "seed-model",
      "seed-researcher",
      "model-out",
      "model-in"
    ),
    mkEdge(
      "seed-edge-model-w",
      "seed-model",
      "seed-writer",
      "model-out",
      "model-in"
    ),

    // Researcher-only memory + web search tool.
    mkEdge(
      "seed-edge-memory",
      "seed-memory",
      "seed-researcher",
      "memory-out",
      "memory-in"
    ),
    mkEdge(
      "seed-edge-tool",
      "seed-web-search",
      "seed-researcher",
      "tool-out",
      "tools-in"
    ),
  ];

  return { nodes, edges };
}

/* ── Debounced graph saver ───────────────────────────────────────────── */
// Collapses rapid graph mutations (drags, field edits, connect/disconnect)
// into a single adapter write ~500ms after the user stops interacting. It
// reads store state at flush time — not at schedule time — so callers do
// not need to pass the current graph snapshot.

const SAVE_DEBOUNCE_MS = 500;

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
      await adapter.saveWorkflow({
        ...wf,
        color: randomPastelColor(),
        graph: buildSeedGraph(),
      });
      projects = [proj];
    }

    // Gather workflow summaries across every project.
    const workflows: WorkflowSummary[] = [];
    for (const p of projects) {
      const list = await adapter.listWorkflows(p.id);
      for (const w of list) {
        workflows.push({
          ...w,
          // Normalise updatedAt in case the adapter stored it as a string.
          updatedAt: new Date(w.updatedAt),
        });
      }
    }
    // Most-recently-updated first — matches dashboard expectation.
    workflows.sort((a, b) => b.updatedAt.getTime() - a.updatedAt.getTime());

    set({
      projects,
      workflows,
      activeProjectId: projects[0].id,
      hydrated: true,
    });
  },

  /* ── Navigation ── */
  currentView: "dashboard",
  workflows: [],
  activeWorkflowId: null,

  /* ── Projects ── */
  projects: [],
  activeProjectId: "",

  createProject: async () => {
    const adapter = getStorageAdapter();
    const project = await adapter.createProject("New Project");
    // Seed the new project with a starter workflow so users land on a
    // non-empty canvas with something to edit or delete.
    const wf = await adapter.createWorkflow(project.id, "Getting Started");
    const color = randomPastelColor();
    await adapter.saveWorkflow({ ...wf, color, graph: buildSeedGraph() });

    const summary: WorkflowSummary = {
      id: wf.id,
      name: wf.name,
      projectId: wf.projectId,
      color,
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
      activeProjectId:
        get().activeProjectId === id ? remaining[0].id : get().activeProjectId,
    });
    await getStorageAdapter().deleteProject(id);
  },

  setActiveProject: (id) => {
    set({ activeProjectId: id });
  },

  createWorkflow: async (name?: string) => {
    const adapter = getStorageAdapter();
    const projectId = get().activeProjectId;
    const wfName = name?.trim() || "Untitled Workflow";
    const wf = await adapter.createWorkflow(projectId, wfName);
    const color = randomPastelColor();
    // Seed with the example graph so new workflows open on a non-empty
    // canvas — users can edit or clear it as a starting point.
    const seed = buildSeedGraph();
    await adapter.saveWorkflow({ ...wf, color, graph: seed });

    const summary: WorkflowSummary = {
      id: wf.id,
      name: wf.name,
      projectId: wf.projectId,
      color,
      updatedAt: new Date(wf.updatedAt),
    };

    set({
      workflows: [summary, ...get().workflows],
      activeWorkflowId: wf.id,
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

  openWorkflow: async (id) => {
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

  goToDashboard: () => {
    // Flush pending graph edits before clearing activeWorkflowId.
    debouncedSaveGraph.flush();
    set({
      currentView: "dashboard",
      activeWorkflowId: null,
      selectedNodeId: null,
    });
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
    const nodes = get().nodes;
    const newEdge: BlockEdge = {
      id: nextEdgeId(),
      source: connection.source,
      target: connection.target,
      sourceHandle: connection.sourceHandle,
      targetHandle: connection.targetHandle,
      animated: false,
      data: {
        accentColor: getEdgeAccentColor(
          nodes,
          connection.source,
          connection.sourceHandle
        ),
      },
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
