"use client";

import { useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import {
  AlertCircle,
  MessageCircle,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  RefreshCw,
  Send,
} from "lucide-react";
import { useStudioStore } from "@/lib/store";
import {
  ComingSoonTab,
  EntityIconSwatch,
  TabBar,
} from "@/components/detail-shell";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ModelSelect } from "@/components/model-select";

type Tab = "chat" | "configuration" | "memory" | "usage";

const TABS: { id: Tab; label: string }[] = [
  { id: "chat", label: "Chat" },
  { id: "configuration", label: "Configuration" },
  { id: "memory", label: "Memory" },
  { id: "usage", label: "Usage" },
];

type AssistantBlock =
  | { kind: "greeting" }
  | { kind: "no-model" }
  | { kind: "error" };

type ChatMessage =
  | { id: string; role: "user"; content: string }
  | { id: string; role: "assistant"; block: AssistantBlock };

const INITIAL_MESSAGES: ChatMessage[] = [
  { id: "greeting", role: "assistant", block: { kind: "greeting" } },
];

/** Hardcoded conversation history. Swap for a store-backed list when the
 *  chat backend lands and real threads start getting persisted. */
const RECENT_CHATS: { id: string; title: string }[] = [
  { id: "r1", title: "How do I create an agent?" },
  { id: "r2", title: "Budget strategy for models" },
  { id: "r3", title: "Debugging a failed workflow" },
  { id: "r4", title: "Schedule a daily digest cron" },
  { id: "r5", title: "Wire a skill to my writer agent" },
];

/** Bordered transparent frame for assistant replies so they blend with the
 *  chat viewport. Only user bubbles carry a filled background. */
function AssistantCard({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-lg border border-border px-4 py-3 text-sm text-foreground">
      {children}
    </div>
  );
}

function GreetingBlock({
  onQuickPrompt,
  disabled,
}: {
  onQuickPrompt: (t: string) => void;
  disabled: boolean;
}) {
  return (
    <AssistantCard>
      <p>
        Hi, I&apos;m Orca — your AI co-pilot for orchestrating workflows,
        agents, and skills. How can I help you today?
      </p>
      <div className="mt-3 flex flex-wrap gap-2">
        <Button
          size="sm"
          disabled={disabled}
          className="bg-[#E2E2E2] text-zinc-900 hover:bg-[#E2E2E2]/90 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
          onClick={() => onQuickPrompt("Help me create a workflow")}
        >
          Create Workflow
        </Button>
        <Button
          size="sm"
          disabled={disabled}
          className="bg-[#E2E2E2] text-zinc-900 hover:bg-[#E2E2E2]/90 disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
          onClick={() => onQuickPrompt("Help me create an agent")}
        >
          Create Agent
        </Button>
      </div>
    </AssistantCard>
  );
}

function NoModelBlock() {
  const models = useStudioStore((s) => s.models);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const orcaModelId = useStudioStore((s) => s.orcaModelId);
  const orcaFallbackModelId = useStudioStore((s) => s.orcaFallbackModelId);
  const setOrcaModels = useStudioStore((s) => s.setOrcaModels);
  const projectModels = models.filter((m) => m.projectId === activeProjectId);

  return (
    <AssistantCard>
      <div className="flex items-start gap-2">
        <AlertCircle className="mt-0.5 size-4 shrink-0 text-destructive" />
        <div className="min-w-0 flex-1">
          <p className="font-medium text-foreground">No model configured.</p>
          <p className="mt-1 text-muted-foreground">
            Add a model in the Models section, then assign it on the
            Configuration tab.
          </p>
        </div>
      </div>
      <div className="mt-3">
        <ModelSelect
          label="Model"
          value={orcaModelId ?? ""}
          models={projectModels}
          placeholder="Select a model"
          onChange={(id) => setOrcaModels(id, orcaFallbackModelId)}
        />
      </div>
    </AssistantCard>
  );
}

