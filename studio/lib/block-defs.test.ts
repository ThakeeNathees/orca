import { describe, it, expect } from "vitest";
import { BLOCK_DEFS, BLOCK_KINDS, PALETTE_GROUPS, canConnect } from "./block-defs";
import type { BlockKind } from "./types";

describe("BLOCK_DEFS", () => {
  it("has a definition for every BlockKind", () => {
    const kinds: BlockKind[] = [
      "agent", "web_search", "code_exec", "api_request",
      "sql_query", "knowledge", "workflow", "input",
      "schema", "cron", "webhook", "chat", "custom_tool", "branch",
    ];
    for (const kind of kinds) {
      expect(BLOCK_DEFS[kind], `Missing BlockDef for "${kind}"`).toBeDefined();
      expect(BLOCK_DEFS[kind].label).toBeTruthy();
      expect(BLOCK_DEFS[kind].icon).toBeTruthy();
    }
  });

  it("BLOCK_KINDS matches BLOCK_DEFS keys", () => {
    expect(BLOCK_KINDS.sort()).toEqual(Object.keys(BLOCK_DEFS).sort());
  });
});

describe("PALETTE_GROUPS", () => {
  it("references only valid block kinds", () => {
    for (const group of PALETTE_GROUPS) {
      for (const kind of group.kinds) {
        expect(
          BLOCK_DEFS[kind],
          `Palette group "${group.label}" references unknown kind "${kind}"`
        ).toBeDefined();
      }
    }
  });
});

describe("canConnect", () => {
  it("allows agent → agent flow", () => {
    expect(canConnect("agent", "agent")).toBe(true);
  });

  it("allows trigger → agent flow", () => {
    expect(canConnect("trigger", "agent")).toBe(true);
  });

  it("rejects targeting a non-agent type", () => {
    expect(canConnect("agent", "trigger")).toBe(false);
  });
});
