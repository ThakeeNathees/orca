"use client";

import { useState, type FormEvent } from "react";
import { MessageCircle, Send } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  ComingSoonTab,
  EntityIconSwatch,
  TabBar,
} from "@/components/detail-shell";
import { ENTITY_ICON_BG, ENTITY_ICON_FG } from "@/lib/entity-icon";

type Tab = "chat" | "configuration" | "memory" | "usage";

const TABS: { id: Tab; label: string }[] = [
  { id: "chat", label: "Chat" },
  { id: "configuration", label: "Configuration" },
  { id: "memory", label: "Memory" },
  { id: "usage", label: "Usage" },
];

type ChatMessage = { id: string; role: "user" | "assistant"; content: string };

function ChatTab() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState("");

  const send = (e: FormEvent) => {
    e.preventDefault();
    const t = draft.trim();
    if (!t) return;
    // Until the Orca backend lands the assistant stubs a fixed reply so
    // users know where to look to make the chat functional.
    setMessages((prev) => [
      ...prev,
      { id: crypto.randomUUID(), role: "user", content: t },
      {
        id: crypto.randomUUID(),
        role: "assistant",
        content:
          "No models configured — add a model in the Models section, then assign it on the Configuration tab.",
      },
    ]);
    setDraft("");
  };

  return (
    <div className="flex min-h-[70vh] flex-1 flex-col gap-3">
      <div className="flex-1 overflow-y-auto rounded-lg border border-border bg-card p-4">
        {messages.length === 0 ? (
          <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
            <MessageCircle className="mb-3 size-8 opacity-40" />
            <p className="text-sm">
              Ask Orca to orchestrate workflows, draft agents, or build skills.
            </p>
          </div>
        ) : (
          <ul className="space-y-3">
            {messages.map((m) => (
              <li
                key={m.id}
                style={
                  m.role === "user"
                    ? { backgroundColor: ENTITY_ICON_FG, color: ENTITY_ICON_BG }
                    : undefined
                }
                className={cn(
                  "rounded-lg px-3 py-2 text-sm",
                  m.role === "assistant" && "text-foreground"
                )}
              >
                {m.content}
              </li>
            ))}
          </ul>
        )}
      </div>

      <form
        onSubmit={send}
        className="mb-6 flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-2"
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
  );
}

export function OrcaPage() {
  const [tab, setTab] = useState<Tab>("chat");

  return (
    <div className="flex h-full flex-1 flex-col bg-sidebar">
      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto flex h-full max-w-5xl flex-col px-8 py-6">
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
            {tab === "chat" ? <ChatTab /> : <ComingSoonTab />}
          </div>
        </div>
      </div>
    </div>
  );
}