function ErrorBlock() {
  const [retrying, setRetrying] = useState(false);

  // Retry is stubbed to a 2s pretend-in-flight state so the UI looks
  // responsive while the chat backend is still unimplemented.
  useEffect(() => {
    if (!retrying) return;
    const id = setTimeout(() => setRetrying(false), 2000);
    return () => clearTimeout(id);
  }, [retrying]);

  return (
    <AssistantCard>
      <div className="flex items-start gap-2">
        <AlertCircle className="mt-0.5 size-4 shrink-0 text-destructive" />
        <div className="min-w-0 flex-1">
          <p className="font-medium text-foreground">Something went wrong.</p>
          <p className="mt-1 text-muted-foreground">
            Chat is not implemented yet (TODO).
          </p>
        </div>
      </div>
      <div className="mt-3">
        <Button
          size="sm"
          variant="ghost"
          disabled={retrying}
          onClick={() => setRetrying(true)}
          className="gap-1.5 cursor-pointer disabled:cursor-not-allowed"
        >
          <RefreshCw
            className={`size-3.5 ${retrying ? "animate-spin" : ""}`}
          />
          {retrying ? "Retrying…" : "Retry"}
        </Button>
      </div>
    </AssistantCard>
  );
}

function ChatTab() {
  const [messages, setMessages] = useState<ChatMessage[]>(INITIAL_MESSAGES);
  const [draft, setDraft] = useState("");
  const orcaModelId = useStudioStore((s) => s.orcaModelId);
  const bottomRef = useRef<HTMLLIElement>(null);

  // Follow the newest message. `block: "end"` keeps the scroll local to the
  // viewport so it doesn't bubble and move the parent page.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages]);

  const pushExchange = (content: string, reply: AssistantBlock) => {
    const t = content.trim();
    if (!t) return;
    setMessages((prev) => [
      ...prev,
      { id: crypto.randomUUID(), role: "user", content: t },
      { id: crypto.randomUUID(), role: "assistant", block: reply },
    ]);
    setDraft("");
  };

  // Quick-prompt buttons are UI shortcuts that always imply the user wants
  // to kick off a workflow/agent creation — they should always surface the
  // "configure a model" card rather than the generic chat-retry stub.
  const onQuickPrompt = (content: string) => {
    pushExchange(content, { kind: "no-model" });
  };

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    pushExchange(draft, orcaModelId ? { kind: "error" } : { kind: "no-model" });
  };

  const newChat = () => {
    setMessages(INITIAL_MESSAGES);
    setDraft("");
  };

  return (
    <div className="flex min-h-0 flex-1 gap-3">
      <ChatSidebar onNewChat={newChat} />
      <div className="flex min-w-0 min-h-0 flex-1 flex-col gap-3">
      <ScrollArea className="min-h-0 flex-1 rounded-lg border border-border bg-card">
        <ul className="space-y-3 p-4">
          {messages.map((m) => {
            if (m.role === "user") {
              return (
                <li
                  key={m.id}
                  className="ml-auto max-w-[70%] w-fit rounded-lg border border-border px-3 py-2 text-sm text-foreground"
                >
                  {m.content}
                </li>
              );
            }
            return (
              <li key={m.id} className="w-full">
                {m.block.kind === "greeting" && (
                  <GreetingBlock
                    onQuickPrompt={onQuickPrompt}
                    disabled={messages.some((x) => x.role === "user")}
                  />
                )}
                {m.block.kind === "no-model" && <NoModelBlock />}
                {m.block.kind === "error" && <ErrorBlock />}
              </li>
            );
          })}
          <li ref={bottomRef} aria-hidden />
        </ul>
      </ScrollArea>

      <form
        onSubmit={onSubmit}
        className="flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-2"
      >
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="Chat with Orca…"
          className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
        />
        <button
          type="submit"
          disabled={!draft.trim()}
          className="flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40 cursor-pointer"
          aria-label="Send"
        >
          <Send className="size-3.5" />
        </button>
      </form>
      </div>
    </div>
  );
}

