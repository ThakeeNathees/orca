import { describe, it, expect } from "vitest";
import { generateOrcaSource, sanitizeIdent } from "./orca-gen";
import { buildSeedGraph } from "./store";
import type { BlockNode, BlockEdge } from "./types";

/**
 * Tiny node constructor — keeps the tests readable by hiding the fields
 * we don't care about (position, React Flow node `type`, etc.).
 */
function n(
  id: string,
  kind: BlockNode["data"]["kind"],
  label: string,
  fields: Record<string, string | number> = {}
): BlockNode {
  return {
    id,
    type: kind,
    position: { x: 0, y: 0 },
    data: { kind, label, fields },
  };
}

function e(
  id: string,
  source: string,
  target: string,
  sourceHandle: string,
  targetHandle: string
): BlockEdge {
  return { id, source, target, sourceHandle, targetHandle, data: {} };
}

describe("sanitizeIdent", () => {
  it("lowercases and snake_cases labels", () => {
    expect(sanitizeIdent("My GPT 4")).toBe("my_gpt_4");
  });

  it("collapses repeated and trailing underscores", () => {
    expect(sanitizeIdent("  foo -- bar  ")).toBe("foo_bar");
  });

  it("prefixes digit-leading names with an underscore", () => {
    expect(sanitizeIdent("4o")).toBe("_4o");
  });

  it("falls back to 'unnamed' for empty input", () => {
    expect(sanitizeIdent("")).toBe("unnamed");
    expect(sanitizeIdent("!!!")).toBe("unnamed");
  });
});

describe("generateOrcaSource", () => {
  it("returns empty string for an empty graph", () => {
    expect(generateOrcaSource([], [])).toBe("");
  });

  it("emits a single agent block with aligned `=`", () => {
    const nodes = [
      n("a1", "agent", "researcher", { persona: "You research." }),
    ];
    const out = generateOrcaSource(nodes, []);
    expect(out).toContain("agent researcher {");
    expect(out).toContain('persona = "You research."');
  });

  it("prepends the repo-url + orca-run header to every non-empty output", () => {
    const nodes = [n("a1", "agent", "researcher", { persona: "p" })];
    const out = generateOrcaSource(nodes, []);
    expect(out.startsWith("// https://github.com/ThakeeNathees/orca\n")).toBe(
      true
    );
    expect(out).toContain("orca run");
  });

  it("prepends @ui(x, y) annotations using rounded node positions", () => {
    const nodes: BlockNode[] = [
      {
        id: "a1",
        type: "agent",
        position: { x: 340.3, y: 120.9 },
        data: { kind: "agent", label: "researcher", fields: { persona: "p" } },
      },
    ];
    const out = generateOrcaSource(nodes, []);
    expect(out).toContain("@ui(340, 121)\nagent researcher {");
  });

  it("skips empty-string field values", () => {
    const nodes = [
      n("t1", "web_search", "search", {
        provider: "tavily",
        max_results: 5,
      }),
      n("t2", "api_request", "api", {
        url: "https://example.com",
        method: "GET",
        headers: "", // empty → not emitted
      }),
    ];
    const out = generateOrcaSource(nodes, []);
    expect(out).not.toContain("headers");
    expect(out).toContain('url    = "https://example.com"');
  });

  it("synthesizes workflow main from trigger and agent-flow edges", () => {
    const nodes = [
      n("w1", "webhook", "hooks_in", { path: "/x" }),
      n("a1", "agent", "researcher", { persona: "p" }),
      n("a2", "agent", "writer", { persona: "p" }),
    ];
    const edges = [
      e("e1", "w1", "a1", "trigger-out", "agent-in"),
      e("e2", "a1", "a2", "agent-out", "agent-in"),
    ];
    const out = generateOrcaSource(nodes, edges);
    expect(out).toContain("workflow main {");
    expect(out).toContain("  hooks_in -> researcher");
    expect(out).toContain("  researcher -> writer");
  });

  it("treats an agent→tool edge as a workflow flow edge", () => {
    const nodes = [
      n("a1", "agent", "writer", { persona: "p" }),
      n("sql", "sql_query", "Articles DB", { dialect: "postgresql" }),
    ];
    const edges = [e("e1", "a1", "sql", "agent-out", "agent-in")];
    const out = generateOrcaSource(nodes, edges);
    expect(out).toContain("workflow main {");
    expect(out).toContain("  writer -> articles_db");
  });

  it("dedupes colliding identifiers with numeric suffixes", () => {
    const nodes = [
      n("a", "agent", "X", { persona: "p" }),
      n("b", "agent", "X", { persona: "p" }),
      n("c", "agent", "X", { persona: "p" }),
    ];
    const out = generateOrcaSource(nodes, []);
    expect(out).toContain("agent x {");
    expect(out).toContain("agent x_1 {");
    expect(out).toContain("agent x_2 {");
  });

  it("suffixes idents that collide with reserved keywords", () => {
    const nodes = [n("w1", "webhook", "webhook", { path: "/x" })];
    const out = generateOrcaSource(nodes, []);
    expect(out).toContain("webhook webhook_1 {");
  });

  it("emits nothing for the fresh seed graph (workflows start empty)", () => {
    const seed = buildSeedGraph();
    expect(generateOrcaSource(seed.nodes, seed.edges)).toBe("");
  });

  it("emits a named branch block with transform and route map, and references it by name in workflow main", () => {
    const nodes: BlockNode[] = [
      n("n1", "agent", "classifier", { persona: "Classify." }),
      n("n2", "agent", "tech writer", { persona: "Tech." }),
      n("n3", "agent", "biz writer", { persona: "Biz." }),
      {
        id: "n4",
        type: "branch",
        position: { x: 0, y: 0 },
        data: {
          kind: "branch",
          label: "router",
          fields: { transform: "\\(out string) -> out" },
          routes: [
            { id: "r1", key: "tech" },
            { id: "r2", key: "business" },
          ],
        },
      },
    ];
    const edges = [
      e("e1", "n1", "n4", "agent-out", "agent-in"),
      e("e2", "n4", "n2", "route-r1", "agent-in"),
      e("e3", "n4", "n3", "route-r2", "agent-in"),
    ];

    const out = generateOrcaSource(nodes, edges);

    expect(out).toContain("branch router {");
    expect(out).toContain("transform = \\(out string) -> out");
    expect(out).toContain('"tech": tech_writer');
    expect(out).toContain('"business": biz_writer');

    expect(out).toContain("classifier -> router");
    expect(out).not.toContain("router -> tech_writer");
    expect(out).not.toContain("router -> biz_writer");
  });
});
