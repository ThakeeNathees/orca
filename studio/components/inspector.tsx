"use client";

import { useStudioStore } from "@/lib/store";
import { BLOCK_DEFS } from "@/lib/block-defs";
import type { FieldDef } from "@/lib/types";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Slider } from "@/components/ui/slider";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { X, Trash2 } from "lucide-react";

function FieldRenderer({
  field,
  value,
  onChange,
}: {
  field: FieldDef;
  value: string | number;
  onChange: (val: string | number) => void;
}) {
  switch (field.type) {
    case "text":
      return (
        <Input
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.placeholder}
          className="h-8 text-[13px] bg-muted/40 border-transparent focus-visible:border-border"
        />
      );

    case "password":
      return (
        <Input
          type="password"
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.placeholder}
          className="h-8 text-[13px] bg-muted/40 border-transparent focus-visible:border-border"
        />
      );

    case "number":
      return (
        <Input
          type="number"
          value={String(value ?? "")}
          onChange={(e) => onChange(Number(e.target.value))}
          placeholder={field.placeholder}
          min={field.min}
          max={field.max}
          step={field.step}
          className="h-8 text-[13px] bg-muted/40 border-transparent focus-visible:border-border"
        />
      );

    case "textarea":
      return (
        <Textarea
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.placeholder}
          className="min-h-[60px] resize-y text-[13px] bg-muted/40 border-transparent focus-visible:border-border"
          rows={3}
        />
      );

    case "select":
      return (
        <Select
          value={String(value ?? "")}
          onValueChange={(v) => onChange(v ?? "")}
        >
          <SelectTrigger className="h-8 text-[13px] bg-muted/40 border-transparent">
            <SelectValue placeholder="Select..." />
          </SelectTrigger>
          <SelectContent>
            {field.options?.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      );

    case "slider":
      return (
        <div className="flex items-center gap-3">
          <Slider
            value={[Number(value ?? field.defaultValue ?? field.min ?? 0)]}
            onValueChange={(vals) => {
              const v = Array.isArray(vals) ? vals[0] : vals;
              onChange(v);
            }}
            min={field.min}
            max={field.max}
            step={field.step}
            className="flex-1"
          />
          <span className="min-w-[2.5rem] text-right text-[13px] tabular-nums text-muted-foreground">
            {Number(value ?? field.defaultValue ?? 0).toFixed(1)}
          </span>
        </div>
      );

    default:
      return null;
  }
}

export function Inspector() {
  const selectedNodeId = useStudioStore((s) => s.selectedNodeId);
  const nodes = useStudioStore((s) => s.nodes);
  const updateNodeData = useStudioStore((s) => s.updateNodeData);
  const updateNodeLabel = useStudioStore((s) => s.updateNodeLabel);
  const removeNode = useStudioStore((s) => s.removeNode);
  const setSelectedNodeId = useStudioStore((s) => s.setSelectedNodeId);

  const selectedNode = nodes.find((n) => n.id === selectedNodeId);

  if (!selectedNode) {
    return null;
  }

  const def = BLOCK_DEFS[selectedNode.data.kind];

  return (
    <aside className="flex w-72 shrink-0 flex-col border-l border-border bg-background">
      {/* Header */}
      <div className="flex items-start justify-between px-4 pt-4 pb-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="text-[14px] font-semibold text-foreground">
              {selectedNode.data.label}
            </h3>
            <Badge
              variant="secondary"
              className="text-[10px] px-1.5 py-0 font-normal text-muted-foreground"
            >
              {selectedNode.id}
            </Badge>
          </div>
          <p className="mt-1 text-[12px] leading-relaxed text-muted-foreground/70">
            {def.description}
          </p>
        </div>
        <button
          onClick={() => setSelectedNodeId(null)}
          className="ml-2 rounded-md p-1 text-muted-foreground/50 transition-colors hover:bg-accent hover:text-foreground"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>

      <div className="mx-4 border-t border-border/50" />

      <ScrollArea className="flex-1">
        <div className="space-y-4 p-4">
          {/* Name */}
          <div className="space-y-1.5">
            <Label className="text-[11px] font-medium text-muted-foreground/80">
              Name
            </Label>
            <Input
              value={selectedNode.data.label}
              onChange={(e) =>
                updateNodeLabel(selectedNode.id, e.target.value)
              }
              className="h-8 text-[13px] bg-muted/40 border-transparent focus-visible:border-border"
            />
          </div>

          {/* Dynamic fields */}
          {def.fields.map((field) => (
            <div key={field.key} className="space-y-1.5">
              <Label className="text-[11px] font-medium text-muted-foreground/80">
                {field.label}
              </Label>
              <FieldRenderer
                field={field}
                value={selectedNode.data.fields[field.key] ?? ""}
                onChange={(val) =>
                  updateNodeData(selectedNode.id, { [field.key]: val })
                }
              />
            </div>
          ))}
        </div>
      </ScrollArea>

      {/* Delete */}
      <div className="border-t border-border/50 p-3">
        <button
          onClick={() => removeNode(selectedNode.id)}
          className="flex w-full items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-[12px] text-muted-foreground/60 transition-colors hover:bg-destructive/10 hover:text-destructive"
        >
          <Trash2 className="h-3 w-3" />
          Delete
        </button>
      </div>
    </aside>
  );
}
