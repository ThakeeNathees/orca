"use client";

import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  Search,
  X,
  Plus,
  Brain,
  Wrench,
  Bot,
  Workflow,
  Clock,
  MessageCircle,
  Inbox,
  DollarSign,
  Activity,
  Settings,
  Compass,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useStudioStore } from "@/lib/store";
import type { SidebarSection } from "@/lib/sidebar-sections";
import { PROVIDER_MODELS } from "@/lib/model-providers";
import { ENTITY_ICON_BG_ELEVATED } from "@/lib/entity-icon";

type PaletteGroup = "Actions" | "Pages" | "Help";

interface PaletteItem {
  id: string;
  group: PaletteGroup;
  label: string;
  icon: LucideIcon;
  keywords?: string[];
  onRun: () => void;
}

const PAGE_ENTRIES: { id: SidebarSection; label: string; icon: LucideIcon }[] =
  [
    { id: "orca", label: "Orca", icon: MessageCircle },
    { id: "inbox", label: "Inbox", icon: Inbox },
    { id: "models", label: "Models", icon: Brain },
    { id: "skills", label: "Skills", icon: Wrench },
    { id: "agents", label: "Agents", icon: Bot },
    { id: "workflows", label: "Workflows", icon: Workflow },
    { id: "crons", label: "Cron Jobs", icon: Clock },
    { id: "cost", label: "Cost", icon: DollarSign },
    { id: "runs", label: "Runs", icon: Activity },
    { id: "settings", label: "Settings", icon: Settings },
  ];

/** Highlight tokens: matches any command that contains the query (case-
 *  insensitive) in its label or keywords. Good enough without pulling in a
 *  fuzzy-match dep; swap for `fuse.js` later if needed. */
