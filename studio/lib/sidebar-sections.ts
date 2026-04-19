// Sidebar section ids + human-readable labels. Extracted from the sidebar
// component so the store (and top-bar breadcrumb) can reference these
// types without pulling in a client component.

export type SidebarSection =
  | "orca"
  | "inbox"
  | "models"
  | "skills"
  | "agents"
  | "workflows"
  | "crons"
  | "cost"
  | "runs"
  | "settings";

export const SECTION_LABELS: Record<SidebarSection, string> = {
  orca: "Orca",
  inbox: "Inbox",
  models: "Models",
  skills: "Skills",
  agents: "Agents",
  workflows: "Workflows",
  crons: "Cron Jobs",
  cost: "Cost",
  runs: "Runs",
  settings: "Settings",
};

/** Parent group name for breadcrumb rendering — null means the section is its own top-level group. */
export const SECTION_PARENT: Record<SidebarSection, string | null> = {
  orca: null,
  inbox: null,
  models: null,
  skills: null,
  agents: null,
  workflows: null,
  crons: null,
  cost: "Operations",
  runs: "Operations",
  settings: "Operations",
};
