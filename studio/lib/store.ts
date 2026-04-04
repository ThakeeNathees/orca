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
} from "./types";
import { BLOCK_DEFS } from "./block-defs";
import { getEdgeAccentColor } from "./handle-colors";

type StudioView = "dashboard" | "editor";

interface StudioState {
  /* ── Navigation ── */
  currentView: StudioView;
  workflows: WorkflowSummary[];
  activeWorkflowId: string | null;

  createWorkflow: () => void;
  openWorkflow: (id: string) => void;
  deleteWorkflow: (id: string) => void;
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
  setSelectedNodeId: (id: string | null) => void;
}

let nodeIdCounter = 0;
function nextNodeId(): string {
  return `node-${++nodeIdCounter}`;
}

let edgeIdCounter = 0;
function nextEdgeId(): string {
  return `edge-${++edgeIdCounter}`;
}

let workflowIdCounter = 2;
function nextWorkflowId(): string {
  return `wf-${++workflowIdCounter}`;
}

/** Nodes are 240px wide. Layout: lone agent centered above a row of model → memory → tools. */
const NODE_W = 240;
const GAP_X = 60;
const ROW_LEFT = 80;
const BOTTOM_Y = 300;
const TOP_Y = 48;

const bottomXs = [0, 1, 2, 3].map((i) => ROW_LEFT + i * (NODE_W + GAP_X));
const rowMidX = ROW_LEFT + (4 * NODE_W + 3 * GAP_X) / 2;
const AGENT_X = rowMidX - NODE_W / 2;

const SAMPLE_NODES: BlockNode[] = [
  {
    id: "sample-agent",
    type: "agent",
    position: { x: AGENT_X, y: TOP_Y },
    data: {
      kind: "agent",
      label: "researcher",
      fields: { persona: "You find and summarize information." },
    },
  },
  {
    id: "sample-model",
    type: "model",
    position: { x: bottomXs[0], y: BOTTOM_Y },
    data: {
      kind: "model",
      label: "gpt4",
      fields: { provider: "openai", model_name: "gpt-4o", temperature: 0.7 },
    },
  },
  {
    id: "sample-memory",
    type: "memory",
    position: { x: bottomXs[1], y: BOTTOM_Y },
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
    id: "sample-tool-1",
    type: "web_search",
    position: { x: bottomXs[2], y: BOTTOM_Y },
    data: {
      kind: "web_search",
      label: "Web Search",
      fields: { provider: "tavily", max_results: 5 },
    },
  },
  {
    id: "sample-tool-2",
    type: "code_exec",
    position: { x: bottomXs[3], y: BOTTOM_Y },
    data: {
      kind: "code_exec",
      label: "Code Interpreter",
      fields: { timeout: 30, sandbox: "docker" },
    },
  },
];

function sampleEdge(
  id: string,
  source: string,
  target: string,
  sourceHandle: string,
  targetHandle: string
): BlockEdge {
  return {
    id,
    source,
    target,
    sourceHandle,
    targetHandle,
    data: {
      accentColor: getEdgeAccentColor(SAMPLE_NODES, source, sourceHandle),
    },
  };
}

const SAMPLE_EDGES: BlockEdge[] = [
  sampleEdge(
    "sample-edge-1",
    "sample-model",
    "sample-agent",
    "model-out",
    "model-in"
  ),
  sampleEdge(
    "sample-edge-2",
    "sample-tool-1",
    "sample-agent",
    "tool-out",
    "tools-in"
  ),
  sampleEdge(
    "sample-edge-3",
    "sample-tool-2",
    "sample-agent",
    "tool-out",
    "tools-in"
  ),
  sampleEdge(
    "sample-edge-4",
    "sample-memory",
    "sample-agent",
    "memory-out",
    "memory-in"
  ),
];

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

/** Seed workflows for the dashboard. */
const SEED_WORKFLOWS: WorkflowSummary[] = [
  {
    id: "wf-1",
    name: "Vector Store RAG",
    updatedAt: new Date(Date.now() - 2 * 60 * 1000),
    color: "#f9a8d4",
  },
  {
    id: "wf-2",
    name: "Basic Prompting",
    updatedAt: new Date(Date.now() - 3 * 60 * 1000),
    color: "#a78bfa",
  },
];

export const useStudioStore = create<StudioState>((set, get) => ({
  /* ── Navigation ── */
  currentView: "dashboard",
  workflows: SEED_WORKFLOWS,
  activeWorkflowId: null,

  createWorkflow: () => {
    const id = nextWorkflowId();
    const wf: WorkflowSummary = {
      id,
      name: "Untitled Workflow",
      updatedAt: new Date(),
      color: randomPastelColor(),
    };
    set({
      workflows: [wf, ...get().workflows],
      activeWorkflowId: id,
      currentView: "editor",
      nodes: [],
      edges: [],
      selectedNodeId: null,
    });
  },

  deleteWorkflow: (id) => {
    set({
      workflows: get().workflows.filter((w) => w.id !== id),
    });
  },

  openWorkflow: (id) => {
    // For now, opening any workflow loads the sample nodes/edges.
    // In the future each workflow will have its own persisted graph.
    set({
      activeWorkflowId: id,
      currentView: "editor",
      nodes: SAMPLE_NODES,
      edges: SAMPLE_EDGES,
      selectedNodeId: null,
    });
  },

  goToDashboard: () => {
    set({
      currentView: "dashboard",
      activeWorkflowId: null,
      selectedNodeId: null,
    });
  },

  /* ── Editor ── */
  nodes: SAMPLE_NODES,
  edges: SAMPLE_EDGES,
  selectedNodeId: null,

  onNodesChange: (changes) => {
    set({ nodes: applyNodeChanges(changes, get().nodes) });
  },

  onEdgesChange: (changes) => {
    set({ edges: applyEdgeChanges(changes, get().edges) });
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
    const newNode: BlockNode = {
      id,
      type: kind,
      position,
      data: {
        kind,
        label: def.label,
        fields: defaultFields,
      },
    };
    set({ nodes: [...get().nodes, newNode] });
  },

  removeNode: (id) => {
    set({
      nodes: get().nodes.filter((n) => n.id !== id),
      edges: get().edges.filter((e) => e.source !== id && e.target !== id),
      selectedNodeId: get().selectedNodeId === id ? null : get().selectedNodeId,
    });
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
  },

  updateNodeLabel: (id, label) => {
    set({
      nodes: get().nodes.map((node) =>
        node.id === id ? { ...node, data: { ...node.data, label } } : node
      ),
    });
  },

  setSelectedNodeId: (id) => {
    set({ selectedNodeId: id });
  },
}));