function matches(item: PaletteItem, q: string): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  if (item.label.toLowerCase().includes(needle)) return true;
  return (item.keywords ?? []).some((k) => k.toLowerCase().includes(needle));
}

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  const setSidebarSection = useStudioStore((s) => s.setSidebarSection);
  const createWorkflow = useStudioStore((s) => s.createWorkflow);
  const createAgent = useStudioStore((s) => s.createAgent);
  const createSkill = useStudioStore((s) => s.createSkill);
  const createModel = useStudioStore((s) => s.createModel);
  const createCronJob = useStudioStore((s) => s.createCronJob);

  // Commands are rebuilt each render so they close over the latest store
  // actions. Cheap — all of these are cheap setters/promises.
  const items: PaletteItem[] = useMemo(() => {
    const close = () => setOpen(false);
    const runPage = (id: SidebarSection) => {
      setSidebarSection(id);
      close();
    };
    return [
      {
        id: "new-workflow",
        group: "Actions",
        label: "Create new workflow",
        icon: Plus,
        keywords: ["workflow", "graph"],
        onRun: () => {
          void createWorkflow();
          close();
        },
      },
      {
        id: "new-agent",
        group: "Actions",
        label: "Create new agent",
        icon: Plus,
        keywords: ["agent", "llm"],
        onRun: () => {
          void createAgent("Untitled Agent");
          close();
        },
      },
      {
        id: "new-model",
        group: "Actions",
        label: "Create new model",
        icon: Plus,
        keywords: ["model", "provider", "llm"],
        onRun: () => {
          void createModel(
            "Untitled Model",
            "openai",
            PROVIDER_MODELS.openai[0]
          );
          close();
        },
      },
      {
        id: "new-skill",
        group: "Actions",
        label: "Create new skill",
        icon: Plus,
        keywords: ["skill", "tool"],
        onRun: () => {
          void createSkill("Untitled Skill");
          close();
        },
      },
      {
        id: "new-cron",
        group: "Actions",
        label: "Create new cron job",
        icon: Plus,
        keywords: ["cron", "schedule", "trigger"],
        onRun: () => {
          void createCronJob("Untitled Cron Job");
          close();
        },
      },
      ...PAGE_ENTRIES.map((p) => ({
        id: `page-${p.id}`,
        group: "Pages" as const,
        label: p.label,
        icon: p.icon,
        keywords: [p.id],
        onRun: () => runPage(p.id),
      })),
      {
        id: "tour",
        group: "Help",
        label: "Take a tour of the studio",
        icon: Compass,
        keywords: ["tour", "onboarding", "help", "walkthrough"],
        onRun: () => {
          // Tour content isn't built yet — placeholder stays inert until
          // the onboarding flow ships.
          close();
        },
      },
    ];
  }, [
    setSidebarSection,
    createWorkflow,
    createAgent,
    createSkill,
    createModel,
    createCronJob,
  ]);

  const visible = useMemo(
    () => items.filter((i) => matches(i, query)),
    [items, query]
  );

  // Clamp the stored cursor on render — avoids a setState-in-effect and
  // keeps arrow keys usable when the filtered list shrinks.
  const activeCursor = Math.min(cursor, Math.max(visible.length - 1, 0));

  // Global Cmd/Ctrl+K toggle + reset-on-open. Listener lives on window once,
  // for the lifetime of the app (CommandPalette is mounted once at the root).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((wasOpen) => {
          if (!wasOpen) {
            setQuery("");
            setCursor(0);
            requestAnimationFrame(() => inputRef.current?.focus());
          }
          return !wasOpen;
        });
      }
      if (e.key === "Escape") {
        // `setOpen` callback form avoids needing to capture `open` in deps.
        setOpen((wasOpen) => {
          if (wasOpen) e.preventDefault();
          return false;
        });
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  if (!open) return null;

  const grouped = groupBy(visible);

  return (
    <div
      className="fixed inset-0 z-[150] flex items-start justify-center pt-[12vh]"
      onClick={() => setOpen(false)}
    >
      <div className="absolute inset-0 bg-black/50" />
      <div
        onClick={(e) => e.stopPropagation()}
        className="relative z-10 flex w-full max-w-xl flex-col overflow-hidden rounded-lg border border-border bg-card shadow-2xl"
      >
        <div className="flex items-center gap-2 border-b border-border px-3 py-2.5">
          <Search className="size-4 shrink-0 text-muted-foreground" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setCursor((c) => Math.min(c + 1, Math.max(visible.length - 1, 0)));
              } else if (e.key === "ArrowUp") {
                e.preventDefault();
                setCursor((c) => Math.max(c - 1, 0));
              } else if (e.key === "Enter") {
                e.preventDefault();
                visible[activeCursor]?.onRun();
              }
            }}
            placeholder="Search actions, pages, help…"
            className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
          <button
            type="button"
            onClick={() => setOpen(false)}
            className="flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground cursor-pointer"
            aria-label="Close"
          >
            <X className="size-4" />
          </button>
        </div>

        <div className="max-h-[60vh] overflow-y-auto py-1.5">
          {visible.length === 0 ? (
            <div className="px-3 py-8 text-center text-sm text-muted-foreground">
              No results
            </div>
          ) : (
            grouped.map(([group, rows]) => (
              <Section key={group} label={group}>
                {rows.map((item) => {
                  const index = visible.indexOf(item);
                  return (
                    <Row
                      key={item.id}
                      item={item}
                      active={index === activeCursor}
                      onHover={() => setCursor(index)}
                    />
                  );
                })}
              </Section>
            ))
          )}
        </div>
      </div>
    </div>
  );
}

function groupBy(items: PaletteItem[]): [PaletteGroup, PaletteItem[]][] {
  const order: PaletteGroup[] = ["Actions", "Pages", "Help"];
  return order
    .map((g): [PaletteGroup, PaletteItem[]] => [
      g,
      items.filter((i) => i.group === g),
    ])
    .filter(([, rows]) => rows.length > 0);
}

function Section({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="px-1.5 pb-1.5">
      <div className="px-2 pt-2 pb-1 text-caption uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className="flex flex-col">{children}</div>
    </div>
  );
}

function Row({
  item,
  active,
  onHover,
}: {
  item: PaletteItem;
  active: boolean;
  onHover: () => void;
}) {
  const Icon = item.icon;
  return (
    <button
      type="button"
      onClick={item.onRun}
      onMouseEnter={onHover}
      style={active ? { backgroundColor: ENTITY_ICON_BG_ELEVATED } : undefined}
      className={cn(
        "flex w-full items-center gap-3 rounded-md px-2 py-2 text-left text-sm text-foreground transition-colors cursor-pointer",
        !active && "hover:bg-[#262626]"
      )}
    >
      <Icon className="size-4 shrink-0 text-muted-foreground" />
      <span className="min-w-0 flex-1 truncate">{item.label}</span>
    </button>
  );
}
