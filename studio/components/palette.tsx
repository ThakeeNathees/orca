"use client";

import { useState, useCallback } from "react";
import { BLOCK_DEFS, PALETTE_GROUPS } from "@/lib/block-defs";
import type { BlockKind } from "@/lib/types";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import {
  Bot,
  BookOpen,
  Braces,
  Zap,
  ChevronRight,
  PanelLeftClose,
  PanelLeftOpen,
  Search,
  Wrench,
  GripVertical,
  type LucideIcon,
} from "lucide-react";
import { ICON_MAP } from "@/lib/icons";

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
  open,
  onOpenChange,
}: {
  label: string;
  kinds: BlockKind[];
  searchQuery: string;
  open: boolean;
  onOpenChange: (next: boolean) => void;
}) {
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

  const isOpen = open || !!searchQuery.trim();

  return (
    <div>
      <button
        type="button"
        onClick={() => onOpenChange(!open)}
        className="flex w-full cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-[13px] transition-colors hover:bg-accent"
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

function filterKinds(kinds: BlockKind[], searchQuery: string): BlockKind[] {
  const q = searchQuery.trim().toLowerCase();
  if (!q) return kinds;
  return kinds.filter((kind) => {
    const def = BLOCK_DEFS[kind];
    return (
      def.label.toLowerCase().includes(q) ||
      def.description.toLowerCase().includes(q)
    );
  });
}

function CollapsedGroupRow({
  label,
  kinds,
  searchQuery,
  onExpand,
}: {
  label: string;
  kinds: BlockKind[];
  searchQuery: string;
  onExpand: (groupLabel: string) => void;
}) {
  const filtered = filterKinds(kinds, searchQuery);
  if (filtered.length === 0) return null;

  const GroupIcon = GROUP_ICONS[label] || Braces;

  return (
    <div className="flex justify-center">
      <Tooltip>
        <TooltipTrigger
          render={({ type: _type, ...btnProps }) => (
            <button
              type="button"
              {...btnProps}
              aria-label={`Expand palette — ${label}`}
              onClick={(e) => {
                btnProps.onClick?.(e);
                onExpand(label);
              }}
              className={cn(
                "flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground",
                btnProps.className
              )}
            >
              <GroupIcon className="h-4 w-4" />
            </button>
          )}
        />
        <TooltipContent side="right" sideOffset={8}>
          {label}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

export function Palette() {
  const [search, setSearch] = useState("");
  const [collapsed, setCollapsed] = useState(false);
  /** Accordion open state per group label (expanded sidebar only). */
  const [groupOpen, setGroupOpen] = useState<Record<string, boolean>>({});

  const setGroupExpanded = useCallback((label: string, expanded: boolean) => {
    setGroupOpen((prev) => ({ ...prev, [label]: expanded }));
  }, []);

  const expandFromCollapsedGroup = (label: string) => {
    setGroupOpen((prev) => ({ ...prev, [label]: true }));
    setCollapsed(false);
  };

  return (
    <aside
      className={cn(
        "flex shrink-0 flex-col border-r border-border bg-sidebar text-sidebar-foreground transition-[width] duration-200 ease-out",
        collapsed ? "w-[3.25rem]" : "w-64"
      )}
    >
      <div
        className={cn(
          "flex shrink-0 items-center gap-2 pb-2 pt-3",
          collapsed ? "flex-col px-1.5" : "px-3"
        )}
      >
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="shrink-0 cursor-pointer text-muted-foreground hover:text-foreground"
          aria-label={collapsed ? "Expand block palette" : "Collapse block palette"}
          aria-expanded={!collapsed}
          onClick={() => setCollapsed((c) => !c)}
        >
          {collapsed ? (
            <PanelLeftOpen className="h-4 w-4" />
          ) : (
            <PanelLeftClose className="h-4 w-4" />
          )}
        </Button>
        {!collapsed && (
          <div className="relative min-w-0 flex-1">
            <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground/60" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search blocks..."
              className="h-8 border-transparent bg-muted/50 pl-8 text-[13px] focus-visible:border-border"
            />
          </div>
        )}
      </div>

      {!collapsed && (
        <div className="px-3 pb-1">
          <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">
            Components
          </span>
        </div>
      )}

      <ScrollArea className="flex-1">
        {collapsed ? (
          <div className="flex flex-col items-center gap-0.5 px-1 pb-3 pt-0.5">
            {PALETTE_GROUPS.map((group) => (
              <CollapsedGroupRow
                key={group.label}
                label={group.label}
                kinds={group.kinds}
                searchQuery={search}
                onExpand={expandFromCollapsedGroup}
              />
            ))}
          </div>
        ) : (
          <div className="space-y-0.5 px-1.5 pb-3">
            {PALETTE_GROUPS.map((group) => (
              <PaletteGroup
                key={group.label}
                label={group.label}
                kinds={group.kinds}
                searchQuery={search}
                open={groupOpen[group.label] ?? false}
                onOpenChange={(next) => setGroupExpanded(group.label, next)}
              />
            ))}
          </div>
        )}
      </ScrollArea>
    </aside>
  );
}
