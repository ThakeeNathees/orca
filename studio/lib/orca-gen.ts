// Visual graph → `.oc` source generator.
//
// Walks the store's (nodes, edges) tuple and emits a textual Orca program
// that the Go compiler should be able to consume. This is a pure function:
// no I/O, no mutation — the Studio calls it on every graph change to keep
// the code tab in sync.
//
// Design notes
// ────────────
// 1. **Reference resolution via edges.** Studio edges carry semantic meaning
//    through their `(sourceHandle, targetHandle)` pair. We translate them
//    into declarative `.oc` references:
//      - `model-out  → model-in`   → agent's `model = <ident>`
//      - `memory-out → memory-in`  → agent's `memory = <ident>`
//      - `tool-out   → tools-in`   → appends to agent's `tools = [...]`
//      - `trigger-out → agent-in`  → flow edge in `workflow main { ... }`
//      - `agent-out   → agent-in`  → flow edge in `workflow main { ... }`
//        (works across agent→agent AND agent→tool when the target is a
//        tool node connected via its left/right "workflow" handles, not
//        the top `tool-out` slot — this is how the Studio distinguishes
//        "use tool from inside agent" from "pipe writer output into DB".)
//
// 2. **Identifier sanitisation.** Node labels are user-facing and can be
//    anything. We lowercase, snake_case, strip leading digits, and dedupe
//    collisions with `_2`, `_3`. Reserved Orca keywords are avoided by
//    starting the dedupe counter at `_1`.
//
// 3. **Emission order.** Declarations must precede references, so we
//    walk a fixed kind order: models → tools → knowledge → memory →
//    agents → triggers → (synthesized) workflow. Within each kind we
//    keep node insertion order for deterministic output.
//
// 4. **Field alignment.** Within every block the `=` signs line up on the
//    longest key. Matches the style in `compiler/codegen/testdata/golden/*.oc`.

import type { BlockNode, BlockEdge, BlockKind } from "./types";
import { BLOCK_DEFS } from "./block-defs";

// Keywords and a few literals that cannot be used as bare identifiers
// without the parser misinterpreting them as new blocks or constants.
const RESERVED = new Set<string>([
  "model",
  "agent",
  "tool",
  "knowledge",
  "memory",
  "workflow",
  "cron",
  "webhook",
  "chat",
  "schema",
  "input",
  "let",
  "true",
  "false",
  "null",
  "builtin",
]);

// Studio BlockKind → Orca block keyword. Kinds absent from this map
// are not emitted as top-level blocks (e.g. `input` has no standalone
// form; `workflow` is synthesized separately after the main loop).
const KEYWORD_FOR_KIND: Partial<Record<BlockKind, string>> = {
  model: "model",
  agent: "agent",
  web_search: "tool",
  code_exec: "tool",
  api_request: "tool",
  sql_query: "tool",
  custom_tool: "tool",
  knowledge: "knowledge",
  memory: "memory",
  cron: "cron",
  webhook: "webhook",
  chat: "chat",
  schema: "schema",
};

// Emission order — each kind must come AFTER every kind whose idents it
// may reference. Agents reference models/tools/memory/knowledge, so all
// of those precede `agent`. Triggers are independent but belong near the
// workflow block for readability.
const EMIT_KINDS: BlockKind[] = [
  "model",
  "web_search",
  "code_exec",
  "api_request",
  "sql_query",
  "custom_tool",
  "knowledge",
  "memory",
  "agent",
  "cron",
  "webhook",
  "chat",
  "schema",
];

/** Turns a human-readable label into a snake_case Orca identifier. */
export function sanitizeIdent(label: string): string {
  let id = label.trim().toLowerCase().replace(/[^a-z0-9_]+/g, "_");
  id = id.replace(/_+/g, "_").replace(/^_+|_+$/g, "");
  if (id.length === 0) id = "unnamed";
  // Idents cannot start with a digit — prefix with underscore.
  if (/^[0-9]/.test(id)) id = "_" + id;
  return id;
}

