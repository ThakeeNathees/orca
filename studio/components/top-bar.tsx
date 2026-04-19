"use client";

import { useCallback, type ReactNode } from "react";
import { ArrowLeft, Hammer, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useStudioStore } from "@/lib/store";
import { generateOrcaSource, sanitizeIdent } from "@/lib/orca-gen";
import { SECTION_LABELS, SECTION_PARENT } from "@/lib/sidebar-sections";

/** Breadcrumb segment — non-interactive by default; pass onClick to make it clickable. */
function Crumb({
  children,
  onClick,
  active,
}: {
  children: ReactNode;
  onClick?: () => void;
  active?: boolean;
}) {
  const base =
    "min-w-0 truncate text-[15px] font-[590] tracking-[-0.012em]";
  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={`${base} cursor-pointer text-muted-foreground hover:text-foreground`}
      >
        {children}
      </button>
    );
  }
  return (
    <span
      className={`${base} ${active ? "text-foreground" : "text-muted-foreground"}`}
    >
      {children}
    </span>
  );
}

function Separator() {
  return <span className="text-muted-foreground/60">/</span>;
}

/**
 * Builds the breadcrumb segments for the current view.
 *
 * Rules:
 *   - Editor: `<SectionGroup> > <entity name> > Editor` (first two clickable)
 *   - Detail: `<SectionGroup> > <entity name>`
 *   - Dashboard: `<Parent> > <Section>` or just `<Section>` for top-level groups
 */
function useBreadcrumb(): { segments: ReactNode[] } {
  const currentView = useStudioStore((s) => s.currentView);
  const sidebarSection = useStudioStore((s) => s.sidebarSection);
  const workflows = useStudioStore((s) => s.workflows);
  const models = useStudioStore((s) => s.models);
  const skills = useStudioStore((s) => s.skills);
  const agents = useStudioStore((s) => s.agents);
  const activeWorkflowId = useStudioStore((s) => s.activeWorkflowId);
  const activeModelId = useStudioStore((s) => s.activeModelId);
  const activeSkillId = useStudioStore((s) => s.activeSkillId);
  const activeAgentId = useStudioStore((s) => s.activeAgentId);
  const openWorkflow = useStudioStore((s) => s.openWorkflow);

  const segments: ReactNode[] = [];

  if (currentView === "dashboard") {
    const parent = SECTION_PARENT[sidebarSection];
    if (parent) {
      segments.push(<Crumb key="parent">{parent}</Crumb>);
      segments.push(<Separator key="sep" />);
      segments.push(
        <Crumb key="section" active>
          {SECTION_LABELS[sidebarSection]}
        </Crumb>
      );
    } else {
      segments.push(
        <Crumb key="section" active>
          {SECTION_LABELS[sidebarSection]}
        </Crumb>
      );
    }
    return { segments };
  }

  // Detail and Editor share the `<Group> > <entity>` prefix. The group
  // name comes from whichever sidebar section the active entity belongs to.
  let entityName: string | null = null;
  if (activeWorkflowId) {
    entityName = workflows.find((w) => w.id === activeWorkflowId)?.name ?? null;
  } else if (activeModelId) {
    entityName = models.find((m) => m.id === activeModelId)?.name ?? null;
  } else if (activeSkillId) {
    entityName = skills.find((s) => s.id === activeSkillId)?.name ?? null;
  } else if (activeAgentId) {
    entityName = agents.find((a) => a.id === activeAgentId)?.name ?? null;
  }

  // Group names are not clickable — they name a sidebar dropdown, not a page.
  segments.push(
    <Crumb key="group">{SECTION_LABELS[sidebarSection]}</Crumb>
  );

  if (entityName) {
    segments.push(<Separator key="sep-1" />);
    if (currentView === "editor" && activeWorkflowId) {
      // In the editor, the entity name returns to detail; the editor itself
      // is the last (active) crumb.
      segments.push(
        <Crumb key="entity" onClick={() => openWorkflow(activeWorkflowId)}>
          {entityName}
        </Crumb>
      );
      segments.push(<Separator key="sep-2" />);
      segments.push(
        <Crumb key="editor" active>
          Editor
        </Crumb>
      );
    } else {
      segments.push(
        <Crumb key="entity" active>
          {entityName}
        </Crumb>
      );
    }
  }

  return { segments };
}

export function TopBar() {
  const currentView = useStudioStore((s) => s.currentView);
  const openWorkflow = useStudioStore((s) => s.openWorkflow);
  const activeWorkflowId = useStudioStore((s) => s.activeWorkflowId);
  const workflows = useStudioStore((s) => s.workflows);
  const nodes = useStudioStore((s) => s.nodes);
  const edges = useStudioStore((s) => s.edges);
  const isEditor = currentView === "editor";

  const activeWorkflow = workflows.find((w) => w.id === activeWorkflowId);
  const activeName = activeWorkflow?.name ?? "Workflow";

  const canBuild = isEditor && nodes.length > 0;

  const handleBuild = useCallback(() => {
    if (!canBuild) return;
    const source = generateOrcaSource(nodes, edges);
    const base = sanitizeIdent(activeName) || "workflow";
    const filename = `${base}.orca`;

    const blob = new Blob([source], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.style.display = "none";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }, [canBuild, nodes, edges, activeName]);

  const { segments } = useBreadcrumb();

  return (
    <header className="flex h-12 shrink-0 items-center border-b border-border bg-sidebar px-4 text-sidebar-foreground">
      <div className="flex min-w-0 flex-1 items-center gap-2">
        {isEditor && activeWorkflowId && (
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => openWorkflow(activeWorkflowId)}
            className="shrink-0 cursor-pointer text-muted-foreground hover:text-foreground"
            aria-label="Back to workflow"
          >
            <ArrowLeft className="size-4" />
          </Button>
        )}
        <nav className="flex min-w-0 items-center gap-1.5">{segments}</nav>
      </div>

      <div className="flex min-w-0 flex-1 items-center justify-end gap-1">
        {isEditor ? (
          <>
            <Button
              variant="ghost"
              size="sm"
              onClick={handleBuild}
              disabled={!canBuild}
              className="gap-1.5 cursor-pointer text-sidebar-foreground hover:bg-sidebar-accent disabled:cursor-not-allowed disabled:opacity-50"
              title={
                canBuild
                  ? "Download .orca source"
                  : "Add nodes to the canvas to build"
              }
            >
              <Hammer className="h-3.5 w-3.5 shrink-0" />
              Build
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled
              className="gap-1.5 cursor-not-allowed text-sidebar-foreground disabled:opacity-50"
              title="Download and run with `orca run` on your machine."
            >
              <Play className="h-3.5 w-3.5 shrink-0" />
              Run
            </Button>
            <a
              href="https://github.com/ThakeeNathees/orca"
              target="_blank"
              rel="noopener noreferrer"
              className="flex size-8 shrink-0 items-center justify-center rounded-md text-sidebar-foreground transition-colors hover:bg-sidebar-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
              aria-label="Orca repository on GitHub"
              title="View source on GitHub"
            >
              <svg
                className="size-[18px] shrink-0"
                viewBox="0 0 98 96"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden
              >
                <path
                  fill="currentColor"
                  fillRule="evenodd"
                  clipRule="evenodd"
                  d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z"
                />
              </svg>
            </a>
          </>
        ) : null}
      </div>
    </header>
  );
}
