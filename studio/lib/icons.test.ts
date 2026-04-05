import { describe, it, expect } from "vitest";
import { ICON_MAP } from "./icons";
import { BLOCK_DEFS } from "./block-defs";
import type { BlockKind } from "./types";

describe("ICON_MAP", () => {
  it("has an icon entry for every BlockKind", () => {
    const kinds = Object.keys(BLOCK_DEFS) as BlockKind[];
    for (const kind of kinds) {
      const def = BLOCK_DEFS[kind];
      expect(
        ICON_MAP[def.icon],
        `Missing icon "${def.icon}" for block kind "${kind}"`
      ).toBeDefined();
    }
  });
});
