import type { BlockKind, HandleDef, FieldDef } from "./types";

export interface BlockDef {
  kind: BlockKind;
  label: string;
  description: string;
  color: string;
  colorMuted: string;
  icon: string;
  handles: HandleDef[];
  fields: FieldDef[];
}

export const BLOCK_DEFS: Record<BlockKind, BlockDef> = {
  model: {
    kind: "model",
    label: "Model",
    description: "LLM provider configuration",
    color: "#3b82f6",
    colorMuted: "#3b82f620",
    icon: "Cpu",
    handles: [
      { id: "model-out", label: "Model", type: "model", position: "top" },
    ],
    fields: [
      {
        key: "provider",
        label: "Provider",
        type: "select",
        options: [
          { value: "openai", label: "OpenAI" },
          { value: "anthropic", label: "Anthropic" },
          { value: "google", label: "Google" },
          { value: "ollama", label: "Ollama" },
        ],
        defaultValue: "openai",
      },
      {
        key: "model_name",
        label: "Model Name",
        type: "text",
        placeholder: "gpt-4o",
      },
      {
        key: "temperature",
        label: "Temperature",
        type: "slider",
        min: 0,
        max: 2,
        step: 0.1,
        defaultValue: 1,
      },
      {
        key: "api_key",
        label: "API Key",
        type: "password",
        placeholder: "sk-...",
      },
    ],
  },

  agent: {
    kind: "agent",
    label: "Agent",
    description: "AI agent with model, memory, and tools",
    color: "#8b5cf6",
    colorMuted: "#8b5cf620",
    icon: "Bot",
    handles: [
      {
        id: "agent-in",
        label: "Input",
        type: "agent",
        position: "left",
        multiple: true,
      },
      { id: "model-in", label: "Model", type: "model", position: "bottom" },
      { id: "memory-in", label: "Memory", type: "memory", position: "bottom" },
      {
        id: "tools-in",
        label: "Tools",
        type: "tool",
        position: "bottom",
        multiple: true,
      },
      { id: "agent-out", label: "Output", type: "agent", position: "right" },
    ],
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
    color: "#22c55e",
    colorMuted: "#22c55e20",
    icon: "Search",
    handles: [
      { id: "tool-out", label: "Tool", type: "tool", position: "top" },
    ],
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
    color: "#22c55e",
    colorMuted: "#22c55e20",
    icon: "Terminal",
    handles: [
      { id: "tool-out", label: "Tool", type: "tool", position: "top" },
    ],
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
    color: "#22c55e",
    colorMuted: "#22c55e20",
    icon: "Send",
    handles: [
      { id: "tool-out", label: "Tool", type: "tool", position: "top" },
    ],
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
    color: "#22c55e",
    colorMuted: "#22c55e20",
    icon: "Database",
    handles: [
      { id: "tool-out", label: "Tool", type: "tool", position: "top" },
    ],
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
    description: "RAG or data source",
    color: "#f59e0b",
    colorMuted: "#f59e0b20",
    icon: "BookOpen",
    handles: [
      {
        id: "knowledge-out",
        label: "Knowledge",
        type: "knowledge",
        position: "right",
      },
    ],
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

  memory: {
    kind: "memory",
    label: "Memory",
    description: "Long-term or session memory store",
    color: "#f59e0b",
    colorMuted: "#f59e0b20",
    icon: "Brain",
    handles: [
      { id: "memory-out", label: "Memory", type: "memory", position: "top" },
    ],
    fields: [
      {
        key: "name",
        label: "Name",
        type: "text",
        placeholder: "session_store",
      },
      {
        key: "desc",
        label: "Description",
        type: "textarea",
        placeholder: "Conversation and tool-call history",
      },
    ],
  },

  workflow: {
    kind: "workflow",
    label: "Workflow",
    description: "Agent orchestration graph",
    color: "#f43f5e",
    colorMuted: "#f43f5e20",
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
    color: "#06b6d4",
    colorMuted: "#06b6d420",
    icon: "ArrowRightToLine",
    handles: [
      { id: "input-out", label: "Output", type: "any", position: "right" },
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
    color: "#f97316",
    colorMuted: "#f9731620",
    icon: "Clock",
    handles: [
      { id: "trigger-out", label: "Trigger", type: "trigger", position: "right" },
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
    color: "#f97316",
    colorMuted: "#f9731620",
    icon: "Globe",
    handles: [
      { id: "trigger-out", label: "Trigger", type: "trigger", position: "right" },
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
    color: "#f97316",
    colorMuted: "#f9731620",
    icon: "MessageCircle",
    handles: [
      { id: "trigger-out", label: "Trigger", type: "trigger", position: "right" },
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

  schema: {
    kind: "schema",
    label: "Schema",
    description: "User-defined record type",
    color: "#64748b",
    colorMuted: "#64748b20",
    icon: "Braces",
    handles: [
      {
        id: "schema-out",
        label: "Schema",
        type: "schema",
        position: "right",
      },
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
  { label: "Models & Agents", kinds: ["model", "agent"] },
  { label: "Tools", kinds: ["web_search", "code_exec", "api_request", "sql_query"] },
  { label: "Data", kinds: ["knowledge", "memory"] },
  { label: "Triggers", kinds: ["cron", "webhook", "chat"] },
];

/**
 * Returns which source handle types can connect to which target handle types.
 * Used for connection validation on the canvas.
 */
export function canConnect(sourceType: string, targetType: string): boolean {
  if (targetType === "any") return true;
  if (sourceType === targetType) return true;
  // Cron / webhook / chat outputs feed agent (and workflow) graph inputs.
  if (sourceType === "trigger" && targetType === "agent") return true;
  return false;
}
