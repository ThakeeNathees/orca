"use client";

import { useMemo, useState } from "react";
import { Inbox as InboxIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  EntityIconSwatch,
  TabBar,
} from "@/components/detail-shell";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogHeader,
  DialogBody,
} from "@/components/ui/dialog";
import { type InboxMessage } from "@/lib/inbox";
import { useStudioStore } from "@/lib/store";
import { ENTITY_ICON_FG } from "@/lib/entity-icon";

type Tab = "all" | "action" | "unread" | "important";

const TABS: { id: Tab; label: string }[] = [
  { id: "all", label: "All" },
  { id: "action", label: "Action Needed" },
  { id: "unread", label: "Unread" },
  { id: "important", label: "Important" },
];

function formatAge(minutes: number): string {
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function formatFull(minutes: number): string {
  const d = new Date(Date.now() - minutes * 60_000);
  return d.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

function filterFor(tab: Tab, messages: InboxMessage[]): InboxMessage[] {
  switch (tab) {
    case "all":
      return messages;
    case "action":
      return messages.filter((m) => m.actions && m.actions.length > 0);
    case "unread":
      return messages.filter((m) => m.unread);
    case "important":
      return messages.filter((m) => m.important);
  }
}

function InboxRow({
  message,
  onClick,
}: {
  message: InboxMessage;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={message.unread ? { borderColor: ENTITY_ICON_FG } : undefined}
      className="flex w-full items-center justify-between gap-4 rounded-lg border border-border bg-card px-4 py-3 text-left transition-colors cursor-pointer hover:bg-accent/30"
    >
      <span
        className={cn(
          "min-w-0 flex-1 truncate text-sm",
          message.unread
            ? "font-medium text-foreground"
            : "text-muted-foreground"
        )}
      >
        {message.title}
      </span>
      <span className="shrink-0 text-xs text-muted-foreground">
        {formatAge(message.ageMinutes)}
      </span>
    </button>
  );
}

function InboxDetailDialog({
  message,
  onClose,
  onToggleUnread,
  onToggleImportant,
}: {
  message: InboxMessage | null;
  onClose: () => void;
  onToggleUnread: (id: string) => void;
  onToggleImportant: (id: string) => void;
}) {
  return (
    <Dialog open={message !== null} onOpenChange={(o) => (o ? null : onClose())}>
      <DialogHeader onClose={onClose}>
        {message?.title ?? ""}
      </DialogHeader>
      {message && (
        <DialogBody className="px-4 pt-3 pb-0">
          <p className="text-xs text-muted-foreground/70">
            {formatFull(message.ageMinutes)}
          </p>
          <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground/80">
            {message.body}
          </p>
          {message.actions && message.actions.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-2">
              {message.actions.map((a) => (
                <Button
                  key={a.kind}
                  size="sm"
                  className="cursor-pointer bg-[#E2E2E2] text-zinc-900 hover:bg-[#E2E2E2]/90"
                >
                  {a.label}
                </Button>
              ))}
            </div>
          )}
          <hr className="mt-3 -mx-4 border-border" />
          <div className="flex justify-end gap-1">
            <button
              type="button"
              onClick={() => onToggleUnread(message.id)}
              className="rounded-md px-2 py-1.5 text-xs text-muted-foreground transition-colors cursor-pointer hover:text-foreground"
            >
              {message.unread ? "Mark as read" : "Mark as unread"}
            </button>
            <button
              type="button"
              onClick={() => onToggleImportant(message.id)}
              className="rounded-md px-2 py-1.5 text-xs text-muted-foreground transition-colors cursor-pointer hover:text-foreground"
            >
              {message.important
                ? "Remove from important"
                : "Move to important"}
            </button>
          </div>
        </DialogBody>
      )}
    </Dialog>
  );
}

export function InboxPage() {
  const [tab, setTab] = useState<Tab>("all");
  const [openId, setOpenId] = useState<string | null>(null);

  const messages = useStudioStore((s) => s.inboxMessages);
  const markRead = useStudioStore((s) => s.markInboxRead);
  const toggleUnread = useStudioStore((s) => s.toggleInboxUnread);
  const toggleImportant = useStudioStore((s) => s.toggleInboxImportant);

  const openMessage = messages.find((m) => m.id === openId) ?? null;
  const visible = useMemo(() => filterFor(tab, messages), [tab, messages]);

  // Opening a message marks it read on the same click — keeps the state
  // transition in the event handler rather than an effect.
  const openInboxItem = (id: string) => {
    setOpenId(id);
    markRead(id);
  };

  return (
    <div className="flex h-full flex-1 flex-col bg-sidebar">
      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto max-w-5xl px-8 py-6">
          <div className="flex items-start gap-4">
            <EntityIconSwatch icon={InboxIcon} />
            <div className="min-w-0 flex-1">
              <h1 className="text-lg font-semibold text-foreground">Inbox</h1>
              <p className="mt-1 text-sm text-muted-foreground">
                Notifications, action items, and announcements from Orca.
              </p>
            </div>
          </div>

          <TabBar tabs={TABS} value={tab} onChange={setTab} />

          <div className="mt-6 space-y-2">
            {visible.length === 0 ? (
              <p className="rounded-lg border border-dashed border-border bg-card px-4 py-8 text-center text-sm text-muted-foreground">
                Nothing here.
              </p>
            ) : (
              visible.map((m) => (
                <InboxRow
                  key={m.id}
                  message={m}
                  onClick={() => openInboxItem(m.id)}
                />
              ))
            )}
          </div>
        </div>
      </div>

      <InboxDetailDialog
        message={openMessage}
        onClose={() => setOpenId(null)}
        onToggleUnread={toggleUnread}
        onToggleImportant={toggleImportant}
      />
    </div>
  );
}
