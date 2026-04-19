"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { Search, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { BLOCK_DEFS, PALETTE_GROUPS } from "@/lib/block-defs";
import type { BlockKind } from "@/lib/types";
import { ICON_MAP } from "@/lib/icons";

/** Pixel offset from the edge of the viewport to keep the menu fully on-screen. */
const EDGE_PADDING = 8;
const MENU_WIDTH = 260;
const MENU_MAX_HEIGHT = 420;

export function BlockPickerMenu({
  anchor,
  onPick,
  onClose,
}: {
  /** Screen-space anchor point (clientX/clientY). */
  anchor: { x: number; y: number };
  onPick: (kind: BlockKind) => void;
  onClose: () => void;
}) {
  const [query, setQuery] = useState("");
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(PALETTE_GROUPS.map((g) => [g.label, true]))
  );
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Clamp the menu inside the viewport so it doesn't spill off the edges
  // when the user right-clicks near the bottom-right corner.
  const pos = useMemo(() => {
    if (typeof window === "undefined") return { left: anchor.x, top: anchor.y };
    const maxLeft = window.innerWidth - MENU_WIDTH - EDGE_PADDING;
    const maxTop = window.innerHeight - MENU_MAX_HEIGHT - EDGE_PADDING;
    return {
      left: Math.min(Math.max(anchor.x, EDGE_PADDING), Math.max(maxLeft, EDGE_PADDING)),
      top: Math.min(Math.max(anchor.y, EDGE_PADDING), Math.max(maxTop, EDGE_PADDING)),
    };
  }, [anchor]);

  const filteredGroups = useMemo(() => {
    const q = query.trim().toLowerCase();
    return PALETTE_GROUPS.map((g) => ({
      ...g,
      kinds: q
        ? g.kinds.filter((k) => {
            const def = BLOCK_DEFS[k];
            return (
              def.label.toLowerCase().includes(q) ||
              def.description.toLowerCase().includes(q)
            );
          })
        : g.kinds,
    })).filter((g) => g.kinds.length > 0);
  }, [query]);

  // Search forces every matching group open so results are visible without
  // extra clicks; the user's manual open/close state resumes once they
  // clear the query.
  const groupOpen = (label: string) =>
    query.trim() ? true : (openGroups[label] ?? true);

  return (
    <div
      className="fixed inset-0 z-[140]"
      onClick={onClose}
      onContextMenu={(e) => {
        e.preventDefault();
        onClose();
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        onContextMenu={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            e.preventDefault();
            onClose();
          }
        }}
        style={{
          left: pos.left,
          top: pos.top,
          width: MENU_WIDTH,
          maxHeight: MENU_MAX_HEIGHT,
        }}
        className="absolute flex flex-col overflow-hidden rounded-lg border border-border bg-card shadow-2xl"
      >
        <div className="flex items-center gap-2 border-b border-border px-2.5 py-2">
          <Search className="size-3.5 shrink-0 text-muted-foreground" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search blocks…"
            className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
        </div>

        <div className="flex-1 overflow-y-auto p-1">
          {filteredGroups.length === 0 ? (
            <p className="px-3 py-6 text-center text-xs text-muted-foreground">
              No blocks match “{query}”.
            </p>
          ) : (
            filteredGroups.map((group) => {
              const open = groupOpen(group.label);
              return (
                <div key={group.label}>
                  <button
                    type="button"
                    onClick={() =>
                      setOpenGroups((prev) => ({
                        ...prev,
                        [group.label]: !(prev[group.label] ?? true),
                      }))
                    }
                    className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-[12px] uppercase tracking-wider text-muted-foreground transition-colors cursor-pointer hover:text-foreground"
                  >
                    <ChevronRight
                      className={cn(
                        "size-3 shrink-0 opacity-70 transition-transform",
                        open && "rotate-90"
                      )}
                    />
                    <span>{group.label}</span>
                  </button>
                  {open && (
                    <div className="flex flex-col">
                      {group.kinds.map((kind) => (
                        <BlockRow
                          key={kind}
                          kind={kind}
                          onPick={() => onPick(kind)}
                        />
                      ))}
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}

function BlockRow({
  kind,
  onPick,
}: {
  kind: BlockKind;
  onPick: () => void;
}) {
  const def = BLOCK_DEFS[kind];
  const Icon = ICON_MAP[def.icon];
  return (
    <button
      type="button"
      onClick={onPick}
      className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm text-foreground/90 transition-colors cursor-pointer hover:bg-accent/40"
    >
      {Icon && <Icon className="size-4 shrink-0 text-muted-foreground/70" />}
      <span className="min-w-0 flex-1 truncate">{def.label}</span>
    </button>
  );
}
