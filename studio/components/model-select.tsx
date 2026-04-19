"use client";

import { Brain } from "lucide-react";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ModelSummary } from "@/lib/types";

/** Dropdown that renders a list of project models by name, keyed by id.
 *  When no models exist the select still opens, revealing a guidance card
 *  that points at the Models section. `alignItemWithTrigger` is disabled
 *  so the popup always feels like a list under the trigger rather than
 *  collapsing onto a single row when there is exactly one model. */
export function ModelSelect({
  label,
  value,
  models,
  placeholder,
  onChange,
}: {
  label: string;
  value: string;
  models: ModelSummary[];
  placeholder: string;
  onChange: (id: string) => void;
}) {
  const empty = models.length === 0;
  return (
    <div className="flex-1 space-y-1.5">
      <Label>{label}</Label>
      <Select value={value} onValueChange={(v) => v && onChange(v)}>
        <SelectTrigger className="w-full">
          <SelectValue placeholder={placeholder}>
            {(v) =>
              models.find((m) => m.id === v)?.name ?? (
                <span className="text-muted-foreground">{placeholder}</span>
              )
            }
          </SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false} align="start">
          {empty ? (
            <div className="flex flex-col items-center gap-2 px-6 py-5 text-center">
              <Brain className="size-6 text-muted-foreground/60" aria-hidden />
              <p className="text-sm text-foreground">No models exist</p>
              <p className="text-xs text-muted-foreground">
                Create one from the Models section
              </p>
            </div>
          ) : (
            models.map((m) => (
              <SelectItem key={m.id} value={m.id}>
                {m.name}
              </SelectItem>
            ))
          )}
        </SelectContent>
      </Select>
    </div>
  );
}
