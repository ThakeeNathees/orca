"use client";

import {
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Pencil, Trash2, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Dialog, DialogHeader, DialogBody } from "@/components/ui/dialog";
import { ENTITY_ICON_BG, ENTITY_ICON_FG } from "@/lib/entity-icon";

/** Colored icon swatch shown in the header of every detail page. */
export function EntityIconSwatch({ icon: Icon }: { icon: LucideIcon }) {
  return (
    <div
      className="flex size-14 shrink-0 items-center justify-center rounded-lg shadow-sm"
      style={{ backgroundColor: ENTITY_ICON_BG }}
      aria-hidden
    >
      <Icon
        className="size-6"
        strokeWidth={2}
        style={{ color: ENTITY_ICON_FG }}
      />
    </div>
  );
}

/** Inline-editable name — click to enter edit mode, Enter/blur to commit. */
export function EditableName({
  name,
  onRename,
}: {
  name: string;
  onRename: (next: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(name);
  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setDraft(name);
  }, [name]);

  useEffect(() => {
    if (editing) {
      ref.current?.focus();
      ref.current?.select();
    }
  }, [editing]);

  const commit = () => {
    const t = draft.trim();
    if (t && t !== name) onRename(t);
    else setDraft(name);
    setEditing(false);
  };

  if (editing) {
    return (
      <Input
        ref={ref}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Enter") commit();
          if (e.key === "Escape") {
            setDraft(name);
            setEditing(false);
          }
        }}
        className="h-9 text-lg font-semibold"
        maxLength={120}
      />
    );
  }

  return (
    <button
      type="button"
      onClick={() => setEditing(true)}
      title="Click to rename"
      className="group flex items-center gap-2 rounded-md px-1 py-0.5 text-left text-lg font-semibold text-foreground hover:bg-accent/50 cursor-pointer"
    >
      <span className="truncate">{name}</span>
      <Pencil className="size-3.5 shrink-0 opacity-0 group-hover:opacity-60" />
    </button>
  );
}

/** Inline-editable description (textarea, click-to-edit). */
export function EditableDescription({
  description,
  onCommit,
}: {
  description: string;
  onCommit: (next: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(description);

  useEffect(() => {
    setDraft(description);
  }, [description]);

  const commit = () => {
    if (draft !== description) onCommit(draft);
    setEditing(false);
  };

  if (editing) {
    return (
      <Textarea
        autoFocus
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            setDraft(description);
            setEditing(false);
          }
        }}
        placeholder="Add a description…"
        rows={2}
        className="mt-1 text-sm"
      />
    );
  }

  return (
    <button
      type="button"
      onClick={() => setEditing(true)}
      className="mt-1 block w-full cursor-pointer rounded-md px-1 py-0.5 text-left text-sm text-muted-foreground hover:bg-accent/50"
    >
      {description || (
        <span className="italic opacity-70">Add a description…</span>
      )}
    </button>
  );
}

/** Tab underline bar. */
export function TabBar<T extends string>({
  tabs,
  value,
  onChange,
}: {
  tabs: { id: T; label: string }[];
  value: T;
  onChange: (id: T) => void;
}) {
  return (
    <div className="mt-6 flex gap-6 border-b border-border">
      {tabs.map((t) => (
        <button
          key={t.id}
          type="button"
          onClick={() => onChange(t.id)}
          className={cn(
            "-mb-px border-b-2 px-1 py-2 text-sm transition-colors cursor-pointer",
            value === t.id
              ? "border-foreground text-foreground"
              : "border-transparent text-muted-foreground hover:text-foreground"
          )}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

export function ComingSoonTab() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center py-20 text-muted-foreground">
      <p className="text-sm">TODO</p>
    </div>
  );
}

/** Confirmation modal shared by every entity delete button. */
export function DeleteEntityDialog({
  open,
  entityLabel,
  name,
  onConfirm,
  onClose,
}: {
  open: boolean;
  entityLabel: string;
  name: string;
  onConfirm: () => void;
  onClose: () => void;
}) {
  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : onClose())}>
      <DialogHeader onClose={onClose}>Delete {entityLabel}</DialogHeader>
      <DialogBody className="px-4 py-4">
        <p className="text-sm text-muted-foreground">
          Are you sure you want to delete{" "}
          <span className="font-medium text-foreground">{name}</span>? This
          action cannot be undone.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={onClose}
            className="cursor-pointer"
          >
            Cancel
          </Button>
          <Button
            size="sm"
            variant="destructive"
            onClick={onConfirm}
            className="cursor-pointer"
          >
            Delete
          </Button>
        </div>
      </DialogBody>
    </Dialog>
  );
}

/** Shared header row: icon + name + description + action slot. */
export function DetailHeader({
  icon,
  name,
  description,
  onRename,
  onDescriptionCommit,
  actions,
}: {
  icon: LucideIcon;
  name: string;
  description: string;
  onRename: (next: string) => void;
  onDescriptionCommit: (next: string) => void;
  actions: ReactNode;
}) {
  return (
    <div className="flex items-start gap-4">
      <EntityIconSwatch icon={icon} />
      <div className="min-w-0 flex-1">
        <EditableName name={name} onRename={onRename} />
        <EditableDescription
          description={description}
          onCommit={onDescriptionCommit}
        />
      </div>
      <div className="flex shrink-0 items-center gap-2">{actions}</div>
    </div>
  );
}

/** Scrollable page frame shared by every detail page. */
export function DetailShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-full flex-1 flex-col bg-sidebar">
      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto max-w-5xl px-8 py-6">{children}</div>
      </div>
    </div>
  );
}

/** Delete button that opens a confirm dialog — used on every detail page.
 *  Styled as a neutral light-chip matching the sibling Edit button so the
 *  header action row reads as a pair; the actual destructive styling lives
 *  on the confirm button inside the dialog. */
export function DeleteButton({ onClick }: { onClick: () => void }) {
  return (
    <Button
      size="sm"
      variant="light"
      onClick={onClick}
      className="cursor-pointer gap-1.5"
    >
      <Trash2 className="size-3.5" />
      Delete
    </Button>
  );
}
