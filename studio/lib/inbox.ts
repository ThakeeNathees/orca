// Hardcoded inbox feed. Lives outside the component so the sidebar can
// read the unread count without importing the UI layer. Swap with a
// server-driven feed when the backend lands.

export type InboxActionKind =
  | "configure-model"
  | "configure-agent"
  | "set-budget";

export interface InboxAction {
  kind: InboxActionKind;
  label: string;
}

export interface InboxMessage {
  id: string;
  title: string;
  body: string;
  /** Minutes ago (keeps seed deterministic without a real clock). */
  ageMinutes: number;
  unread: boolean;
  important: boolean;
  actions?: InboxAction[];
}

export const INBOX_MESSAGES: InboxMessage[] = [
  {
    id: "welcome",
    title: "Welcome to Orca",
    body: "Orca is your AI co-pilot for orchestrating workflows, agents, and skills. Explore the sidebar to create your first entities, or ask Orca to help you get started.",
    ageMinutes: 60 * 24 * 2,
    unread: false,
    important: false,
  },
  {
    id: "configure-first",
    title: "Configure your first agent and model",
    body: "Agents need a model to run. Add a model from the Models section, then create an agent and assign the model under its Configuration tab.",
    ageMinutes: 0,
    unread: true,
    important: false,
    actions: [
      { kind: "configure-model", label: "Add Model" },
      { kind: "configure-agent", label: "Create Agent" },
    ],
  },
  {
    id: "budget",
    title: "Don't forget to set a budget",
    body: "Budgets stop runaway spending before it happens. Configure a monthly cap per model so Orca pauses work when the threshold is reached.",
    ageMinutes: 0,
    unread: true,
    important: true,
    actions: [{ kind: "set-budget", label: "Set Budget" }],
  },
  {
    id: "tour",
    title: "Take a tour of the studio",
    body: "Press Ctrl+K (or ⌘K on macOS) to open the command palette, then pick “Take a tour of the studio”. Everything in Orca is keyboard-accessible.",
    ageMinutes: 60 * 5,
    unread: false,
    important: false,
  },
];
