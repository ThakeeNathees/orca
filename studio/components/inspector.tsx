"use client";

import dynamic from "next/dynamic";
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
import { AlertCircle, ExternalLink, X, Trash2 } from "lucide-react";
import {
  FIELD_INPUT_CLASS,
  FIELD_TEXTAREA_CLASS,
  FIELD_SELECT_TRIGGER_CLASS,
} from "@/lib/styles";
import { ModelSelect } from "@/components/model-select";
import type { BlockNode } from "@/lib/types";

const CodeFieldEditor = dynamic(
  () =>
    import("@/components/code-field-editor").then((m) => m.CodeFieldEditor),
  {
    ssr: false,
    loading: () => (
      <div className="flex h-[120px] items-center justify-center rounded-md bg-muted/40 text-xs text-muted-foreground">
        Loading editor...
      </div>
    ),
  }
);

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
          className={FIELD_INPUT_CLASS}
        />
      );

    case "password":
      return (
        <Input
          type="password"
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.placeholder}
          className={FIELD_INPUT_CLASS}
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
          className={FIELD_INPUT_CLASS}
        />
      );

    case "textarea":
      return (
        <Textarea
          value={String(value ?? "")}
          onChange={(e) => onChange(e.target.value)}
          placeholder={field.placeholder}
          className={FIELD_TEXTAREA_CLASS}
          rows={3}
        />
      );

    case "select":
      return (
        <Select
          value={String(value ?? "")}
          onValueChange={(v) => onChange(v ?? "")}
        >
          <SelectTrigger className={FIELD_SELECT_TRIGGER_CLASS}>
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

    case "code":
      return (
        <CodeFieldEditor
          value={String(value ?? "")}
          onChange={(v) => onChange(v)}
          placeholder={field.placeholder}
          label={field.label}
        />
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
  const isAgent = selectedNode.data.kind === "agent";

  return (
    <aside className="flex w-72 shrink-0 flex-col border-l border-border bg-sidebar text-sidebar-foreground">
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
          {isAgent ? (
            <AgentInspector node={selectedNode} />
          ) : (
            <>
              <div className="space-y-1.5">
                <Label className="text-[11px] font-medium text-muted-foreground/80">
                  Name
                </Label>
                <Input
                  value={selectedNode.data.label}
                  onChange={(e) =>
                    updateNodeLabel(selectedNode.id, e.target.value)
                  }
                  className={FIELD_INPUT_CLASS}
                />
              </div>
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
            </>
          )}
        </div>
      </ScrollArea>

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

/**
 * Inspector panel for an `agent`-kind node. Fields are bound to the
 * underlying AgentSummary entity — edits are two-way synced across every
 * node that references the same agent. When the referenced entity is
 * missing, a relink dropdown is shown so the user can adopt another.
 */
function AgentInspector({ node }: { node: BlockNode }) {
  const agents = useStudioStore((s) => s.agents);
  const activeProjectId = useStudioStore((s) => s.activeProjectId);
  const models = useStudioStore((s) => s.models);
  const renameAgent = useStudioStore((s) => s.renameAgent);
  const updateAgentPersona = useStudioStore((s) => s.updateAgentPersona);
  const updateAgentModels = useStudioStore((s) => s.updateAgentModels);
  const openAgent = useStudioStore((s) => s.openAgent);
  const relinkAgentNode = useStudioStore((s) => s.relinkAgentNode);

  const projectAgents = agents.filter((a) => a.projectId === activeProjectId);
  const projectModels = models.filter((m) => m.projectId === activeProjectId);
  const agent = agents.find((a) => a.id === node.data.agentId) ?? null;

  if (!agent) {
    // Broken link — entity was deleted. Offer a relink dropdown so the
    // node can be salvaged without recreating the edge wiring.
    return (
      <div className="space-y-3">
        <div className="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-[12px] text-destructive">
          <AlertCircle className="mt-0.5 size-3.5 shrink-0" />
          <div className="min-w-0 flex-1">
            <p className="font-medium">Agent was deleted</p>
            <p className="mt-0.5 text-muted-foreground">
              Link this node to another agent or remove it.
            </p>
          </div>
        </div>
        <div className="space-y-1.5">
          <Label className="text-[11px] font-medium text-muted-foreground/80">
            Relink to agent
          </Label>
          <Select
            value=""
            onValueChange={(id) => id && relinkAgentNode(node.id, id)}
          >
            <SelectTrigger className={FIELD_SELECT_TRIGGER_CLASS}>
              <SelectValue placeholder="Pick an agent…" />
            </SelectTrigger>
            <SelectContent>
              {projectAgents.map((a) => (
                <SelectItem key={a.id} value={a.id}>
                  {a.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Label className="text-[11px] font-medium text-muted-foreground/80">
          Name
        </Label>
        <Input
          value={agent.name}
          onChange={(e) => void renameAgent(agent.id, e.target.value)}
          className={FIELD_INPUT_CLASS}
        />
      </div>
      <div className="space-y-1.5">
        <Label className="text-[11px] font-medium text-muted-foreground/80">
          Persona
        </Label>
        <Textarea
          value={agent.persona ?? ""}
          onChange={(e) => void updateAgentPersona(agent.id, e.target.value)}
          placeholder="You're a helpful assistant..."
          className={`${FIELD_TEXTAREA_CLASS} font-mono`}
          rows={6}
        />
      </div>
      <ModelSelect
        label="Model"
        value={agent.modelId ?? ""}
        models={projectModels}
        placeholder="Select a model"
        onChange={(id) =>
          void updateAgentModels(agent.id, id, agent.fallbackModelId)
        }
      />
      <button
        type="button"
        onClick={() => openAgent(agent.id)}
        className="flex w-full items-center justify-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-[12px] text-foreground/90 transition-colors cursor-pointer hover:bg-accent/40"
      >
        <ExternalLink className="size-3.5" />
        Open in Agents section
      </button>
    </div>
  );
}
