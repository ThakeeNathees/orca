"use client";

import { GitBranch } from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useStudioStore } from "@/lib/store";

export function NavSidebar() {
  const currentView = useStudioStore((s) => s.currentView);
  const goToDashboard = useStudioStore((s) => s.goToDashboard);

  const isActive = currentView === "dashboard";

  return (
    <aside className="flex w-[52px] shrink-0 flex-col items-center border-r border-border bg-sidebar py-3 gap-4">
      {/* Workflows nav item */}
      <Tooltip>
        <TooltipTrigger
          render={(props) => {
            const p = { ...props } as Record<string, unknown>;
            delete p.type;
            const btnProps =
              p as React.ButtonHTMLAttributes<HTMLButtonElement>;
            return (
              <button
                type="button"
                {...btnProps}
                onClick={(e) => {
                  btnProps.onClick?.(
                    e as React.MouseEvent<HTMLButtonElement>
                  );
                  goToDashboard();
                }}
                className={cn(
                  "flex size-9 items-center justify-center rounded-md transition-colors",
                  isActive
                    ? "bg-accent text-foreground"
                    : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
                  btnProps.className
                )}
                aria-label="Workflows"
              >
                <GitBranch className="size-[18px]" />
              </button>
            );
          }}
        />
        <TooltipContent side="right" sideOffset={8}>
          Workflows
        </TooltipContent>
      </Tooltip>
    </aside>
  );
}