/**
 * Assigns a unique identifier per node. Collisions and reserved-keyword
 * shadows are resolved by appending `_1`, `_2`, … to the sanitized base
 * until a free name is found. Returns a map of nodeId → ident.
 */
function assignIdents(nodes: BlockNode[]): Map<string, string> {
  const used = new Set<string>();
  const out = new Map<string, string>();

  for (const node of nodes) {
    const base = sanitizeIdent(node.data.label);
    // If the raw base clashes with a keyword or an already-used name, walk
    // the suffix counter starting at 1 until we find a free slot.
    let candidate = base;
    let n = 1;
    while (RESERVED.has(candidate) || used.has(candidate)) {
      candidate = `${base}_${n++}`;
    }
    used.add(candidate);
    out.set(node.id, candidate);
  }
  return out;
}

/** Escapes a string for emission inside double-quoted `.oc` literals. */
function escapeString(s: string): string {
  return s
    .replace(/\\/g, "\\\\")
    .replace(/"/g, '\\"')
    .replace(/\n/g, "\\n")
    .replace(/\r/g, "\\r")
    .replace(/\t/g, "\\t");
}

/** Renders a scalar field value in its `.oc` surface form. */
function formatValue(v: string | number): string {
  if (typeof v === "number") return String(v);
  return `"${escapeString(v)}"`;
}

interface FieldEntry {
  key: string;
  /** Already-rendered right-hand side: quoted string, number, ident, or list. */
  rendered: string;
}

/**
 * Emits a block with `=` signs aligned on the longest key. Empty blocks
 * collapse to a single line so golden diffs stay tidy.
 */
function emitBlock(
  keyword: string,
  name: string,
  fields: FieldEntry[]
): string {
  if (fields.length === 0) return `${keyword} ${name} {}`;
  const maxKey = Math.max(...fields.map((f) => f.key.length));
  const body = fields
    .map((f) => `  ${f.key.padEnd(maxKey)} = ${f.rendered}`)
    .join("\n");
  return `${keyword} ${name} {\n${body}\n}`;
}

/**
 * Is this field non-empty enough to emit? Empty strings are skipped (they
 * come from placeholder password/text inputs the user hasn't filled in),
 * but numeric 0 and boolean false are preserved as legitimate values.
 */
function shouldEmitField(v: string | number | undefined | null): boolean {
  if (v === undefined || v === null) return false;
  if (typeof v === "string" && v.length === 0) return false;
  return true;
}

/* ── Main entry point ────────────────────────────────────────────────── */

/**
 * Generates a complete `.oc` source string from a studio graph. The output
 * ends with a trailing newline (POSIX convention) except when the graph is
 * empty, in which case the empty string is returned.
 */
export function generateOrcaSource(
  nodes: BlockNode[],
  edges: BlockEdge[]
): string {
  if (nodes.length === 0) return "";

  const idents = assignIdents(nodes);
  const nodeById = new Map(nodes.map((n) => [n.id, n]));

  /* ---------- Edge resolution ---------- */
  // Per-agent reference maps derived from semantic edges.
  const modelOfAgent = new Map<string, string>(); // agentId → model ident
  const memoryOfAgent = new Map<string, string>(); // agentId → memory ident
  const toolsOfAgent = new Map<string, string[]>(); // agentId → [tool idents]
  const knowledgeOfAgent = new Map<string, string[]>(); // agentId → [knowledge idents]

  // Flow edges that become lines inside `workflow main { ... }`.
  const flowEdges: { fromId: string; toId: string }[] = [];

  for (const e of edges) {
    const src = nodeById.get(e.source);
    const tgt = nodeById.get(e.target);
    if (!src || !tgt) continue;
    const srcIdent = idents.get(src.id)!;

    const sh = e.sourceHandle ?? "";
    const th = e.targetHandle ?? "";

    if (sh === "model-out" && th === "model-in") {
      modelOfAgent.set(tgt.id, srcIdent);
      continue;
    }
    if (sh === "memory-out" && th === "memory-in") {
      memoryOfAgent.set(tgt.id, srcIdent);
      continue;
    }
    if (sh === "tool-out" && th === "tools-in") {
      const list = toolsOfAgent.get(tgt.id) ?? [];
      if (!list.includes(srcIdent)) list.push(srcIdent);
      toolsOfAgent.set(tgt.id, list);
      continue;
    }
    if (sh === "knowledge-out" && th === "knowledge-in") {
      const list = knowledgeOfAgent.get(tgt.id) ?? [];
      if (!list.includes(srcIdent)) list.push(srcIdent);
      knowledgeOfAgent.set(tgt.id, list);
      continue;
    }

    // Flow edges: triggers into agents, or agent-to-agent / agent-to-tool
    // via the left/right "workflow" handles. Anything else is ignored.
    const isTriggerFlow = sh === "trigger-out" && th === "agent-in";
    const isAgentFlow = sh === "agent-out" && th === "agent-in";
    if (isTriggerFlow || isAgentFlow) {
      flowEdges.push({ fromId: src.id, toId: tgt.id });
    }
  }

  /* ---------- Emit top-level blocks ---------- */
  const sections: string[] = [];

  for (const kind of EMIT_KINDS) {
    const keyword = KEYWORD_FOR_KIND[kind];
    if (!keyword) continue;

    for (const node of nodes) {
      if (node.data.kind !== kind) continue;
      const name = idents.get(node.id)!;
      const fields: FieldEntry[] = [];

      if (kind === "agent") {
        // Canonical agent field order: model → persona/data → memory →
        // tools → knowledge. The first and last three come from edges,
        // persona comes from the node's own field data.
        const modelIdent = modelOfAgent.get(node.id);
        if (modelIdent) {
          fields.push({ key: "model", rendered: modelIdent });
        }

        const def = BLOCK_DEFS[kind];
        for (const fieldDef of def.fields) {
          const v = node.data.fields[fieldDef.key];
          if (!shouldEmitField(v)) continue;
          fields.push({
            key: fieldDef.key,
            rendered: formatValue(v as string | number),
          });
        }

        const memIdent = memoryOfAgent.get(node.id);
        if (memIdent) {
          fields.push({ key: "memory", rendered: memIdent });
        }

        const tools = toolsOfAgent.get(node.id);
        if (tools && tools.length > 0) {
          fields.push({ key: "tools", rendered: `[${tools.join(", ")}]` });
        }

        const know = knowledgeOfAgent.get(node.id);
        if (know && know.length > 0) {
          fields.push({
            key: "knowledge",
            rendered: `[${know.join(", ")}]`,
          });
        }
      } else if (kind === "schema") {
        // The studio schema node's `name` field duplicates the block
        // identifier, so we intentionally emit an empty body — users can
        // add record fields in the code tab once that's editable.
      } else {
        // Generic path: iterate the BlockDef field order for deterministic
        // output regardless of how the node's field map was mutated.
        const def = BLOCK_DEFS[kind];
        for (const fieldDef of def.fields) {
          const v = node.data.fields[fieldDef.key];
          if (!shouldEmitField(v)) continue;
          fields.push({
            key: fieldDef.key,
            rendered: formatValue(v as string | number),
          });
        }
      }

      sections.push(emitBlock(keyword, name, fields));
    }
  }

  /* ---------- Workflow synthesis ---------- */
  if (flowEdges.length > 0) {
    const lines = flowEdges
      .map(
        (e) => `  ${idents.get(e.fromId)!} -> ${idents.get(e.toId)!}`
      )
      .join("\n");
    sections.push(`workflow main {\n${lines}\n}`);
  }

  if (sections.length === 0) return "";
  return sections.join("\n\n") + "\n";
}
