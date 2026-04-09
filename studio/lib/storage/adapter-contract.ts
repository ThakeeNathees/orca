// Reusable conformance suite for StorageAdapter implementations.
//
// Each adapter (in-memory, IndexedDB, future REST) runs this suite so
// behavioural parity is guaranteed: a test that passes for the memory
// adapter must also pass for the IndexedDB adapter. This is how we keep
// store code backend-agnostic with confidence.

import { describe, it, expect, beforeEach } from "vitest";
import type { StorageAdapter, WorkflowData } from "./types";

/**
 * Runs the conformance suite against the adapter produced by `factory`.
 * The factory is invoked before every test so each case starts from a
 * clean backend — no cross-test state leakage.
 */
export function runStorageAdapterContract(
  name: string,
  factory: () => Promise<StorageAdapter> | StorageAdapter
) {
  describe(`StorageAdapter contract: ${name}`, () => {
    let adapter: StorageAdapter;

    beforeEach(async () => {
      adapter = await factory();
    });

    describe("projects", () => {
      it("starts with an empty project list", async () => {
        expect(await adapter.listProjects()).toEqual([]);
      });

      it("creates a project with a fresh id and returns it in listProjects", async () => {
        const p = await adapter.createProject("My Project");
        expect(p.id).toBeTruthy();
        expect(p.name).toBe("My Project");

        const list = await adapter.listProjects();
        expect(list).toHaveLength(1);
        expect(list[0].id).toBe(p.id);
      });

      it("renames an existing project", async () => {
        const p = await adapter.createProject("Old");
        await adapter.renameProject(p.id, "New");

        const list = await adapter.listProjects();
        expect(list[0].name).toBe("New");
      });

      it("deletes a project and its workflows", async () => {
        const p = await adapter.createProject("Doomed");
        await adapter.createWorkflow(p.id, "wf-a");
        await adapter.createWorkflow(p.id, "wf-b");

        await adapter.deleteProject(p.id);
        expect(await adapter.listProjects()).toEqual([]);
        expect(await adapter.listWorkflows(p.id)).toEqual([]);
      });
    });

    describe("workflows", () => {
      it("creates a workflow with an empty graph under a project", async () => {
        const p = await adapter.createProject("P");
        const wf = await adapter.createWorkflow(p.id, "Flow");

        expect(wf.id).toBeTruthy();
        expect(wf.name).toBe("Flow");
        expect(wf.projectId).toBe(p.id);
        expect(wf.graph).toEqual({ nodes: [], edges: [] });

        const summaries = await adapter.listWorkflows(p.id);
        expect(summaries).toHaveLength(1);
        expect(summaries[0].id).toBe(wf.id);
      });

      it("listWorkflows scopes by projectId", async () => {
        const p1 = await adapter.createProject("P1");
        const p2 = await adapter.createProject("P2");
        await adapter.createWorkflow(p1.id, "a");
        await adapter.createWorkflow(p2.id, "b");

        const l1 = await adapter.listWorkflows(p1.id);
        const l2 = await adapter.listWorkflows(p2.id);
        expect(l1).toHaveLength(1);
        expect(l2).toHaveLength(1);
        expect(l1[0].name).toBe("a");
        expect(l2[0].name).toBe("b");
      });

      it("getWorkflow returns null for unknown id", async () => {
        expect(await adapter.getWorkflow("nope")).toBeNull();
      });

      it("saveWorkflow persists graph mutations", async () => {
        const p = await adapter.createProject("P");
        const wf = await adapter.createWorkflow(p.id, "Flow");

        const updated: WorkflowData = {
          ...wf,
          graph: {
            nodes: [
              {
                id: "n1",
                type: "model",
                position: { x: 0, y: 0 },
                data: { kind: "model", label: "gpt4", fields: {} },
              },
            ],
            edges: [],
          },
        };
        await adapter.saveWorkflow(updated);

        const loaded = await adapter.getWorkflow(wf.id);
        expect(loaded?.graph.nodes).toHaveLength(1);
        expect(loaded?.graph.nodes[0].id).toBe("n1");
      });

      it("deleteWorkflow removes the workflow", async () => {
        const p = await adapter.createProject("P");
        const wf = await adapter.createWorkflow(p.id, "Flow");
        await adapter.deleteWorkflow(wf.id);

        expect(await adapter.getWorkflow(wf.id)).toBeNull();
        expect(await adapter.listWorkflows(p.id)).toEqual([]);
      });
    });
  });
}
