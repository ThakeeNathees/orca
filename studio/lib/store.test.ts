import { describe, it, expect, beforeEach, vi } from "vitest";
import { useStudioStore } from "./store";
import { __resetStorageAdapterForTests } from "./storage";

// Mock @xyflow/react — store imports applyNodeChanges/applyEdgeChanges
vi.mock("@xyflow/react", () => ({
  applyNodeChanges: (_changes: unknown[], nodes: unknown[]) => nodes,
  applyEdgeChanges: (_changes: unknown[], edges: unknown[]) => edges,
}));

function getState() {
  return useStudioStore.getState();
}

/**
 * Reset the whole studio singleton (store + memoised adapter) and run
 * hydrate() so each test starts from a deterministic state: one seeded
 * "My Project" with one "Getting Started" workflow. jsdom does not
 * provide `indexedDB`, so the factory hands back a fresh MemoryAdapter.
 */
async function resetAndHydrate() {
  __resetStorageAdapterForTests();
  useStudioStore.setState({
    hydrated: false,
    projects: [],
    workflows: [],
    activeProjectId: "",
    currentView: "dashboard",
    activeWorkflowId: null,
    nodes: [],
    edges: [],
    selectedNodeId: null,
  });
  await getState().hydrate();
}

describe("Store", () => {
  beforeEach(async () => {
    await resetAndHydrate();
  });

  describe("hydrate", () => {
    it("seeds a project and workflow on first visit", () => {
      expect(getState().projects).toHaveLength(1);
      expect(getState().projects[0].name).toBe("My Project");
      expect(getState().workflows).toHaveLength(1);
      expect(getState().workflows[0].name).toBe("Getting Started");
      expect(getState().hydrated).toBe(true);
    });

    it("is idempotent", async () => {
      const countBefore = getState().projects.length;
      await getState().hydrate();
      expect(getState().projects.length).toBe(countBefore);
    });
  });

  describe("addNode", () => {
    it("creates a node with correct kind and default fields", async () => {
      // Open the seeded workflow so the editor has an active graph.
      const wfId = getState().workflows[0].id;
      await getState().openWorkflowEditor(wfId);

      const before = getState().nodes.length;
      getState().addNode("agent", { x: 100, y: 200 });
      const nodes = getState().nodes;
      expect(nodes.length).toBe(before + 1);

      const added = nodes[nodes.length - 1];
      expect(added.data.kind).toBe("agent");
      expect(added.data.label).toBe("Agent");
      expect(added.position).toEqual({ x: 100, y: 200 });
    });
  });

  describe("removeNode", () => {
    it("removes the node and its connected edges", async () => {
      const wfId = getState().workflows[0].id;
      await getState().openWorkflowEditor(wfId);

      getState().addNode("agent", { x: 0, y: 0 });
      const nodes = getState().nodes;
      const added = nodes[nodes.length - 1];
      const nodeId = added.id;

      useStudioStore.setState({
        edges: [
          ...getState().edges,
          {
            id: "test-edge",
            source: nodeId,
            target: "some-other",
            data: {},
          },
        ],
      });

      getState().removeNode(nodeId);
      expect(getState().nodes.find((n) => n.id === nodeId)).toBeUndefined();
      expect(getState().edges.find((e) => e.id === "test-edge")).toBeUndefined();
    });

    it("clears selectedNodeId if removed node was selected", async () => {
      const wfId = getState().workflows[0].id;
      await getState().openWorkflowEditor(wfId);

      getState().addNode("agent", { x: 0, y: 0 });
      const nodeId = getState().nodes[getState().nodes.length - 1].id;
      getState().setSelectedNodeId(nodeId);
      expect(getState().selectedNodeId).toBe(nodeId);

      getState().removeNode(nodeId);
      expect(getState().selectedNodeId).toBeNull();
    });
  });

  describe("updateNodeData", () => {
    it("merges field data correctly", async () => {
      const wfId = getState().workflows[0].id;
      await getState().openWorkflowEditor(wfId);

      getState().addNode("agent", { x: 0, y: 0 });
      const nodeId = getState().nodes[getState().nodes.length - 1].id;

      getState().updateNodeData(nodeId, { provider: "openai" });
      getState().updateNodeData(nodeId, { model_name: "gpt-4o" });

      const node = getState().nodes.find((n) => n.id === nodeId)!;
      expect(node.data.fields.provider).toBe("openai");
      expect(node.data.fields.model_name).toBe("gpt-4o");
    });
  });

  describe("updateNodeLabel", () => {
    it("updates the label", async () => {
      const wfId = getState().workflows[0].id;
      await getState().openWorkflowEditor(wfId);

      getState().addNode("agent", { x: 0, y: 0 });
      const nodeId = getState().nodes[getState().nodes.length - 1].id;

      getState().updateNodeLabel(nodeId, "my-model");
      const node = getState().nodes.find((n) => n.id === nodeId)!;
      expect(node.data.label).toBe("my-model");
    });
  });

  describe("onConnect", () => {
    it("creates an edge between nodes", async () => {
      const wfId = getState().workflows[0].id;
      await getState().openWorkflowEditor(wfId);

      const before = getState().edges.length;
      getState().onConnect({
        source: "seed-researcher",
        target: "seed-writer",
        sourceHandle: "agent-out",
        targetHandle: "agent-in",
      });

      expect(getState().edges.length).toBe(before + 1);
      const edge = getState().edges[getState().edges.length - 1];
      expect(edge.source).toBe("seed-researcher");
      expect(edge.target).toBe("seed-writer");
    });
  });

  describe("createWorkflow", () => {
    it("creates a workflow and switches to detail", async () => {
      const before = getState().workflows.length;
      await getState().createWorkflow();

      expect(getState().workflows.length).toBe(before + 1);
      expect(getState().currentView).toBe("detail");
      expect(getState().activeWorkflowId).toBeTruthy();
    });

    it("persists the new workflow through the adapter", async () => {
      await getState().createWorkflow();
      const id = getState().activeWorkflowId!;

      // Re-hydrate from storage; the workflow should still be there.
      useStudioStore.setState({ hydrated: false });
      await getState().hydrate();
      expect(getState().workflows.find((w) => w.id === id)).toBeTruthy();
    });
  });

  describe("deleteWorkflow", () => {
    it("removes the workflow", async () => {
      await getState().createWorkflow();
      const wfId = getState().activeWorkflowId!;
      const before = getState().workflows.length;

      await getState().deleteWorkflow(wfId);
      expect(getState().workflows.length).toBe(before - 1);
      expect(getState().workflows.find((w) => w.id === wfId)).toBeUndefined();
    });
  });

  describe("createProject", () => {
    it("creates a project and sets it active", async () => {
      const before = getState().projects.length;
      await getState().createProject();

      expect(getState().projects.length).toBe(before + 1);
      const newest = getState().projects[getState().projects.length - 1];
      expect(getState().activeProjectId).toBe(newest.id);
    });
  });

  describe("renameProject", () => {
    it("renames the project", async () => {
      await getState().createProject();
      const proj = getState().projects[getState().projects.length - 1];
      await getState().renameProject(proj.id, "Renamed");

      const updated = getState().projects.find((p) => p.id === proj.id)!;
      expect(updated.name).toBe("Renamed");
    });
  });

  describe("deleteProject", () => {
    it("deletes the project and its workflows", async () => {
      await getState().createProject();
      const proj = getState().projects[getState().projects.length - 1];
      getState().setActiveProject(proj.id);
      await getState().createWorkflow();
      const wfId = getState().workflows.find(
        (w) => w.projectId === proj.id
      )!.id;

      await getState().deleteProject(proj.id);
      expect(getState().projects.find((p) => p.id === proj.id)).toBeUndefined();
      expect(getState().workflows.find((w) => w.id === wfId)).toBeUndefined();
    });

    it("falls back active project when deleting the active one", async () => {
      await getState().createProject();
      const proj = getState().projects[getState().projects.length - 1];
      getState().setActiveProject(proj.id);
      expect(getState().activeProjectId).toBe(proj.id);

      await getState().deleteProject(proj.id);
      expect(getState().activeProjectId).not.toBe(proj.id);
      expect(getState().projects.length).toBeGreaterThan(0);
    });

    it("does not delete the last project", async () => {
      while (getState().projects.length > 1) {
        await getState().deleteProject(
          getState().projects[getState().projects.length - 1].id
        );
      }
      const lastId = getState().projects[0].id;
      await getState().deleteProject(lastId);
      expect(getState().projects.length).toBe(1);
    });
  });

  describe("goToDashboard", () => {
    it("switches to dashboard view", async () => {
      const wfId = getState().workflows[0].id;
      await getState().openWorkflowEditor(wfId);
      expect(getState().currentView).toBe("editor");

      getState().goToDashboard();
      expect(getState().currentView).toBe("dashboard");
      expect(getState().activeWorkflowId).toBeNull();
    });
  });

  describe("openWorkflow", () => {
    it("loads the workflow's graph from storage", async () => {
      const wfId = getState().workflows[0].id;
      await getState().openWorkflowEditor(wfId);

      expect(getState().activeWorkflowId).toBe(wfId);
      expect(getState().currentView).toBe("editor");
      // Seed graph: webhook → researcher → web_search → writer → sql (5 nodes).
      expect(getState().nodes.length).toBe(5);
      // Flow edges: trigger + 3 agent-to-next = 4.
      expect(getState().edges.length).toBe(4);
    });
  });
});
