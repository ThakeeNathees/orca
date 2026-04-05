import { describe, it, expect, beforeEach, vi } from "vitest";
import { useStudioStore } from "./store";

// Mock @xyflow/react — store imports applyNodeChanges/applyEdgeChanges
vi.mock("@xyflow/react", () => ({
  applyNodeChanges: (changes: unknown[], nodes: unknown[]) => nodes,
  applyEdgeChanges: (changes: unknown[], edges: unknown[]) => edges,
}));

function getState() {
  return useStudioStore.getState();
}

describe("Store", () => {
  beforeEach(() => {
    // Reset to initial state by re-importing would be ideal, but Zustand
    // stores are singletons. We'll work with the state as-is and clean up.
  });

  describe("addNode", () => {
    it("creates a node with correct kind and default fields", () => {
      const before = getState().nodes.length;
      getState().addNode("model", { x: 100, y: 200 });
      const nodes = getState().nodes;
      expect(nodes.length).toBe(before + 1);

      const added = nodes[nodes.length - 1];
      expect(added.data.kind).toBe("model");
      expect(added.data.label).toBe("Model");
      expect(added.position).toEqual({ x: 100, y: 200 });
    });
  });

  describe("removeNode", () => {
    it("removes the node and its connected edges", () => {
      // Add a node, then remove it
      getState().addNode("agent", { x: 0, y: 0 });
      const nodes = getState().nodes;
      const added = nodes[nodes.length - 1];
      const nodeId = added.id;

      // Add an edge connected to it
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

    it("clears selectedNodeId if removed node was selected", () => {
      getState().addNode("model", { x: 0, y: 0 });
      const nodeId = getState().nodes[getState().nodes.length - 1].id;
      getState().setSelectedNodeId(nodeId);
      expect(getState().selectedNodeId).toBe(nodeId);

      getState().removeNode(nodeId);
      expect(getState().selectedNodeId).toBeNull();
    });
  });

  describe("updateNodeData", () => {
    it("merges field data correctly", () => {
      getState().addNode("model", { x: 0, y: 0 });
      const nodeId = getState().nodes[getState().nodes.length - 1].id;

      getState().updateNodeData(nodeId, { provider: "openai" });
      getState().updateNodeData(nodeId, { model_name: "gpt-4o" });

      const node = getState().nodes.find((n) => n.id === nodeId)!;
      expect(node.data.fields.provider).toBe("openai");
      expect(node.data.fields.model_name).toBe("gpt-4o");
    });
  });

  describe("updateNodeLabel", () => {
    it("updates the label", () => {
      getState().addNode("model", { x: 0, y: 0 });
      const nodeId = getState().nodes[getState().nodes.length - 1].id;

      getState().updateNodeLabel(nodeId, "my-model");
      const node = getState().nodes.find((n) => n.id === nodeId)!;
      expect(node.data.label).toBe("my-model");
    });
  });

  describe("onConnect", () => {
    it("creates an edge between nodes", () => {
      const before = getState().edges.length;
      getState().onConnect({
        source: "sample-agent",
        target: "sample-model",
        sourceHandle: "agent-out",
        targetHandle: "agent-in",
      });

      expect(getState().edges.length).toBe(before + 1);
      const edge = getState().edges[getState().edges.length - 1];
      expect(edge.source).toBe("sample-agent");
      expect(edge.target).toBe("sample-model");
    });
  });

  describe("createWorkflow", () => {
    it("creates a workflow and switches to editor", () => {
      const before = getState().workflows.length;
      getState().createWorkflow();

      expect(getState().workflows.length).toBe(before + 1);
      expect(getState().currentView).toBe("editor");
      expect(getState().activeWorkflowId).toBeTruthy();
    });
  });

  describe("deleteWorkflow", () => {
    it("removes the workflow", () => {
      getState().createWorkflow();
      const wfId = getState().workflows[0].id;
      const before = getState().workflows.length;

      getState().deleteWorkflow(wfId);
      expect(getState().workflows.length).toBe(before - 1);
      expect(getState().workflows.find((w) => w.id === wfId)).toBeUndefined();
    });
  });

  describe("createProject", () => {
    it("creates a project and sets it active", () => {
      const before = getState().projects.length;
      getState().createProject();

      expect(getState().projects.length).toBe(before + 1);
      const newest = getState().projects[getState().projects.length - 1];
      expect(getState().activeProjectId).toBe(newest.id);
    });
  });

  describe("renameProject", () => {
    it("renames the project", () => {
      getState().createProject();
      const proj = getState().projects[getState().projects.length - 1];
      getState().renameProject(proj.id, "Renamed");

      const updated = getState().projects.find((p) => p.id === proj.id)!;
      expect(updated.name).toBe("Renamed");
    });
  });

  describe("deleteProject", () => {
    it("deletes the project and its workflows", () => {
      getState().createProject();
      const proj = getState().projects[getState().projects.length - 1];
      // Create a workflow in this project
      getState().setActiveProject(proj.id);
      getState().createWorkflow();
      const wfId = getState().workflows.find(
        (w) => w.projectId === proj.id
      )!.id;

      getState().deleteProject(proj.id);
      expect(getState().projects.find((p) => p.id === proj.id)).toBeUndefined();
      expect(getState().workflows.find((w) => w.id === wfId)).toBeUndefined();
    });

    it("falls back active project when deleting the active one", () => {
      getState().createProject();
      const proj = getState().projects[getState().projects.length - 1];
      getState().setActiveProject(proj.id);
      expect(getState().activeProjectId).toBe(proj.id);

      getState().deleteProject(proj.id);
      expect(getState().activeProjectId).not.toBe(proj.id);
      expect(getState().projects.length).toBeGreaterThan(0);
    });

    it("does not delete the last project", () => {
      // Delete all but one
      while (getState().projects.length > 1) {
        getState().deleteProject(getState().projects[getState().projects.length - 1].id);
      }
      const lastId = getState().projects[0].id;
      getState().deleteProject(lastId);
      expect(getState().projects.length).toBe(1);
    });
  });

  describe("goToDashboard", () => {
    it("switches to dashboard view", () => {
      getState().openWorkflow("wf-1");
      expect(getState().currentView).toBe("editor");

      getState().goToDashboard();
      expect(getState().currentView).toBe("dashboard");
      expect(getState().activeWorkflowId).toBeNull();
    });
  });
});
