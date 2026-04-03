"use client";

import { useState } from "react";
import { BLOCK_DEFS, PALETTE_GROUPS } from "@/lib/block-defs";
import type { BlockKind } from "@/lib/types";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Input } from "@/components/ui/input";
import {
  Cpu,
  Bot,
  BookOpen,
  GitBranch,
  ArrowRightToLine,
  Braces,
  Clock,
  Globe,
  MessageCircle,
  Zap,
  ChevronRight,
  Search,
  Terminal,
  Send,
  Database,
  Wrench,
  GripVertical,
  type LucideIcon,
} from "lucide-react";

const ICON_MAP: Record<string, LucideIcon> = {
  Cpu,
  Bot,
  BookOpen,
  GitBranch,
  ArrowRightToLine,
  Braces,
  Clock,
  Globe,
  MessageCircle,
  Search,
  Terminal,
  Send,
  Database,
};

const GROUP_ICONS: Record<string, LucideIcon> = {
  "Models & Agents": Bot,
  Tools: Wrench,
  Data: BookOpen,
  Triggers: Zap,
};

function PaletteItem({ kind }: { kind: BlockKind }) {
  const def = BLOCK_DEFS[kind];
  const Icon = ICON_MAP[def.icon];

  function onDragStart(e: React.DragEvent) {
    e.dataTransfer.setData("application/orca-block-kind", kind);
    e.dataTransfer.effectAllowed = "move";
  }

  return (
    <div
      draggable
      onDragStart={onDragStart}
      className="group flex cursor-grab items-center gap-2 rounded-md px-2.5 py-1.5 text-[13px] transition-colors hover:bg-accent active:cursor-grabbing"
    >
      <GripVertical className="h-3 w-3 shrink-0 text-muted-foreground/0 transition-colors group-hover:text-muted-foreground/40" />
      {Icon && (
        <Icon className="h-4 w-4 shrink-0 text-muted-foreground/70" />
      )}
      <span className="truncate text-foreground/90">{def.label}</span>
    </div>
  );
}

function PaletteGroup({
  label,
  kinds,
  searchQuery,
}: {
  label: string;
  kinds: BlockKind[];
  searchQuery: string;
}) {
  const [open, setOpen] = useState(false);
  const GroupIcon = GROUP_ICONS[label] || Braces;

  const filtered = searchQuery
    ? kinds.filter((k) => {
        const def = BLOCK_DEFS[k];
        const q = searchQuery.toLowerCase();
        return (
          def.label.toLowerCase().includes(q) ||
          def.description.toLowerCase().includes(q)
        );
      })
    : kinds;

  if (filtered.length === 0) return null;

  const isOpen = open || !!searchQuery;

  return (
    <div>
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-[13px] transition-colors hover:bg-accent"
      >
        <GroupIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
        <span className="flex-1 text-left font-medium text-foreground/90">
          {label}
        </span>
        <ChevronRight
          className={`h-3.5 w-3.5 shrink-0 text-muted-foreground/50 transition-transform duration-150 ${
            isOpen ? "rotate-90" : ""
          }`}
        />
      </button>
      {isOpen && (
        <div className="ml-2 border-l border-border/50 pl-1">
          {filtered.map((kind) => (
            <PaletteItem key={kind} kind={kind} />
          ))}
        </div>
      )}
    </div>
  );
}

export function Palette() {
  const [search, setSearch] = useState("");

  return (
    <aside className="flex w-64 shrink-0 flex-col border-r border-border bg-sidebar text-sidebar-foreground">
      <div className="px-3 pt-3 pb-2">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground/60" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search blocks..."
            className="h-8 pl-8 text-[13px] bg-muted/50 border-transparent focus-visible:border-border"
          />
        </div>
      </div>

      <div className="px-3 pb-1">
        <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">
          Components
        </span>
      </div>

      <ScrollArea className="flex-1">
        <div className="space-y-0.5 px-1.5 pb-3">
          {PALETTE_GROUPS.map((group) => (
            <PaletteGroup
              key={group.label}
              label={group.label}
              kinds={group.kinds}
              searchQuery={search}
            />
          ))}
        </div>
      </ScrollArea>
    </aside>
  );
}
