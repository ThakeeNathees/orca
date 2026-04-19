import type { BlockKind, HandleDef, FieldDef } from "./types";

export interface BlockDef {
  kind: BlockKind;
  label: string;
  description: string;
  icon: string;
  handles: HandleDef[];
  fields: FieldDef[];
}

/** Handle id constants. Keep every handle reference going through this
 *  map so a typo in a string literal can't silently break edge wiring. */
export const HANDLE_IDS = {
  agentIn: "agent-in",
  agentOut: "agent-out",
  triggerOut: "trigger-out",
  /** Branch route handles are dynamic, named `route-<routeId>`. */
  routePrefix: "route-",
} as const;

// Every non-trigger node exposes the same purple in/out connector pair.
const FLOW_HANDLES: HandleDef[] = [
  { id: HANDLE_IDS.agentIn, label: "Input", type: "agent", position: "left", multiple: true },
  { id: HANDLE_IDS.agentOut, label: "Output", type: "agent", position: "right" },
];

export const BLOCK_DEFS: Record<BlockKind, BlockDef> = {
  agent: {
    kind: "agent",
    label: "Agent",
    description: "AI agent — model, memory, and tools are configured in the Agents section",
    icon: "Bot",
    handles: [...FLOW_HANDLES],
    fields: [
      {
        key: "persona",
        label: "Persona",
        type: "textarea",
        placeholder: "You are a helpful assistant...",
      },
    ],
  },

  web_search: {
    kind: "web_search",
    label: "Web Search",
    description: "Search the web for real-time information",
    icon: "Search",
    handles: [...FLOW_HANDLES],
    fields: [
      {
        key: "provider",
        label: "Provider",
        type: "select",
        options: [
          { value: "google", label: "Google" },
          { value: "bing", label: "Bing" },
          { value: "tavily", label: "Tavily" },
          { value: "serper", label: "Serper" },
        ],
        defaultValue: "tavily",
      },
      {
        key: "max_results",
        label: "Max Results",
        type: "number",
        placeholder: "5",
        defaultValue: 5,
      },
    ],
  },

  code_exec: {
    kind: "code_exec",
    label: "Code Interpreter",
    description: "Execute Python code and capture output",
    icon: "Terminal",
    handles: [...FLOW_HANDLES],
    fields: [
      {
        key: "timeout",
        label: "Timeout (s)",
        type: "number",
        placeholder: "30",
        defaultValue: 30,
      },
      {
        key: "sandbox",
        label: "Sandbox",
        type: "select",
        options: [
          { value: "local", label: "Local" },
          { value: "docker", label: "Docker" },
          { value: "e2b", label: "E2B" },
        ],
        defaultValue: "docker",
      },
    ],
  },

  api_request: {
    kind: "api_request",
    label: "API Request",
    description: "Make HTTP requests to external APIs",
    icon: "Send",
    handles: [...FLOW_HANDLES],
    fields: [
      {
        key: "url",
        label: "URL",
        type: "text",
        placeholder: "https://api.example.com/v1/...",
      },
      {
        key: "method",
        label: "Method",
        type: "select",
        options: [
          { value: "GET", label: "GET" },
          { value: "POST", label: "POST" },
          { value: "PUT", label: "PUT" },
          { value: "DELETE", label: "DELETE" },
        ],
        defaultValue: "GET",
      },
      {
        key: "headers",
        label: "Headers",
        type: "textarea",
        placeholder: '{"Authorization": "Bearer ..."}',
      },
    ],
  },

  sql_query: {
    kind: "sql_query",
    label: "SQL Query",
    description: "Query a database and return results",
    icon: "Database",
    handles: [...FLOW_HANDLES],
    fields: [
      {
        key: "connection",
        label: "Connection",
        type: "text",
        placeholder: "postgresql://user:pass@host/db",
      },
      {
        key: "dialect",
        label: "Dialect",
        type: "select",
        options: [
          { value: "postgresql", label: "PostgreSQL" },
          { value: "mysql", label: "MySQL" },
          { value: "sqlite", label: "SQLite" },
        ],
        defaultValue: "postgresql",
      },
      {
        key: "max_rows",
        label: "Max Rows",
        type: "number",
        placeholder: "100",
        defaultValue: 100,
      },
    ],
  },

  knowledge: {
    kind: "knowledge",
    label: "Knowledge",
    description: "RAG or data source — participates in the graph like a workflow node",
    icon: "BookOpen",
    handles: [...FLOW_HANDLES],
    fields: [
      {
        key: "name",
        label: "Name",
        type: "text",
        placeholder: "company_docs",
      },
      {
        key: "desc",
        label: "Description",
        type: "textarea",
        placeholder: "Company documentation and policies",
      },
    ],
  },

  workflow: {
    kind: "workflow",
    label: "Workflow",
    description: "Agent orchestration graph",
    icon: "GitBranch",
    handles: [
      {
        id: "agents-in",
        label: "Agents",
        type: "agent",
        position: "left",
        multiple: true,
      },
    ],
    fields: [
      {
        key: "name",
        label: "Name",
        type: "text",
        placeholder: "research_pipeline",
      },
      {
        key: "desc",
        label: "Description",
        type: "textarea",
        placeholder: "Orchestrate agents to research and write",
      },
    ],
  },

  input: {
    kind: "input",
    label: "Input",
    description: "External input parameter",
    icon: "ArrowRightToLine",
    handles: [
      { id: HANDLE_IDS.agentOut, label: "Output", type: "agent", position: "right" },
    ],
    fields: [
      {
        key: "type",
        label: "Type",
        type: "select",
        options: [
          { value: "string", label: "String" },
          { value: "number", label: "Number" },
          { value: "boolean", label: "Boolean" },
        ],
        defaultValue: "string",
      },
      {
        key: "desc",
        label: "Description",
        type: "text",
        placeholder: "User query",
      },
      {
        key: "default",
        label: "Default Value",
        type: "text",
        placeholder: "",
      },
    ],
  },

  cron: {
    kind: "cron",
    label: "Cron",
    description: "Schedule-based trigger",
    icon: "Clock",
    handles: [
      { id: HANDLE_IDS.triggerOut, label: "Trigger", type: "trigger", position: "right" },
    ],
    fields: [
      {
        key: "schedule",
        label: "Schedule",
        type: "text",
        placeholder: "0 9 * * MON-FRI",
      },
      {
        key: "timezone",
        label: "Timezone",
        type: "text",
        placeholder: "UTC",
      },
    ],
  },

  webhook: {
    kind: "webhook",
    label: "Webhook",
    description: "HTTP webhook trigger",
    icon: "Globe",
    handles: [
      { id: HANDLE_IDS.triggerOut, label: "Trigger", type: "trigger", position: "right" },
    ],
    fields: [
      {
        key: "path",
        label: "Path",
        type: "text",
        placeholder: "/api/webhook",
      },
      {
        key: "method",
        label: "Method",
        type: "select",
        options: [
          { value: "POST", label: "POST" },
          { value: "GET", label: "GET" },
          { value: "PUT", label: "PUT" },
        ],
        defaultValue: "POST",
      },
    ],
  },

  chat: {
    kind: "chat",
    label: "Chat",
    description: "Conversational chat trigger",
    icon: "MessageCircle",
    handles: [
      { id: HANDLE_IDS.triggerOut, label: "Trigger", type: "trigger", position: "right" },
    ],
    fields: [
      {
        key: "greeting",
        label: "Greeting",
        type: "textarea",
        placeholder: "Hello! How can I help you?",
      },
    ],
  },

  custom_tool: {
    kind: "custom_tool",
    label: "Custom Tool",
    description: "User-defined tool with inline Python",
    icon: "Wrench",
    handles: [...FLOW_HANDLES],
    fields: [
      {
        key: "desc",
        label: "Description",
        type: "text",
        placeholder: "What this tool does...",
      },
      {
        key: "invoke",
        label: "Invoke",
        type: "code",
        placeholder:
          'def my_tool(input: str) -> str:\n    """Tool implementation."""\n    return input',
      },
    ],
  },

  branch: {
    kind: "branch",
    label: "Branch",
    description: "Conditional routing — transform input to a route key, dispatch to mapped node",
    icon: "Split",
    // Only the left input handle is static. Route handles on the right are
    // rendered dynamically by BranchNode from data.routes — one handle per
    // row. Their handle ids follow the pattern `route-<routeId>` and their
    // type is implicitly "agent" (see canvas isValidConnection).
    handles: [
      {
        id: HANDLE_IDS.agentIn,
        label: "Input",
        type: "agent",
        position: "left",
        multiple: true,
      },
    ],
    fields: [
      {
        key: "transform",
        label: "Transform",
        type: "code",
        placeholder: '\\(out string) -> out.lower()',
      },
    ],
  },

  schema: {
    kind: "schema",
    label: "Schema",
    description: "User-defined record type",
    icon: "Braces",
    handles: [
      { id: HANDLE_IDS.agentOut, label: "Output", type: "agent", position: "right" },
    ],
    fields: [
      {
        key: "name",
        label: "Name",
        type: "text",
        placeholder: "SearchResult",
      },
    ],
  },
};

export const BLOCK_KINDS = Object.keys(BLOCK_DEFS) as BlockKind[];

export const PALETTE_GROUPS: { label: string; kinds: BlockKind[] }[] = [
  { label: "Agents", kinds: ["agent"] },
  { label: "Tools", kinds: ["web_search", "code_exec", "api_request", "sql_query", "custom_tool"] },
  { label: "Data", kinds: ["knowledge"] },
  { label: "Triggers", kinds: ["cron", "webhook", "chat"] },
  { label: "Control Flow", kinds: ["branch"] },
];

/**
 * Every non-trigger node exposes `agent`-typed in/out handles; triggers
 * only output. Any flow connection (agent→agent, trigger→agent) is valid.
 */
export function canConnect(sourceType: string, targetType: string): boolean {
  if (targetType !== "agent") return false;
  return sourceType === "agent" || sourceType === "trigger";
}