function ChatSidebar({ onNewChat }: { onNewChat: () => void }) {
  const [collapsed, setCollapsed] = useState(false);

  if (collapsed) {
    return (
      <aside className="flex w-10 shrink-0 flex-col items-center rounded-lg border border-border bg-card p-1.5">
        <button
          type="button"
          onClick={() => setCollapsed(false)}
          className="flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors cursor-pointer hover:bg-accent/30 hover:text-foreground"
          aria-label="Expand chat sidebar"
          title="Expand"
        >
          <PanelLeftOpen className="size-4" />
        </button>
      </aside>
    );
  }

  return (
    <aside className="flex w-[220px] shrink-0 flex-col overflow-hidden rounded-lg border border-border bg-card">
      <div className="flex items-center gap-1 p-1.5">
        <button
          type="button"
          onClick={onNewChat}
          className="flex flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-sm text-foreground transition-colors cursor-pointer hover:bg-accent/30"
        >
          <Plus className="size-4 shrink-0" />
          New chat
        </button>
        <button
          type="button"
          onClick={() => setCollapsed(true)}
          className="flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors cursor-pointer hover:bg-accent/30 hover:text-foreground"
          aria-label="Collapse chat sidebar"
          title="Collapse"
        >
          <PanelLeftClose className="size-4" />
        </button>
      </div>
      <div className="border-t border-border" />
      <div className="px-3 pt-2 pb-1 text-caption uppercase tracking-wider text-muted-foreground">
        Recent
      </div>
      <ul className="flex-1 overflow-y-auto p-1">
        {RECENT_CHATS.map((r) => (
          <li key={r.id}>
            <button
              type="button"
              className="flex w-full items-center rounded-md px-2 py-1.5 text-left text-sm text-muted-foreground transition-colors cursor-pointer hover:bg-accent/30 hover:text-foreground"
            >
              <span className="min-w-0 truncate">{r.title}</span>
            </button>
          </li>
        ))}
      </ul>
    </aside>
  );
}

function ConfigurationTab() {
  const models = useStudioStore((s) => s.models);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const orcaModelId = useStudioStore((s) => s.orcaModelId);
  const orcaFallbackModelId = useStudioStore((s) => s.orcaFallbackModelId);
  const setOrcaModels = useStudioStore((s) => s.setOrcaModels);
  const projectModels = models.filter((m) => m.projectId === activeProjectId);

  return (
    <div className="flex gap-4">
      <ModelSelect
        label="Model"
        value={orcaModelId ?? ""}
        models={projectModels}
        placeholder="Select a model"
        onChange={(id) => setOrcaModels(id, orcaFallbackModelId)}
      />
      <ModelSelect
        label="Fallback Model"
        value={orcaFallbackModelId ?? ""}
        models={projectModels}
        placeholder="Select a fallback model"
        onChange={(id) => setOrcaModels(orcaModelId, id)}
      />
    </div>
  );
}

export function OrcaPage() {
  const [tab, setTab] = useState<Tab>("chat");

  return (
    <div className="flex h-full min-h-0 flex-1 flex-col bg-sidebar">
      <div className="mx-auto flex min-h-0 w-full max-w-5xl flex-1 flex-col px-8 py-6">
        <div className="flex items-start gap-4">
          <EntityIconSwatch icon={MessageCircle} />
          <div className="min-w-0 flex-1">
            <h1 className="text-lg font-semibold text-foreground">Orca</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Your AI co-pilot for orchestrating workflows, agents, and
              skills.
            </p>
          </div>
        </div>

        <TabBar tabs={TABS} value={tab} onChange={setTab} />

        <div className="mt-6 flex min-h-0 flex-1 flex-col">
          {tab === "chat" && <ChatTab />}
          {tab === "configuration" && <ConfigurationTab />}
          {(tab === "memory" || tab === "usage") && <ComingSoonTab />}
        </div>
      </div>
    </div>
  );
}
